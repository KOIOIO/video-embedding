# RecBole Recommendation Training

This project exports RecBole atomic files from `video-service`, trains a RecBole embedding model, gates offline metrics, imports embeddings into PostgreSQL schema `recsys`, and publishes an active model version.

Default pipeline (run from this directory):

```bash
CONFIG_FILE=../video-service/configs/video.yml ./scripts/run_recbole_pipeline.sh
```

The script defaults to `BPR`, `embedding_size=64`, `20` epochs, a `10000`-event export limit, and a `30`-day lookback. Override only the supported shell variables: `MODEL_VERSION`, `MODEL_NAME`, `RECBOLE_MODEL`, `DATASET`, `DIM`, `EPOCHS`, `SAMPLE_LIMIT`, `DAYS_BACK`, `PYTHON_BIN`, `DATA_ROOT`, `DATA_DIR`, `ARTIFACT_DIR`, `BASELINE_METRICS`, and `PUBLISH_GATE_ENABLED`.

When the publish gate is enabled, the default thresholds are `Recall@20 >= 0.01`, `NDCG@20 >= 0.005`, and no more than a `20%` relative `NDCG@20` drop from the active model. The pipeline intentionally leaves the active version unchanged when this gate fails.

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

Legacy public embedding/version tables are intentionally not created by this pipeline. Run the explicit cleanup tool from the HTTP service directory only after validation and backup:

```bash
cd ../video-service
go run ./tools/drop_legacy_recommendation_tables --execute --confirm drop-legacy-recommendation-tables
```

The Python dependencies are pinned in `requirements.txt`; use a virtual environment compatible with its PyTorch constraint before starting a local training run.
