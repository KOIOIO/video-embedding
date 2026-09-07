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
