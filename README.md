# 🤖 Discord RSS Bot

Automated bot that monitors RSS feeds and posts updates to Discord channels via webhook. Fully configurable with custom feeds and categories. Features timeout handling to prevent hanging on slow RSS feeds.

## ✨ Features

- 🔄 **Automatic RSS monitoring** with configurable intervals
- 📱 **Elegant Discord formatting** with category emojis
- 💾 **Smart caching** to prevent duplicate posts
- ⏱️ **Timeout handling** prevents hanging on slow feeds
- 🛡️ **Robust error handling** for network issues
- 🐳 **Fully containerized** with Docker support
- 📦 **Pre-built images** available on Docker Hub & GHCR
- 🌐 **Multi-platform** (AMD64, ARM64)
- 🎯 **Flexible configuration** - universal or category-based feeds
- 🏗️ **Multiple deployment options** (Docker, System Service)
- 📊 **Comprehensive logging** and health checks
- 🔒 **Security-focused** with non-root containers

## 🚀 Quick Start

### Using Pre-built Container (Recommended)

```bash
# Create directories for persistent data/logs
mkdir -p ~/docker/docker-data/rss-discord-bot/data
mkdir -p ~/docker/docker-data/rss-discord-bot/logs

# Set permissions for container access (container runs as non-root)
chmod 777 ~/docker/docker-data/rss-discord-bot/logs
chmod 777 ~/docker/docker-data/rss-discord-bot/data

# Create .env file with your webhook URL (do not commit this file)
echo "DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/YOUR_ID/YOUR_TOKEN" > .env

# Run with Docker Compose (for testing with pre-built image)
docker compose -f docker-compose.test.yml up -d

# View logs
docker compose -f docker-compose.test.yml logs -f
```

## 📦 Container Images

Pre-built images are available from multiple registries:

- **Docker Hub**: `isyuricunha/discord-rss-bot:latest` | `isyuricunha/discord-rss-bot:2.0.8`
- **GitHub Container Registry**: `ghcr.io/isyuricunha/discord-news-rss-bot:latest` | `ghcr.io/isyuricunha/discord-news-rss-bot:2.0.8`

### Supported Architectures

- `linux/amd64` (x86_64)
- `linux/arm64` (ARM64/AArch64)

### 🏷️ Automatic Versioning

The project uses **Semantic Versioning** and **Conventional Commits** to automate releases.

On each push to the `main` branch, GitHub Actions will:

- Determine the next version based on commit messages
- Create a Git tag in the format `v{version}` (e.g., `v2.0.17`)
- Create a GitHub Release with generated release notes
- If (and only if) a new version was released, build and publish Docker images

Published Docker tags:

- `latest`
- `v{version}` (e.g., `v2.0.17`)
- `{version}` (e.g., `2.0.17`)

Note: Releases are created by `python-semantic-release`.

## 🐳 Docker Deployment Options

### 1. Standard Docker Compose (with .env file)

```bash
docker compose up -d
```

### 2. Inline Environment Variables

```bash
# Edit docker-compose.inline.yml with your webhook URL
docker compose -f docker-compose.inline.yml up -d
```

### 3. Pre-built Image (from .env file)

```bash
# Edit docker-compose.prebuilt.yml and configure .env file
docker compose -f docker-compose.prebuilt.yml up -d
```

### 4. Testing with All Default Feeds

```bash
# Use the test configuration with all feeds pre-configured
# Edit webhook URL in docker-compose.test.yml first
docker compose -f docker-compose.test.yml up -d
```

## 🖥️ System Service Installation

For production deployments, you can install the bot as a system service:

### Debian/Ubuntu

```bash
sudo ./scripts/install-debian.sh --webhook-url "YOUR_WEBHOOK_URL"
sudo systemctl start discord-rss-bot
```

### Fedora/RHEL/CentOS

```bash
sudo ./scripts/install-fedora.sh --webhook-url "YOUR_WEBHOOK_URL"
sudo systemctl start discord-rss-bot
```

### Alpine Linux

```bash
sudo ./scripts/install-alpine.sh --webhook-url "YOUR_WEBHOOK_URL"
sudo rc-service discord-rss-bot start
```

### Windows

```powershell
# Run as Administrator
.\windows\install.ps1 -WebhookUrl "YOUR_WEBHOOK_URL"
net start DiscordRSSBot
```

## 🔧 Publishing to Container Registries

### GitHub Container Registry (Automatic)

Push to `main` branch or create a tag to automatically build and publish:

```bash
git tag v1.0.0
git push origin v1.0.0
```

### Docker Hub Setup

1. Add secrets to your GitHub repository:
   - `DOCKERHUB_USERNAME`: Your Docker Hub username
   - `DOCKERHUB_TOKEN`: Your Docker Hub token
2. Ensure GitHub Actions has permission to create tags/releases (repository setting: Workflow permissions must allow write access to contents)

Secrets used by the workflows:

- `GITHUB_TOKEN`: Automatically provided by GitHub Actions (must have `contents: write`)
- `DOCKERHUB_USERNAME`: Required to push to Docker Hub
- `DOCKERHUB_TOKEN`: Required to push to Docker Hub

Optional note about protected branches:

- If `main` is protected in a way that prevents `GITHUB_TOKEN` from pushing tags, use an admin PAT as a repository secret (for example `GH_RELEASE_TOKEN`) and update the workflow to use it.

## 🧾 Conventional Commits

Commit messages must follow the Conventional Commits specification:

```text
<type>(optional scope): <description>

optional body

optional footer(s)
```

Supported release-relevant types in this repository:

- `feat`: Features
- `fix`: Bug Fixes
- `perf`: Performance Improvements
- `docs`: Documentation
- `build`: Dependencies (and build-related changes)
- `chore`, `ci`, `refactor`, `style`, `test`: Other (no version bump by default)

### Breaking changes

To trigger a major release, include a `BREAKING CHANGE:` paragraph in the commit body:

```text
feat(api): change webhook payload format

BREAKING CHANGE: payload fields were renamed and old fields were removed.
```

### Examples (what release is produced)

- **Patch release**

```text
fix(docker): correct healthcheck path
```

- **Minor release**

```text
feat(rss): add support for multiple webhook targets
```

- **Major release**

```text
feat(config): remove legacy DATA_DIR variable

BREAKING CHANGE: DATA_DIR is no longer supported; use RSS_BOT_DATA.
```

## ⚙️ Configuration

### Required Variables

| Variable              | Description                        |
| :-------------------- | :--------------------------------- |
| `DISCORD_WEBHOOK_URL` | **Required**: Discord webhook URL  |

### Paths (recommended)

| Variable       | Default (Docker image)     | Description                                              |
| :------------- | :------------------------- | :------------------------------------------------------- |
| `RSS_BOT_DATA` | `/app/data`                | Directory where the SQLite database is stored            |
| `RSS_BOT_LOGS` | `/app/logs`                | Directory where log files are written                    |
| `RSS_BOT_PID`  | `/tmp/discord-rss-bot.pid` | PID file path (avoid `/var/run` in non-root containers)  |

### Advanced overrides (optional)

| Variable   | Description                                              |
| :--------- | :------------------------------------------------------- |
| `DB_FILE`  | Explicit SQLite DB file path (overrides `RSS_BOT_DATA`)  |
| `LOG_FILE` | Explicit log file path (overrides `RSS_BOT_LOGS`)        |
| `DATA_DIR` | Legacy alias for `RSS_BOT_DATA`                          |

### Optional Bot Settings

| Variable             | Default | Description                        |
| :------------------- | ------: | :--------------------------------- |
| `CHECK_INTERVAL`     |     300 | Interval between checks (seconds)  |
| `POST_DELAY`         |       3 | Delay between posts (seconds)      |
| `COOLDOWN_DELAY`     |      60 | Delay after rate limit (seconds)   |
| `MAX_POST_LENGTH`    |    1900 | Maximum message length             |
| `MAX_CONTENT_LENGTH` |     800 | Maximum content length             |
| `FEED_TIMEOUT`       |      30 | RSS feed request timeout (seconds) |

### 🎯 RSS Feeds Configuration

The bot supports **two configuration methods** with automatic priority handling:

#### Option 1: Universal Feeds (Simple)

Use a single variable for all your RSS feeds:

```bash
RSS_FEEDS=https://example.com/rss,https://another.com/feed,https://third.com/rss
```

#### Option 2: Category-Based Feeds (Organized)

Use separate variables for different categories:

```bash
RSS_FEEDS_NEWS=https://g1.globo.com/dynamo/rss2.xml,https://rss.uol.com.br/feed/noticias.xml
RSS_FEEDS_TECHNOLOGY=https://canaltech.com.br/rss/,https://tecnoblog.net/feed/
RSS_FEEDS_SPORTS=https://globoesporte.globo.com/rss/ultimas/
RSS_FEEDS_BUSINESS=https://www.infomoney.com.br/rss/
RSS_FEEDS_POLITICS=https://www.gazetadopovo.com.br/rss/brasil.xml
```

#### 🔄 Priority System

1. **RSS_FEEDS** (universal) - takes priority if set
2. **RSS_FEEDS_CATEGORY** (categories) - used if no universal feeds
3. **Default feeds** - Brazilian news feeds if no custom configuration

#### 🎨 Automatic Category Emojis

- **News/Noticias/General** → 📰
- **Technology/Tecnologia** → 💻  
- **Politics/Politica** → 🏛️
- **Sports/Esportes** → ⚽
- **Business/Economia** → 💼
- **Other categories** → 📢

### Docker Environment File (.env)

```bash
# Required
DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/YOUR_ID/YOUR_TOKEN

# Optional - Bot behavior
CHECK_INTERVAL=300
POST_DELAY=3
COOLDOWN_DELAY=60
MAX_POST_LENGTH=1900
MAX_CONTENT_LENGTH=800
FEED_TIMEOUT=30

# Optional - Custom feeds (choose one method)
# Method 1: Universal (simple)
# RSS_FEEDS=https://example.com/rss,https://another.com/feed

# Method 2: Category-based (organized)
# RSS_FEEDS_NEWS=https://g1.globo.com/dynamo/rss2.xml,https://rss.uol.com.br/feed/noticias.xml
# RSS_FEEDS_TECHNOLOGY=https://canaltech.com.br/rss/,https://tecnoblog.net/feed/
# RSS_FEEDS_SPORTS=https://globoesporte.globo.com/rss/ultimas/
```

### Default Feeds

If no custom feeds are configured, the bot uses default Brazilian news feeds across multiple categories.

## 📁 File Structure

```text
discord-rss-bot/
├── bot_service.py                  # Main bot code (works for both Docker and system service)
├── requirements.txt                # Python dependencies
├── Dockerfile                      # Container configuration
├── docker-compose.yml              # Docker orchestration (local build)
├── docker-compose.inline.yml       # Inline environment variables (Docker Hub)
├── docker-compose.prebuilt.yml     # Pre-built image version (Docker Hub)
├── docker-compose.test.yml         # Testing with all default feeds (Docker Hub)
├── .env.example                    # Configuration example
├── .dockerignore                   # Files ignored in build
├── .github/workflows/              # CI/CD workflows
│   └── docker-hub.yml              # Docker build validation for pull requests
├── systemd/                        # Linux systemd service
│   └── discord-rss-bot.service
├── scripts/                        # Installation scripts
│   ├── install-debian.sh           # Debian/Ubuntu installer
│   ├── install-fedora.sh           # Fedora/RHEL installer
│   └── install-alpine.sh           # Alpine Linux installer
├── windows/                        # Windows service
│   └── install.ps1                 # Windows installer
├── data/                           # Volume for database
│   └── posted_hashes.db
└── logs/                           # Volume for logs
    └── rss_bot.log
```

## 📰 Monitored Feeds

### 📰 General News

- G1
- UOL
- Band
- CNN Brasil
- Folha de S.Paulo

### 🏛️ Politics & Conservative

- Gazeta do Povo
- Jovem Pan
- Diário do Poder
- Pragmatismo Político
- Conexão Política
- Poder 360
- Revista Crusoé
- Veja
- Metrópoles
- O Antagonista
- Terra Politics

### 💻 Technology

- Canaltech
- Olhar Digital
- Tecnoblog
- Meio Bit
- ShowMeTech
- TecMundo
- Adrenaline
- Hardware.com.br
- Tudo Celular
- Oficina da Net

## 🔧 Development

### Run Locally (without Docker)

```bash
# Install dependencies
pip install -r requirements.txt

# Configure environment variables
export DISCORD_WEBHOOK_URL="your_webhook_url_here"

# Run the bot
python bot_service.py
```

### Run as System Service

```bash
# For development/testing
python bot_service.py
```

### Customize Feeds

You can customize feeds in two ways:

#### 1. Environment Variables (Recommended)

Use the `RSS_FEEDS_CATEGORY_NAME` format as described in the Configuration section above.

#### 2. Code Modification

Edit the `parse_feeds_from_env()` function in `bot_service.py` to modify default feeds.

## 🐛 Troubleshooting

### Bot Not Posting

1. Check if webhook URL is correct
2. Check logs: `docker compose logs discord-rss-bot`
3. Test webhook manually

### Bot Hanging or Freezing

The bot includes timeout handling to prevent hanging on slow RSS feeds:

- **FEED_TIMEOUT**: Controls how long to wait for each RSS feed (default: 30 seconds)
- If a feed times out, the bot logs a warning and continues with other feeds
- Network errors are handled gracefully without stopping the entire process

### Container Won't Start

1. Verify `DISCORD_WEBHOOK_URL` is configured (via environment or `.env`)
2. Check logs: `docker compose logs`
3. Ensure ports are not in use

### Permission Issues (Docker Volumes)

If you encounter permission errors with Docker volumes:

```bash
# Stop the container first
docker compose down

# Create directories with proper permissions
mkdir -p ~/docker/docker-data/rss-discord-bot/data
mkdir -p ~/docker/docker-data/rss-discord-bot/logs

# Option A: Give full access (simple but less secure)
chmod 777 ~/docker/docker-data/rss-discord-bot/logs
chmod 777 ~/docker/docker-data/rss-discord-bot/data

# Option B: Set proper ownership (more secure)
# First check what user the container runs as:
docker run --rm isyuricunha/discord-rss-bot:latest id
# Then set ownership accordingly (replace 1000:1000 with actual UID:GID)
sudo chown -R 1000:1000 ~/docker/docker-data/rss-discord-bot/

# Start the container again
docker compose up -d
```

### Legacy Permission Fix (for local data/logs directories)

```bash
# Fix volume permissions for local directories
sudo chown -R $USER:$USER data logs
```

### Clean Data

```bash
# Stop the bot
docker compose down

# Remove old data
rm -rf data/* logs/*

# Restart
docker compose up -d
```

### Service Issues (Linux)

```bash
# Check service status
sudo systemctl status discord-rss-bot

# View service logs
sudo journalctl -u discord-rss-bot -f

# Restart service
sudo systemctl restart discord-rss-bot
```

## 📊 Health Check

The container includes a health check that verifies:

- Database connectivity
- Essential file integrity

Status available via:

```bash
docker inspect discord-rss-bot | grep Health -A 10
```

## 🔒 Security

- Container runs with non-root user
- Sensitive variables via `.env` file
- Isolated volumes for data and logs
- Automatic restart on failure
- SELinux support (Fedora/RHEL)
- Systemd security hardening

## 📝 Logs

Logs include:

- Timestamp of each operation
- Status of each feed checked
- Detailed errors and warnings
- Statistics of processed posts

### Log Locations

- **Docker**: `./logs/rss_bot.log`
- **System Service**: `/var/log/discord-rss-bot/rss_bot.log`
- **Windows Service**: `C:\Program Files\DiscordRSSBot\logs\`

## 🤝 Contributing

1. Fork the project
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Open a Pull Request

## 📄 License

This project is licensed under the LGPL-2.1 license. See the LICENSE file for details.
