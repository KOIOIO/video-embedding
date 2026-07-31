#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$ROOT_DIR/.env.migrated.local"
COMPONENT="${1:-api}"

[[ -f "$ENV_FILE" ]] || { echo "run scripts/init-migrated-env.sh first" >&2; exit 1; }
set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a
export VIDEO_APP_ENV_FILE="$ENV_FILE"
export GOCACHE="${GOCACHE:-/private/tmp/video-embedding-go-cache}"
export RUSTFS_ENDPOINT="${COS_ENDPOINT}"
export RUSTFS_ACCESS_KEY="${COS_SECRET_ID}"
export RUSTFS_SECRET_KEY="${COS_SECRET_KEY}"

case "$COMPONENT" in
  api)
    cd "$ROOT_DIR/video-service"
    export HTTP_ADDR="127.0.0.1:${VIDEO_APP_HTTP_PORT:-18083}"
    exec go run ./cmd/httpapi
    ;;
  worker)
    cd "$ROOT_DIR/video-service"
    exec go run ./cmd/worker
    ;;
  frontend)
    cd "$ROOT_DIR/hls-web"
    export VITE_PROXY_TARGET="http://127.0.0.1:${VIDEO_APP_HTTP_PORT:-18083}"
    exec npm run dev -- --host 127.0.0.1 --port "${VIDEO_APP_WEB_PORT:-15173}"
    ;;
  *)
    echo "usage: $0 {api|worker|frontend}" >&2
    exit 2
    ;;
esac
