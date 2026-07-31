#!/usr/bin/env bash
set -euo pipefail

deployment_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
bundle_root=$(cd "${deployment_root}/.." && pwd)
source_root="${bundle_root}/source"
platform=${VIDEO_APP_PLATFORM:-linux/amd64}

if [[ ! -d "${source_root}/video-service" ]]; then
  source_root="${bundle_root}"
fi
if [[ ! -d "${source_root}/video-service" ]]; then
  echo "source build context not found under ${bundle_root}" >&2
  exit 1
fi

docker buildx build --platform "${platform}" --load \
  -t "${VIDEO_APP_API_IMAGE:-video-service-api:latest}" \
  -f "${source_root}/video-service/Dockerfile" "${source_root}"

docker buildx build --platform "${platform}" --load \
  -t "${VIDEO_APP_RECBOLE_IMAGE:-video-recbole-trainer:latest}" \
  -f "${source_root}/recbole-training/Dockerfile.prod" "${source_root}"

docker buildx build --platform "${platform}" --load \
  -t "${VIDEO_APP_FRONTEND_IMAGE:-video-frontend:latest}" \
  -f "${source_root}/hls-web/Dockerfile" "${source_root}"
