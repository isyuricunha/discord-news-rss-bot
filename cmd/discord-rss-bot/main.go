package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/isyuricunha/discord-news-rss-bot/internal/app"
	"github.com/isyuricunha/discord-news-rss-bot/internal/config"
	"github.com/isyuricunha/discord-news-rss-bot/internal/discord"
	"github.com/isyuricunha/discord-news-rss-bot/internal/feed"
	"github.com/isyuricunha/discord-news-rss-bot/internal/health"
	"github.com/isyuricunha/discord-news-rss-bot/internal/storage"
	"github.com/isyuricunha/discord-news-rss-bot/internal/version"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	command := "run"
	if len(args) > 0 {
		command = strings.ToLower(args[0])
	}

	switch command {
	case "run":
		return runService()
	case "healthcheck":
		return runHealthcheck()
	case "validate-config":
		return runValidateConfig()
	case "version":
		fmt.Println(version.String())
		return nil
	case "-h", "--help", "help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

func runService() error {
	cfg, err := config.Load(config.LoadOptions{RequireWebhook: true})
	if err != nil {
		return err
	}
	logger, err := configureLogger(cfg)
	if err != nil {
		return err
	}
	for _, name := range cfg.Deprecated {
		logger.Warn("deprecated setting accepted but ignored by v3 logging/runtime", "setting", name)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := storage.Open(ctx, cfg.DBFile)
	if err != nil {
		return err
	}
	defer store.Close()

	feedClient := feed.New(feed.Options{
		Timeout:    cfg.FeedTimeout,
		MaxBytes:   cfg.MaxFeedBytes,
		MaxEntries: cfg.MaxEntriesPerFeed,
	})
	discordClient := discord.New(discord.Options{
		WebhookURL:       cfg.DiscordWebhookURL,
		HTTPClient:       &http.Client{Timeout: 15 * time.Second},
		MaxRetries:       cfg.DiscordMaxRetries,
		CooldownFallback: cfg.CooldownDelay,
		Logger:           logger,
	})
	recorder := health.NewRecorder(cfg.HealthFile, health.State{
		Version:              version.Version,
		Commit:               version.Commit,
		BuildDate:            version.Date,
		ProcessStart:         time.Now().UTC(),
		TotalConfiguredFeeds: len(cfg.Feeds),
		DatabaseStatus:       "ok",
	})

	service := app.New(app.Options{
		Config:  cfg,
		Store:   store,
		Fetcher: feedClient,
		Poster:  discordClient,
		Health:  recorder,
		Logger:  logger,
	})
	logger.Info("Discord RSS bot started", "version", version.Version, "go", runtime.Version(), "feeds", len(cfg.Feeds), "database", cfg.DBFile, "initial_sync_mode", cfg.InitialSyncMode)
	err = service.Run(ctx)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func runHealthcheck() error {
	cfg, err := config.Load(config.LoadOptions{RequireWebhook: false})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := health.Check(ctx, cfg.HealthFile, cfg.DBFile, cfg.HealthMaxAge); err != nil {
		return err
	}
	fmt.Println("healthy")
	return nil
}

func runValidateConfig() error {
	cfg, err := config.Load(config.LoadOptions{RequireWebhook: true})
	if err != nil {
		return err
	}
	fmt.Printf("configuration valid: %d feed(s), data dir %s\n", len(cfg.Feeds), cfg.DataDir)
	if len(cfg.Deprecated) > 0 {
		fmt.Printf("deprecated settings accepted but ignored: %s\n", strings.Join(cfg.Deprecated, ", "))
	}
	return nil
}

func configureLogger(cfg config.Config) (*slog.Logger, error) {
	level := new(slog.LevelVar)
	switch cfg.LogLevel {
	case "debug":
		level.Set(slog.LevelDebug)
	case "info":
		level.Set(slog.LevelInfo)
	case "warn":
		level.Set(slog.LevelWarn)
	case "error":
		level.Set(slog.LevelError)
	default:
		return nil, fmt.Errorf("unsupported LOG_LEVEL %q", cfg.LogLevel)
	}

	options := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if cfg.LogFormat == "json" {
		handler = slog.NewJSONHandler(os.Stdout, options)
	} else {
		handler = slog.NewTextHandler(os.Stdout, options)
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger, nil
}

func printUsage() {
	fmt.Println(`Usage: discord-rss-bot [command]

Commands:
  run              start the feed polling service (default)
  healthcheck      validate persisted health state and database availability
  validate-config  validate configuration without starting the service
  version          print version metadata`)
}
