# Use Python 3.11 slim image for smaller size
FROM python:3.11-slim

# Set working directory
WORKDIR /app

# Set environment variables
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    RSS_BOT_DATA=/app/data \
    RSS_BOT_LOGS=/app/logs \
    RSS_BOT_PID=/tmp/discord-rss-bot.pid \
    CHECK_INTERVAL=300 \
    POST_DELAY=3 \
    COOLDOWN_DELAY=60 \
    MAX_POST_LENGTH=1900 \
    MAX_CONTENT_LENGTH=800 \
    FEED_TIMEOUT=30

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements first for better caching
COPY requirements.txt .

# Install Python dependencies
RUN pip install --no-cache-dir -r requirements.txt

# Copy application code
COPY bot_service.py .

# Create directories for database and logs
RUN mkdir -p /app/data /app/logs

# Create non-root user for security
RUN groupadd -r botuser && useradd -r -g botuser botuser
RUN chown -R botuser:botuser /app
USER botuser

# Health check
HEALTHCHECK --interval=5m --timeout=30s --start-period=30s --retries=3 \
    CMD python -c "import sqlite3; conn = sqlite3.connect('/app/data/posted_hashes.db'); conn.close()" || exit 1

# Run the bot
CMD ["python", "bot_service.py"]
