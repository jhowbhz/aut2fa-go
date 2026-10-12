package aut2fa

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// The HTTP status and stable problem code drive callers' recovery decisions.
func TestAPIErrorsPreserveProblemDetails(t *testing.T) {
	for _, test := range []struct {
		status     int
		code       string
		retryAfter string
	}{
		{401, "unauthorized", ""}, {402, "insufficient_balance", ""},
		{409, "idempotency_conflict", ""}, {422, "invalid_to", ""},
		{429, "rate_limited", "60"}, {500, "internal_error", ""},
		{503, "unavailable", "Mon, 12 Oct 2026 00:00:00 GMT"},
	} {
		t.Run(test.code, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/problem+json")
				w.Header().Set("Retry-After", test.retryAfter)
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, `{"status":999,"code":"`+test.code+`","detail":"request rejected","errors":{"to":"invalid number"},"extension":true}`)
			}))
			defer server.Close()
			_, err := newTestClient(t, server).Send(context.Background(), SendParams{To: "+5511999999999"})
			var problem *ApiError
			if !errors.As(err, &problem) {
				t.Fatalf("error = %v, want *ApiError", err)
			}
			if problem.Status != test.status || problem.Code != test.code || problem.Detail != "request rejected" ||
				problem.Errors["to"] != "invalid number" || problem.RetryAfter != test.retryAfter {
				t.Fatalf("problem = %#v", problem)
			}
			if calls.Load() != 1 {
				t.Errorf("request made %d times", calls.Load())
			}
		})
	}
}

func TestUnexpectedErrorBodiesKeepStatusWithoutLeakingBody(t *testing.T) {
	for _, body := range []string{
		"secret upstream stack trace", `<html>secret upstream stack trace</html>`,
		"null", "[]", "{}", `{"code":123}`, `{"code":"bad","detail":false}`,
		`{"code":"bad","errors":[]}`, `{"code":""}`, `{"code":"bad"} {}`,
	} {
		t.Run(body, func(t *testing.T) {
			server := responseServer(http.StatusBadGateway, body, "text/html")
			defer server.Close()
			_, err := newTestClient(t, server).Status(context.Background(), "send_abc-123")
			var problem *ApiError
			if !errors.As(err, &problem) || problem.Status != 502 || problem.Code != "http_error" {
				t.Fatalf("error = %#v, want HTTP 502 http_error", err)
			}
			if strings.Contains(problem.Error(), "secret") {
				t.Fatalf("upstream body leaked: %v", problem)
			}
		})
	}
}

func TestMalformedSendResponsesAreIdentifiable(t *testing.T) {
	for _, body := range []string{
		"", "{", "null", "[]", "true", "123", `"send"`, "{}",
		`{"id":"send_abc-123"}`, `{"status":"queued"}`, `{"id":null,"status":"queued"}`,
		`{"id":1,"status":"queued"}`, `{"id":"send_abc-123","status":null}`,
		`{"id":"send_abc-123","status":"queued","created_at":"invalid"}`,
		sendFixture + " {}",
	} {
		t.Run(body, func(t *testing.T) {
			server := responseServer(202, body, "application/json")
			defer server.Close()
			result, err := newTestClient(t, server).Send(context.Background(), SendParams{To: "+5511999999999"})
			if !errors.Is(err, ErrInvalidResponse) || result != nil {
				t.Fatalf("result = %#v, error = %v", result, err)
			}
			var problem *ApiError
			if errors.As(err, &problem) {
				t.Fatal("invalid successful response classified as API rejection")
			}
		})
	}
}

func TestMalformedVerifyResponsesAreIdentifiable(t *testing.T) {
	for _, body := range []string{
		"null", "[]", "{}", `{"verification":"pending","attempts_left":2}`,
		`{"valid":false,"attempts_left":2}`, `{"valid":false,"verification":"pending"}`,
		`{"valid":null,"verification":"pending","attempts_left":2}`,
		`{"valid":false,"verification":null,"attempts_left":2}`,
		`{"valid":false,"verification":"pending","attempts_left":null}`,
		`{"valid":"false","verification":"pending","attempts_left":2}`,
		`{"valid":false,"verification":"pending","attempts_left":1.5}`,
	} {
		t.Run(body, func(t *testing.T) {
			server := responseServer(200, body, "application/json")
			defer server.Close()
			result, err := newTestClient(t, server).Verify(context.Background(), "send_abc-123", "123456")
			if !errors.Is(err, ErrInvalidResponse) || result != nil {
				t.Fatalf("result = %#v, error = %v", result, err)
			}
		})
	}
}
