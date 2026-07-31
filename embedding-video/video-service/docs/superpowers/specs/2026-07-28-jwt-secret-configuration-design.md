# JWT Secret Configuration Design

## Goal

Allow the HTTP API to start without a manually exported shell variable while keeping production JWT secrets out of Git.

## Configuration Sources

- `configs/video.yml` contains a fixed development-only `Auth.JWTSecret` fallback of at least 32 characters.
- `.env.local` contains the active local `JWT_SECRET` and overrides YAML.
- `.env.deploy` contains a separate production `JWT_SECRET` and overrides YAML.
- `configs/video_prod.yml` keeps `Auth.JWTSecret` empty so no production signing secret is committed.
- Existing precedence remains `JWT_SECRET` environment value, then YAML `Auth.JWTSecret`.

## Safety

The real `.env.local` and `.env.deploy` files remain ignored by Git. Example environment files contain placeholders only. Local and deployment secrets must differ so a development token cannot be accepted in production.

## Verification

Configuration tests assert the local YAML fallback is valid and environment values override it. The HTTP API is started from the documented service root entrypoint without a shell-level `JWT_SECRET` export.
