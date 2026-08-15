# Migration Verification

## Revisions

- Source HEAD: `660546c2ec07189296b21da8f711adef66858647`
- Source baseline: current working tree, including tracked and untracked changes
- Target starting HEAD: `f887142`
- Migration branch: `codex/migrate-hengshui-project`

## Baseline

| Command | Exit | Result |
| --- | ---: | --- |
| `go test ./...` in `video-service` | 1 | One stable pre-migration failure in `TestRecommendationEffectMetricsAggregatesByDayAndStrategy` |
| focused target persistence test with `-count=3` | 1 | Failed 3/3 with zero daily rows |
| focused source persistence test with `-count=3` | 0 | Passed 3/3; source migration contains the correction |
| `npm ci && npm test && npm run build` in `hls-web` | 0 | 13 files and 70 tests passed; production build completed |
| `python3 -m pytest -q` in `recbole-training` | 1 | Environment block: system Python 3.14 has no pytest |
| `python3 -m pytest -q` in `two-tower-training` | 1 | Environment block: system Python 3.14 has no pytest |

The Python commands did not load or execute project tests. An isolated supported
Python environment will be prepared before migrated verification.

## Migrated Verification

Fresh post-migration results will be recorded here without environment values
or secret output.

---

# Incremental Sync Verification (2026-08-15)

Source HEAD: `1bb7f83` (yinlihupo/hengshui-tablet-video). Baseline: `660546c`
(previous migration source HEAD). 81 files changed in source delta; ported via
three-way merge (base = transformed `660546c`, theirs = transformed source HEAD,
ours = current target working tree), preserving target naming conventions
(`video-service`, `legacy-video`, `VIDEO_APP_*`, `video-app` database, neutral hosts).

## Results

| Command | Exit | Result |
| --- | ---: | --- |
| `scripts/validate-repository.sh` | 0 | No business identifiers, tracked private env files, key material, or credential assignments |
| `go build ./...` in `video-service` | 0 | Build OK (incl. new `cmd/knowledgevideo-import`) |
| `go test ./...` in `video-service` | 0 | All packages ok |
| `go build ./...` in `legacy-video` | 0 | Build OK |
| `go test ./...` in `legacy-video` | 0 | All packages ok |
| `npm test` in `hls-web` | 0 | 19 files, 89 tests passed |
| `npm run build` in `hls-web` | 0 | Production build completed |
| `PYTHONPATH=src python3 -m unittest discover -s tests` in `recbole-training` | 0 | 25 tests passed |
| `PYTHONPATH=src python3 -m unittest discover -s tests` in `two-tower-training` | 0 | 40 tests passed (archived module, unchanged) |

## Conflicts Resolved Manually

- Root env examples (`.env.local.example`, `.env.deploy.example`): replaced with
  transformed source HEAD templates, fixing line-join mangling from the first migration.
- `deployment/DEPLOYMENT.md`, `deployment/env/*.env.example`,
  `deployment/scripts/healthcheck.sh`, `deployment/tests/validate-layout.sh`:
  kept target placeholder hosts (`services.example.internal`), adopted source
  additions (JWT guidance, `sslmode` notes, `JWTExpireHour` default note).
- `video-service/README.md`: kept target DLQ section structure, appended source
  note on the 8 supported DLQ queues.
- `legacy-video/README.md`, `recbole-training/README.md`: replaced with
  transformed source HEAD.
- `recbole-training/src/recbole_recommendation/config.py`: adopted source HEAD
  (removed target-side `RS` split; benchmark files are pre-split), dataset
  default normalized to `video_app`.
