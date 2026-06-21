package app

import (
	"fmt"
	"strings"

	"github.com/isyuricunha/discord-news-rss-bot/internal/model"
	"github.com/isyuricunha/discord-news-rss-bot/internal/text"
)

func FormatMessage(article model.Article, maxPostLength int, maxContentLength int) string {
	if maxPostLength <= 0 || maxPostLength > text.DiscordContentLimit {
		maxPostLength = text.DiscordContentLimit
	}
	if maxContentLength <= 0 || maxContentLength > maxPostLength {
		maxContentLength = maxPostLength
	}

	title := text.NormalizeWhitespace(article.Title)
	if title == "" {
		title = "Untitled"
	}
	link := strings.TrimSpace(article.Link)
	source := text.NormalizeWhitespace(article.Source)
	if source == "" {
		source = "Source"
	}
	content := text.TruncateRunes(text.CleanHTML(article.Content), maxContentLength)

	full := buildBox(article.CategoryEmoji, title, link, source, content)
	if text.Length(full) <= maxPostLength {
		return full
	}

	empty := buildBox(article.CategoryEmoji, title, link, source, "")
	available := maxPostLength - text.Length(empty) - text.Length("\n├─────────────────────────────────┤\n│ ")
	if available > 20 {
		full = buildBox(article.CategoryEmoji, title, link, source, text.TruncateRunes(content, available))
		if text.Length(full) <= maxPostLength {
			return full
		}
	}

	minimal := fmt.Sprintf("%s **%s**\n\n🔗 %s\n📰 %s", article.CategoryEmoji, title, link, source)
	if text.Length(minimal) <= maxPostLength {
		return minimal
	}

	overhead := text.Length(fmt.Sprintf("%s ****\n\n🔗 %s\n📰 %s", article.CategoryEmoji, link, source))
	titleLimit := maxPostLength - overhead
	if titleLimit < 1 {
		titleLimit = 1
	}
	minimal = fmt.Sprintf("%s **%s**\n\n🔗 %s\n📰 %s", article.CategoryEmoji, text.TruncateRunes(title, titleLimit), link, source)
	if text.Length(minimal) <= maxPostLength {
		return minimal
	}
	return text.TruncateRunes(minimal, maxPostLength)
}

func buildBox(emoji, title, link, source, content string) string {
	if strings.TrimSpace(emoji) == "" {
		emoji = "📢"
	}
	var builder strings.Builder
	builder.WriteString("╭─────────────────────────────────╮\n")
	builder.WriteString("│  ")
	builder.WriteString(emoji)
	builder.WriteString(" **")
	builder.WriteString(title)
	builder.WriteString("**\n")
	builder.WriteString("├─────────────────────────────────┤\n")
	if link != "" {
		builder.WriteString("│ 🔗 ")
		builder.WriteString(link)
		builder.WriteString("\n")
	}
	builder.WriteString("│ 📰 ")
	builder.WriteString(source)
	builder.WriteString("\n")
	if strings.TrimSpace(content) != "" {
		builder.WriteString("├─────────────────────────────────┤\n")
		for _, line := range strings.Split(content, "\n") {
			builder.WriteString("│ ")
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}
	builder.WriteString("╰─────────────────────────────────╯")
	return builder.String()
}
