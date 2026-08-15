# Project Documentation Refresh Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring every current README, parameter reference, deployment guide, and recommendation runbook into agreement with the repository's current code, configuration, scripts, and generated OpenAPI contract.

**Architecture:** Treat runtime code and committed configuration as the source of truth, then update each document according to its ownership boundary instead of duplicating every fact everywhere. Establish the Chinese parameter reference first, mirror it into English, and finish with machine-assisted set comparisons plus an independent reader review.

**Tech Stack:** Markdown, Go 1.26.1, Vue 3/Vite/Vitest, Python unittest, Bash, Docker Compose, jq, ripgrep, Git.

---

### Task 1: Repository README, Documentation Index, and Root Environment Guidance

**Files:**
- Modify: `README.md`
- Modify: `docs/README.md`
- Modify: `.env.local.example`
- Modify: `.env.deploy.example`

- [ ] **Step 1: Capture the current mismatches**

Run:

```bash
rg -n 'hls-web.zip|宿主机 Redis|JWT_SECRET|VIDEO_APP_ENV_FILE' README.md .env.local.example .env.deploy.example docs/README.md
```

Expected: `README.md` still lists the absent `hls-web.zip`, describes Gorse as using host Redis, and omits `JWT_SECRET` from its root Compose parameter table; `.env.local.example` does not explain how root Compose selects it.

- [ ] **Step 2: Update the repository-level guidance**

Make these exact content changes:

- Remove the nonexistent `hls-web.zip` entry from the repository tree.
- Add `JWT_SECRET` to the root Compose variable table and require a unique random value of at least 32 characters before startup.
- Correct the Gorse storage description: both configured stores use the dedicated PostgreSQL schema and the configured blob/vector storage remains separate; do not claim that root Compose connects Gorse to Redis.
- Mention the knowledge-video watch-session signal and the RecBole data-quality gate in the project summary without duplicating the algorithm handoff.
- Keep `video-service/` as the only recommended new integration backend.
- Extend `docs/README.md` navigation to the documentation-refresh design/plan and keep historical plans/specs explicitly non-authoritative.
- Add a root-Compose example to `.env.local.example` using `VIDEO_APP_ENV_FILE=.env.local docker compose up -d`.
- Strengthen `.env.deploy.example` comments so the committed `JWT_SECRET` value is clearly an invalid production placeholder even though it satisfies the length check.

- [ ] **Step 3: Verify repository facts and links**

Run:

```bash
test ! -e hls-web.zip
rg -n 'JWT_SECRET|PostgreSQL|VIDEO_APP_ENV_FILE=.env.local' README.md .env.local.example .env.deploy.example
git diff --check -- README.md docs/README.md .env.local.example .env.deploy.example
```

Expected: `test` exits zero; all three required concepts are present; `git diff --check` prints nothing.

- [ ] **Step 4: Commit the repository-document batch**

```bash
git add README.md docs/README.md .env.local.example .env.deploy.example
git commit -m "docs: refresh repository entry points"
```

### Task 2: Legacy Go Backend README

**Files:**
- Modify: `legacy-video/README.md`

- [ ] **Step 1: Reconfirm legacy entrypoints and configuration names**

Run:

```bash
rg -n 'func main|API_ADDR|RPC_ADDR|CONFIG_FILE|VIDEO_CONFIG_FILE|combined.Run' \
  legacy-video/cmd \
  legacy-video/internal/config/loader.go
```

Expected: the primary commands are `cmd/rpc`, `cmd/api`, and `cmd/worker`; both compatibility worker commands call the combined worker; the legacy API uses `API_ADDR`, not `HTTP_ADDR`.

- [ ] **Step 2: Replace the minimal legacy README with actionable maintenance guidance**

Document all of the following:

- The project is legacy and must not be selected for new Java or public integrations.
- The three primary commands and a local example that explicitly sets `CONFIG_FILE=configs/video.yml`.
- `cmd/transcodeworker` and `cmd/vector_worker` are compatibility aliases that each start the combined worker, so running them together duplicates consumers.
- Windows selects `configs/video.yml`; macOS and Linux select `configs/video_prod.yml`; `CONFIG_FILE` wins over `VIDEO_CONFIG_FILE`.
- `API_ADDR` controls the HTTP listener; `RPC_ADDR` controls the API-to-RPC target; YAML `Host:Port` is the fallback target while the RPC server listens on `:Port`.
- A process/dependency table covering PostgreSQL/pgvector, Redis, object storage, FFmpeg/Docker, and AI keys, including startup migration side effects.
- The API lacks authentication and must remain on a controlled network.
- A compact route/contract pointer to `internal/api/router/router.go` and `video/video_service.proto`, noting that `/api/healthz` is liveness only and the media proxy supports Range requests.
- Known configuration traps: unused legacy fields, system-temp paths, and the vector concurrency cap.

- [ ] **Step 3: Verify the README against source identifiers**

Run:

```bash
rg -n 'cmd/rpc|cmd/api|cmd/worker|API_ADDR|RPC_ADDR|CONFIG_FILE|公网|healthz|video_service.proto' \
  legacy-video/README.md
git diff --check -- legacy-video/README.md
```

Expected: every identifier and safety warning appears; whitespace validation prints nothing.

- [ ] **Step 4: Commit the legacy README**

```bash
git add legacy-video/README.md
git commit -m "docs: document legacy backend boundaries"
```

### Task 3: Frontend Console README

**Files:**
- Modify: `hls-web/README.md`

- [ ] **Step 1: Capture authentication and watch-session behavior from frontend code**

Run:

```bash
rg -n '/api/auth/login|/api/auth/me|Authorization|watch-sessions|watchSession' \
  hls-web/src/App.vue hls-web/src/auth hls-web/src/knowledgeVideo hls-web/src/workspaces
```

Expected: login and session validation call the backend, protected requests use a Bearer token, and knowledge-video progress uses a PUT watch-session endpoint.

- [ ] **Step 2: Correct and complete the frontend README**

Make these content changes:

- Remove the fixed browser-only credentials and the claim that login is only a UI lock.
- Explain backend administrator JWT login, token/profile persistence, `/api/auth/me` validation, and logout/401 invalidation.
- State that account provisioning comes from `sys_user`; do not publish a default password.
- Change “UI unlock state” wording to administrator session state.
- Add `PUT /api/knowledge-videos/:knowledgeVideoId/watch-sessions/:sessionId` and explain absolute watched seconds, retries, and the 60% effective-watch boundary at a high level.
- Add the existing authentication and watch-session test files to the test inventory.
- Keep proxy ports, workspace descriptions, and production Nginx behavior unchanged where already accurate.

- [ ] **Step 3: Run frontend verification**

Run from `hls-web/`:

```bash
npm test
npm run build
```

Expected: Vitest reports zero failed tests and Vite exits zero with a populated `dist/` build.

- [ ] **Step 4: Commit the frontend README**

```bash
git add hls-web/README.md
git commit -m "docs: align frontend authentication guide"
```

### Task 4: RecBole Training and Algorithm Documents

**Files:**
- Modify: `recbole-training/README.md`
- Modify: `recbole-training/ALGORITHM_HANDOFF.md`
- Modify: `video-service/docs/recbole-recommendation-pipeline.md`

- [ ] **Step 1: Inventory the actual pipeline inputs, outputs, and gates**

Run:

```bash
rg -n '^[A-Z_]+="?\$\{|train.inter|valid.inter|test.inter|knowledge_video:|positive_rows|positive_users' \
  recbole-training/scripts/run_recbole_pipeline.sh \
  recbole-training/src/recbole_recommendation \
  video-service/tools/export_recbole_dataset
```

Expected: the output includes the supported shell variables, mandatory benchmark split files, virtual knowledge-video tokens, and positive-row/user quality metrics.

- [ ] **Step 2: Update all three RecBole documents to the same contract**

Document these exact facts:

- `run_recbole_pipeline.sh` supports `SERVICE_DIR`, `CONFIG_FILE`, model/data/output variables, and `PUBLISH_GATE_ENABLED`; gate thresholds are Python CLI flags rather than unsupported shell variables.
- Dependencies use bounded version ranges, not fully pinned versions.
- The exporter produces `.train.inter`, `.valid.inter`, and `.test.inter` plus `.item` and `.user` files.
- Effective knowledge-video watches become namespaced training-only virtual items; virtual items are excluded from valid/test targets, full-sort candidates, item embedding CSV, database import, and online recommendations.
- `metrics.json` carries exporter/filter statistics and the publish gate checks positive rows/users in addition to Recall@20, NDCG@20, and relative NDCG decline.
- Gate failure leaves the active model untouched and keeps candidate artifacts for diagnosis.
- Online serving still reads numeric ordinary video-segment embeddings from `recsys` only.

- [ ] **Step 3: Run RecBole unit tests and document checks**

Run from `recbole-training/`:

```bash
PYTHONPATH=src python3 -m unittest discover -s tests -p 'test_*.py'
```

Then run from the repository root:

```bash
rg -n 'train.inter|valid.inter|test.inter|knowledge_video:|positive_rows|positive_users' \
  recbole-training/README.md \
  recbole-training/ALGORITHM_HANDOFF.md \
  video-service/docs/recbole-recommendation-pipeline.md
git diff --check -- recbole-training video-service/docs/recbole-recommendation-pipeline.md
```

Expected: Python tests pass; all three documents contain the split, virtual-item, and quality-gate contract; whitespace validation prints nothing.

- [ ] **Step 4: Commit the RecBole documentation**

```bash
git add recbole-training/README.md recbole-training/ALGORITHM_HANDOFF.md \
  video-service/docs/recbole-recommendation-pipeline.md
git commit -m "docs: describe current recbole pipeline"
```

### Task 5: Active HTTP Service README and Current Operational Reviews

**Files:**
- Modify: `video-service/README.md`
- Modify: `video-service/docs/downstream-service-readiness-review.md`
- Modify: `video-service/docs/gorse-recommendation-runbook.md`

- [ ] **Step 1: Compare the service README and reviews with active routes/configuration**

Run from `video-service/`:

```bash
jq -r '.paths | keys[]' docs/swagger/swagger.json | sort
rg -n 'adminRoutes\.|public\.|JWT_SECRET|KnowledgeVideo|Gorse|MIN_RECALL|ARTIFACT_RETENTION' \
  internal/http/router/router.go internal/config README.md docs/*.md
```

Expected: the audit exposes missing auth/watch-session/archive-progress/admin routes, completed JWT protection, inaccurate Gorse Redis assumptions, and README-only RecBole environment variables.

- [ ] **Step 2: Update the active service README**

Make these content changes:

- Add `/api/auth/login`, `/api/auth/me`, archive batch progress, knowledge-video watch sessions, knowledge-point compatibility route, administrator recommendation routes, and the internal RecBole candidate endpoint to the correct route classifications.
- Explicitly mark public, administrator-JWT, and internal-only surfaces.
- Add a login example that captures an access token, and add `Authorization: Bearer` to every protected upload, completion, status, and management example.
- Start the local scheduler with `RECBOLE_TRAINER_ENABLED=true CONFIG_FILE=configs/video.yml go run ./cmd/recboletrainer`; explain that the process otherwise remains disabled.
- Explain knowledge-video watch-session idempotency and its training-only RecBole effect.
- Remove `MIN_RECALL_AT_20`, `MIN_NDCG_AT_20`, `MAX_RELATIVE_NDCG_DROP`, and `ARTIFACT_RETENTION_DAYS` from the environment-variable table because the current shell pipeline does not consume them.
- Add actual `SERVICE_DIR`/`CONFIG_FILE` RecBole variables or defer the complete list to the training README.
- Correct `KnowledgeVideoWorker` descriptions to its actual worker-count, task-timeout, and shutdown fields.
- Distinguish local, service production, and delivery configuration values instead of presenting one as a universal default.
- Replace nonexistent `docker exec ... /tmp/dlqctl` examples with the supported host-side `go run ./cmd/dlqctl` flow, and state that the shipped image does not contain `dlqctl`.
- State that `dlqctl` does not currently inspect or replay the separate knowledge-video transcode DLQ.
- Document the random-play default user ID behavior when `user_id` is omitted.
- Limit the uniform JSON-envelope claim to business APIs; health checks, Swagger, and media proxy responses are exceptions.
- Describe root Compose as one Gorse service rather than a cluster.

- [ ] **Step 3: Update the readiness review and Gorse runbook**

For `downstream-service-readiness-review.md`:

- Add a current status note dated 2026-08-04.
- Mark administrator JWT authentication and upload-owner derivation as implemented, with links to current middleware/routes.
- Keep genuinely unresolved idempotency, callbacks, tracing, and service-governance gaps as recommendations.
- Remove present-tense claims that all management endpoints are unauthenticated.

For `gorse-recommendation-runbook.md`:

- Remove Redis from the required Gorse store topology.
- Describe both `GORSE_DATA_STORE` and `GORSE_CACHE_STORE` as PostgreSQL URLs using the isolated schema.
- Keep local blob storage and optional S3 instructions separate from database storage.
- Distinguish root service configs, where synchronization is enabled, from server-delivery config, where Gorse sync/writeback are disabled and RecBole is the production recommendation engine.
- Explain the endpoint split: the checked-in local service config/default uses `localhost:8087`, while the root `gorse-in-one` diagnostics override exposes master HTTP on `localhost:8088`; local host-side use of that Compose service therefore needs `GORSE_ENDPOINT=http://localhost:8088`.
- Replace the stale 2026-07-21 review note with the current verified boundary.

- [ ] **Step 4: Run focused Go verification**

Run from `video-service/`:

```bash
go test ./internal/config ./internal/http/router ./internal/http/handler/knowledgevideos ./tools/export_recbole_dataset
```

Expected: all named packages pass.

Run from the repository root:

```bash
git diff --check -- video-service/README.md video-service/docs
```

Expected: no output.

- [ ] **Step 5: Commit the active service documents**

```bash
git add video-service/README.md \
  video-service/docs/downstream-service-readiness-review.md \
  video-service/docs/gorse-recommendation-runbook.md
git commit -m "docs: align active service operations"
```

### Task 6: Deployment Manual and Delivery Parameter Guidance

**Files:**
- Modify: `deployment/DEPLOYMENT.md`
- Modify if clarification is required: `deployment/env/standalone.env.example`
- Modify if clarification is required: `deployment/env/cloud.env.example`
- Modify if clarification is required: `deployment/env/intranet.env.example`

- [ ] **Step 1: Inventory delivery-script capabilities**

Run:

```bash
rg -n 'VIDEO_APP_PLATFORM|SHA256SUMS|validate-layout|source\|images\|all|JWT_SECRET|RECBOLE_TRAINER_ENABLED' \
  deployment/scripts deployment/tests deployment/env deployment/DEPLOYMENT.md
```

Expected: packaging emits `SHA256SUMS`, layout validation exists, builds default to `linux/amd64` but accept `VIDEO_APP_PLATFORM`, and only API-bearing topologies inject `JWT_SECRET`.

- [ ] **Step 2: Complete the deployment guide**

Add these operational instructions:

- Run `bash deployment/tests/validate-layout.sh` from the complete source checkout before packaging; do not tell archive recipients to run it because it depends on repository-root Dockerfiles that are not present at the same paths inside source/offline bundles.
- State that `SHA256SUMS` is emitted beside the archives, not inside them, and verify it before extraction or offline image loading.
- Describe `linux/amd64` as the default platform and document `VIDEO_APP_PLATFORM` as the supported build override; keep `package.sh`'s delivery behavior explicit.
- Explain which topology owns API, worker, trainer, and frontend, and therefore which environment file needs `JWT_SECRET`.
- State that `deployment/config/video_prod.yml` omits `Auth.JWTExpireHour`, so the application fallback is eight hours unless the delivery config is extended; do not incorrectly copy the HTTP service's 24-hour sample claim.
- Keep the remote PostgreSQL `sslmode=disable` warning and firewall/VPN requirement intact.
- Add comments to deployment environment examples only where a variable's process scope or secret requirement remains ambiguous.

- [ ] **Step 3: Validate the delivery layout**

Run:

```bash
bash deployment/tests/validate-layout.sh
git diff --check -- deployment
```

Expected: the script prints `deployment layout is valid`; whitespace validation prints nothing.

- [ ] **Step 4: Commit deployment documentation**

```bash
git add deployment/DEPLOYMENT.md deployment/env
git commit -m "docs: complete deployment verification guide"
```

### Task 7: Chinese Configuration and Environment Parameter Reference

**Files:**
- Modify: `PROJECT_PARAMETERS.md`

- [ ] **Step 1: Establish the current configuration inventory**

Run:

```bash
rg -n '^type .*Config|yaml:"' video-service/internal/config/types.go
rg -n 'firstEnv|os\.Getenv|LookupEnv' \
  video-service/internal/config \
  video-service/internal/http/app \
  video-service/internal/worker \
  recbole-training/scripts/run_recbole_pipeline.sh
```

Expected: the output includes `AuthConfig`, `KnowledgeVideoStorageConfig`, `KnowledgeVideoWorkerConfig`, all Redis keys, and every environment override consumed by current code/scripts.

- [ ] **Step 2: Complete the Chinese configuration sections**

Without renumbering existing 3.1-3.20 sections, append and link:

- `3.21 AuthConfig` with `JWTSecret`, `JWTExpireHour`, environment precedence, minimum secret length, and the eight-hour code fallback.
- `3.22 KnowledgeVideoStorageConfig` with endpoint/credentials/bucket/TLS/region/lookup/media/temp/archive-limit fields and fallback values.
- `3.23 KnowledgeVideoWorkerConfig` with worker count, task timeout, and shutdown timeout.

Also:

- Add all missing top-level fields and Redis keys to their existing tables.
- Correct stale RustFS/COS, transcode, vector, LLM, and concurrency examples by showing separate local, HTTP-production, and delivery values.
- Clarify that delivery config disables Gorse synchronization/writeback while service sample configs enable it.
- Add `JWT_SECRET`, `REDIS_ADDR`, `GORSE_ENDPOINT`, `VIDEO_APP_ENV_FILE`, `RECBOLE_TRAINER_ENABLED`, and other currently consumed variables with precedence and process scope.
- Remove unsupported RecBole threshold/retention environment variables; document publish-gate threshold CLI flags and positive-row/user defaults instead.

- [ ] **Step 3: Verify required Chinese parameter identifiers**

Run:

```bash
rg -n 'AuthConfig|KnowledgeVideoStorageConfig|KnowledgeVideoWorkerConfig|JWT_SECRET|REDIS_ADDR|GORSE_ENDPOINT|KnowledgeVideoTranscodeQueue|RandomPlayRecent|RandomPlayBucket|positive_rows|positive_users' PROJECT_PARAMETERS.md
git diff --check -- PROJECT_PARAMETERS.md
```

Expected: every required identifier appears and whitespace validation prints nothing.

- [ ] **Step 4: Commit the Chinese configuration reference**

```bash
git add PROJECT_PARAMETERS.md
git commit -m "docs: update chinese runtime parameters"
```

### Task 8: Chinese HTTP API Parameter Reference

**Files:**
- Modify: `PROJECT_PARAMETERS.md`

- [ ] **Step 1: Generate the missing OpenAPI path set**

Run:

```bash
jq -r '.paths | keys[]' video-service/docs/swagger/swagger.json | sort > /tmp/video-app-swagger-paths.txt
rg -o '^#### `(?:GET|POST|PUT|PATCH|DELETE) [^`]+`' PROJECT_PARAMETERS.md \
  | sed -E 's/^#### `[^ ]+ //; s/`$//; s/:([A-Za-z][A-Za-z0-9]*)/{\1}/g; s/\*([A-Za-z][A-Za-z0-9]*)/{\1}/g' \
  | sort -u > /tmp/video-app-zh-doc-paths.txt
comm -23 /tmp/video-app-swagger-paths.txt /tmp/video-app-zh-doc-paths.txt
```

Expected before editing: missing auth, admin recommendation, knowledge-video, archive-progress, internal RecBole, and media-proxy paths are printed.

- [ ] **Step 2: Add the missing Chinese API sections**

Append and link these sections while preserving existing API content:

- `5.8 鉴权与管理员会话` for `/api/auth/login` and `/api/auth/me`, including credentials, JWT response/use, status rules, and `Authorization: Bearer`.
- `5.9 知识点视频接口` for batch import/progress, tree, singular compatibility lookup, plural lookup, legacy playback record, idempotent watch session, and media proxy.
- `5.10 管理推荐与内部候选接口` for all ten `/api/admin/recommendation/*` paths and `/api/internal/recommendations/external/recbole`, including admin/internal boundaries and query/body parameters.
- Add `/api/videos/archive/batches/:batchId/progress` to upload parameters.

Use current DTO field names and validation constraints from source/Swagger. Mark public, administrator-JWT, and internal-only endpoints explicitly. Keep legacy compatibility paths separate from the standard contract.

- [ ] **Step 3: Prove Chinese OpenAPI path coverage**

Repeat the Step 1 path extraction and run:

```bash
comm -23 /tmp/video-app-swagger-paths.txt /tmp/video-app-zh-doc-paths.txt
```

Expected after editing: no output.

- [ ] **Step 4: Commit the Chinese API reference**

```bash
git add PROJECT_PARAMETERS.md
git commit -m "docs: complete chinese api parameters"
```

### Task 9: English Parameter Reference Parity

**Files:**
- Modify: `PROJECT_PARAMETERS_EN.md`

- [ ] **Step 1: Mirror the approved Chinese structure and facts**

Translate and mirror every Task 7 and Task 8 change, preserving identifiers exactly:

- Add TOC entries and sections 3.21-3.23 for Auth and knowledge-video configuration.
- Add all missing top-level fields, Redis keys, environment variables, precedence rules, environment-specific examples, and RecBole CLI gate parameters.
- Add sections 5.8-5.10 with the same HTTP methods, paths, field names, validation rules, authentication boundaries, and compatibility notes as the Chinese document.
- Do not translate literal config keys, environment variables, JSON fields, route paths, table names, or command names.

- [ ] **Step 2: Compare configuration section identifiers**

Run:

```bash
rg -o '^### 3\.[0-9]+ [A-Za-z][A-Za-z0-9]+' PROJECT_PARAMETERS.md \
  | sed -E 's/^### 3\.[0-9]+ //' | sort -u > /tmp/video-app-zh-config-sections.txt
rg -o '^### 3\.[0-9]+ [A-Za-z][A-Za-z0-9]+' PROJECT_PARAMETERS_EN.md \
  | sed -E 's/^### 3\.[0-9]+ //' | sort -u > /tmp/video-app-en-config-sections.txt
diff -u /tmp/video-app-zh-config-sections.txt /tmp/video-app-en-config-sections.txt
```

Expected: no output.

- [ ] **Step 3: Compare Chinese, English, and OpenAPI path sets**

Run the canonical path extraction from Task 8 for both parameter documents, producing `/tmp/video-app-zh-doc-paths.txt` and `/tmp/video-app-en-doc-paths.txt`, then run:

```bash
diff -u /tmp/video-app-zh-doc-paths.txt /tmp/video-app-en-doc-paths.txt
comm -23 /tmp/video-app-swagger-paths.txt /tmp/video-app-en-doc-paths.txt
git diff --check -- PROJECT_PARAMETERS_EN.md
```

Expected: all commands print no differences.

- [ ] **Step 4: Commit the English parameter reference**

```bash
git add PROJECT_PARAMETERS_EN.md
git commit -m "docs: mirror english parameter reference"
```

### Task 10: Whole-Repository Documentation Verification and Reader Test

**Files:**
- Modify only documents from Tasks 1-9 if verification exposes a concrete error.

- [ ] **Step 1: Run formatting, placeholder, and stale-claim scans**

Run:

```bash
git diff ebce77c..HEAD --check
rg -n 'aaddmmiinn|admin123|仅在浏览器内校验|宿主机 Redis|hls-web.zip|MIN_RECALL_AT_20|ARTIFACT_RETENTION_DAYS' \
  README.md PROJECT_PARAMETERS*.md docs/README.md deployment/DEPLOYMENT.md \
  legacy-video*/README.md video-service/docs/*.md \
  hls-web/README.md recbole-training/*.md
```

Expected: whitespace validation prints nothing; the stale-claim scan prints nothing except explicitly quoted historical warnings, which must be reviewed manually.

- [ ] **Step 2: Validate relative Markdown links in current documents**

Run from the repository root:

```bash
node <<'NODE'
const { execFileSync } = require('node:child_process')
const fs = require('node:fs')
const path = require('node:path')

const files = execFileSync(
  'git',
  ['diff', '--name-only', 'ebce77c..HEAD', '--', '*.md'],
  { encoding: 'utf8' },
).trim().split('\n').filter(Boolean)

const failures = []
for (const file of files) {
  const source = fs.readFileSync(file, 'utf8')
  for (const match of source.matchAll(/\[[^\]]*\]\(([^)]+)\)/g)) {
    let target = match[1].trim().replace(/^<|>$/g, '')
    if (/^(?:https?:|mailto:|#)/.test(target)) continue
    target = target.split('#', 1)[0]
    if (!target || target.includes('<') || target.includes('>')) continue
    const resolved = path.resolve(path.dirname(file), decodeURI(target))
    if (!fs.existsSync(resolved)) failures.push(`${file}: ${target}`)
  }
}

if (failures.length) {
  process.stderr.write(`${failures.join('\n')}\n`)
  process.exit(1)
}
console.log(`validated relative links in ${files.length} Markdown files`)
NODE
```

Expected: the script prints the number of checked Markdown files and exits zero.

- [ ] **Step 3: Run the complete focused verification matrix**

Run:

```bash
bash deployment/tests/validate-layout.sh
(cd hls-web && npm test && npm run build)
(cd recbole-training && PYTHONPATH=src python3 -m unittest discover -s tests -p 'test_*.py')
(cd video-service && go test ./internal/config ./internal/http/router ./internal/http/handler/knowledgevideos ./tools/export_recbole_dataset)
```

Expected: deployment validation, frontend tests/build, RecBole tests, and all focused Go packages exit zero.

- [ ] **Step 4: Perform independent reader testing**

Give a fresh reviewer only the modified current documents and ask it to answer these questions:

1. Which backend should a new Java integration call, and how is an administrator authenticated?
2. Which process runs HTTP, combined workers, and RecBole scheduling?
3. How does a knowledge-video watch affect training without becoming an online item candidate?
4. Which configuration or environment value controls JWT lifetime and signing?
5. How do standalone, cloud, and intranet deployments differ?
6. Which RecBole files and gates are required before active-model publication?

Also ask the reviewer to identify contradictions, ambiguous defaults, missing prerequisites, broken links, and claims unsupported by code/configuration. Fix every concrete issue in the owning document and rerun the relevant check.

- [ ] **Step 5: Commit verification fixes if any**

If reader or automated verification required edits:

```bash
git add README.md PROJECT_PARAMETERS.md PROJECT_PARAMETERS_EN.md docs/README.md \
  deployment/DEPLOYMENT.md deployment/env .env.local.example .env.deploy.example \
  legacy-video/README.md video-service/README.md \
  video-service/docs hls-web/README.md recbole-training
git commit -m "docs: resolve documentation verification findings"
```

If no edits were required, do not create an empty commit.
