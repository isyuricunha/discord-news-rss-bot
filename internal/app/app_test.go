package app

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/isyuricunha/discord-news-rss-bot/internal/config"
	"github.com/isyuricunha/discord-news-rss-bot/internal/feed"
	"github.com/isyuricunha/discord-news-rss-bot/internal/model"
	"github.com/isyuricunha/discord-news-rss-bot/internal/storage"
)

type fakeFetcher struct {
	results []feed.Result
	calls   int
}

func (f *fakeFetcher) Fetch(ctx context.Context, cfg model.FeedConfig, state model.FeedState) (feed.Result, error) {
	if f.calls >= len(f.results) {
		return feed.Result{FeedURL: cfg.URL}, nil
	}
	result := f.results[f.calls]
	f.calls++
	return result, nil
}

type fakePoster struct {
	messages []string
	err      error
}

func (p *fakePoster) Post(ctx context.Context, message string) error {
	if p.err != nil {
		return p.err
	}
	p.messages = append(p.messages, message)
	return nil
}

func TestInitialSyncSkipSendsZeroAndRecordsSeen(t *testing.T) {
	service, store, poster := newTestApp(t, config.InitialSyncSkip, []feed.Result{{Articles: []model.Article{article("one", "https://example.com/one")}}}, nil)
	if err := service.RunCycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(poster.messages) != 0 {
		t.Fatalf("skip mode sent messages")
	}
	seen, err := store.CountStatus(context.Background(), "seen")
	if err != nil {
		t.Fatal(err)
	}
	if seen != 1 {
		t.Fatalf("expected one seen article, got %d", seen)
	}
}

func TestNewArticleAfterInitializationPostsExactlyOnce(t *testing.T) {
	first := article("one", "https://example.com/one")
	second := article("two", "https://example.com/two")
	fetcher := &fakeFetcher{results: []feed.Result{
		{Articles: []model.Article{first}},
		{Articles: []model.Article{first, second}},
		{Articles: []model.Article{first, second}},
	}}
	service, _, poster := newTestAppWithFetcher(t, config.InitialSyncSkip, fetcher, nil)
	if err := service.RunCycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := service.RunCycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := service.RunCycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(poster.messages) != 1 {
		t.Fatalf("expected one post, got %d", len(poster.messages))
	}
}

func TestInitialSyncLatestAndBackfill(t *testing.T) {
	articles := []model.Article{
		articleAt("old", "https://example.com/old", time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)),
		articleAt("new", "https://example.com/new", time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)),
	}
	t.Run("latest", func(t *testing.T) {
		service, _, poster := newTestApp(t, config.InitialSyncLatest, []feed.Result{{Articles: articles}}, nil)
		if err := service.RunCycle(context.Background()); err != nil {
			t.Fatal(err)
		}
		if len(poster.messages) != 1 || !contains(poster.messages[0], "new") {
			t.Fatalf("latest mode posted wrong messages %#v", poster.messages)
		}
	})
	t.Run("backfill", func(t *testing.T) {
		service, _, poster := newTestApp(t, config.InitialSyncBackfill, []feed.Result{{Articles: articles}}, func(cfg *config.Config) {
			cfg.InitialSyncMaxPosts = 1
		})
		if err := service.RunCycle(context.Background()); err != nil {
			t.Fatal(err)
		}
		if len(poster.messages) != 1 || !contains(poster.messages[0], "old") {
			t.Fatalf("backfill mode posted wrong messages %#v", poster.messages)
		}
	})
}

func TestFailedInitialPostIsNotMarkedPosted(t *testing.T) {
	service, store, _ := newTestApp(t, config.InitialSyncLatest, []feed.Result{{Articles: []model.Article{article("one", "https://example.com/one")}}}, func(cfg *config.Config) {})
	service.poster = &fakePoster{err: errors.New("discord failed")}
	if err := service.RunCycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	posted, err := store.CountStatus(context.Background(), "posted")
	if err != nil {
		t.Fatal(err)
	}
	if posted != 0 {
		t.Fatalf("failed post was marked posted")
	}
}

func TestGlobalCycleLimitAndRoundRobin(t *testing.T) {
	feeds := []model.FeedConfig{
		{URL: "https://example.com/a", Source: "A", Category: "News", Emoji: "📰"},
		{URL: "https://example.com/b", Source: "B", Category: "News", Emoji: "📰"},
	}
	cfg := baseConfig(t)
	cfg.Feeds = feeds
	cfg.InitialSyncMode = config.InitialSyncBackfill
	cfg.InitialSyncMaxPosts = 3
	cfg.MaxPostsPerCycle = 2
	store, err := storage.Open(context.Background(), cfg.DBFile)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	fetcher := &multiFeedFetcher{results: map[string]feed.Result{
		feeds[0].URL: {Articles: []model.Article{articleForFeed(feeds[0], "a1"), articleForFeed(feeds[0], "a2")}},
		feeds[1].URL: {Articles: []model.Article{articleForFeed(feeds[1], "b1"), articleForFeed(feeds[1], "b2")}},
	}}
	poster := &fakePoster{}
	service := New(Options{Config: cfg, Store: store, Fetcher: fetcher, Poster: poster, Logger: slog.Default(), Sleep: noSleep})
	if err := service.RunCycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(poster.messages) != 2 {
		t.Fatalf("expected global cycle limit of 2 posts, got %d", len(poster.messages))
	}
	if !contains(poster.messages[0], "a1") || !contains(poster.messages[1], "b1") {
		t.Fatalf("expected round-robin order, got %#v", poster.messages)
	}
}

type multiFeedFetcher struct {
	results map[string]feed.Result
}

func (m *multiFeedFetcher) Fetch(ctx context.Context, cfg model.FeedConfig, state model.FeedState) (feed.Result, error) {
	return m.results[cfg.URL], nil
}

func newTestApp(t *testing.T, mode config.InitialSyncMode, results []feed.Result, mutate func(*config.Config)) (*App, *storage.Store, *fakePoster) {
	t.Helper()
	return newTestAppWithFetcher(t, mode, &fakeFetcher{results: results}, mutate)
}

func newTestAppWithFetcher(t *testing.T, mode config.InitialSyncMode, fetcher *fakeFetcher, mutate func(*config.Config)) (*App, *storage.Store, *fakePoster) {
	t.Helper()
	cfg := baseConfig(t)
	cfg.InitialSyncMode = mode
	if mutate != nil {
		mutate(&cfg)
	}
	store, err := storage.Open(context.Background(), cfg.DBFile)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	poster := &fakePoster{}
	return New(Options{Config: cfg, Store: store, Fetcher: fetcher, Poster: poster, Logger: slog.Default(), Sleep: noSleep}), store, poster
}

func baseConfig(t *testing.T) config.Config {
	t.Helper()
	dir := t.TempDir()
	return config.Config{
		DataDir:             dir,
		DBFile:              filepath.Join(dir, "db.sqlite"),
		HealthFile:          filepath.Join(dir, "health.json"),
		Feeds:               []model.FeedConfig{{URL: "https://example.com/feed", Source: "Example", Category: "News", Emoji: "📰"}},
		CheckInterval:       time.Second,
		PostDelay:           0,
		CooldownDelay:       time.Millisecond,
		MaxPostLength:       1900,
		MaxContentLength:    800,
		FeedTimeout:         time.Second,
		InitialSyncMaxPosts: 1,
		MaxEntriesPerFeed:   20,
		MaxPostsPerCycle:    10,
		MaxConcurrentFeeds:  2,
		PostRetentionDays:   365,
		MaxFeedBytes:        1024 * 1024,
		DiscordMaxRetries:   1,
		LogLevel:            "info",
		LogFormat:           "text",
		HealthMaxAge:        time.Minute,
	}
}

func article(title string, link string) model.Article {
	return articleAt(title, link, time.Now().UTC())
}

func articleAt(title string, link string, published time.Time) model.Article {
	article := model.Article{FeedURL: "https://example.com/feed", Source: "Example", Category: "News", CategoryEmoji: "📰", Title: title, Link: link, Content: "<p>content</p>", PublishedAt: &published}
	model.PrepareArticleIdentity(&article)
	return article
}

func articleForFeed(feedConfig model.FeedConfig, title string) model.Article {
	article := model.Article{FeedURL: feedConfig.URL, Source: feedConfig.Source, Category: feedConfig.Category, CategoryEmoji: feedConfig.Emoji, Title: title, Link: feedConfig.URL + "/" + title, Content: title}
	model.PrepareArticleIdentity(&article)
	return article
}

func noSleep(context.Context, time.Duration) error {
	return nil
}

func contains(input, needle string) bool {
	return len(input) >= len(needle) && (input == needle || (len(needle) > 0 && find(input, needle)))
}

func find(input, needle string) bool {
	for i := 0; i+len(needle) <= len(input); i++ {
		if input[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
