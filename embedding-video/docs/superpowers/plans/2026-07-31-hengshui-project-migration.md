# Project Migration and Sanitization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate the source repository's complete current working state into the existing `embedding-video` project, remove business-specific naming and secrets, preserve target-only work and Git history, and prove the resulting stack runs.

**Architecture:** Build from the source filesystem rather than importing its Git metadata. Merge six mapped project areas into the isolated target branch, preserve explicitly target-owned modules, and make deployment configuration environment-driven. Validate each module before integrating the branch into the user's original target checkout.

**Tech Stack:** Go 1.26, Vue 3/Vite/Vitest, Python/Pytest/RecBole/PyTorch, Bash, Docker Compose, PostgreSQL/pgvector, Redis, RustFS/S3.

---

## File Structure

- `video-service/`: active Go API and worker; receives `hengshui-tablet-video-http/`.
- `legacy-video/`: retained legacy Go backend; receives `hengshui-tablet-video/`.
- `hls-web/`: Vue frontend; receives authentication, knowledge-video, and RecBole UI changes.
- `recbole-training/`: current recommendation training pipeline.
- `two-tower-training/`: target-owned module; preserve and regression-test.
- `deployment/`: standalone, cloud, and intranet deployment topologies.
- `docker-compose*.yml`, `.env*.example`: local and compatibility deployment entrypoints.
- `scripts/validate-repository.sh`: create as the non-secret naming/configuration gate.
- `docs/migration/2026-07-31-verification.md`: create as the final command/result record.

### Task 1: Record and Verify the Baseline

**Files:**
- Verify: all existing target files
- Create: `docs/migration/2026-07-31-verification.md`

- [ ] **Step 1: Record immutable source and target state**

Run:

```bash
git -C /Users/xiaoyuwang/yinlihupo-hengshui-project/hengshui-tablet-video status --short --branch
git -C /Users/xiaoyuwang/yinlihupo-hengshui-project/hengshui-tablet-video rev-parse HEAD
git -C .. rev-parse HEAD
git -C .. status --short --branch
```

Expected: source HEAD `660546c` with its known tracked/untracked changes; target branch contains only this plan and design commits.

- [ ] **Step 2: Run target baseline tests**

Run:

```bash
(cd video-service && go test ./...)
(cd hls-web && npm ci && npm test && npm run build)
(cd recbole-training && python3 -m pytest -q)
(cd two-tower-training && python3 -m pytest -q)
```

Expected: every command exits 0. Stop and report any pre-existing failure before migration.

- [ ] **Step 3: Start the verification log**

Create `docs/migration/2026-07-31-verification.md` with this structure and
replace each result after running its command. Do not include environment values.

```markdown
# Migration Verification

## Revisions

- Source HEAD: `660546c`
- Source baseline: current working tree, including tracked and untracked changes
- Target starting HEAD: `f887142`
- Migration branch: `codex/migrate-hengshui-project`

## Baseline

| Command | Exit | Result |
| --- | ---: | --- |
| `go test ./...` in `video-service` | 0 | PASS |
| `npm test && npm run build` in `hls-web` | 0 | PASS |
| `python3 -m pytest -q` in `recbole-training` | 0 | PASS |
| `python3 -m pytest -q` in `two-tower-training` | 0 | PASS |

## Migrated Verification

Results are added after migration without environment values or secret output.
```

- [ ] **Step 4: Commit the baseline record**

```bash
git add docs/migration/2026-07-31-verification.md
git commit -m "docs: record project migration baseline"
```

### Task 2: Merge the Source Working Tree

**Files:**
- Modify: `video-service/**`, `legacy-video/**`, `hls-web/**`
- Modify: `recbole-training/**`, `gorse/**`, root documentation
- Preserve: `two-tower-training/**`, target-only design documents

- [ ] **Step 1: Copy mapped source modules without private or generated data**

Run the following from the target `embedding-video/` directory. The shared
filters exclude private, generated, runtime, and temporary data:

```bash
migration_source=/Users/xiaoyuwang/yinlihupo-hengshui-project/hengshui-tablet-video
rsync -a --exclude='.git' --exclude='.env*' --exclude='.DS_Store' --exclude='node_modules' --exclude='.venv' --exclude='__pycache__' --exclude='.pytest_cache' --exclude='logs' --exclude='storage' --exclude='artifacts' --exclude='data' --exclude='log_tensorboard' --exclude='.tmp' --exclude='tools/check_vector_status_tmp' "${migration_source}/hengshui-tablet-video-http/" video-service/
rsync -a --exclude='.git' --exclude='.env*' --exclude='.DS_Store' --exclude='node_modules' --exclude='.venv' --exclude='__pycache__' --exclude='.pytest_cache' --exclude='logs' --exclude='storage' --exclude='artifacts' --exclude='data' --exclude='log_tensorboard' --exclude='.tmp' "${migration_source}/hengshui-tablet-video/" legacy-video/
rsync -a --exclude='.git' --exclude='.env*' --exclude='.DS_Store' --exclude='node_modules' --exclude='dist' "${migration_source}/hls-web/" hls-web/
rsync -a --exclude='.git' --exclude='.env*' --exclude='.DS_Store' --exclude='.venv' --exclude='__pycache__' --exclude='.pytest_cache' --exclude='artifacts' --exclude='data' --exclude='log' --exclude='log_tensorboard' "${migration_source}/recbole-training/" recbole-training/
rsync -a --exclude='.git' --exclude='.env*' --exclude='.DS_Store' --exclude='bin' --exclude='config.local.toml' --exclude='config.deploy.toml' "${migration_source}/gorse/" gorse/
```

Do not add `--delete`.

- [ ] **Step 2: Copy source-only project documents**

Copy source root README/parameter/report files and all four untracked 2026-07-30 plan/spec files into corresponding target paths. Preserve target-only two-tower and later target documents.

- [ ] **Step 3: Reconcile known target-only implementations**

Keep `two-tower-training/**`, `video-service/internal/worker/vectorworker/title_rewrite*.go`, and target-only recommendation design history. Remove old Gorse performance UI files only after confirming the source RecBole equivalents replace all imports:

```text
hls-web/src/recommendation/components/GorsePerformanceChart.vue
hls-web/src/recommendation/components/GorsePerformanceChart.test.js
hls-web/src/recommendation/gorsePerformance.js
hls-web/src/recommendation/gorsePerformance.test.js
```

- [ ] **Step 4: Run module tests**

Run Go, frontend, RecBole, and two-tower commands from Task 1. Expected: all exit 0. Resolve merge regressions before continuing.

- [ ] **Step 5: Commit functional migration**

```bash
git add video-service legacy-video hls-web recbole-training gorse docs README.md PROJECT_PARAMETERS.md PROJECT_PARAMETERS_EN.md video-vectorization-cost-report.md
git diff --cached --check
git commit -m "feat: migrate latest video platform functionality"
```

### Task 3: Generalize Names and Module Paths

**Files:**
- Modify: `video-service/go.mod`, all Go imports under `video-service/**`
- Modify: `legacy-video/go.mod`, all Go imports under `legacy-video/**`
- Modify: frontend labels, Dockerfiles, Compose files, scripts, and active docs
- Create: `scripts/validate-repository.sh`

- [ ] **Step 1: Add a failing repository naming gate**

Create executable `scripts/validate-repository.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "${repo_root}"

failed=0
while IFS= read -r tracked_file; do
  if test -f "${tracked_file}" && LC_ALL=C rg -Ili 'hengshui|hengtao|yinlihupo' "${tracked_file}" >/dev/null; then
    printf 'business identifier: %s\n' "${tracked_file}" >&2
    failed=1
  fi
done < <(
  git ls-files |
    rg -v '^docs/(superpowers/(specs/2026-07-31-hengshui-project-migration-design\.md|plans/2026-07-31-hengshui-project-migration\.md)|migration/2026-07-31-verification\.md)$'
)
exit "${failed}"
```

Run `chmod +x scripts/validate-repository.sh`.

Run: `./scripts/validate-repository.sh`

Expected: FAIL and list business-named active files.

- [ ] **Step 2: Rename Go modules and imports**

Set the active module to `video-service` and legacy module to `legacy-video`. Mechanically replace their old module import prefixes, then run `gofmt` on changed Go files.

- [ ] **Step 3: Rename runtime and user-visible identifiers**

Update Compose build paths, images, container/project names, deployment variables, scripts, READMEs, Swagger source annotations, presentation generator inputs, and frontend labels to neutral names. Keep source identifiers only in the three migration records.

- [ ] **Step 4: Regenerate generated outputs**

Use the repository's Swagger generator from `video-service/docs/swagger` and regenerate any presentation artifact only when its checked-in generator is runnable without private inputs.

- [ ] **Step 5: Prove the naming gate passes**

Run:

```bash
./scripts/validate-repository.sh
(cd video-service && go test ./...)
(cd hls-web && npm test && npm run build)
```

Expected: all exit 0.

- [ ] **Step 6: Commit naming changes**

```bash
git add .
git diff --cached --check
git commit -m "refactor: remove business-specific project naming"
```

### Task 4: Sanitize Configuration and Deployment

**Files:**
- Modify: `.gitignore`, `.env.deploy.example`, `.env.local.example`
- Modify: `video-service/configs/*.yml`, `gorse/config.toml`
- Modify: `docker-compose*.yml`, `deployment/env/*.env.example`
- Modify: `deployment/**/*.yml`, `deployment/scripts/*.sh`, `deployment/tests/validate-layout.sh`

- [ ] **Step 1: Extend the repository gate with secret patterns**

Before `exit "${failed}"`, add checks for tracked private environment files,
PEM/private-key headers, provider tokens, and credential assignments. The gate
reports only path, line number, and variable name:

```bash
while IFS= read -r private_env; do
  printf 'tracked private environment file: %s\n' "${private_env}" >&2
  failed=1
done < <(git ls-files | rg '(^|/)\.env($|\.(local|deploy)$)')

while IFS= read -r tracked_file; do
  test -f "${tracked_file}" || continue
  if LC_ALL=C rg -Il -- '-----BEGIN ([A-Z ]+ )?PRIVATE KEY-----' "${tracked_file}" >/dev/null; then
    printf 'private key material: %s\n' "${tracked_file}" >&2
    failed=1
  fi
  while IFS=: read -r line_number variable_name; do
    printf 'credential assignment: %s:%s:%s\n' "${tracked_file}" "${line_number}" "${variable_name}" >&2
    failed=1
  done < <(
    awk '
      BEGIN { IGNORECASE=1 }
      match($0, /^[[:space:]]*([A-Za-z0-9_]*(password|passwd|jwt_secret|secret_key|access_key|api_key|token)[A-Za-z0-9_]*)[[:space:]]*[:=]/, m) {
        value=$0
        sub(/^[^:=]*[:=][[:space:]]*/, "", value)
        if (value !~ /^(|\$|\$\{|\{\{|<|change-me|example|placeholder)/) {
          print FNR ":" m[1]
        }
      }
    ' "${tracked_file}" 2>/dev/null || true
  )
done < <(
  git ls-files |
    rg -v '^docs/(superpowers/(specs/2026-07-31-hengshui-project-migration-design\.md|plans/2026-07-31-hengshui-project-migration\.md)|migration/2026-07-31-verification\.md)$'
)
```

Run: `./scripts/validate-repository.sh`

Expected: FAIL for any unsafe tracked defaults.

- [ ] **Step 2: Replace credentials and infrastructure identity**

Remove real hosts, usernames, database names, passwords, JWT values, access keys, secret keys, and API tokens from tracked files. Examples must use neutral hosts and explicit placeholders. Compose must require or inject runtime values instead of embedding production-capable defaults.

- [ ] **Step 3: Update deployment assertions**

Change `deployment/tests/validate-layout.sh` to assert neutral paths/names, placeholder configuration, required `JWT_SECRET`, and absence of private environment files or machine-specific paths.

- [ ] **Step 4: Validate configuration**

Run:

```bash
./scripts/validate-repository.sh
deployment/tests/validate-layout.sh
docker compose -f docker-compose.yml config --quiet
docker compose -f docker-compose.cloud.yml config --quiet
docker compose -f docker-compose.gorse.yml config --quiet
docker compose -f docker-compose.local.yml config --quiet
```

Expected: all exit 0 without printing secrets.

- [ ] **Step 5: Scan reachable history**

Install or download a pinned Gitleaks release only from its official repository, then run:

```bash
gitleaks git .. --redact --no-banner
```

Expected: exit 0. Review findings by type and location only. If a real secret is verified, stop, create `refs/backup/pre-secret-rewrite-20260731`, prepare the narrow history rewrite, and obtain confirmation before any force push.

- [ ] **Step 6: Commit sanitization**

```bash
git add .gitignore .env.deploy.example .env.local.example video-service/configs gorse docker-compose*.yml deployment scripts/validate-repository.sh
git diff --cached --check
git commit -m "security: sanitize project configuration"
```

### Task 5: Verify Migrated Behavior

**Files:**
- Modify only tests or implementation needed to fix migration regressions

- [ ] **Step 1: Run complete Go verification**

```bash
(cd video-service && go test ./...)
(cd legacy-video && go test ./...)
```

Expected: all packages pass.

- [ ] **Step 2: Run complete frontend verification**

```bash
(cd hls-web && npm ci && npm test && npm run build)
```

Expected: Vitest has zero failures and Vite exits 0.

- [ ] **Step 3: Run complete Python verification**

```bash
(cd recbole-training && python3 -m pytest -q)
(cd two-tower-training && python3 -m pytest -q)
```

Expected: both suites have zero failures.

- [ ] **Step 4: Commit only necessary regression fixes**

If fixes were required, stage only affected implementation and tests, run their focused red-green verification, then commit:

```bash
git commit -m "fix: resolve migration regressions"
```

### Task 6: Build and Run the Stack

**Files:**
- Modify: `docs/migration/2026-07-31-verification.md`

- [ ] **Step 1: Render every supported topology**

Run the deployment layout test plus `docker compose config --quiet` for standalone, cloud, and intranet files using their example env files. Expected: all exit 0.

- [ ] **Step 2: Build the local stack**

```bash
docker compose -f docker-compose.yml build
```

Expected: every local image builds successfully.

- [ ] **Step 3: Start and inspect services**

```bash
docker compose -f docker-compose.yml up -d
docker compose -f docker-compose.yml ps
```

Expected: required services reach running or healthy state. Do not expose environment values in logs.

- [ ] **Step 4: Execute health checks**

Run the repository health script and direct HTTP checks for the API and frontend endpoints defined by Compose. Expected: HTTP 2xx responses and healthy database, Redis, object-storage, and recommendation dependencies.

- [ ] **Step 5: Stop the verification stack**

```bash
docker compose -f docker-compose.yml down
```

Do not add `-v`; preserve any pre-existing named volumes.

- [ ] **Step 6: Record and commit evidence**

Update `docs/migration/2026-07-31-verification.md` with commands, exit codes, test counts, service health, and any external blocker. Do not include secrets or complete environment dumps.

```bash
git add docs/migration/2026-07-31-verification.md
git commit -m "docs: record migration verification"
```

### Task 7: Final Audit and Integration

**Files:**
- Verify: complete target branch and original checkout

- [ ] **Step 1: Run the final gate**

```bash
./scripts/validate-repository.sh
git diff wwy_dev...HEAD --check
git status --short --branch
```

Expected: gate passes, diff has no whitespace errors, and worktree is clean.

- [ ] **Step 2: Review commits and preservation guarantees**

```bash
git log --oneline wwy_dev..HEAD
test -d two-tower-training
git diff --name-status wwy_dev...HEAD
```

Expected: focused commits, retained two-tower module, and no unrelated parent-repository paths.

- [ ] **Step 3: Integrate into the original target checkout**

After verifying the original `wwy_dev` checkout is still clean, fast-forward it to `codex/migrate-hengshui-project`. Do not force-push or delete the migration branch. Run `git status --short --branch` in the original checkout and confirm a clean tree.

- [ ] **Step 4: Report final evidence**

Report commit IDs, exact passing test/build commands and counts, stack health, naming/secret scan results, whether history rewrite was required, and any remaining external limitation.
