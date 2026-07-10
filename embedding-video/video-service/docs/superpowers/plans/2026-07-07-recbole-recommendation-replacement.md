# RecBole Recommendation Replacement Plan

## Goal

Make RecBole the only personalized embedding recall implementation in this repo, with Gorse able to use it as `external`.

## Tasks

- [ ] Create `recsys` schema DDL and call it from service migration.
- [ ] Move online RecBole reads to `recsys.recommend_model_version`, `recsys.recommend_user_embedding`, and `recsys.recommend_item_embedding`.
- [ ] Expose `/api/internal/recommendations/external/recbole` and remove the old external route.
- [ ] Add `tools/export_recbole_dataset` for RecBole atomic files using video behavior plus full-system user behavior.
- [ ] Add `recbole-training/` with RecBole config, training wrapper, embedding export, metrics, publish gate, Dockerfile, and pipeline script.
- [ ] Add `tools/import_recsys_embeddings` and `tools/export_active_recsys_model_metrics`.
- [ ] Add dry-run-first `tools/drop_legacy_recommendation_tables`.
- [ ] Delete legacy trainer commands, legacy Go import/export tools, legacy Python training project, and current-link docs that describe the old path.
- [ ] Update `docker-compose.yml`, configs, README, and Gorse runbook to use RecBole names.
- [ ] Regenerate Swagger from annotations if the generator is available.

## Verification

Run from `video-service/`:

```bash
GOCACHE=/private/tmp/hstv-go-build go test ./tools/export_recbole_dataset ./tools/import_recsys_embeddings ./tools/export_active_recsys_model_metrics ./tools/drop_legacy_recommendation_tables -count=1
GOCACHE=/private/tmp/hstv-go-build go test ./internal/application/videoapp ./internal/application/videoapp/recommendation ./internal/infrastructure/persistence ./internal/infrastructure/persistence/sqlqueries -count=1
GOCACHE=/private/tmp/hstv-go-build go test ./...
```

Run from `recbole-training/`:

```bash
PYTHONPATH=src python3 -m unittest discover -s tests
```

Final grep should show no old personalized embedding implementation names in runtime code, configs, README, or current docs. Legacy public table names may remain only in the explicit cleanup and migration-skip tools.
