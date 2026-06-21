package health

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/isyuricunha/discord-news-rss-bot/internal/storage"
)

type State struct {
	Version                        string    `json:"version"`
	Commit                         string    `json:"commit"`
	BuildDate                      string    `json:"build_date"`
	ProcessStart                   time.Time `json:"process_start"`
	LastCycleStart                 time.Time `json:"last_cycle_start,omitempty"`
	LastCycleComplete              time.Time `json:"last_cycle_complete,omitempty"`
	TotalConfiguredFeeds           int       `json:"total_configured_feeds"`
	FeedsAttempted                 int       `json:"feeds_attempted"`
	FeedsSuccessful                int       `json:"feeds_successful"`
	FeedsFailed                    int       `json:"feeds_failed"`
	PostsSent                      int       `json:"posts_sent"`
	DiscordPostingFailures         int       `json:"discord_posting_failures"`
	ConsecutiveTotalFeedFailCycles int       `json:"consecutive_total_feed_failure_cycles"`
	DatabaseStatus                 string    `json:"database_status"`
	ShutdownRequested              bool      `json:"shutdown_requested,omitempty"`
}

type Recorder struct {
	path  string
	state State
}

func NewRecorder(path string, initial State) *Recorder {
	return &Recorder{path: path, state: initial}
}

func (r *Recorder) State() State {
	return r.state
}

func (r *Recorder) Update(update func(*State)) error {
	update(&r.state)
	return WriteAtomic(r.path, r.state)
}

func WriteAtomic(path string, state State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".health-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func Check(ctx context.Context, healthFile string, dbFile string, maxAge time.Duration) error {
	data, err := os.ReadFile(healthFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("health file is missing: %s", healthFile)
		}
		return err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("health file is malformed: %w", err)
	}
	if state.LastCycleComplete.IsZero() {
		return errors.New("no completed cycle recorded")
	}
	if time.Since(state.LastCycleComplete) > maxAge {
		return fmt.Errorf("latest completed cycle is stale: last completed at %s", state.LastCycleComplete.Format(time.RFC3339))
	}
	if state.ConsecutiveTotalFeedFailCycles >= 3 {
		return fmt.Errorf("service has %d consecutive cycles where every feed failed", state.ConsecutiveTotalFeedFailCycles)
	}
	store, err := storage.OpenReadOnly(ctx, dbFile)
	if err != nil {
		return fmt.Errorf("database is unavailable: %w", err)
	}
	defer store.Close()
	if err := store.Ping(ctx); err != nil {
		return fmt.Errorf("database query failed: %w", err)
	}
	return nil
}
