#!/bin/sh
set -e

DATA_DIR="/anheyu/data"
DEFAULT_DIR="/anheyu/default-data"
ANHEYU_UID=1001
ANHEYU_GID=1001

# Ensure data directory exists and is writable by the anheyu user
mkdir -p "$DATA_DIR"
chown -R "$ANHEYU_UID:$ANHEYU_GID" "$DATA_DIR" 2>/dev/null || true
chown -R "$ANHEYU_UID:$ANHEYU_GID" /anheyu/themes 2>/dev/null || true
chown -R "$ANHEYU_UID:$ANHEYU_GID" /anheyu/frontend 2>/dev/null || true

# Seed default files
if [ ! -f "$DATA_DIR/DefaultArticle.md" ] && [ -f "$DEFAULT_DIR/DefaultArticle.md" ]; then
  cp -f "$DEFAULT_DIR/DefaultArticle.md" "$DATA_DIR/DefaultArticle.md"
  echo "[entrypoint] Seeded DefaultArticle.md to $DATA_DIR/DefaultArticle.md"
fi

if [ ! -f "$DATA_DIR/DefaultArticle.html" ] && [ -f "$DEFAULT_DIR/DefaultArticle.html" ]; then
  cp -f "$DEFAULT_DIR/DefaultArticle.html" "$DATA_DIR/DefaultArticle.html"
  echo "[entrypoint] Seeded DefaultArticle.html to $DATA_DIR/DefaultArticle.html"
fi

if [ ! -f "$DATA_DIR/conf.ini" ] && [ -f "$DEFAULT_DIR/conf.ini" ]; then
  cp -f "$DEFAULT_DIR/conf.ini" "$DATA_DIR/conf.ini"
  echo "[entrypoint] Seeded conf.ini to $DATA_DIR/conf.ini"
fi

# Drop privileges: run the application as the non-root anheyu user
exec su-exec "$ANHEYU_UID:$ANHEYU_GID" "$@"
