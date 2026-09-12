package infrai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPublishRetriesRateLimitWithSameIdempotencyKey(t *testing.T) {
	var attempts int
	var keys []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		if r.Method != http.MethodPost || r.URL.Path != "/v1/realtime/publish" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization = %q", got)
		}
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"ok":false,"data":null,"error":{"code":"RATE_LIMITED","message":"retry later"},"metadata":{}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{"published":true},"error":null,"metadata":{}}`))
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.baseURL = server.URL
	client.httpClient = server.Client()
	client.sleep = func(context.Context, time.Duration) error { return nil }
	err := client.Publish(context.Background(), "account:acct_7", "order.delivered", map[string]any{"order_id": "ord_42"}, "acct_7", "order:ord_42:delivered")
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || keys[0] != keys[1] || keys[0] != "order:ord_42:delivered" {
		t.Fatalf("attempts = %d, keys = %v", attempts, keys)
	}
}

func TestPublishReturnsEnvelopeErrorFromBadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"data":null,"error":{"code":"BAD_ORDER","message":"order rejected"},"metadata":{}}`))
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.baseURL = server.URL
	client.httpClient = server.Client()
	err := client.Publish(context.Background(), "account:acct_7", "order.delivered", nil, "acct_7", "order:ord_42:delivered")
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Status != http.StatusBadRequest || apiErr.Code != "BAD_ORDER" {
		t.Fatalf("error = %#v", err)
	}
}
