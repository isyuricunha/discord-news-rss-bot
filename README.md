# Discord RSS Bot

Discord RSS Bot is a small Go service that monitors RSS and Atom feeds and posts new articles to a Discord channel through a webhook. Version 3 is a full Go rewrite focused on safe startup behavior, SQLite migration, bounded concurrency, Discord rate-limit handling, and minimal container/runtime dependencies.

## Features

- RSS and Atom parsing with `github.com/mmcdole/gofeed`
- Legacy feed charset support for UTF-8, ISO-8859-1, and Windows-1252
- SQLite persistence with the CGO-free `modernc.org/sqlite` driver
- Duplicate prevention with GUID, normalized link, fallback identity, and v2 legacy hash compatibility
- Safe initial synchronization; fresh installs skip historical articles by default
- Discord webhook posting with bounded retries and 429 retry timing from Discord
- Mandatory `allowed_mentions.parse=[]` on every webhook payload
- HTTP caching with ETag and Last-Modified
- Bounded concurrent feed fetching
- Persistent `health.json` checked by the same binary
- Graceful SIGINT/SIGTERM shutdown
- Static Linux builds and a scratch, non-root container image
- Native Linux systemd and Windows executable support

## Quick Start With Docker Compose

1. Create `.env` from the example:

```bash
cp .env.example .env
```

2. Edit `.env` and set `DISCORD_WEBHOOK_URL` to your Discord webhook URL.

3. Start the service:

```bash
docker compose up -d
docker compose logs -f discord-rss-bot
```

The default Compose file uses the published image name `isyuricunha/discord-rss-bot:3`, can build locally with `docker compose build`, stores data in a named volume, runs read-only except for `/app/data` and `/tmp`, drops Linux capabilities, and uses the image healthcheck.

For bind mounts instead of named volumes, the container user is `65532:65532`:

```bash
sudo chown -R 65532:65532 /path/to/discord-rss-bot-data
```

Do not use world-writable permissions for the data directory.

## Webhook Setup

In Discord, open the target channel settings, create an Incoming Webhook, and copy its URL into `.env`:

```env
DISCORD_WEBHOOK_URL=replace-with-your-discord-webhook-url
```

The webhook URL is treated as a secret. The application redacts it from logs and errors.

## Commands

```bash
discord-rss-bot              # same as run
discord-rss-bot run
discord-rss-bot healthcheck
discord-rss-bot validate-config
discord-rss-bot validate-feeds
discord-rss-bot version
```

`validate-config` checks configuration without fetching feeds or posting messages. `validate-feeds` fetches configured feeds, does not require a Discord webhook, does not touch SQLite, and returns a non-zero exit code if any configured feed fails. `healthcheck` reads `<RSS_BOT_DATA>/health.json` and opens the SQLite database read-only.

## Configuration

Configuration precedence is:

1. Process environment variables
2. `RSS_BOT_CONFIG` file values when the file exists
3. Defaults

The config file format is simple `KEY=VALUE`, with empty lines and comments ignored. Single-quoted and double-quoted values are supported. Shell syntax is not executed.

| Variable | Default | Description |
| --- | --- | --- |
| `DISCORD_WEBHOOK_URL` | required | Discord webhook URL. |
| `RSS_BOT_CONFIG` | `/etc/discord-rss-bot/config.env` | Optional config file path. |
| `RSS_BOT_DATA` | `/var/lib/discord-rss-bot`, `/app/data` in Docker | Data directory. Preferred over `DATA_DIR`. |
| `DATA_DIR` | unset | Legacy fallback data directory. |
| `DB_FILE` | `<data>/posted_hashes.db` | Explicit SQLite database path. |
| `CHECK_INTERVAL` | `300` | Poll interval in seconds or Go duration. |
| `POST_DELAY` | `3` | Delay between successful Discord posts. |
| `COOLDOWN_DELAY` | `60` | Final fallback delay when Discord gives no retry time. |
| `MAX_POST_LENGTH` | `1900` | Max Discord message length; cannot exceed 2000. |
| `MAX_CONTENT_LENGTH` | `800` | Max article preview length. |
| `FEED_TIMEOUT` | `30` | Per-feed request timeout. |
| `INITIAL_SYNC_MODE` | `skip` | One of `skip`, `latest`, `backfill`. |
| `INITIAL_SYNC_MAX_POSTS` | `1` | Max first-seen backfill posts per feed. |
| `MAX_ENTRIES_PER_FEED` | `20` | Max entries inspected per feed per cycle. |
| `MAX_POSTS_PER_CYCLE` | `10` | Global post cap per cycle. |
| `MAX_CONCURRENT_FEEDS` | `5` | Feed fetch concurrency limit. |
| `POST_RETENTION_DAYS` | `365` | Article record retention; `0` disables deletion. |
| `MAX_FEED_BYTES` | `10485760` | Max feed response size. |
| `DISCORD_MAX_RETRIES` | `5` | Retry count after the first Discord attempt. |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error`. |
| `LOG_FORMAT` | `text` | `text` or `json`. |
| `HEALTH_MAX_AGE` | `0` | `0` derives a limit from `CHECK_INTERVAL`; otherwise seconds or Go duration. |

Deprecated compatibility variables `RSS_BOT_LOGS`, `LOG_FILE`, and `RSS_BOT_PID` are accepted so old environments do not fail validation, but v3 logs only to stdout/stderr and does not manage PID files.

## Feed Configuration

Universal feeds take priority:

```env
RSS_FEEDS=https://example.com/rss,https://example.org/feed
```

Category feeds are used when `RSS_FEEDS` is not set:

```env
RSS_FEEDS_NEWS=https://g1.globo.com/dynamo/rss2.xml,https://rss.uol.com.br/feed/noticias.xml
RSS_FEEDS_TECHNOLOGY=https://canaltech.com.br/rss/,https://rss.tecmundo.com.br/feed
RSS_FEEDS_SPORTS=https://globoesporte.globo.com/rss/ultimas/
RSS_FEEDS_BUSINESS=https://www.infomoney.com.br/rss/
RSS_FEEDS_POLITICS=https://www.gazetadopovo.com.br/feed/rss/republica.xml
```

If no feeds are configured, the bundled Brazilian defaults are used for general news, politics/conservative, and technology. Duplicate feed URLs are polled once, and category/source metadata is kept for message formatting.

The v3.0.1 bundled defaults are:

- General News: G1, UOL, Band, CNN Brasil, Folha
- Politics & Conservative: Gazeta do Povo, Jovem Pan, Diario do Poder, Pragmatismo Politico, Conexao Politica, Poder 360, Crusoe, Veja, Metropoles, O Antagonista
- Technology: Canaltech, Olhar Digital, Tecnoblog, Meio Bit, ShowMeTech, TecMundo, Adrenaline, Hardware.com.br, Tudo Celular, Oficina da Net

Publisher feed URLs can change independently of the bot. Use `discord-rss-bot validate-feeds` after upgrades or when feed failures repeat, and set `RSS_FEEDS` or `RSS_FEEDS_*` when you prefer your own source list.

## Initial Synchronization

`INITIAL_SYNC_MODE=skip` is the default. For a feed seen for the first time, the bot fetches current entries, records them as seen, sends no Discord messages, and marks that feed initialized. Adding a new feed to an existing database uses the same per-feed initialization behavior.

`latest` posts only the newest current entry for that feed. `backfill` posts up to `INITIAL_SYNC_MAX_POSTS`, oldest to newest. Both modes still mark the remaining first-seen entries as seen to avoid startup floods.

During a v2 database upgrade, existing legacy records are used conservatively. Undated historical entries and entries older than the latest legacy post timestamp are skipped during first synchronization.

## Data Persistence

The database defaults to `<RSS_BOT_DATA>/posted_hashes.db` to preserve existing deployments. v3 migrates the old Python `posted` table automatically inside a transaction and keeps legacy hashes for duplicate detection.

The health file is written to `<RSS_BOT_DATA>/health.json` with an atomic temporary-file rename.

## Healthcheck

The healthcheck fails when:

- `health.json` is missing or malformed
- the last completed cycle is stale
- the SQLite database cannot be opened read-only and queried
- three or more consecutive cycles had every attempted feed fail

Docker uses:

```dockerfile
HEALTHCHECK CMD ["/discord-rss-bot", "healthcheck"]
```

No shell, curl, wget, Python, or external binary is required.

## Logs

Logs go to stdout/stderr through `log/slog`. In Docker, use `docker compose logs`. Under systemd, use:

```bash
journalctl -u discord-rss-bot -f
```

No application-managed rotating log file is created.

## Upgrade From v2

Read [MIGRATION.md](MIGRATION.md) before upgrading. At minimum:

1. Stop the v2 service or container.
2. Back up `posted_hashes.db`.
3. Replace the runtime with the v3 binary or image.
4. Keep `RSS_BOT_DATA` or `DB_FILE` pointing at the existing database.
5. Start v3 and check logs plus `discord-rss-bot healthcheck`.

Python, pip, virtual environments, file logging, PID files, logrotate, and DockerSlim are no longer used.

## Native Linux Installation

Build or download the `linux/amd64` or `linux/arm64` release binary, then run:

```bash
sudo scripts/install-linux.sh --binary ./discord-rss-bot --webhook-url "<webhook-url>"
sudo systemctl start discord-rss-bot
sudo systemctl status discord-rss-bot
```

The installer copies the binary to `/usr/local/bin/discord-rss-bot`, writes `/etc/discord-rss-bot/config.env` with private permissions, installs a hardened systemd service, and uses `/var/lib/discord-rss-bot` for state.

To uninstall:

```bash
sudo scripts/uninstall-linux.sh
sudo scripts/uninstall-linux.sh --purge   # also removes config and data
```

## Windows Usage

Download the `windows/amd64` release archive, extract it, and run directly:

```powershell
$env:DISCORD_WEBHOOK_URL="<webhook-url>"
.\discord-rss-bot.exe run
```

For startup registration with built-in Windows Task Scheduler, run PowerShell as Administrator:

```powershell
.\windows\install.ps1 -WebhookUrl "<webhook-url>" -ExecutablePath .\discord-rss-bot.exe
Start-ScheduledTask -TaskName DiscordRSSBot
```

No Python installation is required.

## Build From Source

```bash
go mod tidy
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" ./cmd/discord-rss-bot
```

Cross-build examples:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/discord-rss-bot-linux-amd64 ./cmd/discord-rss-bot
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o dist/discord-rss-bot-linux-arm64 ./cmd/discord-rss-bot
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/discord-rss-bot-windows-amd64.exe ./cmd/discord-rss-bot
```

## Testing

```bash
go vet ./...
go test ./...
go test -race ./...
go test -cover ./...
```

Tests use temporary SQLite databases and httptest servers. They do not require real Discord or RSS endpoints.

To manually validate configured feeds against the network:

```bash
discord-rss-bot validate-feeds
```

## Troubleshooting

- `DISCORD_WEBHOOK_URL is required`: set the environment variable or put it in `RSS_BOT_CONFIG`.
- Healthcheck says stale: inspect service logs; the service may be unable to complete cycles.
- Feed failures repeat: run `discord-rss-bot validate-feeds`, verify the feed URL, timeout, charset, and response size; sensitive URL query values are redacted in logs.
- No posts after first start: this is expected with `INITIAL_SYNC_MODE=skip`.
- Bind-mount permission errors: set ownership to UID/GID `65532:65532`.

## Security Notes

- Discord mentions are always disabled with `allowed_mentions.parse=[]`.
- Webhook secrets and sensitive feed URL query values are redacted from application logs.
- Feed response size, retry loops, post loops, and concurrency are bounded.
- The production container runs as non-root from `scratch`.
- Compose drops Linux capabilities and enables `no-new-privileges`.
- The service exposes no HTTP management port and sends data only to configured feed URLs and the configured Discord webhook.

## License

The current [LICENSE](LICENSE) file is the source of truth and contains the GNU Affero General Public License v3.0.
