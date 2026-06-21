package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateFeedsCommandUsesLocalFixtures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>T</title><item><guid>g1</guid><title>Local article</title><link>https://example.com/a</link><description>Body</description></item></channel></rss>`))
	}))
	defer server.Close()

	dataDir := t.TempDir()
	t.Setenv("RSS_BOT_DATA", dataDir)
	t.Setenv("RSS_FEEDS", server.URL)
	t.Setenv("FEED_TIMEOUT", "5s")

	if err := run([]string{"validate-feeds"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "posted_hashes.db")); !os.IsNotExist(err) {
		t.Fatalf("validate-feeds must not create a database, stat error: %v", err)
	}
}

func TestValidateFeedsCommandFailsForHTMLFixture(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("<!doctype html><html><body>not a feed</body></html>"))
	}))
	defer server.Close()

	t.Setenv("RSS_BOT_DATA", t.TempDir())
	t.Setenv("RSS_FEEDS", server.URL)
	t.Setenv("FEED_TIMEOUT", "5s")

	err := run([]string{"validate-feeds"})
	if err == nil || !strings.Contains(err.Error(), "failed validation") {
		t.Fatalf("expected validation failure, got %v", err)
	}
}
