package model

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"
)

func PrepareArticleIdentity(article *Article) {
	article.NormalizedLink = NormalizeArticleLink(article.Link)
	article.LegacyHash = LegacyHash(article.Title, article.Link)
	article.ArticleKey = ArticleKey(article)
}

func LegacyHash(title, link string) string {
	sum := sha256.Sum256([]byte(title + link))
	return hex.EncodeToString(sum[:])
}

func ArticleKey(article *Article) string {
	material := ""
	if guid := strings.TrimSpace(article.GUID); guid != "" {
		material = "guid:" + guid
	} else if article.NormalizedLink != "" {
		material = "link:" + article.NormalizedLink
	} else {
		published := ""
		if article.PublishedAt != nil {
			published = article.PublishedAt.UTC().Format(time.RFC3339Nano)
		}
		material = "fallback:" + strings.TrimSpace(article.Source) + "\x00" + strings.TrimSpace(article.Title) + "\x00" + published
	}

	scope := NormalizeFeedURL(article.FeedURL)
	sum := sha256.Sum256([]byte(scope + "\x00" + material))
	return hex.EncodeToString(sum[:])
}

func NormalizeFeedURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return strings.TrimSpace(raw)
	}
	normalizeURLParts(parsed, false)
	return parsed.String()
}

func NormalizeArticleLink(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.TrimSpace(raw)
	}
	normalizeURLParts(parsed, true)
	return parsed.String()
}

func normalizeURLParts(parsed *url.URL, removeTracking bool) {
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	hostname := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	if (parsed.Scheme == "http" && port == "80") || (parsed.Scheme == "https" && port == "443") {
		port = ""
	}
	if port != "" {
		parsed.Host = net.JoinHostPort(hostname, port)
	} else {
		parsed.Host = hostname
	}
	parsed.Fragment = ""
	if removeTracking {
		query := parsed.Query()
		for key := range query {
			if isTrackingParameter(key) {
				query.Del(key)
			}
		}
		parsed.RawQuery = encodeSortedQuery(query)
	}
	if parsed.Path == "" {
		parsed.Path = "/"
	}
}

func isTrackingParameter(key string) bool {
	lower := strings.ToLower(key)
	if strings.HasPrefix(lower, "utm_") {
		return true
	}
	switch lower {
	case "fbclid", "gclid", "dclid", "gbraid", "wbraid", "mc_cid", "mc_eid", "igshid":
		return true
	default:
		return false
	}
}

func encodeSortedQuery(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		vals := append([]string(nil), values[key]...)
		sort.Strings(vals)
		escapedKey := url.QueryEscape(key)
		for _, value := range vals {
			parts = append(parts, escapedKey+"="+url.QueryEscape(value))
		}
	}
	return strings.Join(parts, "&")
}
