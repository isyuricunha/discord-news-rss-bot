package app

import (
	"strings"
	"testing"

	"github.com/isyuricunha/discord-news-rss-bot/internal/model"
	"github.com/isyuricunha/discord-news-rss-bot/internal/text"
)

func TestFormatMessageCleansHTMLAndPreservesEssentials(t *testing.T) {
	article := model.Article{CategoryEmoji: "📰", Title: "Title", Link: "https://example.com/a", Source: "Source", Content: "<p>Hello<br>world</p>"}
	message := FormatMessage(article, 1900, 800)
	for _, expected := range []string{"Title", "https://example.com/a", "Source", "Hello\n│ world"} {
		if !strings.Contains(message, expected) {
			t.Fatalf("message missing %q:\n%s", expected, message)
		}
	}
}

func TestFormatMessageUnicodeSafeExactLimit(t *testing.T) {
	article := model.Article{CategoryEmoji: "📰", Title: strings.Repeat("😀", 300), Link: "https://example.com/a", Source: "Source", Content: strings.Repeat("body ", 400)}
	message := FormatMessage(article, 200, 800)
	if text.Length(message) > 200 {
		t.Fatalf("message length %d exceeds limit", text.Length(message))
	}
	if !strings.Contains(message, "https://example.com/a") || !strings.Contains(message, "Source") {
		t.Fatalf("essential fields missing:\n%s", message)
	}
}

func TestFormatMessageEmptyContentNoEmptyMessage(t *testing.T) {
	article := model.Article{CategoryEmoji: "📰", Title: "", Link: "", Source: "", Content: ""}
	message := FormatMessage(article, 1900, 800)
	if strings.TrimSpace(message) == "" {
		t.Fatal("empty message")
	}
}
