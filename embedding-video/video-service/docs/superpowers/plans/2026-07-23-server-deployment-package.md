# Server Deployment Package Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build source and offline-image deployment packages that work with the current single-host configuration and include a future cloud/intranet split layout.

**Architecture:** Track reusable deployment definitions under `deployment/`, with separate standalone, cloud, and intranet Compose files sharing explicit image names and environment contracts. Add runtime endpoint overrides to the Go configuration and render the frontend Nginx upstream from `API_UPSTREAM`; package scripts then assemble reproducible source and `linux/amd64` image archives under `outputs/server-deployment/`.

**Tech Stack:** Docker Compose, Docker Buildx, POSIX shell, Go configuration loader, Nginx templates, Vue/Vite, tar/gzip, SHA-256 checksums.

---

### Task 1: Add remote service endpoint overrides

**Files:**
- Modify: `video-service/internal/config/loader.go`
- Modify: `video-service/internal/config/loader_test.go`

- [ ] **Step 1: Write failing configuration tests**

Extend the environment override test with `REDIS_ADDR` and `GORSE_ENDPOINT`, then assert:

```go
t.Setenv("REDIS_ADDR", "cloud.example:6379")
t.Setenv("GORSE_ENDPOINT", "http://gorse.internal:8088")
if cfg.Redis.Addr != "cloud.example:6379" { t.Fatalf(...) }
if cfg.Gorse.Endpoint != "http://gorse.internal:8088" { t.Fatalf(...) }
```

- [ ] **Step 2: Run the focused test and verify failure**

Run: `GOCACHE=/tmp/video-go-cache go test ./internal/config -run TestMustLoadAppliesEnvironmentOverrides -count=1`

Expected: FAIL because both endpoint values still come from YAML.

- [ ] **Step 3: Implement minimal overrides**

Add to `applyEnvOverrides`:

```go
if value := firstEnv("REDIS_ADDR"); value != "" {
    c.Redis.Addr = value
}
if value := firstEnv("GORSE_ENDPOINT"); value != "" {
    c.Gorse.Endpoint = value
}
```

- [ ] **Step 4: Run configuration tests**

Run: `GOCACHE=/tmp/video-go-cache go test ./internal/config -count=1`

Expected: PASS.

### Task 2: Make the production frontend proxy runtime-configurable

**Files:**
- Modify: `hls-web/Dockerfile`
- Delete: `hls-web/nginx.conf`
- Create: `hls-web/default.conf.template`
- Modify: `hls-web/src/knowledgeVideo/proxyConfig.test.js`

- [ ] **Step 1: Update the proxy contract test first**

Make the test load `default.conf.template` and require `${API_UPSTREAM}` for `/api/`, `/videos/`, `/swagger/`, and `/knowledge-video-media/`. Require proxy HTTP/1.1 and Range forwarding for media requests.

- [ ] **Step 2: Run the frontend test and verify failure**

Run: `npm test -- --run src/knowledgeVideo/proxyConfig.test.js`

Expected: FAIL because the template does not exist yet.

- [ ] **Step 3: Add Nginx template and Docker runtime default**

Use the official Nginx `/etc/nginx/templates/default.conf.template` mechanism. Set:

```dockerfile
ENV API_UPSTREAM=http://api:8081
COPY hls-web/default.conf.template /etc/nginx/templates/default.conf.template
```

Each proxied location must use `proxy_pass ${API_UPSTREAM};` and forward `Host`, `X-Real-IP`, `X-Forwarded-For`, `X-Forwarded-Proto`, and `Range`/`If-Range` where applicable.

- [ ] **Step 4: Run frontend tests and build**

Run: `npm test && npm run build`

Expected: all tests pass and Vite creates `dist/`.

### Task 3: Add deployment definitions and safe environment templates

**Files:**
- Create: `deployment/standalone/compose.yml`
- Create: `deployment/cloud/compose.yml`
- Create: `deployment/intranet/compose.yml`
- Create: `deployment/env/standalone.env.example`
- Create: `deployment/env/cloud.env.example`
- Create: `deployment/env/intranet.env.example`
- Create: `deployment/config/video_prod.yml`

- [ ] **Step 1: Add a static deployment validation test**

Create `deployment/tests/validate-layout.sh` that checks all required files, rejects known secret-file names, verifies every Compose service has an explicit image, and asserts the topology:

```text
standalone: api worker recbole_trainer frontend
cloud: api
intranet: worker recbole_trainer frontend
```

- [ ] **Step 2: Run validation and verify failure**

Run: `bash deployment/tests/validate-layout.sh`

Expected: FAIL because deployment files are not present.

- [ ] **Step 3: Implement the three Compose layouts**

Use image variables with defaults:

```yaml
image: ${VIDEO_APP_API_IMAGE:-video-service-api:latest}
```

The standalone stack preserves the current topology. The cloud stack contains only API and connects to existing PostgreSQL and Redis services. The intranet stack has no API, PostgreSQL, or Redis service and requires `POSTGRES_DSN`, `REDIS_ADDR`, and `API_UPSTREAM` from its env file.

- [ ] **Step 4: Add sanitized env templates and copied production config**

Templates contain placeholders only. Keep current non-secret OSS endpoints and bucket names in `deployment/config/video_prod.yml`, with all credentials empty and supplied by environment variables.

- [ ] **Step 5: Validate layout and Compose rendering**

Run:

```bash
bash deployment/tests/validate-layout.sh
docker compose --env-file deployment/env/standalone.env.example -f deployment/standalone/compose.yml config
docker compose --env-file deployment/env/cloud.env.example -f deployment/cloud/compose.yml config
docker compose --env-file deployment/env/intranet.env.example -f deployment/intranet/compose.yml config
```

Expected: all commands exit 0 and no unresolved variables remain.

### Task 4: Add deployment operations and documentation

**Files:**
- Create: `deployment/scripts/init-env.sh`
- Create: `deployment/scripts/load-images.sh`
- Create: `deployment/scripts/compose.sh`
- Create: `deployment/scripts/healthcheck.sh`
- Create: `deployment/DEPLOYMENT.md`

- [ ] **Step 1: Extend layout tests for executable scripts**

Require all scripts to be executable and reject absolute build-machine paths.

- [ ] **Step 2: Implement scripts**

`init-env.sh <mode>` copies the matching example without overwriting an existing file. `compose.sh <mode> <compose args...>` resolves paths relative to the bundle. `load-images.sh` loads every `images/*.tar.gz`. `healthcheck.sh <mode>` checks the configured API/frontend endpoints.

- [ ] **Step 3: Write deployment runbook**

Document prerequisites, current standalone deployment, future cloud-first/intranet-second startup, required firewall rules, PostgreSQL/Redis security, frontend `API_UPSTREAM`, backups, upgrades, rollback, health checks, and the fact that OSS remains configured by `video_prod.yml`.

- [ ] **Step 4: Run shell and layout checks**

Run:

```bash
bash -n deployment/scripts/*.sh deployment/tests/*.sh
bash deployment/tests/validate-layout.sh
```

Expected: PASS.

### Task 5: Add reproducible source and offline-image packaging

**Files:**
- Create: `deployment/scripts/package.sh`
- Modify: `.gitignore`

- [ ] **Step 1: Extend validation for package behavior**

Require `package.sh` to support `source`, `images`, and `all`, target `linux/amd64`, write SHA-256 checksums, and exclude `.env.deploy`, `.git`, logs, storage, node modules, and prior outputs.

- [ ] **Step 2: Implement source packaging**

Assemble a temporary `video-deploy/` with deployment files and only required build context, then create:

```text
outputs/server-deployment/video-deploy-source-<git-sha>.tar.gz
```

- [ ] **Step 3: Implement offline image packaging**

Build/pull `linux/amd64` images, export each with `docker save | gzip`, copy the deployment runtime files, generate `images/manifest.txt`, and create:

```text
outputs/server-deployment/video-deploy-images-linux-amd64-<git-sha>.tar.gz
```

- [ ] **Step 4: Ignore generated artifacts**

Add only `outputs/server-deployment/` to `.gitignore`; do not change ownership of unrelated output directories.

### Task 6: Build and verify deliverables

**Files:**
- Generated: `outputs/server-deployment/*.tar.gz`
- Generated: `outputs/server-deployment/SHA256SUMS`

- [ ] **Step 1: Run relevant application tests**

Run:

```bash
GOCACHE=/tmp/video-go-cache go test ./internal/config ./cmd/httpapi ./internal/http/...
(cd hls-web && npm test && npm run build)
```

Expected: PASS.

- [ ] **Step 2: Build both package variants**

Run: `bash deployment/scripts/package.sh all`

Expected: source archive, offline image archive, and checksum file exist.

- [ ] **Step 3: Inspect packages and scan for secrets**

List both archives, extract to temporary directories, run `docker load` against every image archive, verify `linux/amd64` image metadata, and scan filenames/content for `.env.deploy` and known local secret markers.

- [ ] **Step 4: Run final repository checks**

Run:

```bash
git diff --check
bash deployment/tests/validate-layout.sh
```

Expected: PASS, with generated archives remaining ignored and source changes visible for review.
