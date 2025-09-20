#!/usr/bin/env python3
"""
Discord RSS Bot - System Service Version
Designed to run as a system service on Linux/Windows
"""

import feedparser
import time
import requests
import hashlib
import sqlite3
from datetime import datetime
import html2text
import logging
import os
import re
import sys
import signal
from pathlib import Path
import urllib.request
import urllib.error
from urllib.parse import urlparse
import socket

# ===================== CONFIGURATION =====================
# Load configurations from environment variables or config file
DISCORD_WEBHOOK_URL = os.getenv('DISCORD_WEBHOOK_URL')
CONFIG_FILE = os.getenv('RSS_BOT_CONFIG', '/etc/discord-rss-bot/config.env')

# Try to load from config file if webhook not in env
if not DISCORD_WEBHOOK_URL and os.path.exists(CONFIG_FILE):
    try:
        with open(CONFIG_FILE, 'r') as f:
            for line in f:
                if line.strip() and not line.startswith('#'):
                    key, value = line.strip().split('=', 1)
                    os.environ[key] = value.strip('"\'')
        DISCORD_WEBHOOK_URL = os.getenv('DISCORD_WEBHOOK_URL')
    except Exception as e:
        print(f"Error loading config file {CONFIG_FILE}: {e}")

def parse_feeds_from_env():
    """Parse RSS feeds from environment variables"""
    feeds = {}
    
    # Check for custom feeds from environment variables
    # Format: RSS_FEEDS_CATEGORY_NAME=url1,url2,url3
    # Example: RSS_FEEDS_NEWS=https://example.com/rss,https://another.com/feed
    for key, value in os.environ.items():
        if key.startswith('RSS_FEEDS_'):
            category_key = key[10:]  # Remove 'RSS_FEEDS_' prefix
            category_name = category_key.replace('_', ' ').title()
            
            # Add emoji based on category name
            if any(word in category_name.lower() for word in ['news', 'noticias', 'general']):
                category_name = f"📰 {category_name}"
            elif any(word in category_name.lower() for word in ['tech', 'technology', 'tecnologia']):
                category_name = f"💻 {category_name}"
            elif any(word in category_name.lower() for word in ['politics', 'politica', 'conservative']):
                category_name = f"🏛️ {category_name}"
            elif any(word in category_name.lower() for word in ['sports', 'esportes']):
                category_name = f"⚽ {category_name}"
            elif any(word in category_name.lower() for word in ['business', 'economia', 'finance']):
                category_name = f"💼 {category_name}"
            else:
                category_name = f"📢 {category_name}"
            
            # Parse URLs (comma-separated)
            urls = [url.strip() for url in value.split(',') if url.strip()]
            if urls:
                feeds[category_name] = urls
    
    # If no custom feeds are configured, use default feeds
    if not feeds:
        feeds = {
            "📰 General News": [
                "https://g1.globo.com/dynamo/rss2.xml",                    # G1 - Working
                "https://rss.uol.com.br/feed/noticias.xml",                # UOL - Has issues but works
                "https://www.band.uol.com.br/rss/noticias.xml",            # Band
                "https://www.cnnbrasil.com.br/rss/",                       # CNN Brasil
                "https://feeds.folha.uol.com.br/folha/rss02.xml",          # Folha - Alternative feed
            ],
            "🏛️ Politics & Conservative": [
                "https://www.gazetadopovo.com.br/rss/brasil.xml",          # Gazeta do Povo - Brazil Feed
                "https://jovempan.com.br/rss.xml",                         # Jovem Pan - Alternative feed
                "https://www.diariodopoder.com.br/feed/",                  # Diário do Poder - Working
                "https://www.pragmatismopolitico.com.br/feed/",            # Pragmatismo - Working
                "https://conexaopolitica.com.br/feed/",                    # Conexão Política - Working
                "https://www.poder360.com.br/feed/",                       # Poder 360
                "https://crusoe.uol.com.br/rss/",                          # Revista Crusoé
                "https://veja.abril.com.br/rss/",                          # Veja
                "https://www.metropoles.com/rss.xml",                      # Metrópoles
                "https://www.oantagonista.com/rss/",                       # O Antagonista
                "https://www.terra.com.br/rss/politica/",                  # Terra Politics
            ],
            "💻 Technology": [
                "https://canaltech.com.br/rss/",                           # Canaltech - Working
                "https://olhardigital.com.br/feed/",                       # Olhar Digital - Working
                "https://tecnoblog.net/feed/",                              # Tecnoblog - Working
                "https://meiobit.com/feed/",                                # Meio Bit - Working
                "https://www.showmetech.com.br/feed/",                     # Showmetech - Working
                "https://www.tecmundo.com.br/rss",                         # TecMundo
                "https://www.adrenaline.com.br/rss/",                      # Adrenaline
                "https://www.hardware.com.br/rss/",                        # Hardware.com.br
                "https://www.tudocelular.com/rss/",                        # Tudo Celular
                "https://www.oficinadanet.com.br/rss",                     # Oficina da Net
            ]
        }
    
    return feeds

# Parse feeds from environment or use defaults
FEEDS = parse_feeds_from_env()

CHECK_INTERVAL = int(os.getenv('CHECK_INTERVAL', '300'))
POST_DELAY = int(os.getenv('POST_DELAY', '3'))
COOLDOWN_DELAY = int(os.getenv('COOLDOWN_DELAY', '60'))
MAX_POST_LENGTH = int(os.getenv('MAX_POST_LENGTH', '1900'))
MAX_CONTENT_LENGTH = int(os.getenv('MAX_CONTENT_LENGTH', '800'))
FEED_TIMEOUT = int(os.getenv('FEED_TIMEOUT', '30'))  # Timeout for RSS feed requests

# Service-specific paths
if os.name == 'nt':  # Windows
    DATA_DIR = os.getenv('RSS_BOT_DATA', os.path.expanduser('~\\AppData\\Local\\DiscordRSSBot'))
    LOG_DIR = os.getenv('RSS_BOT_LOGS', os.path.expanduser('~\\AppData\\Local\\DiscordRSSBot\\logs'))
else:  # Linux/Unix
    DATA_DIR = os.getenv('RSS_BOT_DATA', '/var/lib/discord-rss-bot')
    LOG_DIR = os.getenv('RSS_BOT_LOGS', '/var/log/discord-rss-bot')

DB_FILE = os.path.join(DATA_DIR, 'posted_hashes.db')
LOG_FILE = os.path.join(LOG_DIR, 'rss_bot.log')
PID_FILE = os.getenv('RSS_BOT_PID', '/var/run/discord-rss-bot.pid')

# ===================== LOGGING =====================
# Create directories if they don't exist
os.makedirs(DATA_DIR, exist_ok=True)
os.makedirs(LOG_DIR, exist_ok=True)

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s',
    handlers=[
        logging.FileHandler(LOG_FILE, encoding='utf-8'),
        logging.StreamHandler()
    ]
)
logger = logging.getLogger(__name__)

# ===================== SERVICE MANAGEMENT =====================
class ServiceManager:
    def __init__(self):
        self.running = True
        signal.signal(signal.SIGTERM, self.signal_handler)
        signal.signal(signal.SIGINT, self.signal_handler)
        if hasattr(signal, 'SIGHUP'):
            signal.signal(signal.SIGHUP, self.signal_handler)

    def signal_handler(self, signum, frame):
        logger.info(f"Received signal {signum}, shutting down gracefully...")
        self.running = False

    def write_pid_file(self):
        try:
            with open(PID_FILE, 'w') as f:
                f.write(str(os.getpid()))
            logger.info(f"PID file written: {PID_FILE}")
        except Exception as e:
            logger.warning(f"Could not write PID file: {e}")

    def remove_pid_file(self):
        try:
            if os.path.exists(PID_FILE):
                os.remove(PID_FILE)
                logger.info("PID file removed")
        except Exception as e:
            logger.warning(f"Could not remove PID file: {e}")

# ===================== BOT FUNCTIONS =====================
def init_db():
    """Initialize database with proper error handling"""
    try:
        conn = sqlite3.connect(DB_FILE)
        c = conn.cursor()
        
        # Check if table already exists and its columns
        c.execute("PRAGMA table_info(posted)")
        columns = [column[1] for column in c.fetchall()]
        
        if not columns:
            # Table doesn't exist, create new one
            c.execute("""
                CREATE TABLE posted (
                    hash TEXT PRIMARY KEY,
                    posted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                    title TEXT,
                    source TEXT
                )
            """)
        elif 'title' not in columns or 'source' not in columns:
            # Table exists but without new columns, add them
            if 'title' not in columns:
                c.execute("ALTER TABLE posted ADD COLUMN title TEXT")
            if 'source' not in columns:
                c.execute("ALTER TABLE posted ADD COLUMN source TEXT")
        
        conn.commit()
        return conn
    except Exception as e:
        logger.error(f"Database initialization failed: {e}")
        raise

def has_posted(conn, entry_hash):
    c = conn.cursor()
    c.execute("SELECT 1 FROM posted WHERE hash = ?", (entry_hash,))
    return c.fetchone() is not None

def mark_posted(conn, entry_hash, title, source):
    c = conn.cursor()
    c.execute(
        "INSERT INTO posted (hash, title, source) VALUES (?, ?, ?)", 
        (entry_hash, title, source)
    )
    conn.commit()

def hash_entry(entry):
    h = hashlib.sha256()
    text_for_hash = entry.get("title", "") + entry.get("link", "")
    h.update(text_for_hash.encode("utf-8"))
    return h.hexdigest()

def clean_html(html_content):
    if not html_content:
        return ""
    
    h = html2text.HTML2Text()
    h.ignore_links = True
    h.ignore_images = True
    h.body_width = 0
    text = h.handle(html_content).strip()
    
    # Clean excessive line breaks
    text = re.sub(r'\n\s*\n', '\n\n', text)
    text = re.sub(r'\n{3,}', '\n\n', text)
    
    return text

def get_source_name(feed_url):
    """Extract friendly source name"""
    domain_mapping = {
        'g1.globo.com': 'G1',
        'rss.uol.com.br': 'UOL',
        'band.uol.com.br': 'Band',
        'cnnbrasil.com.br': 'CNN Brasil',
        'feeds.folha.uol.com.br': 'Folha',
        'gazetadopovo.com.br': 'Gazeta do Povo',
        'jovempan.com.br': 'Jovem Pan',
        'diariodopoder.com.br': 'Diário do Poder',
        'pragmatismopolitico.com.br': 'Pragmatismo Político',
        'conexaopolitica.com.br': 'Conexão Política',
        'poder360.com.br': 'Poder 360',
        'crusoe.uol.com.br': 'Crusoé',
        'veja.abril.com.br': 'Veja',
        'metropoles.com': 'Metrópoles',
        'oantagonista.com': 'O Antagonista',
        'terra.com.br': 'Terra',
        'canaltech.com.br': 'Canaltech',
        'olhardigital.com.br': 'Olhar Digital',
        'tecnoblog.net': 'Tecnoblog',
        'meiobit.com': 'Meio Bit',
        'showmetech.com.br': 'ShowMeTech',
        'tecmundo.com.br': 'TecMundo',
        'adrenaline.com.br': 'Adrenaline',
        'hardware.com.br': 'Hardware.com.br',
        'tudocelular.com': 'Tudo Celular',
        'oficinadanet.com.br': 'Oficina da Net',
    }
    
    for domain, name in domain_mapping.items():
        if domain in feed_url:
            return name
    
    # Fallback: extract domain
    try:
        from urllib.parse import urlparse
        domain = urlparse(feed_url).netloc
        return domain.replace('www.', '').split('.')[0].title()
    except:
        return "Source"

def get_category_emoji(feed_url):
    """Returns emoji based on feed category"""
    for category, urls in FEEDS.items():
        if feed_url in urls:
            return category.split()[0]  # Get only the emoji
    return "📢"

def format_post(entry, source_name, category_emoji):
    title = entry.get("title", "No title").strip()
    link = entry.get("link", "")
    content = ""

    # Search for content
    if hasattr(entry, 'content') and entry.content:
        content = entry.content[0].value
    elif "summary" in entry:
        content = entry.summary
    elif "description" in entry:
        content = entry.description

    text_content = clean_html(content)
    
    # Limit content size
    if len(text_content) > MAX_CONTENT_LENGTH:
        text_content = text_content[:MAX_CONTENT_LENGTH] + "..."

    # Improved formatting
    post_text = (
        f"╭─────────────────────────────────╮\n"
        f"│  {category_emoji} **{title}**\n"
        f"├─────────────────────────────────┤\n"
        f"│ 🔗 {link}\n"
        f"│ 📰 {source_name}\n"
    )
    
    if text_content:
        post_text += f"├─────────────────────────────────┤\n"
        post_text += f"│ {text_content}\n"
    
    post_text += f"╰─────────────────────────────────╯"

    if len(post_text) > MAX_POST_LENGTH:
        # Cut content if still too large
        available_space = MAX_POST_LENGTH - len(post_text) + len(text_content) - 20
        if available_space > 50:
            short_content = text_content[:available_space] + "..."
            post_text = (
                f"╭─────────────────────────────────╮\n"
                f"│  {category_emoji} **{title}**\n"
                f"├─────────────────────────────────┤\n"
                f"│ 🔗 {link}\n"
                f"│ 📰 {source_name}\n"
                f"├─────────────────────────────────┤\n"
                f"│ {short_content}\n"
                f"╰─────────────────────────────────╯"
            )
        else:
            # Minimalist version if still too large
            post_text = (
                f"{category_emoji} **{title}**\n\n"
                f"🔗 {link}\n"
                f"📰 {source_name}"
            )

    return post_text

def post_to_discord(text):
    if not text.strip():
        logger.warning("Empty message, will not post.")
        return False

    data = {"content": text}
    try:
        res = requests.post(DISCORD_WEBHOOK_URL, json=data, timeout=15)
        if res.status_code == 429:
            logger.warning(f"Discord rate limit reached! Waiting {COOLDOWN_DELAY}s...")
            time.sleep(COOLDOWN_DELAY)
            return False
        res.raise_for_status()
        logger.info("✅ Successfully posted to Discord!")
        return True
    except requests.exceptions.RequestException as e:
        logger.error(f"Error posting to Discord: {e}")
        return False

def fetch_feed_with_timeout(feed_url, timeout=FEED_TIMEOUT):
    """Fetch RSS feed with proper timeout handling"""
    try:
        # Set socket timeout globally for this operation
        old_timeout = socket.getdefaulttimeout()
        socket.setdefaulttimeout(timeout)
        
        # Create a custom opener with timeout
        opener = urllib.request.build_opener()
        opener.addheaders = [('User-Agent', 'Mozilla/5.0 (compatible; Discord RSS Bot/2.0)')]
        
        # Fetch the feed with timeout
        with opener.open(feed_url, timeout=timeout) as response:
            feed_data = response.read()
        
        # Parse the feed data
        feed = feedparser.parse(feed_data)
        
        return feed, None
        
    except (urllib.error.URLError, socket.timeout, socket.error) as e:
        return None, f"Network timeout or error: {e}"
    except Exception as e:
        return None, f"Unexpected error: {e}"
    finally:
        # Restore original timeout
        socket.setdefaulttimeout(old_timeout)

def check_feeds(conn):
    total_new = 0
    
    for category, feed_urls in FEEDS.items():
        category_new = 0
        logger.info(f"🔍 Checking category: {category}")
        
        for feed_url in feed_urls:
            try:
                logger.info(f"   📡 {get_source_name(feed_url)}...")
                
                # Fetch feed with timeout
                feed, error = fetch_feed_with_timeout(feed_url)
                
                if error:
                    logger.warning(f"   ⚠️  Feed timeout/error: {error}")
                    continue
                
                if not feed or not hasattr(feed, 'entries'):
                    logger.warning(f"   ⚠️  Invalid feed data received")
                    continue
                
                if feed.bozo and hasattr(feed, 'bozo_exception'):
                    logger.warning(f"   ⚠️  Feed has issues: {feed.bozo_exception}")
                
                source_name = get_source_name(feed_url)
                category_emoji = get_category_emoji(feed_url)
                
                # Process only the 5 most recent posts from each feed
                for entry in feed.entries[:5]:
                    entry_hash = hash_entry(entry)
                    if not has_posted(conn, entry_hash):
                        post_text = format_post(entry, source_name, category_emoji)
                        
                        if post_to_discord(post_text):
                            mark_posted(conn, entry_hash, entry.get("title", ""), source_name)
                            category_new += 1
                            total_new += 1
                            time.sleep(POST_DELAY)
                        else:
                            time.sleep(POST_DELAY)
                            
            except Exception as e:
                logger.error(f"   ❌ Error processing {feed_url}: {e}")
                continue
        
        if category_new > 0:
            logger.info(f"   📌 {category_new} new articles from {category}")
    
    logger.info(f"🎯 Total: {total_new} new articles processed.")
    return total_new

def cleanup_old_entries(conn, days=30):
    """Remove old entries from database to prevent indefinite growth"""
    c = conn.cursor()
    c.execute(
        "DELETE FROM posted WHERE posted_at < datetime('now', '-{} days')".format(days)
    )
    deleted = c.rowcount
    conn.commit()
    if deleted > 0:
        logger.info(f"🧹 Removed {deleted} old entries from database.")

def main():
    """Main service function"""
    service_manager = ServiceManager()
    
    logger.info("🚀 Discord RSS Bot Service started.")
    logger.info(f"📊 Monitoring {sum(len(urls) for urls in FEEDS.values())} RSS feeds")
    logger.info(f"🔧 Settings: CHECK_INTERVAL={CHECK_INTERVAL}s, POST_DELAY={POST_DELAY}s")
    logger.info(f"💾 Database: {DB_FILE}")
    logger.info(f"📝 Logs: {LOG_FILE}")
    
    # Check if webhook is configured
    if not DISCORD_WEBHOOK_URL:
        logger.error("❌ DISCORD_WEBHOOK_URL is not configured!")
        logger.error("Set environment variable or create config file at /etc/discord-rss-bot/config.env")
        return 1
    
    # Write PID file for service management
    service_manager.write_pid_file()
    
    try:
        conn = init_db()
        loop_count = 0
        
        while service_manager.running:
            loop_count += 1
            logger.info(f"🔄 Cycle {loop_count} - {datetime.now().strftime('%H:%M:%S')}")
            
            new_posts = check_feeds(conn)
            
            # Database cleanup every 24 cycles (approximately 2 hours if CHECK_INTERVAL=300)
            if loop_count % 24 == 0:
                cleanup_old_entries(conn)
            
            if new_posts == 0:
                logger.info("😴 No new articles. Waiting for next cycle...")
            
            # Sleep in small intervals to allow for graceful shutdown
            sleep_time = CHECK_INTERVAL
            while sleep_time > 0 and service_manager.running:
                time.sleep(min(10, sleep_time))
                sleep_time -= 10
                
    except KeyboardInterrupt:
        logger.info("🛑 Bot interrupted by user.")
    except Exception as e:
        logger.error(f"💥 Fatal error: {e}")
        return 1
    finally:
        if 'conn' in locals():
            conn.close()
        service_manager.remove_pid_file()
        logger.info("🔚 Bot service finished.")
    
    return 0

if __name__ == "__main__":
    sys.exit(main())
