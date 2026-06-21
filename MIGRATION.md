# Migration to v3.0.0

v3.0.0 replaces the Python runtime with a static Go service. The database filename and environment-variable compatibility are preserved so existing users can upgrade in place after taking a backup.

## Backup First

Stop the v2 service or container, then copy the SQLite database before starting v3:

```bash
cp /path/to/posted_hashes.db /path/to/posted_hashes.db.v2-backup
```

The application does not automatically copy the database on every startup because production databases may be large and storage policies vary.

## Database Migration Behavior

The v2 Python application used a SQLite `posted` table with at least:

- `hash`
- `posted_at`
- `title`
- `source`

On first v3 startup, the migration runs in a transaction:

1. Detect the legacy `posted` table.
2. Rename it to `posted_v2_legacy`.
3. Create the v3 schema.
4. Copy every legacy row into the new `posted` table with its original hash and metadata.
5. Record schema version `1`.

If the migration fails, the transaction rolls back.

v3 keeps the old Python SHA-256 hash of `title + link` as a legacy duplicate key. When a fetched article matches a legacy hash, v3 associates that record with the new article identity and does not repost it.

## Startup Flood Protection During Upgrade

For a non-empty v2 database with no feed-state records, v3 treats startup as an upgrade. During first synchronization it favors avoiding duplicate floods:

- Existing legacy hashes are treated as already posted.
- Undated historical entries are skipped.
- Entries with publication timestamps older than or equal to the newest legacy `posted_at` timestamp are skipped.
- The normal per-feed `INITIAL_SYNC_MODE` still applies to genuinely new first-seen entries.

The default `INITIAL_SYNC_MODE=skip` sends no messages for current entries when a feed is first seen.

## Configuration Compatibility

Still supported:

- `DISCORD_WEBHOOK_URL`
- `RSS_BOT_CONFIG`
- `RSS_FEEDS`
- `RSS_FEEDS_*`
- `CHECK_INTERVAL`
- `POST_DELAY`
- `COOLDOWN_DELAY`
- `MAX_POST_LENGTH`
- `MAX_CONTENT_LENGTH`
- `FEED_TIMEOUT`
- `RSS_BOT_DATA`
- `DATA_DIR`
- `DB_FILE`
- `RSS_BOT_LOGS`
- `LOG_FILE`
- `RSS_BOT_PID`

`RSS_BOT_DATA` remains the preferred data-directory variable. `DATA_DIR` remains a fallback. `DB_FILE` still overrides the database path.

Deprecated variables:

- `RSS_BOT_LOGS`
- `LOG_FILE`
- `RSS_BOT_PID`

These are accepted to avoid breaking existing environments, but v3 logs to stdout/stderr and does not manage PID files.

## Removed Runtime Requirements

v3 does not require:

- Python
- pip
- virtual environments
- Python package installation
- application log files
- PID files
- logrotate integration
- DockerSlim

## Container UID and Bind Mounts

The production image runs as UID/GID `65532:65532`. Named volumes work without manual permission changes. For bind mounts:

```bash
sudo chown -R 65532:65532 /path/to/discord-rss-bot-data
```

Do not use world-writable permissions.

## Rollback

To roll back to v2:

1. Stop v3.
2. Restore the v2 backup database.
3. Restore the previous v2 image or Python service files.
4. Restore old service/container definitions.

Do not point the v2 runtime at a v3-migrated database unless you have verified compatibility in a copy.

## Verify Success

After starting v3:

```bash
discord-rss-bot validate-config
discord-rss-bot healthcheck
sqlite3 /path/to/posted_hashes.db "SELECT COUNT(*) FROM posted;"
```

With Docker:

```bash
docker compose logs -f discord-rss-bot
docker compose ps
```

Look for a successful cycle summary and a healthy container state.
