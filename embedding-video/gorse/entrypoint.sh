#!/bin/sh
set -eu

template=/etc/gorse/config.template.toml
runtime=/tmp/gorse-config.toml

dashboard_username=${GORSE_DASHBOARD_USERNAME:-admin}
dashboard_password=${GORSE_DASHBOARD_PASSWORD:-admin123}
cache_store=${GORSE_CACHE_STORE:-postgres://postgres:postgres@host.docker.internal:5432/hengshui-tablet?sslmode=disable&search_path=gorse,public}
data_store=${GORSE_DATA_STORE:-postgres://postgres:postgres@host.docker.internal:5432/hengshui-tablet?sslmode=disable&search_path=gorse,public}
server_api_key=${GORSE_SERVER_API_KEY:-}

escape_sed_replacement() {
  printf '%s' "$1" | sed 's/[|&\\]/\\&/g'
}

escaped_username=$(escape_sed_replacement "$dashboard_username")
escaped_password=$(escape_sed_replacement "$dashboard_password")
escaped_cache_store=$(escape_sed_replacement "$cache_store")
escaped_data_store=$(escape_sed_replacement "$data_store")
escaped_server_api_key=$(escape_sed_replacement "$server_api_key")

sed \
  -e "s|__GORSE_DASHBOARD_USERNAME__|$escaped_username|g" \
  -e "s|__GORSE_DASHBOARD_PASSWORD__|$escaped_password|g" \
  -e "s|__GORSE_CACHE_STORE__|$escaped_cache_store|g" \
  -e "s|__GORSE_DATA_STORE__|$escaped_data_store|g" \
  -e "s|__GORSE_SERVER_API_KEY__|$escaped_server_api_key|g" \
  "$template" > "$runtime"

exec /usr/bin/gorse-in-one -c "$runtime" "$@"
