# 🤖 Discord RSS Bot

Automated bot that monitors RSS feeds and posts updates to Discord channels via webhook. Fully configurable with custom feeds and categories.

## 📋 Features

- ✅ **Configurable RSS feeds** via environment variables
- 🔄 Automatic checking for new posts
- 📱 Elegant Discord message formatting with categories
- 💾 Cache system to avoid duplicate posts
- 🧹 Automatic cleanup of old data
- 🐳 Fully containerized with Docker
- 📊 Detailed logging and health checks
- 🏗️ Multiple deployment options (Docker, System Service)
- 📦 Pre-built container images available
- 🎯 **Multi-category support** with automatic emoji assignment
- 🌐 **Multi-platform support** (AMD64, ARM64)

## 🚀 Quick Start

### Using Pre-built Container (Recommended)

```bash
# Create directories
mkdir -p data logs

# Create .env file with your webhook URL
echo "DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/YOUR_ID/YOUR_TOKEN" > .env

# Run with pre-built image
docker run -d \
  --name discord-rss-bot \
  --env-file .env \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/logs:/app/logs \
  --restart unless-stopped \
  ghcr.io/YOUR_USERNAME/discord-rss-bot:latest
```

### Using Docker Compose

```bash
# Clone repository
git clone https://github.com/YOUR_USERNAME/discord-rss-bot
cd discord-rss-bot

# Configure environment
cp .env.example .env
# Edit .env with your Discord webhook URL

# Run with Docker Compose
docker-compose up -d

# View logs
docker-compose logs -f
```

## 📦 Container Images

Pre-built images are available from multiple registries:

- **GitHub Container Registry**: `ghcr.io/YOUR_USERNAME/discord-rss-bot:latest`
- **Docker Hub**: `YOUR_USERNAME/discord-rss-bot:latest`

### Supported Architectures
- `linux/amd64` (x86_64)
- `linux/arm64` (ARM64/AArch64)

## 🐳 Docker Deployment Options

### 1. Standard Docker Compose (with .env file)
```bash
docker-compose up -d
```

### 2. Inline Environment Variables
```bash
# Edit docker-compose.inline.yml with your webhook URL
docker-compose -f docker-compose.inline.yml up -d
```

### 3. Pre-built Image
```bash
# Edit docker-compose.prebuilt.yml with your username
docker-compose -f docker-compose.prebuilt.yml up -d
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
   - `DOCKER_USERNAME`: Your Docker Hub username
   - `DOCKER_PASSWORD`: Your Docker Hub password/token
2. Update `.github/workflows/docker-hub.yml` with your username
3. Push to trigger the build

## ⚙️ Configuration

### Required Variables
| Variable | Description |
|----------|-------------|
| `DISCORD_WEBHOOK_URL` | **Required**: Discord webhook URL |

### Optional Bot Settings
| Variable | Default | Description |
|----------|---------|-------------|
| `CHECK_INTERVAL` | 300 | Interval between checks (seconds) |
| `POST_DELAY` | 3 | Delay between posts (seconds) |
| `COOLDOWN_DELAY` | 60 | Delay after rate limit (seconds) |
| `MAX_POST_LENGTH` | 1900 | Maximum message length |
| `MAX_CONTENT_LENGTH` | 800 | Maximum content length |

### Custom RSS Feeds Configuration

You can configure custom RSS feeds using environment variables with the format:
```
RSS_FEEDS_CATEGORY_NAME=url1,url2,url3
```

#### Examples:
```bash
# News feeds
RSS_FEEDS_NEWS=https://g1.globo.com/dynamo/rss2.xml,https://rss.uol.com.br/feed/noticias.xml

# Technology feeds  
RSS_FEEDS_TECHNOLOGY=https://canaltech.com.br/rss/,https://tecnoblog.net/feed/

# Sports feeds
RSS_FEEDS_SPORTS=https://globoesporte.globo.com/rss/ultimas/

# Business feeds
RSS_FEEDS_BUSINESS=https://www.infomoney.com.br/rss/

# Custom category
RSS_FEEDS_MY_CATEGORY=https://example.com/rss,https://another.com/feed
```

#### Automatic Category Emojis:
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

# Optional - Custom feeds
RSS_FEEDS_NEWS=https://g1.globo.com/dynamo/rss2.xml,https://rss.uol.com.br/feed/noticias.xml
RSS_FEEDS_TECHNOLOGY=https://canaltech.com.br/rss/,https://tecnoblog.net/feed/
RSS_FEEDS_SPORTS=https://globoesporte.globo.com/rss/ultimas/
```

### Default Feeds
If no custom feeds are configured, the bot uses default Brazilian news feeds across multiple categories.

## 📁 File Structure

```
discord-rss-bot/
├── bot.py                          # Main bot code (Docker version)
├── bot_service.py                  # System service version
├── requirements.txt                # Python dependencies
├── Dockerfile                      # Container configuration
├── docker-compose.yml              # Docker orchestration
├── docker-compose.inline.yml       # Inline environment variables
├── docker-compose.prebuilt.yml     # Pre-built image version
├── .env                            # Environment variables
├── .env.example                    # Configuration example
├── .dockerignore                   # Files ignored in build
├── .github/workflows/              # CI/CD workflows
│   ├── docker-publish.yml          # GitHub Container Registry
│   └── docker-hub.yml              # Docker Hub publishing
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
python bot.py
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
2. Check logs: `docker-compose logs discord-rss-bot`
3. Test webhook manually

### Container Won't Start
1. Verify `.env` file exists and is configured
2. Check logs: `docker-compose logs`
3. Ensure ports are not in use

### Permission Issues
```bash
# Fix volume permissions
sudo chown -R $USER:$USER data logs
```

### Clean Data
```bash
# Stop the bot
docker-compose down

# Remove old data
rm -rf data/* logs/*

# Restart
docker-compose up -d
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

This project is licensed under the MIT License. See the LICENSE file for details.
