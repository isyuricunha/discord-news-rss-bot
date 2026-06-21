#!/usr/bin/env bash
set -euo pipefail

SERVICE_NAME="discord-rss-bot"
BINARY_PATH=""
WEBHOOK_URL=""
VERSION=""
INSTALL_BIN="/usr/local/bin/discord-rss-bot"
CONFIG_DIR="/etc/discord-rss-bot"
CONFIG_FILE="$CONFIG_DIR/config.env"
DATA_DIR="/var/lib/discord-rss-bot"

usage() {
  cat <<'USAGE'
Usage:
  sudo scripts/install-linux.sh --binary ./discord-rss-bot [--webhook-url URL]
  sudo scripts/install-linux.sh --version v3.0.0 [--webhook-url URL]

Installs the Go binary and a hardened systemd service. When --version is used,
the script downloads a release archive for linux/amd64 or linux/arm64 and
extracts only the discord-rss-bot binary.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --binary)
      BINARY_PATH="${2:-}"
      shift 2
      ;;
    --webhook-url)
      WEBHOOK_URL="${2:-}"
      shift 2
      ;;
    --version)
      VERSION="${2:-}"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if [[ "$(id -u)" -ne 0 ]]; then
  echo "This installer must run as root." >&2
  exit 1
fi

detect_platform() {
  case "$(uname -m)" in
    x86_64|amd64) echo "linux-amd64" ;;
    aarch64|arm64) echo "linux-arm64" ;;
    *) echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
  esac
}

download_release() {
  local platform archive tmpdir url
  platform="$(detect_platform)"
  archive="discord-rss-bot-${VERSION#v}-${platform}.tar.gz"
  url="https://github.com/isyuricunha/discord-news-rss-bot/releases/download/${VERSION}/${archive}"
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$tmpdir/$archive"
  elif command -v wget >/dev/null 2>&1; then
    wget -q "$url" -O "$tmpdir/$archive"
  else
    echo "curl or wget is required when using --version." >&2
    exit 1
  fi
  tar -xzf "$tmpdir/$archive" -C "$tmpdir"
  BINARY_PATH="$tmpdir/discord-rss-bot"
}

if [[ -n "$VERSION" ]]; then
  download_release
fi

if [[ -z "$BINARY_PATH" || ! -f "$BINARY_PATH" ]]; then
  echo "Provide --binary with a built discord-rss-bot executable or --version with a release tag." >&2
  exit 1
fi

install -d -m 0755 "$(dirname "$INSTALL_BIN")"
install -m 0755 "$BINARY_PATH" "$INSTALL_BIN"
install -d -m 0750 "$CONFIG_DIR"
install -d -m 0750 "$DATA_DIR"

if [[ ! -f "$CONFIG_FILE" ]]; then
  umask 077
  {
    echo "# Discord RSS Bot configuration"
    if [[ -n "$WEBHOOK_URL" ]]; then
      printf 'DISCORD_WEBHOOK_URL=%s\n' "$WEBHOOK_URL"
    else
      echo "DISCORD_WEBHOOK_URL="
    fi
    echo "RSS_BOT_DATA=$DATA_DIR"
    echo "INITIAL_SYNC_MODE=skip"
    echo "LOG_LEVEL=info"
    echo "LOG_FORMAT=text"
  } > "$CONFIG_FILE"
fi
chmod 0600 "$CONFIG_FILE"

install -m 0644 systemd/discord-rss-bot.service /etc/systemd/system/discord-rss-bot.service
systemctl daemon-reload
systemctl enable "$SERVICE_NAME"

echo "Installed $SERVICE_NAME."
echo "Configuration: $CONFIG_FILE"
echo "Data: $DATA_DIR"
echo "Start with: systemctl start $SERVICE_NAME"
