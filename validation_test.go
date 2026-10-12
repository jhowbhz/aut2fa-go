package aut2fa

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewRejectsInvalidCredentials(t *testing.T) {
	for _, key := range []string{"", "   ", "key\r\nInjected: yes", "key\x00", "key\t", "key\u0085"} {
		t.Run(key, func(t *testing.T) {
			if client, err := New(key); err == nil || client != nil {
				t.Fatalf("client = %#v, error = %v", client, err)
			}
		})
	}
}

func TestNewRejectsInvalidConfiguration(t *testing.T) {
	for _, test := range []struct {
		name   string
		option Option
	}{
		{"empty base", WithBaseURL("")}, {"relative base", WithBaseURL("/local")},
		{"unsupported protocol", WithBaseURL("ftp://localhost")},
		{"missing host", WithBaseURL("https:///api")},
		{"credentials", WithBaseURL("https://user:password@localhost")},
		{"query", WithBaseURL("https://localhost?key=secret")},
		{"empty query", WithBaseURL("https://localhost?")},
		{"fragment", WithBaseURL("https://localhost#api")},
		{"empty fragment", WithBaseURL("https://localhost#")},
		{"invalid port", WithBaseURL("http://localhost:invalid")},
		{"out of range port", WithBaseURL("http://localhost:65536")},
		{"zero timeout", WithTimeout(0)}, {"negative timeout", WithTimeout(-time.Second)},
		{"nil HTTP client", WithHTTPClient(nil)}, {"nil option", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			if client, err := New("app_test_key", test.option); err == nil || client != nil {
				t.Fatalf("client = %#v, error = %v", client, err)
			}
		})
	}
}

func TestInvalidIDsNeverReachServer(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, sendFixture)
	}))
	defer server.Close()
	client := newTestClient(t, server)
	for _, id := range []string{"", "../send", "a/b", "a?b", "a#b", "a%2Fb", "a b", "á", "a\nb"} {
		t.Run(id, func(t *testing.T) {
			if _, err := client.Status(context.Background(), id); err == nil {
				t.Error("status accepted invalid ID")
			}
			if _, err := client.Verify(context.Background(), id, "123456"); err == nil {
				t.Error("verify accepted invalid ID")
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid IDs made %d requests", calls.Load())
	}
}

func TestEmptyRequiredInputNeverReachesServer(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, sendFixture)
	}))
	defer server.Close()
	client := newTestClient(t, server)
	for _, value := range []string{"", "   "} {
		if _, err := client.Send(context.Background(), SendParams{To: value}); err == nil {
			t.Error("send accepted empty destination")
		}
		if _, err := client.Verify(context.Background(), "send_abc-123", value); err == nil {
			t.Error("verify accepted empty code")
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("empty input made %d requests", calls.Load())
	}
}

func TestInvalidIdempotencyKeysNeverReachServer(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, sendFixture)
	}))
	defer server.Close()
	client := newTestClient(t, server)
	for _, key := range []string{strings.Repeat("a", 256), strings.Repeat("é", 128), "key\n", "key\r", "key\x00", "key\t", "key\u0085"} {
		if _, err := client.Send(context.Background(), SendParams{To: "+5511999999999", IdempotencyKey: key}); err == nil {
			t.Errorf("send accepted invalid idempotency key %q", key)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid keys made %d requests", calls.Load())
	}
}

func TestIdempotencyKeyAllows255Bytes(t *testing.T) {
	key := strings.Repeat("a", 255)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Idempotency-Key") != key {
			t.Error("maximum length key was changed")
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, sendFixture)
	}))
	defer server.Close()
	if _, err := newTestClient(t, server).Send(context.Background(), SendParams{To: "+5511999999999", IdempotencyKey: key}); err != nil {
		t.Fatal(err)
	}
}
