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

source_file="${deployment_root}/env/${mode}.env.example"
target_file="${deployment_root}/env/${mode}.env"

if [[ -e "${target_file}" ]]; then
  echo "environment file already exists: ${target_file}" >&2
  exit 1
fi

cp "${source_file}" "${target_file}"
chmod 600 "${target_file}"
echo "created ${target_file}; replace every change-me value before deployment"
