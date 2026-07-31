#!/usr/bin/env bash
set -euo pipefail

deployment_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
repo_root=$(cd "${deployment_root}/.." && pwd)

required_files=(
  "standalone/compose.yml"
  "cloud/compose.yml"
  "intranet/compose.yml"
  "env/standalone.env.example"
  "env/cloud.env.example"
  "env/intranet.env.example"
  "config/video_prod.yml"
  "scripts/init-env.sh"
  "scripts/load-images.sh"
  "scripts/build-images.sh"
  "scripts/compose.sh"
  "scripts/healthcheck.sh"
  "scripts/package.sh"
  "DEPLOYMENT.md"
)

for relative_path in "${required_files[@]}"; do
  test -f "${deployment_root}/${relative_path}" || {
    echo "missing deployment file: ${relative_path}" >&2
    exit 1
  }
done

for script in "${deployment_root}"/scripts/*.sh "${deployment_root}"/tests/*.sh; do
  test -x "${script}" || {
    echo "script is not executable: ${script#"${repo_root}/"}" >&2
    exit 1
  }
done

assert_services() {
  local mode=$1
  shift
  local compose_file="${deployment_root}/${mode}/compose.yml"
  local rendered
  rendered=$(docker compose --env-file "${deployment_root}/env/${mode}.env.example" -f "${compose_file}" config --services)
  local expected
  expected=$(printf '%s\n' "$@" | sort)
  local actual
  actual=$(printf '%s\n' "${rendered}" | sort)
  test "${actual}" = "${expected}" || {
    echo "${mode} services differ" >&2
    diff -u <(printf '%s\n' "${expected}") <(printf '%s\n' "${actual}") >&2 || true
    exit 1
  }

  local service
  while IFS= read -r service; do
    docker compose --env-file "${deployment_root}/env/${mode}.env.example" -f "${compose_file}" config \
      | awk -v service="${service}" '
          $0 == "  " service ":" { in_service=1; next }
          in_service && /^  [a-zA-Z0-9_-]+:/ { exit }
          in_service && /^    image:/ { found=1 }
          END { exit(found ? 0 : 1) }
        ' || {
          echo "service ${mode}/${service} has no explicit image" >&2
          exit 1
        }
  done <<< "${rendered}"
}

assert_services standalone api worker recbole_trainer frontend
assert_services cloud api
assert_services intranet worker recbole_trainer frontend

rg -Fq 'host.docker.internal:host-gateway' "${deployment_root}/cloud/compose.yml"
rg -Fq 'host=host.docker.internal' "${deployment_root}/env/cloud.env.example"
rg -Fq 'user=video_app' "${deployment_root}/env/cloud.env.example"
rg -Fq 'dbname=video-app' "${deployment_root}/env/cloud.env.example"
rg -Fq 'port=15432' "${deployment_root}/env/cloud.env.example"
rg -Fq 'REDIS_ADDR=host.docker.internal:6379' "${deployment_root}/env/cloud.env.example"
rg -Fq 'host=services.example.internal' "${deployment_root}/env/intranet.env.example"
rg -Fq 'user=video_app' "${deployment_root}/env/intranet.env.example"
rg -Fq 'dbname=video-app' "${deployment_root}/env/intranet.env.example"
rg -Fq 'port=15432' "${deployment_root}/env/intranet.env.example"
rg -Fq 'sslmode=disable' "${deployment_root}/env/intranet.env.example"
rg -Fq 'REDIS_ADDR=services.example.internal:6379' "${deployment_root}/env/intranet.env.example"
rg -Fq 'API_UPSTREAM: ${API_UPSTREAM}' "${deployment_root}/intranet/compose.yml"
rg -Fq 'API_UPSTREAM=http://services.example.internal:8083' "${deployment_root}/env/intranet.env.example"
rg -Fq 'JWT_SECRET: ${JWT_SECRET}' "${deployment_root}/cloud/compose.yml"
rg -Fq 'JWT_SECRET: ${JWT_SECRET}' "${deployment_root}/standalone/compose.yml"
rg -Fq 'JWT_SECRET=change-me-at-least-32-characters-long' "${deployment_root}/env/cloud.env.example"
rg -Fq 'JWT_SECRET=change-me-at-least-32-characters-long' "${deployment_root}/env/standalone.env.example"

for compose_file in "${deployment_root}/standalone/compose.yml" "${deployment_root}/cloud/compose.yml" "${deployment_root}/intranet/compose.yml"; do
  if rg -n 'image:.*video-service-api' "${compose_file}" >/dev/null; then
    api_service_count=$(rg -c 'image:.*video-service-api' "${compose_file}")
    cleared_entrypoint_count=$(rg -c 'entrypoint: \[\]' "${compose_file}")
    (( cleared_entrypoint_count >= api_service_count )) || {
      echo "business image entrypoint is not cleared in ${compose_file}" >&2
      exit 1
    }
  fi
done

if find "${deployment_root}" -type f \( -name '.env' -o -name '.env.deploy' -o -name '.env.local' \) | grep -q .; then
  echo "deployment tree contains a private environment file" >&2
  exit 1
fi

if rg -n '/Users/|/home/debian/dev-ops' "${deployment_root}" --glob '!validate-layout.sh'; then
  echo "deployment tree contains a build-machine absolute path" >&2
  exit 1
fi

package_script="${deployment_root}/scripts/package.sh"
rg -q 'source\|images\|all' "${package_script}"
rg -q 'linux/amd64' "${package_script}"
rg -q 'SHA256SUMS' "${package_script}"
rg -Fq -- "-name '*.env'" "${package_script}"
rg -Fq -- "-name '.env.*'" "${package_script}"
rg -q 'VIDEO_APP_SKIP_BUILD' "${package_script}"
rg -Fq 'source_root="${bundle_root}"' "${deployment_root}/scripts/build-images.sh"
rg -Fq 'COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt' \
  "${repo_root}/video-service/Dockerfile"

echo "deployment layout is valid"
