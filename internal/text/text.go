package text

import (
	"html"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const DiscordContentLimit = 2000

var (
	blockTagPattern = regexp.MustCompile(`(?i)</?(p|div|br|li|ul|ol|section|article|header|footer|h[1-6]|blockquote|tr|table)[^>]*>`)
	tagPattern      = regexp.MustCompile(`(?s)<[^>]*>`)
	spacePattern    = regexp.MustCompile(`[ \t\r\f\v]+`)
	newlinePattern  = regexp.MustCompile(`\n{3,}`)
)

func CleanHTML(input string) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}
	text := blockTagPattern.ReplaceAllString(input, "\n")
	text = tagPattern.ReplaceAllString(text, " ")
	text = html.UnescapeString(text)
	text = strings.ReplaceAll(text, "\u00a0", " ")
	text = NormalizeWhitespace(text)
	return text
}

func NormalizeWhitespace(input string) string {
	input = strings.ReplaceAll(input, "\r\n", "\n")
	input = strings.ReplaceAll(input, "\r", "\n")
	lines := strings.Split(input, "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		line = spacePattern.ReplaceAllString(line, " ")
		line = strings.TrimFunc(line, unicode.IsSpace)
		if line != "" {
			normalized = append(normalized, line)
		}
	}
	return strings.TrimSpace(newlinePattern.ReplaceAllString(strings.Join(normalized, "\n"), "\n\n"))
}

func TruncateRunes(input string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(input) <= max {
		return input
	}
	if max <= 3 {
		return string([]rune(input)[:max])
	}
	runes := []rune(input)
	return string(runes[:max-3]) + "..."
}

func Length(input string) int {
	return utf8.RuneCountInString(input)
}

func SanitizeURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "[invalid-url]"
	}
	parsed.User = nil
	query := parsed.Query()
	for key := range query {
		if isSensitiveQueryKey(key) {
			query.Set(key, "REDACTED")
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func RedactSecret(input string) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}
	if parsed, err := url.Parse(input); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		parsed.User = nil
		pathParts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(pathParts) >= 3 && pathParts[0] == "api" && pathParts[1] == "webhooks" {
			pathParts[len(pathParts)-1] = "REDACTED"
			parsed.Path = "/" + strings.Join(pathParts, "/")
		}
		return parsed.String()
	}
	return "[REDACTED]"
}

func isSensitiveQueryKey(key string) bool {
	lower := strings.ToLower(key)
	if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "key") || strings.Contains(lower, "password") || strings.Contains(lower, "signature") {
		return true
	}
	return false
}
