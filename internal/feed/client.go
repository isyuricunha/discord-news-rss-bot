package feed

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/isyuricunha/discord-news-rss-bot/internal/model"
)

const userAgent = "discord-rss-bot/3.0 (+https://github.com/isyuricunha/discord-news-rss-bot)"

var ErrNotModified = errors.New("feed not modified")

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
	FeedURL       string
	Source        string
	Category      string
	CategoryEmoji string
	Articles      []model.Article
	ETag          string
	LastModified  string
	NotModified   bool
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
	req.Header.Set("User-Agent", userAgent)
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
		FeedURL:       feedConfig.URL,
		Source:        feedConfig.Source,
		Category:      feedConfig.Category,
		CategoryEmoji: feedConfig.Emoji,
		ETag:          res.Header.Get("ETag"),
		LastModified:  res.Header.Get("Last-Modified"),
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
	parsed, err := c.parser.Parse(bytes.NewReader(body))
	if err != nil {
		return Result{}, fmt.Errorf("parse feed: %w", err)
	}
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
