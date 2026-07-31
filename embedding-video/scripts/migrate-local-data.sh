#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET_ENV_FILE="$ROOT_DIR/.env.migrated.local"
COMPOSE=(docker compose --env-file "$TARGET_ENV_FILE" -f "$ROOT_DIR/docker-compose.migrated.yml")
MIGRATION_DIR="$ROOT_DIR/.migration-data"

require_file() {
  [[ -f "$1" ]] || { echo "required file not found: $1" >&2; exit 1; }
}

wait_healthy() {
  local container="$1"
  for _ in $(seq 1 60); do
    if [[ "$(docker inspect -f '{{.State.Health.Status}}' "$container" 2>/dev/null || true)" == "healthy" ]]; then
      return 0
    fi
    sleep 2
  done
  echo "$container did not become healthy" >&2
  docker logs --tail 80 "$container" >&2 || true
  return 1
}

require_file "$TARGET_ENV_FILE"
mkdir -p "$MIGRATION_DIR"
chmod 700 "$MIGRATION_DIR"

set -a
# shellcheck disable=SC1090
source "$TARGET_ENV_FILE"
set +a

SOURCE_ROOT="${MIGRATION_SOURCE_ROOT:?MIGRATION_SOURCE_ROOT is required}"
SOURCE_NETWORK="${MIGRATION_SOURCE_NETWORK:?MIGRATION_SOURCE_NETWORK is required}"
SOURCE_VIDEO_BUCKET="${MIGRATION_SOURCE_VIDEO_BUCKET:?MIGRATION_SOURCE_VIDEO_BUCKET is required}"
SOURCE_ENV_FILE="${SOURCE_ENV_FILE:-$SOURCE_ROOT/.env.local}"
require_file "$SOURCE_ENV_FILE"

set -a
# shellcheck disable=SC1090
source "$SOURCE_ENV_FILE"
SOURCE_MINIO_ACCESS_KEY="${RUSTFS_ACCESS_KEY:?source RUSTFS_ACCESS_KEY is required}"
SOURCE_MINIO_SECRET_KEY="${RUSTFS_SECRET_KEY:?source RUSTFS_SECRET_KEY is required}"
set +a

set -a
# shellcheck disable=SC1090
source "$TARGET_ENV_FILE"
set +a

echo "starting isolated PostgreSQL, Redis, and MinIO containers"
"${COMPOSE[@]}" up -d
wait_healthy video-embedding-postgres
wait_healthy video-embedding-redis
wait_healthy video-embedding-minio

echo "copying PostgreSQL database into $POSTGRES_DB"
docker exec postgres sh -lc 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' > "$MIGRATION_DIR/postgres.dump"
docker exec -i video-embedding-postgres pg_restore \
  -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  --clean --if-exists --no-owner --no-privileges < "$MIGRATION_DIR/postgres.dump"

echo "copying Redis RDB snapshot"
docker exec redis redis-cli BGSAVE >/dev/null
docker cp redis:/data/dump.rdb "$MIGRATION_DIR/redis.rdb" >/dev/null
"${COMPOSE[@]}" stop redis >/dev/null
docker cp "$MIGRATION_DIR/redis.rdb" video-embedding-redis:/data/dump.rdb >/dev/null
"${COMPOSE[@]}" start redis >/dev/null
wait_healthy video-embedding-redis

echo "mirroring MinIO buckets with neutral target names"
docker run --rm --network "$SOURCE_NETWORK" \
  -e SOURCE_ACCESS_KEY="$SOURCE_MINIO_ACCESS_KEY" \
  -e SOURCE_SECRET_KEY="$SOURCE_MINIO_SECRET_KEY" \
  -e SOURCE_VIDEO_BUCKET="$SOURCE_VIDEO_BUCKET" \
  -e TARGET_ACCESS_KEY="$MINIO_ROOT_USER" \
  -e TARGET_SECRET_KEY="$MINIO_ROOT_PASSWORD" \
  --entrypoint /bin/sh minio/mc:latest -ec '
    mc alias set source http://minio:9000 "$SOURCE_ACCESS_KEY" "$SOURCE_SECRET_KEY" >/dev/null
    mc alias set target http://host.docker.internal:19000 "$TARGET_ACCESS_KEY" "$TARGET_SECRET_KEY" >/dev/null
    mc mb --ignore-existing target/video-cloud-drive >/dev/null
    mc mb --ignore-existing target/knowledge-point-videos >/dev/null
    mc mirror --quiet --overwrite --remove "source/$SOURCE_VIDEO_BUCKET" target/video-cloud-drive
    mc mirror --quiet --overwrite --remove source/knowledge-point-videos target/knowledge-point-videos
  '

source_db_size="$(docker exec postgres sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "select pg_database_size(current_database())"')"
target_db_size="$(docker exec video-embedding-postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc 'select pg_database_size(current_database())')"
source_redis_keys="$(docker exec redis redis-cli DBSIZE | tr -d '\r')"
target_redis_keys="$(docker exec -e REDISCLI_AUTH="$REDIS_PASSWORD" video-embedding-redis redis-cli --no-auth-warning DBSIZE | tr -d '\r')"

echo "PostgreSQL bytes: source=$source_db_size target=$target_db_size"
echo "Redis keys: source=$source_redis_keys target=$target_redis_keys"
echo "MinIO verification:"
docker run --rm \
  -e TARGET_ACCESS_KEY="$MINIO_ROOT_USER" \
  -e TARGET_SECRET_KEY="$MINIO_ROOT_PASSWORD" \
  --entrypoint /bin/sh minio/mc:latest -ec '
    mc alias set target http://host.docker.internal:19000 "$TARGET_ACCESS_KEY" "$TARGET_SECRET_KEY" >/dev/null
    mc du target/video-cloud-drive
    mc du target/knowledge-point-videos
  '

[[ "$source_redis_keys" == "$target_redis_keys" ]] || {
  echo "Redis key count mismatch" >&2
  exit 1
}

echo "data migration completed; source containers were left running"
