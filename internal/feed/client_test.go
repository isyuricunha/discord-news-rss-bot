package feed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/isyuricunha/discord-news-rss-bot/internal/model"
)

func TestFetchValidRSSAndCacheHeaders(t *testing.T) {
	var gotETag, gotModified string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotETag = r.Header.Get("If-None-Match")
		gotModified = r.Header.Get("If-Modified-Since")
		w.Header().Set("ETag", `"abc"`)
		w.Header().Set("Last-Modified", "Sun, 21 Jun 2026 00:00:00 GMT")
		w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>T</title><item><guid>g1</guid><title>Hello</title><link>https://example.com/a?utm_source=x</link><description><![CDATA[<p>Body</p>]]></description><pubDate>Sun, 21 Jun 2026 00:00:00 GMT</pubDate></item></channel></rss>`))
	}))
	defer server.Close()

	client := New(Options{Timeout: time.Second, MaxBytes: 1024 * 1024, MaxEntries: 20})
	result, err := client.Fetch(context.Background(), model.FeedConfig{URL: server.URL, Source: "Source", Category: "News", Emoji: "📰"}, model.FeedState{ETag: `"old"`, LastModified: "Sun, 20 Jun 2026 00:00:00 GMT"})
	if err != nil {
		t.Fatal(err)
	}
	if gotETag != `"old"` || gotModified == "" {
		t.Fatalf("cache request headers missing: etag=%q modified=%q", gotETag, gotModified)
	}
	if len(result.Articles) != 1 || result.Articles[0].GUID != "g1" || result.ETag != `"abc"` {
		t.Fatalf("unexpected result %#v", result)
	}
}

func TestFetchValidAtom(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"><title>T</title><entry><id>id1</id><title>Atom title</title><link href="https://example.com/a"/><updated>2026-06-21T00:00:00Z</updated><summary>Summary</summary></entry></feed>`))
	}))
	defer server.Close()
	client := New(Options{Timeout: time.Second, MaxBytes: 1024 * 1024, MaxEntries: 20})
	result, err := client.Fetch(context.Background(), model.FeedConfig{URL: server.URL, Source: "Source", Category: "News", Emoji: "📰"}, model.FeedState{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Articles) != 1 || result.Articles[0].Title != "Atom title" {
		t.Fatalf("unexpected atom result %#v", result)
	}
}

func TestFetchMalformedOversizedRedirectAnd304(t *testing.T) {
	t.Run("malformed", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("<rss><broken>"))
		}))
		defer server.Close()
		_, err := New(Options{Timeout: time.Second, MaxBytes: 1024, MaxEntries: 20}).Fetch(context.Background(), model.FeedConfig{URL: server.URL}, model.FeedState{})
		if err == nil {
			t.Fatal("expected parse error")
		}
	})
	t.Run("oversized", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(strings.Repeat("x", 20)))
		}))
		defer server.Close()
		_, err := New(Options{Timeout: time.Second, MaxBytes: 10, MaxEntries: 20}).Fetch(context.Background(), model.FeedConfig{URL: server.URL}, model.FeedState{})
		if err == nil || !strings.Contains(err.Error(), "MAX_FEED_BYTES") {
			t.Fatalf("expected oversized error, got %v", err)
		}
	})
	t.Run("redirect limit", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, r.URL.String(), http.StatusFound)
		}))
		defer server.Close()
		_, err := New(Options{Timeout: time.Second, MaxBytes: 1024, MaxEntries: 20}).Fetch(context.Background(), model.FeedConfig{URL: server.URL}, model.FeedState{})
		if err == nil {
			t.Fatal("expected redirect error")
		}
	})
	t.Run("not modified", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotModified)
		}))
		defer server.Close()
		result, err := New(Options{Timeout: time.Second, MaxBytes: 1024, MaxEntries: 20}).Fetch(context.Background(), model.FeedConfig{URL: server.URL}, model.FeedState{})
		if err != nil || !result.NotModified {
			t.Fatalf("expected 304 result, got %#v err %v", result, err)
		}
	})
}

func TestFetchCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := New(Options{Timeout: time.Second, MaxBytes: 1024, MaxEntries: 20}).Fetch(ctx, model.FeedConfig{URL: server.URL}, model.FeedState{})
	if err == nil {
		t.Fatal("expected cancellation error")
	}
}
