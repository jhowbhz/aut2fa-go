package aut2fa

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// Following a redirect can leak the application key or duplicate a mutable request.
func TestRedirectsAreReturnedWithoutFollowing(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var targetCalls atomic.Int32
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				targetCalls.Add(1)
				_, _ = io.WriteString(w, sendFixture)
			}))
			defer target.Close()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, target.URL+"/v1/send", status)
			}))
			defer server.Close()
			client, err := New("app_test_key", WithBaseURL(server.URL), WithHTTPClient(&http.Client{
				CheckRedirect: func(*http.Request, []*http.Request) error { return nil },
			}))
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Send(context.Background(), SendParams{To: "+5511999999999", IdempotencyKey: "redirect"})
			var problem *ApiError
			if !errors.As(err, &problem) || problem.Status != status || problem.Code != "http_error" {
				t.Fatalf("error = %v, want HTTP %d", err, status)
			}
			if targetCalls.Load() != 0 {
				t.Fatalf("redirect target received %d requests", targetCalls.Load())
			}
		})
	}
}

func TestTimeoutRemainsANetworkError(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
	defer server.Close()
	client, err := New("app_test_key", WithBaseURL(server.URL), WithTimeout(100*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Send(context.Background(), SendParams{To: "+5511999999999"})
	var networkError net.Error
	var problem *ApiError
	if !errors.As(err, &networkError) || !networkError.Timeout() || errors.As(err, &problem) {
		t.Fatalf("error = %v, want identifiable timeout", err)
	}
	if calls.Load() != 1 {
		t.Errorf("request made %d times", calls.Load())
	}
}

func TestCallerDeadlineIsPreserved(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
	defer server.Close()
	_, err := newTestClient(t, server).Verify(ctx, "send_abc-123", "123456")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context.DeadlineExceeded", err)
	}
}

func TestCallerCancellationIsPreserved(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		cancel()
		<-r.Context().Done()
	}))
	defer server.Close()
	_, err := newTestClient(t, server).Verify(ctx, "send_abc-123", "123456")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestDroppedConnectionDoesNotReplayIdempotentSend(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusAccepted)
			_, _ = io.WriteString(w, sendFixture)
			return
		}
		if calls.Load() > 2 {
			_, _ = io.WriteString(w, sendFixture)
			return
		}
		connection, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		_ = connection.Close()
	}))
	defer server.Close()
	client := newTestClient(t, server)
	params := SendParams{To: "+5511999999999", IdempotencyKey: "one-logical-send"}
	if _, err := client.Send(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	_, err := client.Send(context.Background(), params)
	var problem *ApiError
	if err == nil || errors.As(err, &problem) || calls.Load() != 2 {
		t.Fatalf("error = %v, server calls = %d; dropped send must not replay", err, calls.Load())
	}
}
