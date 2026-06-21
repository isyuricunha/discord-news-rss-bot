#!/usr/bin/env bash
set -euo pipefail

PURGE=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --purge)
      PURGE=1
      shift
      ;;
    --help|-h)
      echo "Usage: sudo scripts/uninstall-linux.sh [--purge]"
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      exit 1
      ;;
  esac
done

if [[ "$(id -u)" -ne 0 ]]; then
  echo "This uninstaller must run as root." >&2
  exit 1
fi

systemctl stop discord-rss-bot 2>/dev/null || true
systemctl disable discord-rss-bot 2>/dev/null || true
rm -f /etc/systemd/system/discord-rss-bot.service
systemctl daemon-reload
rm -f /usr/local/bin/discord-rss-bot

if [[ "$PURGE" -eq 1 ]]; then
  rm -rf /etc/discord-rss-bot /var/lib/discord-rss-bot
fi

echo "Uninstalled discord-rss-bot."
