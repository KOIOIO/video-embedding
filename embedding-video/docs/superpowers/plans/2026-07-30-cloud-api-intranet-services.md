# Cloud API And Intranet Services Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Generate a verified offline deployment archive where the cloud server runs only HTTP API and the current server runs frontend, worker, and RecBole trainer.

**Architecture:** Move the frontend service from the cloud Compose project to the intranet Compose project. The frontend proxies to `http://49.232.25.254:8083`, while worker and trainer connect directly to the cloud PostgreSQL and Redis endpoints; JWT remains cloud-API-only.

**Tech Stack:** Docker Engine, Docker Compose v2, Bash, Nginx, Go service image, RecBole trainer image

---

### Task 1: Lock Service Ownership In Layout Tests

**Files:**
- Modify: `deployment/tests/validate-layout.sh`

- [ ] **Step 1: Change the expected topology and endpoint assertions**

Require these services and values:

```bash
assert_services standalone api worker recbole_trainer frontend
assert_services cloud api
assert_services intranet worker recbole_trainer frontend

rg -Fq 'API_UPSTREAM: ${API_UPSTREAM}' "${deployment_root}/intranet/compose.yml"
rg -Fq 'API_UPSTREAM=http://49.232.25.254:8083' "${deployment_root}/env/intranet.env.example"
rg -Fq 'JWT_SECRET: ${JWT_SECRET}' "${deployment_root}/cloud/compose.yml"
rg -Fq 'JWT_SECRET: ${JWT_SECRET}' "${deployment_root}/standalone/compose.yml"
```

Remove assertions that cloud owns the frontend or uses `API_UPSTREAM`.

- [ ] **Step 2: Run the layout test and verify RED**

Run:

```bash
./deployment/tests/validate-layout.sh
```

Expected: FAIL because cloud still renders `api frontend` and intranet lacks `frontend`.

### Task 2: Move Frontend To The Current Server

**Files:**
- Modify: `deployment/cloud/compose.yml`
- Modify: `deployment/intranet/compose.yml`
- Modify: `deployment/env/cloud.env.example`
- Modify: `deployment/env/intranet.env.example`

- [ ] **Step 1: Remove frontend from cloud**

Delete the complete `frontend` service from `deployment/cloud/compose.yml`. Remove `VIDEO_APP_WEB_PORT` and `VIDEO_APP_FRONTEND_IMAGE` from `deployment/env/cloud.env.example`; retain `JWT_SECRET`, API image, PostgreSQL, Redis, COS, and AI settings.

- [ ] **Step 2: Add frontend to intranet Compose**

Add this service without dependencies on local worker or trainer:

```yaml
  frontend:
    image: ${VIDEO_APP_FRONTEND_IMAGE:-video-frontend:latest}
    environment:
      API_UPSTREAM: ${API_UPSTREAM}
    ports:
      - "${INTRANET_BIND_ADDRESS:-0.0.0.0}:${VIDEO_APP_WEB_PORT:-1325}:8080"
    restart: unless-stopped
```

- [ ] **Step 3: Restore intranet frontend environment values**

Add these values before the image declarations in `deployment/env/intranet.env.example`:

```dotenv
INTRANET_BIND_ADDRESS=0.0.0.0
VIDEO_APP_WEB_PORT=1325
VIDEO_APP_FRONTEND_IMAGE=video-frontend:latest
API_UPSTREAM=http://49.232.25.254:8083
```

Keep the existing PostgreSQL endpoint `49.232.25.254:15432`, Redis endpoint `49.232.25.254:6379`, and secret placeholders.

- [ ] **Step 4: Run the layout test and verify GREEN**

Run:

```bash
./deployment/tests/validate-layout.sh
```

Expected: `deployment layout is valid`.

### Task 3: Align Health Checks And Documentation

**Files:**
- Modify: `deployment/scripts/healthcheck.sh`
- Modify: `deployment/DEPLOYMENT.md`

- [ ] **Step 1: Update mode-specific HTTP checks**

Use:

```bash
  cloud)
    check_url api "http://127.0.0.1:$(read_env VIDEO_APP_HTTP_PORT 8083)/healthz"
    ;;
  intranet)
    check_url cloud-api "$(read_env API_UPSTREAM http://49.232.25.254:8083)/healthz"
    check_url frontend "http://127.0.0.1:$(read_env VIDEO_APP_WEB_PORT 1325)/"
    ;;
```

- [ ] **Step 2: Update operator documentation**

Document that cloud starts only `api`, intranet starts `frontend worker recbole_trainer`, users visit the current server's port `1325`, and cloud ports `8083`, `15432`, and `6379` should allow only the current server's fixed outbound IP. Retain JWT generation and secret handling guidance.

- [ ] **Step 3: Verify scripts and Compose rendering**

Run:

```bash
bash -n deployment/scripts/*.sh deployment/tests/*.sh
docker compose --env-file deployment/env/cloud.env.example -f deployment/cloud/compose.yml config --services
docker compose --env-file deployment/env/intranet.env.example -f deployment/intranet/compose.yml config --services
git diff --check
```

Expected: shell syntax exits zero; cloud prints only `api`; intranet prints `worker`, `recbole_trainer`, and `frontend`; diff check exits zero.

### Task 4: Package And Verify The Offline Archive

**Files:**
- Generated: `outputs/server-deployment/video-deploy-images-linux-amd64-660546c-dirty.tar.gz`
- Generated: `outputs/server-deployment/SHA256SUMS`

- [ ] **Step 1: Verify local image architecture**

Run:

```bash
docker image inspect --format '{{.Os}}/{{.Architecture}}' \
  video-service-api:latest \
  video-recbole-trainer:latest \
  video-frontend:latest
```

Expected: `linux/amd64` three times.

- [ ] **Step 2: Repackage without rebuilding unchanged images**

Run:

```bash
VIDEO_APP_SKIP_BUILD=1 ./deployment/scripts/package.sh images
```

Expected: a new `video-deploy-images-linux-amd64-660546c-dirty.tar.gz` and updated `SHA256SUMS`.

- [ ] **Step 3: Verify archive topology and secret exclusion**

Run the layout test freshly, compute the archive SHA-256, inspect both Compose files from the archive, verify all three image archives are listed, and fail if any private `*.env` file or supplied database/Redis credential appears. Expected: every check exits zero and the final SHA-256 is reported for deployment.

### Task 5: Prepare The Two-Server Handoff

**Files:**
- Reference: `deployment/DEPLOYMENT.md`

- [ ] **Step 1: Provide exact cloud commands**

Use the cloud release path under `/home/video/video-embedding/releases`, initialize `cloud.env`, generate JWT with `openssl rand -hex 32`, load images, and start only:

```bash
./scripts/compose.sh cloud up -d api
```

- [ ] **Step 2: Provide exact current-server commands**

Use `/home/debian/dev-ops/video-embedding/releases`, initialize `intranet.env`, load images, and start only:

```bash
./scripts/compose.sh intranet up -d frontend worker recbole_trainer
```

- [ ] **Step 3: State external acceptance limits**

State that local verification cannot prove cloud security-group rules, PostgreSQL `pg_hba.conf`, Redis network policy, or the current server's fixed outbound IP. Give runtime health and log commands for both servers.
