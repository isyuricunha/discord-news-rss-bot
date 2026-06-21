package feed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/isyuricunha/discord-news-rss-bot/internal/model"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

func TestFetchValidRSSAndHeaders(t *testing.T) {
	var gotETag, gotModified, gotAccept, gotUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotETag = r.Header.Get("If-None-Match")
		gotModified = r.Header.Get("If-Modified-Since")
		gotAccept = r.Header.Get("Accept")
		gotUserAgent = r.Header.Get("User-Agent")
		w.Header().Set("ETag", `"abc"`)
		w.Header().Set("Last-Modified", "Sun, 21 Jun 2026 00:00:00 GMT")
		w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>T</title><link>https://publisher.example/news</link><image><url>/logo.png</url><title>Logo</title><link>https://publisher.example</link></image><item><guid>g1</guid><title>Hello</title><link>https://example.com/a?utm_source=x</link><author>Reporter</author><description><![CDATA[<p>Body</p><img src="/image.jpg">]]></description><pubDate>Sun, 21 Jun 2026 00:00:00 GMT</pubDate></item></channel></rss>`))
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
	if !strings.Contains(gotAccept, "application/rss+xml") || !strings.Contains(gotAccept, "application/atom+xml") {
		t.Fatalf("missing feed accept header: %q", gotAccept)
	}
	if !strings.HasPrefix(gotUserAgent, "discord-rss-bot/") || strings.Contains(gotUserAgent, "3.0 ") {
		t.Fatalf("unexpected user agent: %q", gotUserAgent)
	}
	if len(result.Articles) != 1 || result.Articles[0].GUID != "g1" || result.ETag != `"abc"` {
		t.Fatalf("unexpected result %#v", result)
	}
	article := result.Articles[0]
	if article.Description == "" || article.AuthorName != "Reporter" || article.SourceURL != "https://publisher.example/news" {
		t.Fatalf("metadata missing: %#v", article)
	}
	if article.SourceIconURL != "https://publisher.example/logo.png" || article.ImageURL != "https://example.com/image.jpg" {
		t.Fatalf("image metadata missing: %#v", article)
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

func TestFetchCharsetHandling(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        []byte
		wantTitle   string
		wantText    string
	}{
		{
			name:        "utf8",
			contentType: "application/rss+xml; charset=utf-8",
			body:        []byte(rssDocument("Notícias de hoje", "Descrição correta")),
			wantTitle:   "Notícias de hoje",
			wantText:    "Descrição correta",
		},
		{
			name:        "utf8 bom",
			contentType: "application/rss+xml; charset=utf-8",
			body:        append([]byte{0xEF, 0xBB, 0xBF}, []byte(rssDocument("Título com BOM", "Resumo"))...),
			wantTitle:   "Título com BOM",
			wantText:    "Resumo",
		},
		{
			name:        "iso-8859-1",
			contentType: "text/xml; charset=ISO-8859-1",
			body:        encodeString(t, rssDocument("Notícias e política", "Descrição com acentuação"), charmap.ISO8859_1.NewEncoder()),
			wantTitle:   "Notícias e política",
			wantText:    "Descrição com acentuação",
		},
		{
			name:        "windows-1252",
			contentType: "application/rss+xml; charset=windows-1252",
			body:        encodeString(t, rssDocument("“Tecnologia” avançada", "Coração e inovação"), charmap.Windows1252.NewEncoder()),
			wantTitle:   "“Tecnologia” avançada",
			wantText:    "Coração e inovação",
		},
		{
			name:        "invalid utf8 fallback",
			contentType: "application/rss+xml; charset=utf-8",
			body:        encodeString(t, rssDocument("“Título” inválido", "Descrição"), charmap.Windows1252.NewEncoder()),
			wantTitle:   "“Título” inválido",
			wantText:    "Descrição",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				w.Write(tt.body)
			}))
			defer server.Close()

			result, err := New(Options{Timeout: time.Second, MaxBytes: 1024 * 1024, MaxEntries: 20}).Fetch(context.Background(), model.FeedConfig{URL: server.URL, Source: "Source", Category: "News", Emoji: "📰"}, model.FeedState{})
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Articles) != 1 {
				t.Fatalf("expected one article, got %#v", result.Articles)
			}
			if result.Articles[0].Title != tt.wantTitle {
				t.Fatalf("title mismatch: got %q want %q", result.Articles[0].Title, tt.wantTitle)
			}
			if !strings.Contains(result.Articles[0].Description, tt.wantText) {
				t.Fatalf("description mismatch: got %q want text %q", result.Articles[0].Description, tt.wantText)
			}
		})
	}
}

func TestFetchUnsupportedCharset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml; charset=x-unknown-feed-charset")
		w.Write([]byte("<rss><broken>"))
	}))
	defer server.Close()

	_, err := New(Options{Timeout: time.Second, MaxBytes: 1024 * 1024, MaxEntries: 20}).Fetch(context.Background(), model.FeedConfig{URL: server.URL}, model.FeedState{})
	if err == nil || !strings.Contains(err.Error(), "unsupported feed charset") {
		t.Fatalf("expected unsupported charset error, got %v", err)
	}
}

func TestFetchHTMLDetection(t *testing.T) {
	tests := map[string]string{
		"doctype":   "<!doctype html><html><body>not a feed</body></html>",
		"uppercase": "<HTML><body>not a feed</body></HTML>",
		"leading":   " \n\t<html><body>not a feed</body></html>",
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write([]byte(body))
			}))
			defer server.Close()

			_, err := New(Options{Timeout: time.Second, MaxBytes: 1024, MaxEntries: 20}).Fetch(context.Background(), model.FeedConfig{URL: server.URL}, model.FeedState{})
			if err == nil || !strings.Contains(err.Error(), "HTML instead of RSS or Atom") {
				t.Fatalf("expected HTML error, got %v", err)
			}
		})
	}
}

func TestFetchXMLServedAsHTMLStillParses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(rssDocument("XML feed", "Served with a bad content type")))
	}))
	defer server.Close()

	result, err := New(Options{Timeout: time.Second, MaxBytes: 1024 * 1024, MaxEntries: 20}).Fetch(context.Background(), model.FeedConfig{URL: server.URL}, model.FeedState{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Articles) != 1 || result.Articles[0].Title != "XML feed" {
		t.Fatalf("unexpected result %#v", result)
	}
}

func TestFetchMalformedOversizedRedirectAnd304(t *testing.T) {
	t.Run("malformed", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("<rss><broken>"))
		}))
		defer server.Close()
		_, err := New(Options{Timeout: time.Second, MaxBytes: 1024, MaxEntries: 20}).Fetch(context.Background(), model.FeedConfig{URL: server.URL}, model.FeedState{})
		if err == nil || !strings.Contains(err.Error(), "malformed XML") {
			t.Fatalf("expected malformed XML error, got %v", err)
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

func rssDocument(title, description string) string {
	return `<?xml version="1.0"?><rss version="2.0"><channel><title>T</title><item><guid>g1</guid><title>` +
		title +
		`</title><link>https://example.com/a</link><description>` +
		description +
		`</description><pubDate>Sun, 21 Jun 2026 00:00:00 GMT</pubDate></item></channel></rss>`
}

func encodeString(t *testing.T, value string, transformer transform.Transformer) []byte {
	t.Helper()
	encoded, _, err := transform.String(transformer, value)
	if err != nil {
		t.Fatal(err)
	}
	return []byte(encoded)
}
