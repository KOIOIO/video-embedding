#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$ROOT_DIR/.env.migrated.local"

if [[ -e "$ENV_FILE" ]]; then
  echo "$ENV_FILE already exists; leaving it unchanged"
  exit 0
fi

postgres_password="$(openssl rand -hex 24)"
redis_password="$(openssl rand -hex 24)"
minio_password="$(openssl rand -hex 24)"
jwt_secret="$(openssl rand -hex 32)"

umask 077
sed \
  -e "s/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=$postgres_password/" \
  -e "s/^REDIS_PASSWORD=.*/REDIS_PASSWORD=$redis_password/" \
  -e "s/^MINIO_ROOT_PASSWORD=.*/MINIO_ROOT_PASSWORD=$minio_password/" \
  -e "s#^POSTGRES_DSN=.*#POSTGRES_DSN=\"host=127.0.0.1 user=video_app password=$postgres_password dbname=video_app port=15432 sslmode=disable TimeZone=Asia/Shanghai\"#" \
  -e "s/^COS_SECRET_ID=.*/COS_SECRET_ID=videoapp/" \
  -e "s/^COS_SECRET_KEY=.*/COS_SECRET_KEY=$minio_password/" \
  -e "s/^RUSTFS_ACCESS_KEY=.*/RUSTFS_ACCESS_KEY=videoapp/" \
  -e "s/^RUSTFS_SECRET_KEY=.*/RUSTFS_SECRET_KEY=$minio_password/" \
  -e "s/^JWT_SECRET=.*/JWT_SECRET=$jwt_secret/" \
  "$ROOT_DIR/.env.migrated.example" > "$ENV_FILE"

echo "created $ENV_FILE with mode 600"
