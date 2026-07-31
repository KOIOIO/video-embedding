#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "${repo_root}"

failed=0
while IFS= read -r -d '' repository_file; do
  case "${repository_file}" in
    docs/superpowers/specs/2026-07-31-hengshui-project-migration-design.md|\
    docs/superpowers/plans/2026-07-31-hengshui-project-migration.md|\
    docs/migration/2026-07-31-verification.md|\
    scripts/validate-repository.sh)
      continue
      ;;
  esac

  if test -f "${repository_file}" &&
    LC_ALL=C rg -Ili 'hengshui|hengtao|yinlihupo|hstv|1323601202|衡水|恒涛|银梨琥珀' "${repository_file}" >/dev/null; then
    printf 'business identifier: %s\n' "${repository_file}" >&2
    failed=1
  fi
done < <(rg --files -0)

while IFS= read -r private_env; do
  if git ls-files --error-unmatch "${private_env}" >/dev/null 2>&1; then
    printf 'tracked private environment file: %s\n' "${private_env}" >&2
    failed=1
  fi
done < <(find . -type f \( -name '.env' -o -name '.env.local' -o -name '.env.deploy' \) -print)

while IFS= read -r -d '' repository_file; do
  if LC_ALL=C rg -Il -- '-----BEGIN ([A-Z ]+ )?PRIVATE KEY-----' "${repository_file}" >/dev/null; then
    printf 'private key material: %s\n' "${repository_file}" >&2
    failed=1
  fi
done < <(rg --files -0)

while IFS= read -r config_file; do
  while IFS=: read -r line_number variable_name; do
    test -n "${line_number}" || continue
    printf 'credential assignment: %s:%s:%s\n' "${config_file}" "${line_number}" "${variable_name}" >&2
    failed=1
  done < <(
    perl -ne '
      if (/^\s*([A-Za-z0-9_.-]*(?:password|passwd|jwt_?secret|secret_?(?:id|key)|access_?key|api_?key|token))\s*[:=]\s*(.*?)\s*$/i) {
        ($key, $value) = ($1, $2);
        $value =~ s/\s+#.*$//;
        $value =~ s/^"|"$//g;
        if ($value ne q{} && $value !~ /^(?:change-me|example|placeholder|\$|\{|__)/i) {
          print "$.:$key\n";
        }
      }
      close ARGV if eof;
    ' "${config_file}"
  )
done < <(
  rg --files \
    -g '*.yml' -g '*.yaml' -g '*.toml' \
    -g '.env*.example'
)

exit "${failed}"
