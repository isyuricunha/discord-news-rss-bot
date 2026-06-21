package discord

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPayloadIsEmbedOnlyAndDisablesMentions(t *testing.T) {
	var parsed Message
	var raw map[string]any
	var userAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent = r.Header.Get("User-Agent")
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(raw)
		if err := json.Unmarshal(encoded, &parsed); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	message := NewMessage(Embed{Title: "@everyone title", URL: "https://example.com/a"})
	if err := New(Options{WebhookURL: server.URL, MaxRetries: 0}).Post(context.Background(), message); err != nil {
		t.Fatal(err)
	}
	if _, exists := raw["content"]; exists {
		t.Fatalf("content must be omitted from embed payload: %#v", raw)
	}
	if len(parsed.Embeds) != 1 {
		t.Fatalf("expected one embed, got %#v", parsed.Embeds)
	}
	if parsed.AllowedMentions.Parse == nil || len(parsed.AllowedMentions.Parse) != 0 {
		t.Fatalf("allowed_mentions.parse must be an empty array, got %#v", parsed.AllowedMentions.Parse)
	}
	if !strings.HasPrefix(userAgent, "discord-rss-bot/") || strings.Contains(userAgent, "3.0 ") {
		t.Fatalf("unexpected user agent %q", userAgent)
	}
}

func TestNoContentAndOKAreSuccess(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusOK} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			defer server.Close()
			if err := New(Options{WebhookURL: server.URL}).Post(context.Background(), testMessage()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRetriesReuseSamePayload(t *testing.T) {
	var attempts atomic.Int32
	var firstBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := readBody(t, r)
		if attempts.Add(1) == 1 {
			firstBody = body
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if body != firstBody {
			t.Fatalf("retry payload changed:\nfirst=%s\nsecond=%s", firstBody, body)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := New(Options{WebhookURL: server.URL, MaxRetries: 1, Sleep: func(context.Context, time.Duration) error { return nil }})
	if err := client.Post(context.Background(), testMessage()); err != nil {
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
	if err := client.Post(context.Background(), testMessage()); err != nil {
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
			if err := client.Post(context.Background(), testMessage()); err != nil {
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
		if err := client.Post(context.Background(), testMessage()); err != nil {
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
		if err := client.Post(context.Background(), testMessage()); err == nil {
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
	err := New(Options{WebhookURL: server.URL + "/sensitive-value", MaxRetries: 0}).Post(context.Background(), testMessage())
	if err == nil {
		t.Fatal("expected network error")
	}
	if strings.Contains(err.Error(), "sensitive-value") {
		t.Fatalf("error leaked sensitive path value: %v", err)
	}
}

func TestMalformedEmbedErrorIsSanitized(t *testing.T) {
	err := New(Options{WebhookURL: "https://example.invalid/webhook/secret"}).Post(context.Background(), Message{})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "discord.com/api/webhooks") {
		t.Fatalf("validation error leaked webhook details: %v", err)
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
	if err := client.Post(ctx, testMessage()); err == nil {
		t.Fatal("expected cancellation error")
	}
}

func testMessage() Message {
	return NewMessage(Embed{Title: "Title", URL: "https://example.com/a"})
}

func readBody(t *testing.T, r *http.Request) string {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
