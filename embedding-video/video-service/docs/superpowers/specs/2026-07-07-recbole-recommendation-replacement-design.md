# RecBole Recommendation Replacement Design

## Goal

Replace the legacy custom personalized embedding path with a RecBole-based embedding recall path. New model state is owned by PostgreSQL schema `recsys`, and Gorse external fallback should call `/api/internal/recommendations/external/recbole`.

## Serving Boundary

Online serving uses these repository concepts:

- `GetActiveRecBoleModelVersion`
- `GetUserRecBoleEmbedding`
- `FindRecommendedSegmentsForRecBole`

Online reads target:

- `recsys.recommend_model_version`
- `recsys.recommend_user_embedding`
- `recsys.recommend_item_embedding`

If no active RecBole model or no usable user embedding exists, the service returns no RecBole candidates and the higher-level fallback path handles the request.

## Training Boundary

The training project is `recbole-training/`.

Pipeline:

1. `go run ./tools/export_recbole_dataset` exports RecBole atomic `.inter`, `.item`, and `.user` files.
2. `python3 -m recbole_recommendation.train` trains a RecBole model, initially `BPR`.
3. `python3 -m recbole_recommendation.publish_gate` validates offline metrics.
4. `go run ./tools/import_recsys_embeddings --publish` imports embeddings and publishes the active version in `recsys`.

User data must include video behavior plus broader system behavior: profile, mastery, answers, feedback, search, practice, vocabulary, English learning sessions, and profile snapshots.

## Legacy Cleanup

The service and importer must not recreate legacy public recommendation embedding/version tables. Table removal is explicit and dry-run-first via:

```bash
go run ./tools/drop_legacy_recommendation_tables
go run ./tools/drop_legacy_recommendation_tables --execute --confirm drop-legacy-recommendation-tables
```

Do not drop production tables until a RecBole version has been imported, published, and validated from `recsys`.

## Rollout

Default production config may remain `Recommendation.Engine=gorse`. RecBole can be validated either through Gorse external fallback or by setting `Recommendation.Engine=recbole` in a validation environment.
