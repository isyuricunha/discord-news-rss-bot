#!/bin/bash
# Discord RSS Bot - Fedora/RHEL/CentOS Installation Script

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
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   print_error "This script must be run as root (use sudo)"
   exit 1
fi

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --webhook-url)
            WEBHOOK_URL="$2"
            shift 2
            ;;
        --help)
            echo "Usage: $0 --webhook-url <discord_webhook_url>"
            echo "Install Discord RSS Bot as a system service on Fedora/RHEL/CentOS"
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

if [[ -z "$WEBHOOK_URL" ]]; then
    print_error "Discord webhook URL is required. Use --webhook-url parameter"
    exit 1
fi

print_status "Installing Discord RSS Bot on Fedora/RHEL/CentOS..."

# Install Python and dependencies
print_status "Installing Python and system dependencies..."
if command -v dnf &> /dev/null; then
    # Fedora
    dnf install -y python3 python3-pip python3-virtualenv git curl
elif command -v yum &> /dev/null; then
    # RHEL/CentOS
    yum install -y python3 python3-pip git curl
    python3 -m pip install virtualenv
else
    print_error "Neither dnf nor yum package manager found"
    exit 1
fi

# Create service user
print_status "Creating service user: $SERVICE_USER"
if ! id "$SERVICE_USER" &>/dev/null; then
    useradd --system --shell /bin/false --home-dir $DATA_DIR --create-home $SERVICE_USER
fi

# Create directories
print_status "Creating directories..."
mkdir -p $INSTALL_DIR $DATA_DIR $LOG_DIR $CONFIG_DIR
chown $SERVICE_USER:$SERVICE_USER $DATA_DIR $LOG_DIR

# Copy application files
print_status "Installing application files..."
cp bot_service.py $INSTALL_DIR/
cp requirements.txt $INSTALL_DIR/
chown -R $SERVICE_USER:$SERVICE_USER $INSTALL_DIR

# Create Python virtual environment
print_status "Creating Python virtual environment..."
python3 -m venv $INSTALL_DIR/venv
chown -R $SERVICE_USER:$SERVICE_USER $INSTALL_DIR/venv

# Install Python dependencies
print_status "Installing Python dependencies..."
sudo -u $SERVICE_USER $INSTALL_DIR/venv/bin/pip install -r $INSTALL_DIR/requirements.txt

# Create configuration file
print_status "Creating configuration file..."
cat > $CONFIG_DIR/config.env << EOF
# Discord RSS Bot Configuration
DISCORD_WEBHOOK_URL=$WEBHOOK_URL

# Bot Configuration
CHECK_INTERVAL=300
POST_DELAY=3
COOLDOWN_DELAY=60
MAX_POST_LENGTH=1900
MAX_CONTENT_LENGTH=800

# Custom RSS Feeds Configuration
# Format: RSS_FEEDS_CATEGORY_NAME=url1,url2,url3
# Examples (uncomment and modify as needed):
# RSS_FEEDS_NEWS=https://g1.globo.com/dynamo/rss2.xml,https://rss.uol.com.br/feed/noticias.xml
# RSS_FEEDS_TECHNOLOGY=https://canaltech.com.br/rss/,https://tecnoblog.net/feed/
# RSS_FEEDS_SPORTS=https://globoesporte.globo.com/rss/ultimas/
# RSS_FEEDS_BUSINESS=https://www.infomoney.com.br/rss/
# RSS_FEEDS_POLITICS=https://www.gazetadopovo.com.br/rss/brasil.xml,https://jovempan.com.br/rss.xml

# Note: If no custom feeds are configured, the bot will use default Brazilian news feeds
# Category names will automatically get emojis: News(📰), Technology(💻), Politics(🏛️), Sports(⚽), Business(💼), Others(📢)

# System Configuration
RSS_BOT_DATA=$DATA_DIR
RSS_BOT_LOGS=$LOG_DIR
EOF

chmod 600 $CONFIG_DIR/config.env
chown $SERVICE_USER:$SERVICE_USER $CONFIG_DIR/config.env

# Create systemd service file
print_status "Creating systemd service..."
cat > /etc/systemd/system/discord-rss-bot.service << EOF
[Unit]
Description=Discord RSS Bot Service
Documentation=https://github.com/YOUR_USERNAME/discord-rss-bot
After=network.target
Wants=network.target

[Service]
Type=simple
User=$SERVICE_USER
Group=$SERVICE_USER
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/venv/bin/python $INSTALL_DIR/bot_service.py
ExecReload=/bin/kill -HUP \$MAINPID
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=discord-rss-bot

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$DATA_DIR $LOG_DIR
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true

# Environment
Environment=RSS_BOT_DATA=$DATA_DIR
Environment=RSS_BOT_LOGS=$LOG_DIR
Environment=RSS_BOT_CONFIG=$CONFIG_DIR/config.env
Environment=RSS_BOT_PID=/var/run/discord-rss-bot.pid

[Install]
WantedBy=multi-user.target
EOF

# Configure SELinux if enabled
if command -v getenforce &> /dev/null && [[ "$(getenforce)" != "Disabled" ]]; then
    print_status "Configuring SELinux..."
    setsebool -P httpd_can_network_connect 1
    semanage fcontext -a -t bin_t "$INSTALL_DIR/venv/bin/python" 2>/dev/null || true
    restorecon -R $INSTALL_DIR 2>/dev/null || true
fi

# Reload systemd and enable service
print_status "Configuring systemd service..."
systemctl daemon-reload
systemctl enable discord-rss-bot

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
        systemctl reload discord-rss-bot > /dev/null 2>&1 || true
    endscript
}
EOF

print_status "Installation completed successfully!"
echo
print_status "Service Management Commands:"
echo "  Start service:   systemctl start discord-rss-bot"
echo "  Stop service:    systemctl stop discord-rss-bot"
echo "  Service status:  systemctl status discord-rss-bot"
echo "  View logs:       journalctl -u discord-rss-bot -f"
echo
print_status "File Locations:"
echo "  Application:     $INSTALL_DIR"
echo "  Configuration:   $CONFIG_DIR/config.env"
echo "  Data:           $DATA_DIR"
echo "  Logs:           $LOG_DIR"
echo
print_warning "To start the service: systemctl start discord-rss-bot"
print_status "Installation complete!"
