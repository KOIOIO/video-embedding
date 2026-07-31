#!/usr/bin/env bash
set -euo pipefail

deployment_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
repo_root=$(cd "${deployment_root}/.." && pwd)
mode=${1:-all}
platform=linux/amd64

case "${mode}" in
  source|images|all) ;;
  *)
    echo "usage: $0 <source|images|all>" >&2
    exit 2
    ;;
esac

if [[ ! -d "${repo_root}/.git" ]]; then
  echo "package.sh must run from the source repository deployment directory" >&2
  exit 1
fi

git_sha=$(git -C "${repo_root}" rev-parse --short HEAD)
version=${git_sha}
if [[ -n "$(git -C "${repo_root}" status --porcelain --untracked-files=normal)" ]]; then
  version="${git_sha}-dirty"
fi
output_dir="${repo_root}/outputs/server-deployment"
mkdir -p "${output_dir}"

copy_runtime() {
  local target=$1
  mkdir -p "${target}"
  cp -R "${deployment_root}/standalone" "${target}/"
  cp -R "${deployment_root}/cloud" "${target}/"
  cp -R "${deployment_root}/intranet" "${target}/"
  cp -R "${deployment_root}/config" "${target}/"
  cp -R "${deployment_root}/env" "${target}/"
  cp -R "${deployment_root}/scripts" "${target}/"
  cp -R "${deployment_root}/tests" "${target}/"
  cp "${deployment_root}/DEPLOYMENT.md" "${target}/"
  find "${target}" -type f \( -name '*.env' -o -name '.env.*' \) -delete
}

build_source_archive() {
  local temp_dir
  temp_dir=$(mktemp -d)
  local bundle="${temp_dir}/video-deploy"
  copy_runtime "${bundle}/deployment"
  mkdir -p "${bundle}/source"
  tar -C "${repo_root}" -cf - \
    --exclude='.env' --exclude='.env.*' --exclude='.DS_Store' --exclude='node_modules' --exclude='dist' \
    --exclude='logs' --exclude='storage' --exclude='outputs' --exclude='.git' \
    video-service hls-web recbole-training \
    | tar -C "${bundle}/source" -xf -
  local archive="${output_dir}/video-deploy-source-${version}.tar.gz"
  tar -C "${temp_dir}" -czf "${archive}" video-deploy
  rm -rf "${temp_dir}"
  echo "created ${archive}"
}

build_offline_archive() {
  if [[ "${VIDEO_APP_SKIP_BUILD:-0}" != "1" ]]; then
    VIDEO_APP_PLATFORM="${platform}" "${deployment_root}/scripts/build-images.sh"
  fi

  local images=(
    "video-service-api:latest"
    "video-recbole-trainer:latest"
    "video-frontend:latest"
  )
  local image
  for image in "${images[@]:3}"; do
    docker pull --platform "${platform}" "${image}"
  done

  local temp_dir
  temp_dir=$(mktemp -d)
  local bundle="${temp_dir}/video-deploy"
  copy_runtime "${bundle}/deployment"
  mkdir -p "${bundle}/deployment/images"
  : > "${bundle}/deployment/images/manifest.txt"
  for image in "${images[@]}"; do
    local filename
    filename=$(printf '%s' "${image}" | tr '/:' '__')
    docker image inspect --format '{{.Os}}/{{.Architecture}}' "${image}" | grep -qx "${platform}"
    docker save "${image}" | gzip -1 > "${bundle}/deployment/images/${filename}.tar.gz"
    printf '%s\t%s\n' "${image}" "${filename}.tar.gz" >> "${bundle}/deployment/images/manifest.txt"
  done
  local archive="${output_dir}/video-deploy-images-linux-amd64-${version}.tar.gz"
  tar -C "${temp_dir}" -czf "${archive}" video-deploy
  rm -rf "${temp_dir}"
  echo "created ${archive}"
}

case "${mode}" in
  source) build_source_archive ;;
  images) build_offline_archive ;;
  all)
    build_source_archive
    build_offline_archive
    ;;
esac

: > "${output_dir}/SHA256SUMS"
for archive in "${output_dir}"/*.tar.gz; do
  filename=$(basename "${archive}")
  if command -v sha256sum >/dev/null 2>&1; then
    (cd "${output_dir}" && sha256sum "${filename}") >> "${output_dir}/SHA256SUMS"
  else
    (cd "${output_dir}" && shasum -a 256 "${filename}") >> "${output_dir}/SHA256SUMS"
  fi
done
echo "checksums written to ${output_dir}/SHA256SUMS"
