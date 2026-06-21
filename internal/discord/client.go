package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/isyuricunha/discord-news-rss-bot/internal/text"
	"github.com/isyuricunha/discord-news-rss-bot/internal/version"
)

type Client struct {
	webhookURL       string
	httpClient       *http.Client
	maxRetries       int
	cooldownFallback time.Duration
	logger           *slog.Logger
	sleep            func(context.Context, time.Duration) error
}

type Options struct {
	WebhookURL       string
	HTTPClient       *http.Client
	MaxRetries       int
	CooldownFallback time.Duration
	Logger           *slog.Logger
	Sleep            func(context.Context, time.Duration) error
}

func New(options Options) *Client {
	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	sleep := options.Sleep
	if sleep == nil {
		sleep = sleepContext
	}
	return &Client{
		webhookURL:       options.WebhookURL,
		httpClient:       httpClient,
		maxRetries:       options.MaxRetries,
		cooldownFallback: options.CooldownFallback,
		logger:           logger,
		sleep:            sleep,
	}
}

func (c *Client) Post(ctx context.Context, message Message) error {
	if err := message.Validate(); err != nil {
		return err
	}
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}

	attempts := c.maxRetries + 1
	for attempt := 1; attempt <= attempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.webhookURL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create Discord webhook request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", userAgent())

		res, err := c.httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if attempt >= attempts {
				return fmt.Errorf("temporary Discord request failure after %d attempts", attempt)
			}
			delay := retryBackoff(attempt)
			c.logRetry(attempt, 0, delay, "network")
			if err := c.sleep(ctx, delay); err != nil {
				return err
			}
			continue
		}

		responseBody, readErr := io.ReadAll(io.LimitReader(res.Body, 64*1024))
		closeErr := res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read Discord response: %w", readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close Discord response: %w", closeErr)
		}

		if res.StatusCode >= 200 && res.StatusCode < 300 {
			return nil
		}

		switch res.StatusCode {
		case http.StatusTooManyRequests:
			if attempt >= attempts {
				return fmt.Errorf("Discord rate limit persisted after %d attempts", attempt)
			}
			delay := retryAfterDelay(res.Header.Get("Retry-After"), responseBody, c.cooldownFallback)
			c.logRetry(attempt, res.StatusCode, delay, "rate_limit")
			if err := c.sleep(ctx, delay); err != nil {
				return err
			}
			continue
		case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
			return fmt.Errorf("Discord webhook rejected request with status %d; verify the webhook URL and permissions", res.StatusCode)
		}

		if res.StatusCode >= 500 {
			if attempt >= attempts {
				return fmt.Errorf("Discord temporary server error status %d after %d attempts", res.StatusCode, attempt)
			}
			delay := retryBackoff(attempt)
			c.logRetry(attempt, res.StatusCode, delay, "server_error")
			if err := c.sleep(ctx, delay); err != nil {
				return err
			}
			continue
		}

		return fmt.Errorf("Discord webhook returned non-retryable status %d", res.StatusCode)
	}

	return errors.New("Discord webhook request exhausted retries")
}

func retryAfterDelay(header string, responseBody []byte, fallback time.Duration) time.Duration {
	if header != "" {
		if seconds, err := strconv.ParseFloat(strings.TrimSpace(header), 64); err == nil && seconds >= 0 {
			return durationFromSeconds(seconds)
		}
		if when, err := http.ParseTime(header); err == nil {
			delay := time.Until(when)
			if delay > 0 {
				return delay
			}
		}
	}
	var decoded struct {
		RetryAfter *float64 `json:"retry_after"`
	}
	if err := json.Unmarshal(responseBody, &decoded); err == nil && decoded.RetryAfter != nil && *decoded.RetryAfter >= 0 {
		return durationFromSeconds(*decoded.RetryAfter)
	}
	if fallback > 0 {
		return fallback
	}
	return time.Second
}

func durationFromSeconds(seconds float64) time.Duration {
	return time.Duration(math.Ceil(seconds * float64(time.Second)))
}

func retryBackoff(attempt int) time.Duration {
	base := 500 * time.Millisecond
	delay := base * time.Duration(1<<(attempt-1))
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}
	jitter := time.Duration(time.Now().UnixNano() % int64(delay/5+time.Millisecond))
	return delay + jitter
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Client) logRetry(attempt int, status int, delay time.Duration, reason string) {
	c.logger.Warn(
		"retrying Discord webhook request",
		"attempt", attempt,
		"status", status,
		"delay", delay.String(),
		"reason", reason,
		"webhook", text.RedactSecret(c.webhookURL),
	)
}

func userAgent() string {
	value := strings.TrimSpace(version.Version)
	if value == "" {
		value = "dev"
	}
	return "discord-rss-bot/" + value
}
