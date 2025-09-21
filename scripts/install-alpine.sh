#!/bin/sh
# Discord RSS Bot - Alpine Linux Installation Script

set -e

WEBHOOK_URL=""
INSTALL_DIR="/opt/discord-rss-bot"
SERVICE_USER="discord-rss-bot"
DATA_DIR="/var/lib/discord-rss-bot"
LOG_DIR="/var/log/discord-rss-bot"
CONFIG_DIR="/etc/discord-rss-bot"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

print_status() {
    printf "${GREEN}[INFO]${NC} %s\n" "$1"
}

print_warning() {
    printf "${YELLOW}[WARNING]${NC} %s\n" "$1"
}

print_error() {
    printf "${RED}[ERROR]${NC} %s\n" "$1"
}

# Check if running as root
if [ "$(id -u)" -ne 0 ]; then
   print_error "This script must be run as root (use sudo)"
   exit 1
fi

# Parse command line arguments
while [ $# -gt 0 ]; do
    case $1 in
        --webhook-url)
            WEBHOOK_URL="$2"
            shift 2
            ;;
        --help)
            echo "Usage: $0 --webhook-url <discord_webhook_url>"
            echo "Install Discord RSS Bot as a system service on Alpine Linux"
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

if [ -z "$WEBHOOK_URL" ]; then
    print_error "Discord webhook URL is required. Use --webhook-url parameter"
    exit 1
fi

print_status "Installing Discord RSS Bot on Alpine Linux..."

# Update package index
print_status "Updating package index..."
apk update

# Install Python and dependencies
print_status "Installing Python and system dependencies..."
apk add --no-cache python3 py3-pip py3-virtualenv git curl openrc

# Create service user
print_status "Creating service user: $SERVICE_USER"
if ! id "$SERVICE_USER" >/dev/null 2>&1; then
    adduser -S -D -H -h "$DATA_DIR" -s /bin/false "$SERVICE_USER"
fi

# Create directories
print_status "Creating directories..."
mkdir -p "$INSTALL_DIR" "$DATA_DIR" "$LOG_DIR" "$CONFIG_DIR"
chown "$SERVICE_USER:$SERVICE_USER" "$DATA_DIR" "$LOG_DIR"

# Copy application files
print_status "Installing application files..."
cp bot_service.py "$INSTALL_DIR/"
cp requirements.txt "$INSTALL_DIR/"
chown -R "$SERVICE_USER:$SERVICE_USER" "$INSTALL_DIR"

# Create Python virtual environment
print_status "Creating Python virtual environment..."
python3 -m venv "$INSTALL_DIR/venv"
chown -R "$SERVICE_USER:$SERVICE_USER" "$INSTALL_DIR/venv"

# Install Python dependencies
print_status "Installing Python dependencies..."
su "$SERVICE_USER" -s /bin/sh -c "$INSTALL_DIR/venv/bin/pip install -r $INSTALL_DIR/requirements.txt"

# Create configuration file
print_status "Creating configuration file..."
cat > "$CONFIG_DIR/config.env" << EOF
# Discord RSS Bot Configuration
DISCORD_WEBHOOK_URL=$WEBHOOK_URL

# Bot Configuration
CHECK_INTERVAL=300
POST_DELAY=3
COOLDOWN_DELAY=60
MAX_POST_LENGTH=1900
MAX_CONTENT_LENGTH=800

# Custom RSS Feeds Configuration
# Option 1: Universal feeds (simple, all feeds in one variable)
# RSS_FEEDS=https://example.com/rss,https://another.com/feed,https://third.com/rss

# Option 2: Category-based feeds (organized by categories with emojis)
# RSS_FEEDS_NEWS=https://g1.globo.com/dynamo/rss2.xml,https://rss.uol.com.br/feed/noticias.xml
# RSS_FEEDS_TECHNOLOGY=https://canaltech.com.br/rss/,https://tecnoblog.net/feed/
# RSS_FEEDS_SPORTS=https://globoesporte.globo.com/rss/ultimas/
# RSS_FEEDS_BUSINESS=https://www.infomoney.com.br/rss/
# RSS_FEEDS_POLITICS=https://www.gazetadopovo.com.br/rss/brasil.xml,https://jovempan.com.br/rss.xml

# Note: RSS_FEEDS takes priority over category-based feeds
# If no custom feeds are configured, the bot will use default Brazilian news feeds
# Category names automatically get emojis: News(📰), Technology(💻), Politics(🏛️), Sports(⚽), Business(💼), Others(📢)

# System Configuration
RSS_BOT_DATA=$DATA_DIR
RSS_BOT_LOGS=$LOG_DIR
EOF

chmod 600 "$CONFIG_DIR/config.env"
chown "$SERVICE_USER:$SERVICE_USER" "$CONFIG_DIR/config.env"

# Create OpenRC service script
print_status "Creating OpenRC service..."
cat > /etc/init.d/discord-rss-bot << 'EOF'
#!/sbin/openrc-run

name="Discord RSS Bot"
description="Automated Discord RSS Bot that monitors Brazilian news feeds"

command="/opt/discord-rss-bot/venv/bin/python"
command_args="/opt/discord-rss-bot/bot_service.py"
command_user="discord-rss-bot:discord-rss-bot"
command_background=true
pidfile="/var/run/discord-rss-bot.pid"

directory="/opt/discord-rss-bot"

output_log="/var/log/discord-rss-bot/service.log"
error_log="/var/log/discord-rss-bot/service_error.log"

depend() {
    need net
    after firewall
}

start_pre() {
    # Export environment variables
    export RSS_BOT_DATA="/var/lib/discord-rss-bot"
    export RSS_BOT_LOGS="/var/log/discord-rss-bot"
    export RSS_BOT_CONFIG="/etc/discord-rss-bot/config.env"
    export RSS_BOT_PID="/var/run/discord-rss-bot.pid"
    
    # Ensure log directory exists
    checkpath --directory --owner discord-rss-bot:discord-rss-bot --mode 0755 \
        /var/log/discord-rss-bot /var/lib/discord-rss-bot
}
EOF

chmod +x /etc/init.d/discord-rss-bot

# Enable service
print_status "Enabling service..."
rc-update add discord-rss-bot default

# Create logrotate configuration
print_status "Setting up log rotation..."
cat > /etc/logrotate.d/discord-rss-bot << EOF
$LOG_DIR/*.log {
    daily
    missingok
    rotate 30
    compress
    delaycompress
    notifempty
    create 644 $SERVICE_USER $SERVICE_USER
    postrotate
        /etc/init.d/discord-rss-bot reload > /dev/null 2>&1 || true
    endscript
}
EOF

print_status "Installation completed successfully!"
echo
print_status "Service Management Commands:"
echo "  Start service:   rc-service discord-rss-bot start"
echo "  Stop service:    rc-service discord-rss-bot stop"
echo "  Service status:  rc-service discord-rss-bot status"
echo "  Restart service: rc-service discord-rss-bot restart"
echo
print_status "File Locations:"
echo "  Application:     $INSTALL_DIR"
echo "  Configuration:   $CONFIG_DIR/config.env"
echo "  Data:           $DATA_DIR"
echo "  Logs:           $LOG_DIR"
echo
print_warning "To start the service: rc-service discord-rss-bot start"
print_status "Installation complete!"
