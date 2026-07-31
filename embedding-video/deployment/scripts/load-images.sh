#!/usr/bin/env bash
set -euo pipefail

deployment_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
image_dir="${deployment_root}/images"

shopt -s nullglob
archives=("${image_dir}"/*.tar.gz)
if (( ${#archives[@]} == 0 )); then
  echo "no image archives found in ${image_dir}" >&2
  exit 1
fi

for archive in "${archives[@]}"; do
  echo "loading $(basename "${archive}")"
  gzip -dc "${archive}" | docker load
done
