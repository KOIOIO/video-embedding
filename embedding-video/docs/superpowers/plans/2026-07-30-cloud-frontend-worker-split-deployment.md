# Cloud Frontend And Worker Split Deployment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce a tested offline deployment package where cloud server `49.232.25.254` runs API and frontend while the current server runs worker and RecBole trainer.

**Architecture:** Move frontend ownership from the intranet Compose project to the cloud Compose project and proxy API traffic over their shared Docker network. Let the cloud API reach host PostgreSQL and Redis through Docker's `host-gateway`; let remote background services reach them through `49.232.25.254` with firewall restrictions.

**Tech Stack:** Docker Engine, Docker Compose v2, Bash, Nginx, Go service image, RecBole trainer image

---

### Task 1: Lock The New Topology In Layout Tests

**Files:**
- Modify: `deployment/tests/validate-layout.sh`

- [ ] **Step 1: Change expected service ownership**

Replace the service assertions with:

```bash
assert_services standalone api worker recbole_trainer frontend
assert_services cloud api frontend
assert_services intranet worker recbole_trainer
```

Add assertions that cloud uses the internal API upstream and host gateway, and that concrete non-secret endpoint examples are correct:

```bash
rg -Fq 'API_UPSTREAM: http://api:8081' "${deployment_root}/cloud/compose.yml"
rg -Fq 'host.docker.internal:host-gateway' "${deployment_root}/cloud/compose.yml"
rg -Fq 'host=host.docker.internal' "${deployment_root}/env/cloud.env.example"
rg -Fq 'port=15432' "${deployment_root}/env/cloud.env.example"
rg -Fq 'REDIS_ADDR=host.docker.internal:6379' "${deployment_root}/env/cloud.env.example"
rg -Fq 'host=49.232.25.254' "${deployment_root}/env/intranet.env.example"
rg -Fq 'port=15432' "${deployment_root}/env/intranet.env.example"
rg -Fq 'REDIS_ADDR=49.232.25.254:6379' "${deployment_root}/env/intranet.env.example"
```

- [ ] **Step 2: Run the test and verify RED**

Run:

```bash
./deployment/tests/validate-layout.sh
```

Expected: FAIL because cloud currently exposes only `api` and intranet still contains `frontend`.

### Task 2: Implement Cloud API And Frontend Ownership

**Files:**
- Modify: `deployment/cloud/compose.yml`
- Modify: `deployment/env/cloud.env.example`

- [ ] **Step 1: Give the API access to host services**

Add to the cloud API service:

```yaml
    extra_hosts:
      - "host.docker.internal:host-gateway"
```

- [ ] **Step 2: Add the frontend service**

Add:

```yaml
  frontend:
    image: ${VIDEO_APP_FRONTEND_IMAGE:-video-frontend:latest}
    environment:
      API_UPSTREAM: http://api:8081
    ports:
      - "${CLOUD_BIND_ADDRESS:-0.0.0.0}:${VIDEO_APP_WEB_PORT:-1325}:8080"
    depends_on:
      api:
        condition: service_healthy
    restart: unless-stopped
```

- [ ] **Step 3: Set safe cloud endpoint examples**

Use these non-secret values in `cloud.env.example`, retaining `change-me` for secrets:

```dotenv
VIDEO_APP_WEB_PORT=1325
VIDEO_APP_FRONTEND_IMAGE=video-frontend:latest
POSTGRES_DSN=host=host.docker.internal user=video_app password=change-me dbname=video-app port=15432 sslmode=disable TimeZone=Asia/Shanghai
REDIS_ADDR=host.docker.internal:6379
```

`sslmode=disable` reflects host-local Docker traffic. Operators may enable PostgreSQL TLS without changing Compose.

### Task 3: Restrict The Current Server To Background Services

**Files:**
- Modify: `deployment/intranet/compose.yml`
- Modify: `deployment/env/intranet.env.example`

- [ ] **Step 1: Remove the intranet frontend service**

Delete the complete `frontend` service. Keep the `worker`, `recbole_trainer`, and their volumes unchanged.

- [ ] **Step 2: Remove unused frontend environment values**

Delete `INTRANET_BIND_ADDRESS`, `VIDEO_APP_WEB_PORT`, `VIDEO_APP_FRONTEND_IMAGE`, and `API_UPSTREAM` from `intranet.env.example`.

- [ ] **Step 3: Configure concrete cloud endpoints**

Use:

```dotenv
POSTGRES_DSN=host=49.232.25.254 user=video_app password=change-me dbname=video-app port=15432 sslmode=disable TimeZone=Asia/Shanghai
REDIS_ADDR=49.232.25.254:6379
```

Keep secrets as `change-me`. The supplied PostgreSQL connection does not use TLS, so remote traffic requires a VPN or strict source-IP allowlist until PostgreSQL TLS is enabled.

- [ ] **Step 4: Run layout test and verify GREEN**

Run:

```bash
./deployment/tests/validate-layout.sh
```

Expected: `deployment layout is valid`.

### Task 4: Align Health Checks And Operator Documentation

**Files:**
- Modify: `deployment/scripts/healthcheck.sh`
- Modify: `deployment/DEPLOYMENT.md`

- [ ] **Step 1: Check cloud frontend and stop expecting intranet frontend**

Change health-check behavior to:

```bash
  cloud)
    check_url api "http://127.0.0.1:$(read_env VIDEO_APP_HTTP_PORT 8083)/healthz"
    check_url frontend "http://127.0.0.1:$(read_env VIDEO_APP_WEB_PORT 1325)/"
    ;;
  intranet)
    echo "intranet services have no HTTP endpoint; inspect worker and recbole_trainer health and logs with compose.sh"
    ;;
```

- [ ] **Step 2: Document exact server operations**

Update `DEPLOYMENT.md` so cloud starts `api frontend`, current server starts `worker recbole_trainer`, and users visit `http://49.232.25.254:1325`. Explain `host.docker.internal`, remote TLS, firewall source restrictions, and secret replacement without embedding real credentials.

- [ ] **Step 3: Run shell and Compose verification**

Run:

```bash
bash -n deployment/scripts/*.sh deployment/tests/*.sh
./deployment/tests/validate-layout.sh
docker compose --env-file deployment/env/cloud.env.example -f deployment/cloud/compose.yml config
docker compose --env-file deployment/env/intranet.env.example -f deployment/intranet/compose.yml config
git diff --check
```

Expected: all commands exit zero; rendered cloud services are `api frontend`, rendered intranet services are `worker recbole_trainer`.

### Task 5: Build And Verify The Offline Image Archive

**Files:**
- Generated: `outputs/server-deployment/video-deploy-images-linux-amd64-<version>.tar.gz`
- Generated: `outputs/server-deployment/SHA256SUMS`

- [ ] **Step 1: Build the package**

Run from the repository root:

```bash
./deployment/scripts/package.sh images
```

Expected: three images build for `linux/amd64`, image archives are saved, and the final deployment archive plus checksum file are created.

- [ ] **Step 2: Verify checksums and contents**

Run:

```bash
(cd outputs/server-deployment && shasum -a 256 -c SHA256SUMS)
tar -tzf outputs/server-deployment/video-deploy-images-linux-amd64-*.tar.gz
```

Expected: checksum reports `OK`; archive includes `deployment/cloud`, `deployment/intranet`, `deployment/images`, scripts, environment examples, and documentation.

- [ ] **Step 3: Verify image architecture**

Run:

```bash
docker image inspect --format '{{.Os}}/{{.Architecture}}' \
  video-service-api:latest \
  video-recbole-trainer:latest \
  video-frontend:latest
```

Expected: `linux/amd64` for each image.

- [ ] **Step 4: Record server-only acceptance checks**

The handoff must state that real secrets still need to be entered on each server and that runtime verification requires:

```bash
./scripts/healthcheck.sh cloud
./scripts/compose.sh intranet ps
./scripts/compose.sh intranet logs --tail=200 worker recbole_trainer
```

No local test can prove the cloud firewall, PostgreSQL `pg_hba.conf`, PostgreSQL TLS certificate, Redis bind/protected-mode, or current-server source IP configuration.

### Task 6: Prevent Runtime Environment Files From Entering Packages

**Files:**
- Modify: `deployment/scripts/package.sh`
- Modify: `deployment/tests/validate-layout.sh`

- [ ] **Step 1: Require broad runtime environment cleanup**

Assert that the packaging script matches both generated `*.env` files and hidden `.env.*` files.

- [ ] **Step 2: Replace exact-name cleanup with pattern cleanup**

Use:

```bash
find "${target}" -type f \( -name '*.env' -o -name '.env.*' \) -delete
```

This keeps checked-in `cloud.env.example`, `intranet.env.example`, and `standalone.env.example` while removing generated private environment files.

- [ ] **Step 3: Verify behavior with a fake secret file**

Create `deployment/env/leak-test.env` containing only a fake value, package with `VIDEO_APP_SKIP_BUILD=1`, and verify the archive excludes it while retaining all three `.env.example` files. Delete the fake file after the check.
