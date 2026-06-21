package text

import (
	"regexp"
	"strings"
	"unicode"
)

const DiscordEmbedTextLimit = 6000

var (
	maskedLinkPattern = regexp.MustCompile(`\[([^\]\n]+)\]\([^)]+\)`)
	mentionPattern    = regexp.MustCompile(`<(@!?|@&|#)([^>]+)>`)
	onlyURLPattern    = regexp.MustCompile(`(?i)^https?://\S+$`)
	whatsAppCTASuffix = regexp.MustCompile(`(?i)\s*(clique aqui para seguir[^.!?\n]*whatsapp|siga nosso canal[^.!?\n]*whatsapp).*$`)
)

func CleanSummary(title string, description string, content string, maxLength int) string {
	if maxLength <= 0 {
		return ""
	}
	if maxLength > DiscordEmbedTextLimit {
		maxLength = DiscordEmbedTextLimit
	}

	cleanedTitle := NormalizeComparable(title)
	candidates := []string{description, content}
	for _, candidate := range candidates {
		summary := cleanupSummaryText(candidate, cleanedTitle)
		if !usefulSummary(summary, cleanedTitle) {
			continue
		}
		return TruncateClean(summary, maxLength)
	}
	return ""
}

func SanitizeDiscordText(input string) string {
	if strings.TrimSpace(input) == "" {
		return ""
	}
	output := maskedLinkPattern.ReplaceAllString(input, "$1")
	output = mentionPattern.ReplaceAllString(output, "‹$1$2›")
	output = strings.ReplaceAll(output, "@everyone", "@\u200beveryone")
	output = strings.ReplaceAll(output, "@here", "@\u200bhere")
	output = strings.ReplaceAll(output, "```", "")
	output = strings.ReplaceAll(output, "`", "")
	output = strings.ReplaceAll(output, "||", "")
	for _, marker := range []string{"**", "__", "~~"} {
		output = strings.ReplaceAll(output, marker, "")
	}
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimLeft(trimmed, "#")
		trimmed = strings.TrimLeft(trimmed, ">")
		lines[i] = strings.TrimSpace(trimmed)
	}
	return NormalizeWhitespace(strings.Join(lines, "\n"))
}

func TruncateClean(input string, maxLength int) string {
	if maxLength <= 0 {
		return ""
	}
	if Length(input) <= maxLength {
		return input
	}
	if maxLength <= 1 {
		return "…"
	}
	limit := maxLength - 1
	runes := []rune(input)
	cut := limit
	if cut > len(runes) {
		cut = len(runes)
	}
	prefix := string(runes[:cut])
	if sentence := lastSentenceBoundary(prefix); sentence > 30 {
		prefix = string([]rune(prefix)[:sentence])
	} else if word := lastWordBoundary(prefix); word > 20 {
		prefix = string([]rune(prefix)[:word])
	}
	prefix = strings.TrimRightFunc(prefix, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(",;:.-–—", r)
	})
	if prefix == "" {
		prefix = string(runes[:limit])
	}
	return prefix + "…"
}

func NormalizeComparable(input string) string {
	input = strings.ToLower(NormalizeWhitespace(CleanHTML(input)))
	return strings.TrimSpace(input)
}

func cleanupSummaryText(input string, comparableTitle string) string {
	input = CleanHTML(input)
	input = SanitizeDiscordText(input)
	lines := strings.Split(input, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimSpace(whatsAppCTASuffix.ReplaceAllString(line, ""))
		if line == "" || isBoilerplateLine(line) {
			continue
		}
		if comparableTitle != "" && NormalizeComparable(line) == comparableTitle {
			continue
		}
		if len(cleaned) > 0 && NormalizeComparable(cleaned[len(cleaned)-1]) == NormalizeComparable(line) {
			continue
		}
		cleaned = append(cleaned, dedupeAdjacentSentences(line))
	}
	return NormalizeWhitespace(strings.Join(cleaned, "\n"))
}

func dedupeAdjacentSentences(line string) string {
	parts := strings.SplitAfter(line, ".")
	if len(parts) < 2 {
		return line
	}
	deduped := make([]string, 0, len(parts))
	previous := ""
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		current := NormalizeComparable(part)
		if current == previous {
			continue
		}
		deduped = append(deduped, part)
		previous = current
	}
	if len(deduped) == 0 {
		return line
	}
	return strings.Join(deduped, " ")
}

func usefulSummary(summary string, comparableTitle string) bool {
	summary = strings.TrimSpace(summary)
	if summary == "" || onlyURLPattern.MatchString(summary) || onlyURLPattern.MatchString(strings.ReplaceAll(summary, " ", "")) {
		return false
	}
	if comparableTitle != "" && NormalizeComparable(summary) == comparableTitle {
		return false
	}
	letters := 0
	for _, r := range summary {
		if unicode.IsLetter(r) {
			letters++
		}
	}
	return letters >= 3
}

func isBoilerplateLine(line string) bool {
	normalized := NormalizeComparable(line)
	if normalized == "" {
		return true
	}
	switch normalized {
	case "leia mais", "continue lendo", "reprodução", "reproducao":
		return true
	}
	if strings.HasPrefix(normalized, "reprod") && len([]rune(normalized)) <= 12 {
		return true
	}
	if strings.Contains(normalized, "whatsapp") &&
		(strings.Contains(normalized, "clique aqui") || strings.Contains(normalized, "siga nosso canal") || strings.Contains(normalized, "seguir o canal")) {
		return true
	}
	return false
}

func lastSentenceBoundary(input string) int {
	runes := []rune(input)
	for i := len(runes) - 1; i >= 0; i-- {
		if strings.ContainsRune(".!?", runes[i]) {
			return i + 1
		}
	}
	return -1
}

func lastWordBoundary(input string) int {
	runes := []rune(input)
	for i := len(runes) - 1; i >= 0; i-- {
		if unicode.IsSpace(runes[i]) {
			return i
		}
	}
	return -1
}
