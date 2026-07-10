# RecBole Recommendation Training

This project exports RecBole atomic files from `video-service`, trains a RecBole embedding model, gates offline metrics, imports embeddings into PostgreSQL schema `recsys`, and publishes an active model version.

Default pipeline:

```bash
CONFIG_FILE=../video-service/configs/video.yml ./scripts/run_recbole_pipeline.sh
```

Important outputs:

- `data/<model_version>/video_dataset/video_dataset.inter`
- `data/<model_version>/video_dataset/video_dataset.item`
- `data/<model_version>/video_dataset/video_dataset.user`
- `artifacts/<model_version>/item_embeddings.csv`
- `artifacts/<model_version>/user_embeddings.csv`
- `artifacts/<model_version>/metrics.json`

Online serving reads only:

- `recsys.recommend_model_version`
- `recsys.recommend_user_embedding`
- `recsys.recommend_item_embedding`

Legacy public embedding/version tables are intentionally not created by this pipeline. Use `go run ./tools/drop_legacy_recommendation_tables --execute --confirm drop-legacy-recommendation-tables` only after validation and backup.
