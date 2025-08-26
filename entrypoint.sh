#!/bin/sh
set -e

# 目标挂载目录
DATA_DIR="/anheyu/data"
DEFAULT_DIR="/anheyu/default-data"

mkdir -p "$DATA_DIR/geoip"

# GeoIP 数据填充
if [ ! -f "$DATA_DIR/geoip/GeoLite2-City.mmdb" ] && [ -f "$DEFAULT_DIR/geoip/GeoLite2-City.mmdb" ]; then
  cp -f "$DEFAULT_DIR/geoip/GeoLite2-City.mmdb" "$DATA_DIR/geoip/GeoLite2-City.mmdb"
  echo "[entrypoint] Seeded GeoIP database to $DATA_DIR/geoip/GeoLite2-City.mmdb"
fi

# DefaultArticle.md 填充
if [ ! -f "$DATA_DIR/DefaultArticle.md" ] && [ -f "$DEFAULT_DIR/DefaultArticle.md" ]; then
  cp -f "$DEFAULT_DIR/DefaultArticle.md" "$DATA_DIR/DefaultArticle.md"
  echo "[entrypoint] Seeded DefaultArticle.md to $DATA_DIR/DefaultArticle.md"
fi

# conf.ini 填充
if [ ! -f "$DATA_DIR/conf.ini" ] && [ -f "$DEFAULT_DIR/conf.ini" ]; then
  cp -f "$DEFAULT_DIR/conf.ini" "$DATA_DIR/conf.ini"
  echo "[entrypoint] Seeded conf.ini to $DATA_DIR/conf.ini"
fi

exec "$@"


