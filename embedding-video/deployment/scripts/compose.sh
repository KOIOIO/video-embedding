#!/usr/bin/env bash
set -euo pipefail

deployment_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
mode=${1:-}

case "${mode}" in
  standalone|cloud|intranet) ;;
  *)
    echo "usage: $0 <standalone|cloud|intranet> <docker compose arguments...>" >&2
    exit 2
    ;;
esac
shift

env_file="${deployment_root}/env/${mode}.env"
compose_file="${deployment_root}/${mode}/compose.yml"

if [[ ! -f "${env_file}" ]]; then
  echo "missing ${env_file}; run scripts/init-env.sh ${mode} first" >&2
  exit 1
fi

exec docker compose --env-file "${env_file}" -f "${compose_file}" "$@"
