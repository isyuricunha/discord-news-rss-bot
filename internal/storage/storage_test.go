package storage_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/isyuricunha/discord-news-rss-bot/internal/model"
	"github.com/isyuricunha/discord-news-rss-bot/internal/storage"
)

func TestFreshDatabaseCreationAndIdempotentMigration(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "posted_hashes.db")
	store, err := storage.Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = storage.Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Ping(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationFromPythonSchemaPreservesLegacyRecords(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "posted_hashes.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE posted(hash TEXT PRIMARY KEY, posted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, title TEXT, source TEXT);
INSERT INTO posted(hash, posted_at, title, source) VALUES('legacyhash', '2026-06-20 10:00:00', 'Legacy title', 'Legacy source')`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	store, err := storage.Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	count, cutoff, err := store.LegacyStats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || cutoff == nil {
		t.Fatalf("legacy record not preserved: count=%d cutoff=%v", count, cutoff)
	}
}

func TestLegacyHashAssociationPreventsRepost(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "posted_hashes.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	legacy := model.LegacyHash("Title", "https://example.com/a")
	_, err = db.Exec(`CREATE TABLE posted(hash TEXT PRIMARY KEY, posted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, title TEXT, source TEXT);
INSERT INTO posted(hash, title, source) VALUES(?, 'Title', 'Source')`, legacy)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	store, err := storage.Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	article := model.Article{FeedURL: "https://example.com/feed", Title: "Title", Link: "https://example.com/a", Source: "Source"}
	model.PrepareArticleIdentity(&article)
	known, err := store.IsKnownOrAssociateLegacy(ctx, article)
	if err != nil {
		t.Fatal(err)
	}
	if !known {
		t.Fatal("legacy hash was not recognized")
	}
}

func TestFeedStatePersistence(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	feed := model.FeedConfig{URL: "https://example.com/feed", Source: "Example", Category: "News"}
	now := time.Now().UTC()
	if err := store.CompleteSuccessfulFetch(ctx, feed, model.FeedState{}, true, `"etag"`, "modified", now, nil); err != nil {
		t.Fatal(err)
	}
	state, ok, err := store.GetFeedState(ctx, feed)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || !state.Initialized || state.ETag != `"etag"` || state.LastModified != "modified" {
		t.Fatalf("unexpected state %#v ok=%v", state, ok)
	}
}

func TestRetentionCleanupAndDisable(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	old := time.Now().UTC().AddDate(-2, 0, 0)
	article := model.Article{FeedURL: "https://example.com/feed", Title: "Old", Link: "https://example.com/old", Source: "Source"}
	model.PrepareArticleIdentity(&article)
	if err := store.MarkPosted(ctx, article, old); err != nil {
		t.Fatal(err)
	}
	deleted, ran, err := store.Cleanup(ctx, 0, time.Now().UTC())
	if err != nil || ran || deleted != 0 {
		t.Fatalf("disabled cleanup should not run: deleted=%d ran=%v err=%v", deleted, ran, err)
	}
	deleted, ran, err = store.Cleanup(ctx, 365, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if !ran || deleted != 1 {
		t.Fatalf("expected one cleanup deletion, deleted=%d ran=%v", deleted, ran)
	}
}

func TestConcurrentAccessDoesNotCorruptDatabase(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			article := model.Article{FeedURL: "https://example.com/feed", Title: "Title", Link: "https://example.com/a", Source: "Source"}
			model.PrepareArticleIdentity(&article)
			_ = store.MarkPosted(ctx, article, time.Now().UTC())
		}(i)
	}
	wg.Wait()
	count, err := store.CountStatus(ctx, "posted")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one deduplicated row, got %d", count)
	}
}
