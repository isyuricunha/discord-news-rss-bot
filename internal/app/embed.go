package app

import (
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/isyuricunha/discord-news-rss-bot/internal/discord"
	"github.com/isyuricunha/discord-news-rss-bot/internal/model"
	"github.com/isyuricunha/discord-news-rss-bot/internal/text"
)

const (
	colorNews       = 0x3498DB
	colorPolitics   = 0xE67E22
	colorTechnology = 0x8B5CF6
	colorSports     = 0x2ECC71
	colorBusiness   = 0xF1C40F
	colorDefault    = 0x5865F2
)

func BuildArticleMessage(article model.Article, maxPostLength int, maxContentLength int) (discord.Message, error) {
	if maxPostLength <= 0 || maxPostLength > discord.EmbedTextLimit {
		if maxPostLength > discord.EmbedTextLimit {
			maxPostLength = discord.EmbedTextLimit
		} else {
			maxPostLength = text.DiscordContentLimit
		}
	}
	if maxContentLength <= 0 || maxContentLength > discord.EmbedDescriptionLimit {
		maxContentLength = 800
	}

	source := text.SanitizeDiscordText(article.Source)
	if source == "" {
		source = "Source"
	}
	emoji := strings.TrimSpace(article.CategoryEmoji)
	if emoji == "" {
		emoji = "📢"
	}
	authorName := text.TruncateClean(strings.TrimSpace(emoji+" "+source), boundedComponentLimit(maxPostLength, 5, 32, discord.EmbedAuthorNameLimit))

	title := text.SanitizeDiscordText(text.NormalizeWhitespace(article.Title))
	if title == "" {
		title = "Untitled article"
	}
	title = text.TruncateClean(title, boundedComponentLimit(maxPostLength, 3, 64, discord.EmbedTitleLimit))

	category := categoryText(article.Category)
	footerText := footerText(article.AuthorName, category)
	footerText = text.TruncateClean(footerText, boundedComponentLimit(maxPostLength, 6, 32, discord.EmbedFooterTextLimit))

	descriptionLimit := maxContentLength
	if descriptionLimit > discord.EmbedDescriptionLimit {
		descriptionLimit = discord.EmbedDescriptionLimit
	}
	used := text.Length(authorName) + text.Length(title) + text.Length(footerText)
	if available := maxPostLength - used; available < descriptionLimit {
		descriptionLimit = available
	}
	if descriptionLimit < 0 {
		descriptionLimit = 0
	}
	description := text.CleanSummary(article.Title, article.Description, article.Content, descriptionLimit)

	embed := discord.Embed{
		Author: &discord.EmbedAuthor{
			Name: authorName,
		},
		Title: title,
		Color: categoryColor(article.Category),
		Footer: &discord.EmbedFooter{
			Text: footerText,
		},
	}
	if validHTTPURL(article.SourceURL) {
		embed.Author.URL = article.SourceURL
	}
	if validHTTPURL(article.SourceIconURL) {
		embed.Author.IconURL = article.SourceIconURL
	}
	if validHTTPURL(article.Link) {
		embed.URL = article.Link
	}
	if description != "" {
		embed.Description = description
	}
	if validHTTPURL(article.ImageURL) {
		embed.Image = &discord.EmbedImage{URL: article.ImageURL}
	}
	if article.PublishedAt != nil {
		embed.Timestamp = article.PublishedAt.UTC().Format(time.RFC3339)
	}

	message := discord.NewMessage(embed)
	if err := message.Validate(); err != nil {
		return discord.Message{}, err
	}
	if len(message.Embeds) != 1 {
		return discord.Message{}, errors.New("article message must contain exactly one embed")
	}
	return message, nil
}

func categoryColor(category string) int {
	lower := strings.ToLower(category)
	switch {
	case strings.Contains(lower, "technology"), strings.Contains(lower, "tech"), strings.Contains(lower, "tecnologia"):
		return colorTechnology
	case strings.Contains(lower, "politics"), strings.Contains(lower, "politica"), strings.Contains(lower, "conservative"):
		return colorPolitics
	case strings.Contains(lower, "sports"), strings.Contains(lower, "esportes"):
		return colorSports
	case strings.Contains(lower, "business"), strings.Contains(lower, "finance"), strings.Contains(lower, "economia"):
		return colorBusiness
	case strings.Contains(lower, "news"), strings.Contains(lower, "noticias"), strings.Contains(lower, "general"):
		return colorNews
	default:
		return colorDefault
	}
}

func categoryText(category string) string {
	category = strings.TrimSpace(category)
	if category == "" {
		return "RSS"
	}
	fields := strings.Fields(category)
	if len(fields) > 1 && !isASCIIAlphaNumStart(fields[0]) {
		return strings.Join(fields[1:], " ")
	}
	return category
}

func footerText(authorName string, category string) string {
	authorName = text.SanitizeDiscordText(text.NormalizeWhitespace(authorName))
	category = text.SanitizeDiscordText(text.NormalizeWhitespace(category))
	if category == "" {
		category = "RSS"
	}
	if cleanAuthor := cleanFooterAuthor(authorName); cleanAuthor != "" {
		return "By " + cleanAuthor + " • " + category
	}
	return category + " • RSS"
}

func cleanFooterAuthor(authorName string) string {
	authorName = strings.TrimSpace(authorName)
	if authorName == "" {
		return ""
	}
	lower := strings.ToLower(authorName)
	if strings.Contains(authorName, "@") && !strings.Contains(authorName, " ") {
		return ""
	}
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return ""
	}
	return authorName
}

func validHTTPURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func boundedComponentLimit(total int, divisor int, minimum int, maximum int) int {
	if total <= 0 {
		return minimum
	}
	limit := total / divisor
	if limit < minimum {
		limit = minimum
	}
	if limit > maximum {
		limit = maximum
	}
	return limit
}

func isASCIIAlphaNumStart(value string) bool {
	if value == "" {
		return false
	}
	first := value[0]
	return (first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || (first >= '0' && first <= '9')
}
