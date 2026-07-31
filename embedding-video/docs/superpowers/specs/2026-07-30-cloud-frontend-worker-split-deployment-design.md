# Cloud Frontend And Worker Split Deployment Design

## Goal

Prepare an offline `linux/amd64` deployment package for this topology:

- Debian 13 cloud server `49.232.25.254` runs `httpapi` and `hls-web`.
- The cloud server also hosts PostgreSQL on port `15432` and Redis on port `6379`.
- The current server runs `worker` and `recbole_trainer` only.
- Users access the frontend without a domain at `http://49.232.25.254:1325`.

The package must not contain database passwords, Redis passwords, object-storage credentials, AI keys, or other runtime secrets.

## Deployment Layout

The existing `cloud` deployment becomes the cloud-facing stack and contains two services:

- `api`, published on configurable host port `8083` and container port `8081`.
- `frontend`, published on configurable host port `1325` and container port `8080`.

Both services share the Compose network. The frontend uses `http://api:8081` as its Nginx upstream, so browser requests to `/api`, `/videos`, `/swagger`, and `/knowledge-video-media` remain same-origin and do not require a domain or public API port.

The existing `intranet` deployment contains `worker` and `recbole_trainer` only. Both services connect to the same PostgreSQL, Redis, object storage, and AI providers as the cloud API.

## Database And Redis Connectivity

PostgreSQL and Redis run on the cloud host, outside this deployment package. Their credentials remain in runtime environment files.

The cloud API container must not use `127.0.0.1` for host services because that address refers to the container itself. The Compose service maps `host.docker.internal` to Docker's `host-gateway`, and the cloud environment template uses:

- PostgreSQL host `host.docker.internal`, port `15432`.
- Redis host `host.docker.internal`, port `6379`.

The current server environment uses cloud address `49.232.25.254` for PostgreSQL and Redis. Cloud firewall and database access rules must allow those ports only from the current server's fixed public IP or private/VPN address. PostgreSQL and Redis must not be exposed to `0.0.0.0/0`.

## Configuration Changes

- Add `frontend` and its image/port settings to `deployment/cloud/compose.yml` and `cloud.env.example`.
- Remove `frontend`, `API_UPSTREAM`, and frontend settings from the intranet deployment.
- Update layout assertions so cloud expects `api frontend` and intranet expects `worker recbole_trainer`.
- Update health checks to test both cloud services and avoid checking an absent intranet frontend.
- Update deployment instructions with the concrete IP and ports while retaining placeholders for all secrets.

The checked-in environment examples are safe templates. Operators create private `.env` files with `init-env.sh` on each server after extracting the package.

## Offline Packaging

The existing packaging script builds and exports three `linux/amd64` images:

- `video-service-api:latest`, used by both `api` and `worker`.
- `video-recbole-trainer:latest`.
- `video-frontend:latest`.

One generated image archive can be copied to both servers. Each server loads all images for operational simplicity, even though it starts only its assigned services.

## Verification

Local verification must cover:

1. Deployment layout tests pass.
2. Rendered cloud Compose contains only `api` and `frontend` and uses the internal API upstream.
3. Rendered intranet Compose contains only `worker` and `recbole_trainer` and uses `49.232.25.254:15432` and `49.232.25.254:6379` from its example environment.
4. Shell scripts pass syntax checks.
5. When Docker and network access are available, the offline package builds successfully, its checksum verifies, and all exported images report `linux/amd64`.

Runtime acceptance remains server-side: API and frontend health checks pass on the cloud server, and worker/trainer logs show successful PostgreSQL and Redis connections on the current server.
