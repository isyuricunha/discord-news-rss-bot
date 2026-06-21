package feed

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mmcdole/gofeed"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"

	"github.com/isyuricunha/discord-news-rss-bot/internal/model"
	"github.com/isyuricunha/discord-news-rss-bot/internal/version"
)

const acceptHeader = "application/rss+xml, application/atom+xml, application/xml, text/xml, */*;q=0.1"

var ErrNotModified = errors.New("feed not modified")

var xmlEncodingPattern = regexp.MustCompile(`(?i)<\?xml[^>]*encoding=["']([^"']+)`)

type Client struct {
	httpClient *http.Client
	parser     *gofeed.Parser
	maxBytes   int64
	maxEntries int
}

type Options struct {
	Timeout    time.Duration
	MaxBytes   int64
	MaxEntries int
	Transport  http.RoundTripper
}

type Result struct {
	FeedURL         string
	FinalURL        string
	Source          string
	Category        string
	CategoryEmoji   string
	Articles        []model.Article
	ETag            string
	LastModified    string
	ContentType     string
	ContentEncoding string
	Encoding        string
	FeedType        string
	NotModified     bool
}

func New(options Options) *Client {
	transport := options.Transport
	if transport == nil {
		transport = &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		}
	}
	maxBytes := options.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 10_485_760
	}
	maxEntries := options.MaxEntries
	if maxEntries <= 0 {
		maxEntries = 20
	}
	return &Client{
		httpClient: &http.Client{
			Timeout:   options.Timeout,
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return errors.New("stopped after 5 redirects")
				}
				return nil
			},
		},
		parser:     gofeed.NewParser(),
		maxBytes:   maxBytes,
		maxEntries: maxEntries,
	}
}

func (c *Client) Fetch(ctx context.Context, feedConfig model.FeedConfig, state model.FeedState) (Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedConfig.URL, nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("User-Agent", userAgent())
	req.Header.Set("Accept", acceptHeader)
	if state.ETag != "" {
		req.Header.Set("If-None-Match", state.ETag)
	}
	if state.LastModified != "" {
		req.Header.Set("If-Modified-Since", state.LastModified)
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		return Result{}, err
	}
	defer res.Body.Close()

	result := Result{
		FeedURL:         feedConfig.URL,
		FinalURL:        res.Request.URL.String(),
		Source:          feedConfig.Source,
		Category:        feedConfig.Category,
		CategoryEmoji:   feedConfig.Emoji,
		ETag:            res.Header.Get("ETag"),
		LastModified:    res.Header.Get("Last-Modified"),
		ContentType:     res.Header.Get("Content-Type"),
		ContentEncoding: res.Header.Get("Content-Encoding"),
	}
	if res.StatusCode == http.StatusNotModified {
		result.NotModified = true
		return result, nil
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return Result{}, fmt.Errorf("feed returned HTTP status %d", res.StatusCode)
	}

	body, err := readLimited(res.Body, c.maxBytes)
	if err != nil {
		return Result{}, err
	}
	parsed, encoding, err := c.parse(body, result.ContentType)
	if err != nil {
		return Result{}, err
	}
	result.Encoding = encoding
	result.FeedType = parsed.FeedType
	limit := c.maxEntries
	if len(parsed.Items) < limit {
		limit = len(parsed.Items)
	}
	for i, item := range parsed.Items[:limit] {
		article := model.Article{
			FeedURL:       feedConfig.URL,
			Source:        feedConfig.Source,
			Category:      feedConfig.Category,
			CategoryEmoji: feedConfig.Emoji,
			GUID:          item.GUID,
			Link:          item.Link,
			Title:         item.Title,
			Content:       item.Content,
			Sequence:      i,
		}
		if article.Content == "" {
			article.Content = item.Description
		}
		if item.PublishedParsed != nil {
			published := item.PublishedParsed.UTC()
			article.PublishedAt = &published
		} else if item.UpdatedParsed != nil {
			updated := item.UpdatedParsed.UTC()
			article.PublishedAt = &updated
		}
		model.PrepareArticleIdentity(&article)
		result.Articles = append(result.Articles, article)
	}
	return result, nil
}

func (c *Client) parse(body []byte, contentType string) (*gofeed.Feed, string, error) {
	if isHTMLDocument(body) {
		return nil, "", errors.New("feed endpoint returned HTML instead of RSS or Atom")
	}

	encoding := declaredEncoding(body, contentType)
	parsed, err := c.parser.Parse(bytes.NewReader(body))
	if err == nil {
		return parsed, encoding, nil
	}
	if !shouldRetryDecoded(body, contentType, err) {
		return nil, encoding, classifyParseError(err)
	}

	decoded, decodedEncoding, decodeErr := decodeToUTF8(body, contentType, c.maxBytes)
	if decodeErr != nil {
		return nil, decodedEncoding, decodeErr
	}
	if isHTMLDocument(decoded) {
		return nil, decodedEncoding, errors.New("feed endpoint returned HTML instead of RSS or Atom")
	}
	parsed, err = c.parser.Parse(bytes.NewReader(decoded))
	if err != nil {
		return nil, decodedEncoding, fmt.Errorf("decode feed charset %q: %w", decodedEncoding, classifyParseError(err))
	}
	return parsed, decodedEncoding, nil
}

func readLimited(reader io.Reader, maxBytes int64) ([]byte, error) {
	limited := io.LimitReader(reader, maxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("feed response exceeds MAX_FEED_BYTES (%d)", maxBytes)
	}
	return body, nil
}

func readDecodedLimited(reader io.Reader, rawLimit int64) ([]byte, error) {
	limit := rawLimit * 4
	if rawLimit > 1<<60 {
		limit = 1 << 62
	}
	limited := io.LimitReader(reader, limit+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("decoded feed response exceeds safe size limit (%d)", limit)
	}
	return body, nil
}

func decodeToUTF8(body []byte, contentType string, rawLimit int64) ([]byte, string, error) {
	label := declaredEncoding(body, contentType)
	if !utf8.Valid(body) && (label == "" || isUTF8Encoding(label)) {
		decoded, err := readDecodedLimited(transform.NewReader(bytes.NewReader(body), charmap.Windows1252.NewDecoder()), rawLimit)
		if err != nil {
			return nil, "windows-1252", fmt.Errorf("decode feed charset %q: %w", "windows-1252", err)
		}
		return decoded, "windows-1252", nil
	}
	if label != "" && !isUTF8Encoding(label) {
		if encoding, _ := charset.Lookup(label); encoding == nil {
			return nil, label, fmt.Errorf("unsupported feed charset %q", label)
		}
	}

	effectiveContentType := contentType
	if label != "" {
		effectiveContentType = contentTypeWithCharset(contentType, label)
	}
	reader, err := charset.NewReader(bytes.NewReader(body), effectiveContentType)
	if err != nil {
		if label == "" {
			label = "unknown"
		}
		return nil, label, fmt.Errorf("unsupported feed charset %q: %w", label, err)
	}
	decoded, err := readDecodedLimited(reader, rawLimit)
	if err != nil {
		if label == "" {
			label = "detected"
		}
		return nil, label, fmt.Errorf("decode feed charset %q: %w", label, err)
	}
	if label == "" {
		label = "detected"
	}
	return decoded, label, nil
}

func shouldRetryDecoded(body []byte, contentType string, err error) bool {
	if hasUTF8BOM(body) || !utf8.Valid(body) {
		return true
	}
	encoding := declaredEncoding(body, contentType)
	if encoding != "" && !isUTF8Encoding(encoding) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "utf-8") ||
		strings.Contains(message, "utf8") ||
		strings.Contains(message, "encoding")
}

func classifyParseError(err error) error {
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "failed to detect feed type"):
		return errors.New("valid XML is not RSS or Atom")
	case strings.Contains(message, "xml syntax error"):
		return fmt.Errorf("malformed XML: %w", err)
	default:
		return fmt.Errorf("parse feed: %w", err)
	}
}

func isHTMLDocument(body []byte) bool {
	body = bytes.TrimPrefix(body, []byte{0xEF, 0xBB, 0xBF})
	prefix := strings.ToLower(string(bytes.TrimSpace(body[:min(len(body), 128)])))
	return strings.HasPrefix(prefix, "<!doctype html") || strings.HasPrefix(prefix, "<html")
}

func declaredEncoding(body []byte, contentType string) string {
	if _, params, err := mime.ParseMediaType(contentType); err == nil {
		if value := strings.TrimSpace(params["charset"]); value != "" {
			return strings.ToLower(value)
		}
	}
	if hasUTF8BOM(body) {
		return "utf-8"
	}
	if match := xmlEncodingPattern.FindSubmatch(body[:min(len(body), 512)]); len(match) == 2 {
		return strings.ToLower(strings.TrimSpace(string(match[1])))
	}
	return ""
}

func contentTypeWithCharset(contentType, encoding string) string {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType == "" {
		mediaType = "application/xml"
		params = map[string]string{}
	}
	params["charset"] = encoding
	return mime.FormatMediaType(mediaType, params)
}

func isUTF8Encoding(encoding string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(encoding), "_", "-"))
	return normalized == "utf-8" || normalized == "utf8"
}

func hasUTF8BOM(body []byte) bool {
	return bytes.HasPrefix(body, []byte{0xEF, 0xBB, 0xBF})
}

func userAgent() string {
	value := strings.TrimSpace(version.Version)
	if value == "" {
		value = "dev"
	}
	return fmt.Sprintf("discord-rss-bot/%s (+https://github.com/isyuricunha/discord-news-rss-bot)", value)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
