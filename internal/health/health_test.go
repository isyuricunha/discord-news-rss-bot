package health

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isyuricunha/discord-news-rss-bot/internal/storage"
)

func TestHealthyRecentState(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.sqlite")
	store, err := storage.Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	store.Close()
	healthPath := filepath.Join(dir, "health.json")
	if err := WriteAtomic(healthPath, State{ProcessStart: time.Now().UTC(), LastCycleComplete: time.Now().UTC(), DatabaseStatus: "ok"}); err != nil {
		t.Fatal(err)
	}
	if err := Check(ctx, healthPath, dbPath, time.Minute); err != nil {
		t.Fatal(err)
	}
}

func TestHealthFailures(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.sqlite")
	store, err := storage.Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	store.Close()

	t.Run("missing", func(t *testing.T) {
		err := Check(ctx, filepath.Join(dir, "missing.json"), dbPath, time.Minute)
		if err == nil || !strings.Contains(err.Error(), "missing") {
			t.Fatalf("expected missing health error, got %v", err)
		}
	})
	t.Run("malformed", func(t *testing.T) {
		path := filepath.Join(dir, "bad.json")
		os.WriteFile(path, []byte("{"), 0o644)
		err := Check(ctx, path, dbPath, time.Minute)
		if err == nil || !strings.Contains(err.Error(), "malformed") {
			t.Fatalf("expected malformed error, got %v", err)
		}
	})
	t.Run("stale", func(t *testing.T) {
		path := filepath.Join(dir, "stale.json")
		WriteAtomic(path, State{LastCycleComplete: time.Now().UTC().Add(-2 * time.Hour)})
		err := Check(ctx, path, dbPath, time.Minute)
		if err == nil || !strings.Contains(err.Error(), "stale") {
			t.Fatalf("expected stale error, got %v", err)
		}
	})
	t.Run("database", func(t *testing.T) {
		path := filepath.Join(dir, "dbfail.json")
		WriteAtomic(path, State{LastCycleComplete: time.Now().UTC()})
		err := Check(ctx, path, filepath.Join(dir, "missing.db"), time.Minute)
		if err == nil || !strings.Contains(err.Error(), "database") {
			t.Fatalf("expected database error, got %v", err)
		}
	})
	t.Run("total failures", func(t *testing.T) {
		path := filepath.Join(dir, "failures.json")
		WriteAtomic(path, State{LastCycleComplete: time.Now().UTC(), ConsecutiveTotalFeedFailCycles: 3})
		err := Check(ctx, path, dbPath, time.Minute)
		if err == nil || !strings.Contains(err.Error(), "consecutive") {
			t.Fatalf("expected consecutive failure error, got %v", err)
		}
	})
}

func TestAtomicWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "health.json")
	if err := WriteAtomic(path, State{LastCycleComplete: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
