package app

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/isyuricunha/discord-news-rss-bot/internal/config"
	"github.com/isyuricunha/discord-news-rss-bot/internal/discord"
	"github.com/isyuricunha/discord-news-rss-bot/internal/feed"
	"github.com/isyuricunha/discord-news-rss-bot/internal/health"
	"github.com/isyuricunha/discord-news-rss-bot/internal/model"
	"github.com/isyuricunha/discord-news-rss-bot/internal/storage"
	"github.com/isyuricunha/discord-news-rss-bot/internal/text"
	"github.com/isyuricunha/discord-news-rss-bot/internal/version"
)

type Poster interface {
	Post(context.Context, discord.Message) error
}

type Fetcher interface {
	Fetch(context.Context, model.FeedConfig, model.FeedState) (feed.Result, error)
}

type App struct {
	cfg     config.Config
	store   *storage.Store
	fetcher Fetcher
	poster  Poster
	health  *health.Recorder
	logger  *slog.Logger
	started time.Time
	sleep   func(context.Context, time.Duration) error
}

type Options struct {
	Config  config.Config
	Store   *storage.Store
	Fetcher Fetcher
	Poster  Poster
	Health  *health.Recorder
	Logger  *slog.Logger
	Sleep   func(context.Context, time.Duration) error
}

func New(options Options) *App {
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	sleep := options.Sleep
	if sleep == nil {
		sleep = sleepContext
	}
	started := time.Now().UTC()
	recorder := options.Health
	if recorder == nil {
		recorder = health.NewRecorder(options.Config.HealthFile, health.State{
			Version:              version.Version,
			Commit:               version.Commit,
			BuildDate:            version.Date,
			ProcessStart:         started,
			TotalConfiguredFeeds: len(options.Config.Feeds),
			DatabaseStatus:       "ok",
		})
	}
	return &App{
		cfg:     options.Config,
		store:   options.Store,
		fetcher: options.Fetcher,
		poster:  options.Poster,
		health:  recorder,
		logger:  logger,
		started: started,
		sleep:   sleep,
	}
}

func (a *App) Run(ctx context.Context) error {
	if err := a.health.Update(func(state *health.State) {
		state.DatabaseStatus = "ok"
		state.TotalConfiguredFeeds = len(a.cfg.Feeds)
	}); err != nil {
		a.logger.Warn("failed to write initial health state", "error", err)
	}
	defer a.health.Update(func(state *health.State) {
		state.ShutdownRequested = true
	})

	for {
		if err := a.RunCycle(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			a.logger.Error("cycle failed", "error", err)
		}
		if err := a.sleep(ctx, a.cfg.CheckInterval); err != nil {
			return nil
		}
	}
}

func (a *App) RunCycle(ctx context.Context) error {
	cycleStart := time.Now().UTC()
	if err := a.health.Update(func(state *health.State) {
		state.LastCycleStart = cycleStart
		state.TotalConfiguredFeeds = len(a.cfg.Feeds)
		state.DatabaseStatus = "ok"
	}); err != nil {
		a.logger.Warn("failed to write cycle health start", "error", err)
	}

	results := a.fetchFeeds(ctx, cycleStart)
	stats := model.CycleStats{}
	pendingByFeed := make([][]model.Article, len(a.cfg.Feeds))

	legacyCount, legacyCutoff, legacyErr := a.store.LegacyStats(ctx)
	if legacyErr != nil {
		return legacyErr
	}

	for _, result := range results {
		if result.skipped {
			continue
		}
		stats.FeedsAttempted++
		feedConfig := result.feed
		state := result.state
		if result.err != nil {
			failures := state.ConsecutiveFailures + 1
			nextAttempt := cycleStart.Add(failureBackoff(failures))
			if err := a.store.UpdateFeedFailure(ctx, feedConfig, failures, cycleStart, nextAttempt); err != nil {
				return err
			}
			stats.FeedsFailed++
			a.logger.Warn("feed fetch failed", "feed", text.SanitizeURL(feedConfig.URL), "category", feedConfig.Category, "error", result.err, "next_attempt", nextAttempt.Format(time.RFC3339))
			continue
		}
		if result.result.NotModified {
			if err := a.store.UpdateFeedNotModified(ctx, feedConfig, cycleStart); err != nil {
				return err
			}
			stats.FeedsSuccessful++
			continue
		}

		selected, seen, err := a.processArticles(ctx, feedConfig, state, result.result.Articles, legacyCount, legacyCutoff)
		if err != nil {
			return err
		}
		if err := a.store.CompleteSuccessfulFetch(ctx, feedConfig, state, true, result.result.ETag, result.result.LastModified, cycleStart, seen); err != nil {
			return err
		}
		pendingByFeed[result.index] = selected
		stats.FeedsSuccessful++
	}

	toPost := fairSelect(pendingByFeed, a.cfg.MaxPostsPerCycle)
	for _, article := range toPost {
		if ctx.Err() != nil {
			break
		}
		message, err := BuildArticleMessage(article, a.cfg.MaxPostLength, a.cfg.MaxContentLength)
		if err != nil {
			stats.DiscordFailures++
			a.logger.Error("Discord message build failed", "source", article.Source, "category", article.Category, "error", err)
			continue
		}
		if err := a.poster.Post(ctx, message); err != nil {
			stats.DiscordFailures++
			a.logger.Error("Discord post failed", "source", article.Source, "category", article.Category, "error", err)
			continue
		}
		if err := a.store.MarkPosted(ctx, article, time.Now().UTC()); err != nil {
			return err
		}
		stats.PostsSent++
		if a.cfg.PostDelay > 0 {
			if err := a.sleep(ctx, a.cfg.PostDelay); err != nil {
				break
			}
		}
	}

	if stats.FeedsAttempted > 0 && stats.FeedsFailed == stats.FeedsAttempted {
		stats.ConsecutiveTotalFails = a.health.State().ConsecutiveTotalFeedFailCycles + 1
	} else {
		stats.ConsecutiveTotalFails = 0
	}

	if deleted, ran, err := a.store.Cleanup(ctx, a.cfg.PostRetentionDays, time.Now().UTC()); err != nil {
		return err
	} else if ran && deleted > 0 {
		a.logger.Info("cleaned old article records", "deleted", deleted)
		_ = a.store.Optimize(ctx)
	}

	complete := time.Now().UTC()
	if err := a.health.Update(func(state *health.State) {
		state.LastCycleComplete = complete
		state.FeedsAttempted = stats.FeedsAttempted
		state.FeedsSuccessful = stats.FeedsSuccessful
		state.FeedsFailed = stats.FeedsFailed
		state.PostsSent = stats.PostsSent
		state.DiscordPostingFailures = stats.DiscordFailures
		state.ConsecutiveTotalFeedFailCycles = stats.ConsecutiveTotalFails
		state.DatabaseStatus = "ok"
	}); err != nil {
		a.logger.Warn("failed to write cycle health completion", "error", err)
	}

	a.logger.Info("cycle complete", "duration", time.Since(cycleStart).String(), "feeds_attempted", stats.FeedsAttempted, "feeds_successful", stats.FeedsSuccessful, "feeds_failed", stats.FeedsFailed, "posts_sent", stats.PostsSent, "discord_failures", stats.DiscordFailures)
	return nil
}

func (a *App) processArticles(ctx context.Context, feedConfig model.FeedConfig, state model.FeedState, articles []model.Article, legacyCount int, legacyCutoff *time.Time) ([]model.Article, []model.Article, error) {
	ordered := feed.OldestFirst(articles)
	if !state.Initialized {
		return a.initialArticles(ctx, ordered, legacyCount, legacyCutoff)
	}

	selected := make([]model.Article, 0, len(ordered))
	for _, article := range ordered {
		known, err := a.store.IsKnownOrAssociateLegacy(ctx, article)
		if err != nil {
			return nil, nil, err
		}
		if !known {
			selected = append(selected, article)
		}
	}
	_ = feedConfig
	return selected, nil, nil
}

func (a *App) initialArticles(ctx context.Context, ordered []model.Article, legacyCount int, legacyCutoff *time.Time) ([]model.Article, []model.Article, error) {
	candidates := make([]model.Article, 0, len(ordered))
	seen := make([]model.Article, 0, len(ordered))
	for _, article := range ordered {
		known, err := a.store.IsKnownOrAssociateLegacy(ctx, article)
		if err != nil {
			return nil, nil, err
		}
		if known {
			seen = append(seen, article)
			continue
		}
		if legacyCount > 0 {
			if article.PublishedAt == nil || (legacyCutoff != nil && !article.PublishedAt.After(*legacyCutoff)) {
				seen = append(seen, article)
				continue
			}
		}
		candidates = append(candidates, article)
	}

	switch a.cfg.InitialSyncMode {
	case config.InitialSyncSkip:
		seen = append(seen, candidates...)
		return nil, seen, nil
	case config.InitialSyncLatest:
		newest := feed.NewestFirst(candidates)
		if len(newest) == 0 {
			return nil, seen, nil
		}
		selected := newest[:1]
		for _, article := range candidates {
			if article.ArticleKey != selected[0].ArticleKey {
				seen = append(seen, article)
			}
		}
		return selected, seen, nil
	case config.InitialSyncBackfill:
		limit := a.cfg.InitialSyncMaxPosts
		if limit > len(candidates) {
			limit = len(candidates)
		}
		selected := append([]model.Article(nil), candidates[:limit]...)
		selectedSet := map[string]struct{}{}
		for _, article := range selected {
			selectedSet[article.ArticleKey] = struct{}{}
		}
		for _, article := range candidates {
			if _, ok := selectedSet[article.ArticleKey]; !ok {
				seen = append(seen, article)
			}
		}
		return selected, seen, nil
	default:
		seen = append(seen, candidates...)
		return nil, seen, nil
	}
}

type fetchResult struct {
	index   int
	feed    model.FeedConfig
	state   model.FeedState
	result  feed.Result
	err     error
	skipped bool
}

func (a *App) fetchFeeds(ctx context.Context, now time.Time) []fetchResult {
	results := make([]fetchResult, len(a.cfg.Feeds))
	jobs := make(chan int)
	var wg sync.WaitGroup
	workers := a.cfg.MaxConcurrentFeeds
	if workers > len(a.cfg.Feeds) {
		workers = len(a.cfg.Feeds)
	}
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				if ctx.Err() != nil {
					results[index] = fetchResult{index: index, feed: a.cfg.Feeds[index], skipped: true}
					continue
				}
				feedConfig := a.cfg.Feeds[index]
				state, _, err := a.store.GetFeedState(ctx, feedConfig)
				if err != nil {
					results[index] = fetchResult{index: index, feed: feedConfig, err: err}
					continue
				}
				if state.NextAttemptAt != nil && state.NextAttemptAt.After(now) {
					results[index] = fetchResult{index: index, feed: feedConfig, state: state, skipped: true}
					continue
				}
				fetched, err := a.fetcher.Fetch(ctx, feedConfig, state)
				results[index] = fetchResult{index: index, feed: feedConfig, state: state, result: fetched, err: err}
			}
		}()
	}
	for index := range a.cfg.Feeds {
		if ctx.Err() != nil {
			break
		}
		jobs <- index
	}
	close(jobs)
	wg.Wait()
	return results
}

func fairSelect(queues [][]model.Article, limit int) []model.Article {
	selected := make([]model.Article, 0, limit)
	for len(selected) < limit {
		added := false
		for i := range queues {
			if len(queues[i]) == 0 {
				continue
			}
			selected = append(selected, queues[i][0])
			queues[i] = queues[i][1:]
			added = true
			if len(selected) >= limit {
				break
			}
		}
		if !added {
			break
		}
	}
	return selected
}

func failureBackoff(failures int) time.Duration {
	if failures < 1 {
		failures = 1
	}
	base := time.Minute
	delay := base * time.Duration(math.Pow(2, float64(failures-1)))
	if delay > 6*time.Hour {
		delay = 6 * time.Hour
	}
	jitter := time.Duration(time.Now().UnixNano() % int64(delay/10+time.Millisecond))
	return delay + jitter
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
