package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isyuricunha/discord-news-rss-bot/internal/discord"
	"github.com/isyuricunha/discord-news-rss-bot/internal/model"
	"github.com/isyuricunha/discord-news-rss-bot/internal/text"
)

func TestCategoryColorMapping(t *testing.T) {
	tests := map[string]int{
		"📰 General News":             colorNews,
		"News":                       colorNews,
		"🏛️ Politics & Conservative": colorPolitics,
		"💻 Technology":               colorTechnology,
		"Sports":                     colorSports,
		"Business & Finance":         colorBusiness,
		"Universal Feeds":            colorDefault,
	}
	for category, want := range tests {
		if got := categoryColor(category); got != want {
			t.Fatalf("%q color got %d want %d", category, got, want)
		}
	}
}

func TestBuildArticleMessageCompleteArticle(t *testing.T) {
	message := buildTestMessage(t, completeArticle())
	embed := message.Embeds[0]
	if message.Content != "" {
		t.Fatalf("content should be empty, got %q", message.Content)
	}
	if len(message.Embeds) != 1 {
		t.Fatalf("expected exactly one embed")
	}
	if embed.Author == nil || embed.Author.Name != "📰 G1" || embed.Author.URL != "https://source.example" || embed.Author.IconURL != "https://source.example/logo.png" {
		t.Fatalf("unexpected author %#v", embed.Author)
	}
	if embed.Title != "Tamanduá fica preso em portão de residência" || embed.URL != "https://news.example/article" {
		t.Fatalf("unexpected title/url %#v", embed)
	}
	if strings.Contains(embed.Description, embed.Title) || strings.Contains(embed.Description, "WhatsApp") || strings.Contains(embed.Description, "Leia mais") {
		t.Fatalf("description was not cleaned: %q", embed.Description)
	}
	if embed.Color != colorNews {
		t.Fatalf("unexpected color %d", embed.Color)
	}
	if embed.Image == nil || embed.Image.URL != "https://cdn.example/image.jpg" {
		t.Fatalf("unexpected image %#v", embed.Image)
	}
	if embed.Footer == nil || embed.Footer.Text != "By Reporter Name • General News" {
		t.Fatalf("unexpected footer %#v", embed.Footer)
	}
	if embed.Timestamp != "2026-06-21T19:41:00Z" {
		t.Fatalf("unexpected timestamp %q", embed.Timestamp)
	}
}

func TestBuildArticleMessageGracefulFallbacks(t *testing.T) {
	t.Run("no image", func(t *testing.T) {
		article := completeArticle()
		article.ImageURL = ""
		message := buildTestMessage(t, article)
		if message.Embeds[0].Image != nil {
			t.Fatalf("image should be omitted")
		}
	})
	t.Run("no source icon", func(t *testing.T) {
		article := completeArticle()
		article.SourceIconURL = ""
		message := buildTestMessage(t, article)
		if message.Embeds[0].Author == nil || message.Embeds[0].Author.IconURL != "" {
			t.Fatalf("source icon should be omitted")
		}
	})
	t.Run("no description", func(t *testing.T) {
		article := completeArticle()
		article.Description = ""
		article.Content = ""
		message := buildTestMessage(t, article)
		if message.Embeds[0].Description != "" {
			t.Fatalf("description should be omitted")
		}
	})
	t.Run("no article author", func(t *testing.T) {
		article := completeArticle()
		article.AuthorName = ""
		message := buildTestMessage(t, article)
		if message.Embeds[0].Footer.Text != "General News • RSS" {
			t.Fatalf("unexpected footer %q", message.Embeds[0].Footer.Text)
		}
	})
	t.Run("no publication time", func(t *testing.T) {
		article := completeArticle()
		article.PublishedAt = nil
		message := buildTestMessage(t, article)
		if message.Embeds[0].Timestamp != "" {
			t.Fatalf("timestamp should be omitted")
		}
	})
	t.Run("invalid article link", func(t *testing.T) {
		article := completeArticle()
		article.Link = "javascript:alert(1)"
		message := buildTestMessage(t, article)
		if message.Embeds[0].URL != "" {
			t.Fatalf("invalid article URL should be omitted")
		}
	})
	t.Run("minimal article", func(t *testing.T) {
		message := buildTestMessage(t, model.Article{Title: "Only title"})
		if message.Embeds[0].Title != "Only title" || message.Embeds[0].Author.Name != "📢 Source" {
			t.Fatalf("unexpected minimal embed %#v", message.Embeds[0])
		}
	})
}

func TestBuildArticleMessageLimitsAndSanitization(t *testing.T) {
	published := time.Date(2026, 6, 21, 19, 41, 0, 0, time.UTC)
	article := model.Article{
		Source:        strings.Repeat("Fonte ", 80),
		Category:      "💻 Technology",
		CategoryEmoji: "💻",
		Title:         strings.Repeat("Título😀 ", 80),
		Link:          "https://tech.example/article",
		Description:   strings.Repeat("Esta é uma frase longa com acentuação portuguesa. ", 200),
		AuthorName:    strings.Repeat("Autor ", 100),
		PublishedAt:   &published,
	}
	message := buildTestMessage(t, article)
	embed := message.Embeds[0]
	if text.Length(embed.Title) > discord.EmbedTitleLimit {
		t.Fatalf("title length %d exceeds limit", text.Length(embed.Title))
	}
	if text.Length(embed.Description) > 300 {
		t.Fatalf("description should honor max content length, got %d", text.Length(embed.Description))
	}
	if text.Length(embed.Author.Name) > discord.EmbedAuthorNameLimit || text.Length(embed.Footer.Text) > discord.EmbedFooterTextLimit {
		t.Fatalf("metadata limits exceeded")
	}
	total := text.Length(embed.Title) + text.Length(embed.Description) + text.Length(embed.Author.Name) + text.Length(embed.Footer.Text)
	if total > 400 {
		t.Fatalf("combined text cap exceeded: %d", total)
	}
	if strings.ContainsAny(embed.Title+embed.Description, "╭╮├┤╰╯") {
		t.Fatalf("box drawing characters must not be present")
	}
}

func TestGoldenPayloads(t *testing.T) {
	tests := map[string]model.Article{
		"complete-news-article":         completeArticle(),
		"technology-article-with-image": technologyArticle(),
		"article-without-image":         withoutImageArticle(),
		"minimal-article":               {Title: "Only a title", Source: "Example", Category: "Universal Feeds", CategoryEmoji: "📢"},
		"long-portuguese-article":       longPortugueseArticle(),
	}
	for name, article := range tests {
		t.Run(name, func(t *testing.T) {
			message := buildTestMessage(t, article)
			assertGoldenJSON(t, name, message)
		})
	}
}

func buildTestMessage(t *testing.T, article model.Article) discord.Message {
	t.Helper()
	message, err := BuildArticleMessage(article, 400, 300)
	if err != nil {
		t.Fatal(err)
	}
	if message.AllowedMentions.Parse == nil || len(message.AllowedMentions.Parse) != 0 {
		t.Fatalf("allowed mentions parse must be []")
	}
	return message
}

func completeArticle() model.Article {
	published := time.Date(2026, 6, 21, 19, 41, 0, 0, time.UTC)
	return model.Article{
		Source:        "G1",
		Category:      "📰 General News",
		CategoryEmoji: "📰",
		Title:         "Tamanduá fica preso em portão de residência",
		Link:          "https://news.example/article",
		Description:   "<p>Tamanduá fica preso em portão de residência</p><p>Animal foi resgatado com segurança pela equipe ambiental.</p><p>Clique aqui para seguir o canal do G1 no WhatsApp</p><p>Leia mais</p>",
		Content:       "<p>Conteúdo completo que não deve ser preferido.</p>",
		ImageURL:      "https://cdn.example/image.jpg",
		AuthorName:    "Reporter Name",
		SourceURL:     "https://source.example",
		SourceIconURL: "https://source.example/logo.png",
		PublishedAt:   &published,
	}
}

func technologyArticle() model.Article {
	article := completeArticle()
	article.Source = "Tecnoblog"
	article.Category = "💻 Technology"
	article.CategoryEmoji = "💻"
	article.Title = "Novo chip promete notebooks mais rápidos"
	article.Link = "https://tech.example/chip"
	article.Description = "Fabricante apresentou um processador com foco em eficiência e desempenho."
	article.ImageURL = "https://cdn.example/chip.webp"
	article.AuthorName = ""
	article.SourceURL = "https://tech.example"
	article.SourceIconURL = ""
	return article
}

func withoutImageArticle() model.Article {
	article := completeArticle()
	article.ImageURL = ""
	article.SourceIconURL = ""
	article.AuthorName = ""
	return article
}

func longPortugueseArticle() model.Article {
	article := completeArticle()
	article.Title = "Relatório mostra avanço da tecnologia no Brasil"
	article.Description = strings.Repeat("O relatório destaca avanços importantes em pesquisa, conectividade e inovação. ", 10) + "Siga nosso canal no WhatsApp"
	article.AuthorName = "Equipe de Tecnologia"
	return article
}

func assertGoldenJSON(t *testing.T, name string, message discord.Message) {
	t.Helper()
	got, err := json.MarshalIndent(message, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", "golden", name+".json")
	var gotJSON any
	if err := json.Unmarshal(got, &gotJSON); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, append(normalizedBytes(gotJSON), '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var wantJSON any
	if err := json.Unmarshal(want, &wantJSON); err != nil {
		t.Fatal(err)
	}
	normalizedGot, _ := json.MarshalIndent(gotJSON, "", "  ")
	normalizedWant, _ := json.MarshalIndent(wantJSON, "", "  ")
	if string(normalizedGot) != string(normalizedWant) {
		t.Fatalf("golden mismatch for %s\ngot:\n%s\nwant:\n%s", name, normalizedGot, normalizedWant)
	}
}

func normalizedBytes(value any) []byte {
	normalized, _ := json.MarshalIndent(value, "", "  ")
	return normalized
}
