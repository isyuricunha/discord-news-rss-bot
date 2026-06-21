package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvironmentOverridesConfigFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.env")
	if err := os.WriteFile(configPath, []byte("DISCORD_WEBHOOK_URL=https://example.invalid/file\nCHECK_INTERVAL=600\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(LoadOptions{
		RequireWebhook: true,
		OS:             "linux",
		Env: map[string]string{
			"RSS_BOT_CONFIG":      configPath,
			"DISCORD_WEBHOOK_URL": "https://example.invalid/env",
			"CHECK_INTERVAL":      "10",
			"RSS_BOT_DATA":        dir,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DiscordWebhookURL != "https://example.invalid/env" {
		t.Fatalf("expected env webhook, got %q", cfg.DiscordWebhookURL)
	}
	if cfg.CheckInterval.String() != "10s" {
		t.Fatalf("expected env interval, got %s", cfg.CheckInterval)
	}
}

func TestConfigFileQuotedValues(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.env")
	content := `
# comment
DISCORD_WEBHOOK_URL='https://example.invalid/file'
RSS_FEEDS="https://example.com/rss, https://example.org/feed"
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(LoadOptions{RequireWebhook: true, OS: "linux", Env: map[string]string{"RSS_BOT_CONFIG": configPath, "RSS_BOT_DATA": dir}})
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Feeds) != 2 {
		t.Fatalf("expected 2 feeds, got %d", len(cfg.Feeds))
	}
}

func TestMalformedConfigLineReportsLine(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.env")
	if err := os.WriteFile(configPath, []byte("DISCORD_WEBHOOK_URL=x\nbad line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(LoadOptions{Env: map[string]string{"RSS_BOT_CONFIG": configPath}})
	if err == nil || !strings.Contains(err.Error(), ":2:") {
		t.Fatalf("expected line-numbered error, got %v", err)
	}
}

func TestInvalidSettings(t *testing.T) {
	tests := []map[string]string{
		{"DISCORD_WEBHOOK_URL": "https://example.invalid/webhook", "CHECK_INTERVAL": "0"},
		{"DISCORD_WEBHOOK_URL": "https://example.invalid/webhook", "MAX_POST_LENGTH": "2001"},
		{"DISCORD_WEBHOOK_URL": "https://example.invalid/webhook", "MAX_CONCURRENT_FEEDS": "-1"},
		{"DISCORD_WEBHOOK_URL": "https://example.invalid/webhook", "LOG_FORMAT": "xml"},
		{"DISCORD_WEBHOOK_URL": "https://example.invalid/webhook", "INITIAL_SYNC_MODE": "flood"},
	}
	for _, env := range tests {
		env["RSS_BOT_DATA"] = t.TempDir()
		if _, err := Load(LoadOptions{RequireWebhook: true, Env: env}); err == nil {
			t.Fatalf("expected error for %#v", env)
		}
	}
}

func TestMissingWebhook(t *testing.T) {
	_, err := Load(LoadOptions{RequireWebhook: true, Env: map[string]string{"RSS_BOT_DATA": t.TempDir()}})
	if err == nil || !strings.Contains(err.Error(), "DISCORD_WEBHOOK_URL") {
		t.Fatalf("expected missing webhook error, got %v", err)
	}
}

func TestUniversalFeedsTakePriorityAndDeduplicate(t *testing.T) {
	cfg, err := Load(LoadOptions{
		RequireWebhook: true,
		Env: map[string]string{
			"DISCORD_WEBHOOK_URL": "https://example.invalid/webhook",
			"RSS_BOT_DATA":        t.TempDir(),
			"RSS_FEEDS":           "https://example.com/rss, https://example.com/rss#fragment, https://example.org/feed",
			"RSS_FEEDS_NEWS":      "https://ignored.example/feed",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Feeds) != 2 {
		t.Fatalf("expected 2 deduplicated universal feeds, got %d", len(cfg.Feeds))
	}
	if cfg.Feeds[0].Category != "📢 Universal Feeds" {
		t.Fatalf("unexpected category %q", cfg.Feeds[0].Category)
	}
}

func TestCategoryFeedsAndLegacyPaths(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(LoadOptions{
		RequireWebhook: true,
		Env: map[string]string{
			"DISCORD_WEBHOOK_URL":  "https://example.invalid/webhook",
			"DATA_DIR":             dir,
			"DB_FILE":              filepath.Join(dir, "custom.db"),
			"RSS_BOT_LOGS":         "/ignored/logs",
			"LOG_FILE":             "/ignored/log",
			"RSS_BOT_PID":          "/ignored/pid",
			"RSS_FEEDS_TECHNOLOGY": "https://example.com/tech",
			"RSS_FEEDS_NEWS":       "https://example.com/news",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataDir != dir || cfg.DBFile != filepath.Join(dir, "custom.db") {
		t.Fatalf("legacy path fallback failed: %#v", cfg)
	}
	if len(cfg.Deprecated) != 3 {
		t.Fatalf("expected deprecated setting warnings, got %#v", cfg.Deprecated)
	}
	if len(cfg.Feeds) != 2 {
		t.Fatalf("expected category feeds, got %d", len(cfg.Feeds))
	}
}

func TestInvalidFeedURLIsSanitized(t *testing.T) {
	_, err := Load(LoadOptions{
		RequireWebhook: true,
		Env: map[string]string{
			"DISCORD_WEBHOOK_URL": "https://example.invalid/webhook",
			"RSS_BOT_DATA":        t.TempDir(),
			"RSS_FEEDS":           "ftp://user:pass@example.com/feed?token=secret",
		},
	})
	if err == nil {
		t.Fatal("expected invalid feed URL error")
	}
	if strings.Contains(err.Error(), "pass") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("error leaked sensitive URL data: %v", err)
	}
}
