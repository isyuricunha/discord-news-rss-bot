package discord

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPayloadAlwaysDisablesMentions(t *testing.T) {
	var parsed struct {
		Content         string `json:"content"`
		AllowedMentions struct {
			Parse []string `json:"parse"`
		} `json:"allowed_mentions"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&parsed); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := New(Options{WebhookURL: server.URL, MaxRetries: 0})
	if err := client.Post(context.Background(), "@everyone <@123> <@&456>"); err != nil {
		t.Fatal(err)
	}
	if parsed.AllowedMentions.Parse == nil || len(parsed.AllowedMentions.Parse) != 0 {
		t.Fatalf("allowed_mentions.parse must be an empty array, got %#v", parsed.AllowedMentions.Parse)
	}
}

func TestNoContentIsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	if err := New(Options{WebhookURL: server.URL}).Post(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
}

func TestRateLimitRetryAfterHeader(t *testing.T) {
	var attempts atomic.Int32
	var delays []time.Duration
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			w.Header().Set("Retry-After", "0.25")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := New(Options{
		WebhookURL: server.URL,
		MaxRetries: 1,
		Sleep: func(ctx context.Context, delay time.Duration) error {
			delays = append(delays, delay)
			return nil
		},
	})
	if err := client.Post(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("expected retry, got %d attempts", attempts.Load())
	}
	if len(delays) != 1 || delays[0] != 250*time.Millisecond {
		t.Fatalf("unexpected retry delay %#v", delays)
	}
}

func TestRateLimitRetryAfterJSONAndFallback(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		fallback time.Duration
		want     time.Duration
	}{
		{name: "json", body: `{"retry_after":0.5}`, fallback: 3 * time.Second, want: 500 * time.Millisecond},
		{name: "fallback", body: `{}`, fallback: 3 * time.Second, want: 3 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var attempts atomic.Int32
			var delay time.Duration
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if attempts.Add(1) == 1 {
					w.WriteHeader(http.StatusTooManyRequests)
					w.Write([]byte(tt.body))
					return
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()
			client := New(Options{
				WebhookURL:       server.URL,
				MaxRetries:       1,
				CooldownFallback: tt.fallback,
				Sleep: func(ctx context.Context, got time.Duration) error {
					delay = got
					return nil
				},
			})
			if err := client.Post(context.Background(), "hello"); err != nil {
				t.Fatal(err)
			}
			if delay != tt.want {
				t.Fatalf("got %s want %s", delay, tt.want)
			}
		})
	}
}

func TestServerErrorRetriesAndBadRequestDoesNot(t *testing.T) {
	t.Run("500 retries", func(t *testing.T) {
		var attempts atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if attempts.Add(1) == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()
		client := New(Options{WebhookURL: server.URL, MaxRetries: 1, Sleep: func(context.Context, time.Duration) error { return nil }})
		if err := client.Post(context.Background(), "hello"); err != nil {
			t.Fatal(err)
		}
		if attempts.Load() != 2 {
			t.Fatalf("expected retry")
		}
	})
	t.Run("400 does not retry", func(t *testing.T) {
		var attempts atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts.Add(1)
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()
		client := New(Options{WebhookURL: server.URL, MaxRetries: 5, Sleep: func(context.Context, time.Duration) error { return nil }})
		if err := client.Post(context.Background(), "hello"); err == nil {
			t.Fatal("expected error")
		}
		if attempts.Load() != 1 {
			t.Fatalf("bad request retried %d times", attempts.Load())
		}
	})
}

func TestWebhookSecretNotInErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	err := New(Options{WebhookURL: server.URL + "/sensitive-value", MaxRetries: 0}).Post(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected network error")
	}
	if strings.Contains(err.Error(), "sensitive-value") {
		t.Fatalf("error leaked sensitive path value: %v", err)
	}
}

func TestContextCancellationInterruptsRetryWait(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	client := New(Options{
		WebhookURL: server.URL,
		MaxRetries: 1,
		Sleep: func(ctx context.Context, delay time.Duration) error {
			cancel()
			return ctx.Err()
		},
	})
	if err := client.Post(ctx, "hello"); err == nil {
		t.Fatal("expected cancellation error")
	}
}
