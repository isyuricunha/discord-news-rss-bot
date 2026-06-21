CREATE TABLE IF NOT EXISTS posted (
    article_key TEXT PRIMARY KEY,
    hash TEXT UNIQUE,
    feed_url TEXT,
    source TEXT,
    category TEXT,
    guid TEXT,
    normalized_link TEXT,
    title TEXT,
    published_at TEXT,
    first_seen_at TEXT NOT NULL,
    posted_at TEXT,
    status TEXT NOT NULL CHECK (status IN ('seen', 'posted')),
    legacy_only INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_posted_hash ON posted(hash);
CREATE INDEX IF NOT EXISTS idx_posted_feed_url ON posted(feed_url);
CREATE INDEX IF NOT EXISTS idx_posted_posted_at ON posted(posted_at);
CREATE INDEX IF NOT EXISTS idx_posted_first_seen_at ON posted(first_seen_at);

CREATE TABLE IF NOT EXISTS feed_states (
    feed_url TEXT PRIMARY KEY,
    source TEXT NOT NULL,
    category TEXT NOT NULL,
    initialized INTEGER NOT NULL DEFAULT 0,
    etag TEXT,
    last_modified TEXT,
    last_checked_at TEXT,
    last_success_at TEXT,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_feed_states_next_attempt ON feed_states(next_attempt_at);

CREATE TABLE IF NOT EXISTS maintenance_state (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
);
