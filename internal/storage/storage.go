package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/isyuricunha/discord-news-rss-bot/internal/model"
	"github.com/isyuricunha/discord-news-rss-bot/migrations"
)

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &Store{db: db}
	if err := store.configure(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := store.Migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func OpenReadOnly(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	store := &Store{db: db}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	var one int
	return s.db.QueryRowContext(ctx, "SELECT 1").Scan(&one)
}

func (s *Store) configure(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, "PRAGMA synchronous=NORMAL"); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		return err
	}
	return s.db.PingContext(ctx)
}

func (s *Store) Migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	hasMigrations, err := tableExists(ctx, tx, "schema_migrations")
	if err != nil {
		return err
	}
	if hasMigrations {
		var version int
		err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
		if err != nil {
			return err
		}
		if version >= 1 {
			return tx.Commit()
		}
	}

	if err := migratePostedTable(ctx, tx); err != nil {
		return err
	}

	sqlBytes, err := migrations.Files.ReadFile("001_schema.sql")
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, string(sqlBytes)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES(1, ?)", formatTime(time.Now())); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "PRAGMA user_version=1"); err != nil {
		return err
	}
	return tx.Commit()
}

func migratePostedTable(ctx context.Context, tx *sql.Tx) error {
	exists, err := tableExists(ctx, tx, "posted")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	columns, err := tableColumns(ctx, tx, "posted")
	if err != nil {
		return err
	}
	if columns["article_key"] {
		return nil
	}
	if !columns["hash"] {
		return fmt.Errorf("existing posted table is missing required legacy hash column")
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE posted RENAME TO posted_v2_legacy`); err != nil {
		return err
	}
	sqlBytes, err := migrations.Files.ReadFile("001_schema.sql")
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, string(sqlBytes)); err != nil {
		return err
	}
	titleExpr := "NULL"
	if columns["title"] {
		titleExpr = "title"
	}
	sourceExpr := "NULL"
	if columns["source"] {
		sourceExpr = "source"
	}
	postedExpr := "CURRENT_TIMESTAMP"
	if columns["posted_at"] {
		postedExpr = "COALESCE(posted_at, CURRENT_TIMESTAMP)"
	}
	copySQL := fmt.Sprintf(`
INSERT OR IGNORE INTO posted(article_key, hash, source, title, first_seen_at, posted_at, status, legacy_only)
SELECT 'legacy:' || hash, hash, %s, %s, %s, %s, 'posted', 1
FROM posted_v2_legacy
WHERE hash IS NOT NULL AND hash != ''`, sourceExpr, titleExpr, postedExpr, postedExpr)
	if _, err := tx.ExecContext(ctx, copySQL); err != nil {
		return err
	}
	return nil
}

func (s *Store) GetFeedState(ctx context.Context, feed model.FeedConfig) (model.FeedState, bool, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT feed_url, source, category, initialized, etag, last_modified, last_checked_at, last_success_at, consecutive_failures, next_attempt_at
FROM feed_states WHERE feed_url = ?`, feed.URL)
	state, err := scanFeedState(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.FeedState{
			URL:      feed.URL,
			Source:   feed.Source,
			Category: feed.Category,
		}, false, nil
	}
	if err != nil {
		return model.FeedState{}, false, err
	}
	return state, true, nil
}

func (s *Store) UpsertFeedStateSuccess(ctx context.Context, feed model.FeedConfig, state model.FeedState, initialized bool, etag string, lastModified string, checkedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO feed_states(feed_url, source, category, initialized, etag, last_modified, last_checked_at, last_success_at, consecutive_failures, next_attempt_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, 0, NULL)
ON CONFLICT(feed_url) DO UPDATE SET
	source = excluded.source,
	category = excluded.category,
	initialized = CASE WHEN excluded.initialized = 1 THEN 1 ELSE feed_states.initialized END,
	etag = COALESCE(NULLIF(excluded.etag, ''), feed_states.etag),
	last_modified = COALESCE(NULLIF(excluded.last_modified, ''), feed_states.last_modified),
	last_checked_at = excluded.last_checked_at,
	last_success_at = excluded.last_success_at,
	consecutive_failures = 0,
	next_attempt_at = NULL`,
		feed.URL,
		feed.Source,
		feed.Category,
		boolInt(initialized || state.Initialized),
		etag,
		lastModified,
		formatTime(checkedAt),
		formatTime(checkedAt),
	)
	return err
}

func (s *Store) CompleteSuccessfulFetch(ctx context.Context, feed model.FeedConfig, state model.FeedState, initialized bool, etag string, lastModified string, checkedAt time.Time, seenArticles []model.Article) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		for _, article := range seenArticles {
			if err := insertArticle(ctx, tx, article, "seen", checkedAt, nil); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `
INSERT INTO feed_states(feed_url, source, category, initialized, etag, last_modified, last_checked_at, last_success_at, consecutive_failures, next_attempt_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, 0, NULL)
ON CONFLICT(feed_url) DO UPDATE SET
	source = excluded.source,
	category = excluded.category,
	initialized = CASE WHEN excluded.initialized = 1 THEN 1 ELSE feed_states.initialized END,
	etag = COALESCE(NULLIF(excluded.etag, ''), feed_states.etag),
	last_modified = COALESCE(NULLIF(excluded.last_modified, ''), feed_states.last_modified),
	last_checked_at = excluded.last_checked_at,
	last_success_at = excluded.last_success_at,
	consecutive_failures = 0,
	next_attempt_at = NULL`,
			feed.URL,
			feed.Source,
			feed.Category,
			boolInt(initialized || state.Initialized),
			etag,
			lastModified,
			formatTime(checkedAt),
			formatTime(checkedAt),
		)
		return err
	})
}

func (s *Store) UpdateFeedNotModified(ctx context.Context, feed model.FeedConfig, checkedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO feed_states(feed_url, source, category, initialized, last_checked_at, consecutive_failures)
VALUES(?, ?, ?, 1, ?, 0)
ON CONFLICT(feed_url) DO UPDATE SET
	source = excluded.source,
	category = excluded.category,
	last_checked_at = excluded.last_checked_at,
	consecutive_failures = 0,
	next_attempt_at = NULL`,
		feed.URL,
		feed.Source,
		feed.Category,
		formatTime(checkedAt),
	)
	return err
}

func (s *Store) UpdateFeedFailure(ctx context.Context, feed model.FeedConfig, failures int, checkedAt time.Time, nextAttempt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO feed_states(feed_url, source, category, initialized, last_checked_at, consecutive_failures, next_attempt_at)
VALUES(?, ?, ?, 0, ?, ?, ?)
ON CONFLICT(feed_url) DO UPDATE SET
	source = excluded.source,
	category = excluded.category,
	last_checked_at = excluded.last_checked_at,
	consecutive_failures = excluded.consecutive_failures,
	next_attempt_at = excluded.next_attempt_at`,
		feed.URL,
		feed.Source,
		feed.Category,
		formatTime(checkedAt),
		failures,
		formatTime(nextAttempt),
	)
	return err
}

func (s *Store) MarkSeenBatch(ctx context.Context, articles []model.Article, seenAt time.Time) error {
	if len(articles) == 0 {
		return nil
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		for _, article := range articles {
			if err := insertArticle(ctx, tx, article, "seen", seenAt, nil); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) MarkPosted(ctx context.Context, article model.Article, postedAt time.Time) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		return insertArticle(ctx, tx, article, "posted", postedAt, &postedAt)
	})
}

func (s *Store) IsKnownOrAssociateLegacy(ctx context.Context, article model.Article) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var existing string
	err = tx.QueryRowContext(ctx, "SELECT article_key FROM posted WHERE article_key = ?", article.ArticleKey).Scan(&existing)
	if err == nil {
		return true, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	if article.LegacyHash == "" {
		return false, tx.Commit()
	}

	var legacyKey string
	err = tx.QueryRowContext(ctx, "SELECT article_key FROM posted WHERE hash = ?", article.LegacyHash).Scan(&legacyKey)
	if errors.Is(err, sql.ErrNoRows) {
		return false, tx.Commit()
	}
	if err != nil {
		return false, err
	}
	if legacyKey != article.ArticleKey {
		_, err = tx.ExecContext(ctx, `
UPDATE posted
SET article_key = ?, feed_url = COALESCE(NULLIF(feed_url, ''), ?), source = COALESCE(NULLIF(source, ''), ?),
	category = COALESCE(NULLIF(category, ''), ?), guid = COALESCE(NULLIF(guid, ''), ?),
	normalized_link = COALESCE(NULLIF(normalized_link, ''), ?), title = COALESCE(NULLIF(title, ''), ?),
	published_at = COALESCE(NULLIF(published_at, ''), ?), legacy_only = 0
WHERE article_key = ? AND NOT EXISTS(SELECT 1 FROM posted WHERE article_key = ?)`,
			article.ArticleKey,
			article.FeedURL,
			article.Source,
			article.Category,
			article.GUID,
			article.NormalizedLink,
			article.Title,
			formatOptionalTime(article.PublishedAt),
			legacyKey,
			article.ArticleKey,
		)
		if err != nil {
			if strings.Contains(err.Error(), "constraint") {
				return true, tx.Commit()
			}
			return false, err
		}
	}
	return true, tx.Commit()
}

func (s *Store) LegacyStats(ctx context.Context) (int, *time.Time, error) {
	var count int
	var maxPosted sql.NullString
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*), MAX(posted_at) FROM posted WHERE legacy_only = 1 OR hash IS NOT NULL").Scan(&count, &maxPosted)
	if err != nil {
		return 0, nil, err
	}
	if maxPosted.Valid {
		parsed, err := parseTime(maxPosted.String)
		if err == nil {
			return count, &parsed, nil
		}
	}
	return count, nil, nil
}

func (s *Store) CountStatus(ctx context.Context, status string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM posted WHERE status = ?", status).Scan(&count)
	return count, err
}

func (s *Store) Cleanup(ctx context.Context, retentionDays int, now time.Time) (int64, bool, error) {
	if retentionDays == 0 {
		return 0, false, nil
	}
	last, _ := s.maintenanceValue(ctx, "last_cleanup_at")
	if last != "" {
		if parsed, err := parseTime(last); err == nil && now.Sub(parsed) < 24*time.Hour {
			return 0, false, nil
		}
	}
	cutoff := now.AddDate(0, 0, -retentionDays)
	var deleted int64
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `
DELETE FROM posted
WHERE COALESCE(posted_at, first_seen_at) < ?`, formatTime(cutoff))
		if err != nil {
			return err
		}
		deleted, err = result.RowsAffected()
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
INSERT INTO maintenance_state(key, value) VALUES('last_cleanup_at', ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value`, formatTime(now))
		return err
	})
	if err != nil {
		return 0, false, err
	}
	return deleted, true, nil
}

func (s *Store) Optimize(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "PRAGMA optimize")
	return err
}

func (s *Store) maintenanceValue(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM maintenance_state WHERE key = ?", key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return value, err
}

func insertArticle(ctx context.Context, tx *sql.Tx, article model.Article, status string, seenAt time.Time, postedAt *time.Time) error {
	posted := ""
	if postedAt != nil {
		posted = formatTime(*postedAt)
	}
	_, err := tx.ExecContext(ctx, `
INSERT INTO posted(article_key, hash, feed_url, source, category, guid, normalized_link, title, published_at, first_seen_at, posted_at, status, legacy_only)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, 0)
ON CONFLICT(article_key) DO UPDATE SET
	feed_url = excluded.feed_url,
	source = excluded.source,
	category = excluded.category,
	guid = COALESCE(NULLIF(excluded.guid, ''), posted.guid),
	normalized_link = COALESCE(NULLIF(excluded.normalized_link, ''), posted.normalized_link),
	title = COALESCE(NULLIF(excluded.title, ''), posted.title),
	published_at = COALESCE(NULLIF(excluded.published_at, ''), posted.published_at),
	posted_at = CASE WHEN excluded.status = 'posted' THEN excluded.posted_at ELSE posted.posted_at END,
	status = CASE WHEN excluded.status = 'posted' THEN 'posted' ELSE posted.status END,
	legacy_only = 0`,
		article.ArticleKey,
		article.LegacyHash,
		article.FeedURL,
		article.Source,
		article.Category,
		article.GUID,
		article.NormalizedLink,
		article.Title,
		formatOptionalTime(article.PublishedAt),
		formatTime(seenAt),
		posted,
		status,
	)
	return err
}

func (s *Store) withTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanFeedState(row scanner) (model.FeedState, error) {
	var state model.FeedState
	var initialized int
	var etag, lastModified sql.NullString
	var lastChecked, lastSuccess, nextAttempt sql.NullString
	err := row.Scan(&state.URL, &state.Source, &state.Category, &initialized, &etag, &lastModified, &lastChecked, &lastSuccess, &state.ConsecutiveFailures, &nextAttempt)
	if err != nil {
		return model.FeedState{}, err
	}
	state.Initialized = initialized == 1
	state.ETag = nullString(etag)
	state.LastModified = nullString(lastModified)
	state.LastCheckedAt = parseOptionalTime(lastChecked)
	state.LastSuccessAt = parseOptionalTime(lastSuccess)
	state.NextAttemptAt = parseOptionalTime(nextAttempt)
	return state, nil
}

func tableExists(ctx context.Context, tx *sql.Tx, table string) (bool, error) {
	var name string
	err := tx.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func tableColumns(ctx context.Context, tx *sql.Tx, table string) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		columns[name] = true
	}
	return columns, rows.Err()
}

func formatOptionalTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return formatTime(*value)
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseOptionalTime(value sql.NullString) *time.Time {
	if !value.Valid || value.String == "" {
		return nil
	}
	parsed, err := parseTime(value.String)
	if err != nil {
		return nil
	}
	return &parsed
}

func parseTime(value string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed.UTC(), nil
	}
	if parsed, err := time.Parse("2006-01-02 15:04:05", value); err == nil {
		return parsed.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("invalid timestamp %q", value)
}

func nullString(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
