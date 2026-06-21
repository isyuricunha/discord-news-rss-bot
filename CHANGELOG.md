# Changelog

## v3.0.0

- Rewrote the runtime in Go with a compact package layout and no Python dependency.
- Added a static CGO-free build using `modernc.org/sqlite`.
- Added a scratch production image that runs as numeric non-root UID/GID `65532:65532`.
- Added transactional SQLite migrations from the v2 Python `posted` table.
- Added improved duplicate detection with GUID, normalized link, fallback identity, and legacy hash matching.
- Added default startup flood protection through per-feed `INITIAL_SYNC_MODE=skip`.
- Added bounded concurrent feed fetching with ETag and Last-Modified persistence.
- Added Discord 429 handling using `Retry-After` or JSON `retry_after` before falling back to `COOLDOWN_DELAY`.
- Added mandatory Discord mention protection with `allowed_mentions.parse=[]` on every webhook request.
- Added persisted health state and a real `healthcheck` subcommand.
- Replaced application-managed log files with stdout/stderr `log/slog` logging.
- Replaced the auto-release-on-main workflow with separate CI and tag-only release workflows.
- Added Dependabot configuration for Go modules, GitHub Actions, and Docker base images.
- Deprecated `RSS_BOT_LOGS`, `LOG_FILE`, and `RSS_BOT_PID`; they are accepted but ignored by the v3 runtime.
- Removed obsolete Python deployment, virtual environment, logrotate, PID-file, and DockerSlim behavior.
