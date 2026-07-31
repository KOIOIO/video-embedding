#!/usr/bin/env bash
set -euo pipefail

deployment_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
mode=${1:-}

case "${mode}" in
  standalone|cloud|intranet) ;;
  *)
    echo "usage: $0 <standalone|cloud|intranet>" >&2
    exit 2
    ;;
esac

env_file="${deployment_root}/env/${mode}.env"
if [[ ! -f "${env_file}" ]]; then
  echo "missing ${env_file}" >&2
  exit 1
fi

read_env() {
  local key=$1
  local fallback=$2
  local value
  value=$(sed -n "s/^${key}=//p" "${env_file}" | tail -n 1)
  printf '%s' "${value:-${fallback}}"
}

check_url() {
  local label=$1
  local url=$2
  echo "checking ${label}: ${url}"
  curl --fail --silent --show-error --max-time 10 "${url}" >/dev/null
}

case "${mode}" in
  standalone)
    check_url api "http://127.0.0.1:$(read_env VIDEO_APP_HTTP_PORT 8083)/healthz"
    check_url frontend "http://127.0.0.1:$(read_env VIDEO_APP_WEB_PORT 1325)/"
    ;;
  cloud)
    check_url api "http://127.0.0.1:$(read_env VIDEO_APP_HTTP_PORT 8083)/healthz"
    ;;
  intranet)
    check_url cloud-api "$(read_env API_UPSTREAM http://services.example.internal:8083)/healthz"
    check_url frontend "http://127.0.0.1:$(read_env VIDEO_APP_WEB_PORT 1325)/"
    ;;
esac

echo "${mode} health checks passed"
