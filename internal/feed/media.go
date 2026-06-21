package feed

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/mmcdole/gofeed"
	ext "github.com/mmcdole/gofeed/extensions"
	"golang.org/x/net/html"
)

func articleAuthor(item *gofeed.Item) string {
	if item == nil {
		return ""
	}
	for _, author := range item.Authors {
		if author == nil {
			continue
		}
		if name := cleanAuthorName(author.Name); name != "" {
			return name
		}
	}
	if item.Author != nil {
		if name := cleanAuthorName(item.Author.Name); name != "" {
			return name
		}
	}
	if item.DublinCoreExt != nil {
		for _, name := range append(item.DublinCoreExt.Creator, item.DublinCoreExt.Author...) {
			if cleaned := cleanAuthorName(name); cleaned != "" {
				return cleaned
			}
		}
	}
	return ""
}

func cleanAuthorName(name string) string {
	name = strings.Join(strings.Fields(strings.TrimSpace(name)), " ")
	if name == "" {
		return ""
	}
	if strings.Contains(name, "@") && !strings.Contains(name, " ") {
		return ""
	}
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return ""
	}
	switch lower {
	case "admin", "redacao", "redação", "rss", "feed", "wordpress":
		return ""
	default:
		return name
	}
}

func sourceURL(feed *gofeed.Feed, articleLink string, finalFeedURL string) string {
	if feed != nil {
		if validHTTPURL(feed.Link) {
			return feed.Link
		}
		for _, link := range feed.Links {
			if validHTTPURL(link) {
				return link
			}
		}
	}
	if origin := originURL(articleLink); origin != "" {
		return origin
	}
	return originURL(finalFeedURL)
}

func sourceIconURL(feed *gofeed.Feed, baseURLs ...string) string {
	if feed == nil || feed.Image == nil {
		return ""
	}
	return resolveImageURL(feed.Image.URL, baseURLs...)
}

func articleImageURL(item *gofeed.Item, baseURLs ...string) string {
	if item == nil {
		return ""
	}
	if item.Image != nil {
		if imageURL := resolveImageURL(item.Image.URL, baseURLs...); imageURL != "" {
			return imageURL
		}
	}
	for _, enclosure := range item.Enclosures {
		if enclosure == nil {
			continue
		}
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(enclosure.Type)), "image/") {
			if imageURL := resolveImageURL(enclosure.URL, baseURLs...); imageURL != "" {
				return imageURL
			}
		}
	}
	if imageURL := mediaExtensionImageURL(item.Extensions, baseURLs...); imageURL != "" {
		return imageURL
	}
	if imageURL := htmlImageURL(item.Description, baseURLs...); imageURL != "" {
		return imageURL
	}
	return htmlImageURL(item.Content, baseURLs...)
}

func mediaExtensionImageURL(extensions ext.Extensions, baseURLs ...string) string {
	for namespace, values := range extensions {
		if !strings.EqualFold(namespace, "media") && !strings.EqualFold(namespace, "mediaGroup") {
			continue
		}
		if imageURL := extensionImageURL(values, "content", true, baseURLs...); imageURL != "" {
			return imageURL
		}
		if imageURL := extensionImageURL(values, "image", false, baseURLs...); imageURL != "" {
			return imageURL
		}
		if imageURL := extensionImageURL(values, "thumbnail", false, baseURLs...); imageURL != "" {
			return imageURL
		}
	}
	return ""
}

func extensionImageURL(values map[string][]ext.Extension, name string, requireImageType bool, baseURLs ...string) string {
	for key, entries := range values {
		if !strings.EqualFold(key, name) {
			continue
		}
		for _, entry := range entries {
			if isTrackingPixel(entry.Attrs) {
				continue
			}
			if requireImageType && !extensionLooksLikeImage(entry) {
				continue
			}
			candidates := []string{entry.Attrs["url"], entry.Attrs["href"], entry.Value}
			for _, candidate := range candidates {
				if imageURL := resolveImageURL(candidate, baseURLs...); imageURL != "" {
					return imageURL
				}
			}
		}
	}
	return ""
}

func extensionLooksLikeImage(entry ext.Extension) bool {
	medium := strings.ToLower(strings.TrimSpace(entry.Attrs["medium"]))
	contentType := strings.ToLower(strings.TrimSpace(entry.Attrs["type"]))
	if medium == "image" || strings.HasPrefix(contentType, "image/") {
		return true
	}
	if medium == "" && contentType == "" {
		return looksLikeImagePath(entry.Attrs["url"]) || looksLikeImagePath(entry.Value)
	}
	return false
}

func htmlImageURL(input string, baseURLs ...string) string {
	tokenizer := html.NewTokenizer(strings.NewReader(input))
	for {
		tokenType := tokenizer.Next()
		switch tokenType {
		case html.ErrorToken:
			return ""
		case html.SelfClosingTagToken, html.StartTagToken:
			token := tokenizer.Token()
			if !strings.EqualFold(token.Data, "img") {
				continue
			}
			attrs := map[string]string{}
			for _, attr := range token.Attr {
				attrs[strings.ToLower(attr.Key)] = attr.Val
			}
			if isTrackingPixel(attrs) {
				continue
			}
			for _, key := range []string{"src", "data-src"} {
				if imageURL := resolveImageURL(attrs[key], baseURLs...); imageURL != "" {
					return imageURL
				}
			}
		}
	}
}

func resolveImageURL(raw string, baseURLs ...string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(strings.ToLower(raw), "data:") {
		return ""
	}
	if strings.Contains(strings.ToLower(raw), "base64,") {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if parsed.IsAbs() {
		if validImageURL(parsed.String()) {
			return parsed.String()
		}
		return ""
	}
	for _, base := range baseURLs {
		baseParsed, err := url.Parse(strings.TrimSpace(base))
		if err != nil || baseParsed.Scheme == "" || baseParsed.Host == "" {
			continue
		}
		resolved := baseParsed.ResolveReference(parsed)
		if validImageURL(resolved.String()) {
			return resolved.String()
		}
	}
	return ""
}

func validImageURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	path := strings.ToLower(parsed.Path)
	if strings.Contains(path, "spacer") || strings.Contains(path, "transparent") || strings.Contains(path, "tracking") || strings.Contains(path, "pixel") {
		return false
	}
	return true
}

func validHTTPURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func originURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func isTrackingPixel(attrs map[string]string) bool {
	width, widthOK := pixelDimension(attrs["width"])
	height, heightOK := pixelDimension(attrs["height"])
	return widthOK && heightOK && width <= 1 && height <= 1
}

func pixelDimension(value string) (int, bool) {
	value = strings.TrimSpace(strings.TrimSuffix(value, "px"))
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(value)
	return parsed, err == nil
}

func looksLikeImagePath(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	path := strings.ToLower(parsed.Path)
	for _, suffix := range []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".avif"} {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}
