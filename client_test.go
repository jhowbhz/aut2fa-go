package aut2fa

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"
)

const sendFixture = `{
	"id":"send_abc-123","status":"queued","verification":"pending","mode":"live",
	"channel":null,"from":null,"expires_at":"2026-10-11T21:00:00Z",
	"created_at":"2026-10-11T20:55:00Z","sent_at":null,"delivered_at":null,
	"read_at":null,"verified_at":null,"failed_at":null,"failure_reason":null,
	"future_field":{"supported":true}
}`

// A changed request envelope can prevent sends or break idempotency at the API.
func TestSendMatchesHTTPContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/send" {
			t.Errorf("request = %s %s, want POST /v1/send", r.Method, r.URL.Path)
		}
		for header, want := range map[string]string{
			"Authorization":   "Bearer app_test_key",
			"Accept":          "application/json",
			"Content-Type":    "application/json",
			"Idempotency-Key": "signup-user-123",
		} {
			if got := r.Header.Get(header); got != want {
				t.Errorf("%s = %q, want %q", header, got, want)
			}
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		want := map[string]any{"to": "+5511999999999", "code": "123456", "locale": "pt-BR"}
		if !reflect.DeepEqual(body, want) {
			t.Errorf("body = %#v, want %#v", body, want)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, sendFixture)
	}))
	defer server.Close()

	client, err := New("app_test_key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	send, err := client.Send(context.Background(), SendParams{
		To: "+5511999999999", Code: "123456", Locale: "pt-BR", IdempotencyKey: "signup-user-123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if send.ID != "send_abc-123" || send.Status != "queued" || send.Verification != "pending" || send.Mode != "live" {
		t.Fatalf("send = %#v", send)
	}
	if send.Channel != nil || send.From != nil || send.SentAt != nil || send.DeliveredAt != nil ||
		send.ReadAt != nil || send.VerifiedAt != nil || send.FailedAt != nil || send.FailureReason != nil {
		t.Fatalf("null fields lost: %#v", send)
	}
	wantCreated := time.Date(2026, 10, 11, 20, 55, 0, 0, time.UTC)
	wantExpires := time.Date(2026, 10, 11, 21, 0, 0, 0, time.UTC)
	if !send.CreatedAt.Equal(wantCreated) || !send.ExpiresAt.Equal(wantExpires) {
		t.Fatalf("timestamps = %v / %v", send.CreatedAt, send.ExpiresAt)
	}
}

func TestSendOmitsOptionalFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"to":"+5511999999999"}` {
			t.Errorf("body = %s", body)
		}
		if got := r.Header.Get("Idempotency-Key"); got != "" {
			t.Errorf("unexpected idempotency key: %q", got)
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, sendFixture)
	}))
	defer server.Close()
	client := newTestClient(t, server)
	if _, err := client.Send(context.Background(), SendParams{To: "+5511999999999"}); err != nil {
		t.Fatal(err)
	}
}

func TestStatusUsesGETWithoutContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/proxy/v1/send/SEND_a-123/status" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer app_test_key" || r.Header.Get("Accept") != "application/json" {
			t.Error("missing authentication or Accept")
		}
		if r.Header.Get("Content-Type") != "" || r.ContentLength != 0 {
			t.Error("GET request must not have a JSON body or Content-Type")
		}
		_, _ = io.WriteString(w, `{
			"id":"SEND_a-123","status":"failed","verification":"expired","mode":"sandbox",
			"channel":"whatsapp","from":"+5511888888888","expires_at":"2026-10-11T21:00:00Z",
			"created_at":"2026-10-11T20:55:00Z","sent_at":"2026-10-11T20:55:01.123Z",
			"delivered_at":"2026-10-11T20:55:02Z","read_at":"2026-10-11T20:55:03Z",
			"verified_at":"2026-10-11T20:55:04Z","failed_at":"2026-10-11T20:55:05Z",
			"failure_reason":"provider rejected"
		}`)
	}))
	defer server.Close()
	client, err := New("app_test_key", WithBaseURL(server.URL+"/proxy/"))
	if err != nil {
		t.Fatal(err)
	}
	send, err := client.Status(context.Background(), "SEND_a-123")
	if err != nil {
		t.Fatal(err)
	}
	if send.ID != "SEND_a-123" || send.Channel == nil || *send.Channel != "whatsapp" ||
		send.From == nil || *send.From != "+5511888888888" ||
		send.FailureReason == nil || *send.FailureReason != "provider rejected" {
		t.Fatalf("send = %#v", send)
	}
	for name, stamp := range map[string]*time.Time{
		"sent_at": send.SentAt, "delivered_at": send.DeliveredAt, "read_at": send.ReadAt,
		"verified_at": send.VerifiedAt, "failed_at": send.FailedAt,
	} {
		if stamp == nil || stamp.IsZero() {
			t.Errorf("%s was not decoded", name)
		}
	}
	if send.SentAt != nil && !send.SentAt.Equal(time.Date(2026, 10, 11, 20, 55, 1, 123000000, time.UTC)) {
		t.Errorf("sent_at = %v", send.SentAt)
	}
}

func TestVerifyReturnsInvalidCodeAsNormalResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/send/send_abc-123/verify" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Authorization") != "Bearer app_test_key" {
			t.Error("missing JSON content type or authentication")
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"code":"000000"}` {
			t.Errorf("body = %s", body)
		}
		_, _ = io.WriteString(w, `{"valid":false,"verification":"pending","attempts_left":2,"future":true}`)
	}))
	defer server.Close()
	result, err := newTestClient(t, server).Verify(context.Background(), "send_abc-123", "000000")
	if err != nil {
		t.Fatal(err)
	}
	if result.Valid || result.Verification != "pending" || result.AttemptsLeft != 2 {
		t.Fatalf("result = %#v", result)
	}
}

func TestDefaultBaseAndTimeoutApplyToInjectedHTTPClient(t *testing.T) {
	server := responseServer(200, sendFixture, "application/json")
	defer server.Close()
	target, _ := url.Parse(server.URL)
	var seenURL string
	var deadlineRemaining time.Duration
	transport := http.DefaultTransport.(*http.Transport).Clone()
	defer transport.CloseIdleConnections()
	injected := &http.Client{Timeout: time.Millisecond, Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seenURL = r.URL.String()
		deadline, ok := r.Context().Deadline()
		if !ok {
			t.Error("request has no timeout")
		}
		deadlineRemaining = time.Until(deadline)
		local := r.Clone(r.Context())
		local.URL.Scheme, local.URL.Host = target.Scheme, target.Host
		return transport.RoundTrip(local)
	})}
	client, err := New("app_test_key", WithHTTPClient(injected))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Status(context.Background(), "send_abc-123"); err != nil {
		t.Fatal(err)
	}
	if seenURL != "https://api.aut2fa.com/v1/send/send_abc-123/status" {
		t.Errorf("URL = %s", seenURL)
	}
	if deadlineRemaining < 9*time.Second || deadlineRemaining > 10*time.Second {
		t.Errorf("request deadline remaining = %v, want approximately 10s", deadlineRemaining)
	}
	if injected.Timeout != time.Millisecond {
		t.Error("SDK mutated supplied HTTP client")
	}
}

func TestInjectedTLSClientRetainsTransport(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, sendFixture)
	}))
	defer server.Close()
	client, err := New("app_test_key", WithBaseURL(server.URL), WithHTTPClient(server.Client()), WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Status(context.Background(), "send_abc-123"); err != nil {
		t.Fatal(err)
	}
}

func newTestClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	client, err := New("app_test_key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func responseServer(status int, body, contentType string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }
