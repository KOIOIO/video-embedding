# Cloud API And Intranet Services Deployment Design

## Goal

Produce one offline deployment archive for a split deployment where the cloud
server at `49.232.25.254` runs only the HTTP API and the current server runs the
frontend, worker, and RecBole trainer.

## Service Ownership

- Cloud Compose contains only `api`, running the `httpapi` command.
- Intranet Compose contains `frontend`, `worker`, and `recbole_trainer`.
- Standalone Compose remains unchanged and continues to contain all four
  services.

## Communication

- The cloud API connects to PostgreSQL and Redis on the cloud host through
  `host.docker.internal` and Docker's `host-gateway` mapping.
- The intranet worker and RecBole trainer connect directly to PostgreSQL at
  `49.232.25.254:15432` and Redis at `49.232.25.254:6379`.
- The intranet frontend listens on host port `1325`. Its Nginx proxy forwards
  API and media routes to `http://49.232.25.254:8083`.
- HTTPS and certificate management are outside this deployment. All cross-host
  application traffic uses plain HTTP or the native PostgreSQL/Redis protocols.

## Secrets And Access Control

- `JWT_SECRET` is present only in the cloud API environment and must contain at
  least 32 characters.
- Database, Redis, object-storage, and AI credentials remain in private runtime
  `.env` files and must not enter the archive.
- The cloud firewall should allow ports `8083`, `15432`, and `6379` only from
  the current server's fixed outbound IP.
- Port `1325` is opened on the current server according to the required user
  access range.

## Deployment Package Changes

- Remove `frontend` and its frontend-specific environment values from the cloud
  Compose project and cloud environment template.
- Add `frontend` to the intranet Compose project with
  `API_UPSTREAM=${API_UPSTREAM}` and host port `VIDEO_APP_WEB_PORT`.
- Add the frontend image, port, bind address, and API upstream values to the
  intranet environment template.
- Update health checks, layout tests, and operator documentation for the new
  ownership.
- Repackage the existing verified `linux/amd64` images because only deployment
  metadata changes; rebuilding application images is unnecessary.

## Acceptance Criteria

- Cloud Compose renders exactly `api`.
- Intranet Compose renders exactly `frontend`, `worker`, and
  `recbole_trainer`.
- The intranet frontend renders an upstream of
  `http://49.232.25.254:8083`.
- JWT configuration remains limited to the cloud API and standalone API.
- Shell syntax, Compose rendering, deployment layout tests, archive content,
  image architecture, and SHA-256 verification all pass.
- No private runtime environment file or supplied credential is present in the
  generated archive.
