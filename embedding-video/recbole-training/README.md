# RecBole Recommendation Training

This directory owns the offline RecBole pipeline for the active HTTP service. It exports interaction data from `video-service`, trains an embedding model, evaluates a candidate, applies the publish gate, and (only after the gate passes) imports numeric video-segment and user embeddings into PostgreSQL schema `recsys`.

## Run The Pipeline

Run from this directory. The checked-in local service configuration is an example; use the configuration that points at the database to export.

```bash
CONFIG_FILE=../video-service/configs/video.yml ./scripts/run_recbole_pipeline.sh
```

The script first prefers the prebuilt `export_recbole_dataset`, `export_active_recsys_model_metrics`, and `import_recsys_embeddings` binaries on `PATH`. When they are not available, it runs the corresponding `go run ./tools/...` command from `SERVICE_DIR`. The Python stages run with `PYTHON_BIN`; an executable `./.venv/bin/python` is preferred, then `python3`.

The stages are deliberately ordered:

1. Export atomic files and exporter statistics from PostgreSQL.
2. Require `<DATASET>.train.inter`, `<DATASET>.valid.inter`, and `<DATASET>.test.inter`; reject a `knowledge_video:` token in validation or test.
3. Export the active model metrics as the baseline.
4. Train and evaluate RecBole, then write `metrics.json` and embedding CSVs.
5. Run the Python publish gate when `PUBLISH_GATE_ENABLED=true`.
6. Validate that every item embedding ID is a positive decimal video-segment ID, import into `recsys`, and publish the model version.

The final import command is not reached when a required file, virtual-item check, embedding check, training step, or enabled gate fails.

## Supported Shell Variables

These are the variables consumed by `scripts/run_recbole_pipeline.sh`:

| Variable | Default | Purpose |
| --- | --- | --- |
| `SERVICE_DIR` | repository `video-service` | HTTP service directory used for Go tools and configuration lookup |
| `CONFIG_FILE` | `${SERVICE_DIR}/configs/video.yml` | PostgreSQL and service configuration passed to the Go tools |
| `MODEL_VERSION` | `recbole_<YYYYMMDD_HHMMSS>` | Candidate version and output directory name |
| `MODEL_NAME` | `recbole` | Online model name written to `recsys` |
| `RECBOLE_MODEL` | `BPR` | RecBole model class |
| `DATASET` | `_video` | RecBole dataset prefix |
| `DIM` | `64` | User/item embedding dimension |
| `EPOCHS` | `20` | RecBole training epochs |
| `SAMPLE_LIMIT` | `10000` | Maximum ordinary interaction rows queried by the exporter |
| `DAYS_BACK` | `30` | Interaction and knowledge-video watch lookback window |
| `PYTHON_BIN` | project `.venv/bin/python`, otherwise `python3` | Python executable for training and gate stages |
| `DATA_ROOT` | `data/${MODEL_VERSION}` | Parent directory for exported datasets |
| `DATA_DIR` | `${DATA_ROOT}/${DATASET}` | Directory containing this run's atomic files |
| `ARTIFACT_DIR` | `artifacts/${MODEL_VERSION}` | Candidate checkpoints, metrics, and embeddings |
| `BASELINE_METRICS` | `${ARTIFACT_DIR}/baseline_metrics.json` | Baseline metrics exported before training |
| `PUBLISH_GATE_ENABLED` | `true` | Whether to execute `recbole_recommendation.publish_gate` |

`MIN_RECALL_AT_20`, `MIN_NDCG_AT_20`, `MAX_RELATIVE_NDCG_DROP`, and `ARTIFACT_RETENTION_DAYS` are not shell inputs to this script. Gate thresholds are Python command-line flags: `--min-recall-at-20`, `--min-ndcg-at-20`, `--max-relative-ndcg-drop`, `--min-positive-rows`, and `--min-positive-users`. The default metric thresholds are Recall@20 `>= 0.01`, NDCG@20 `>= 0.005`, and at most a `20%` relative NDCG@20 decline from the baseline; the two positive-data thresholds default to `1`.

## Dataset Contract

`tools/export_recbole_dataset` writes all of the following under `DATA_DIR`:

```text
<DATASET>.train.inter
<DATASET>.valid.inter
<DATASET>.test.inter
<DATASET>.item
<DATASET>.user
<DATASET>.export_stats.json
```

The interaction files have `user_id`, `item_id`, `rating`, `timestamp`, `source`, and `weight` columns. Ordinary items are numeric video-segment IDs. The exporter combines video reactions, watch/exposure events, and question-search follow-up events, deduplicating each user/video-segment pair in favor of the higher-value interaction. The `.user` file contains the broader learning-profile features used by the current exporter; `.item` contains video-segment/content metadata.

The benchmark split is deterministic and time ordered per user: the last ordinary interaction is test, the preceding ordinary interaction is validation, and earlier ordinary interactions are training. Users with only one or two ordinary interactions are handled by the exporter without inventing validation rows. RecBole is configured with `benchmark_filename=["train", "valid", "test"]`, user grouping, chronological order, and full-sort evaluation.

### Knowledge-video signal

An effective knowledge-video watch is an aggregated watch session whose watched seconds reach at least `60%` of the video duration. It is represented only during training as a namespaced virtual item such as `knowledge_video:88`.

Virtual items are intentionally excluded from validation and test targets, masked out of RecBole full-sort scores, and filtered from `item_embeddings.csv`. Therefore they are not imported into `recsys`, are not database video-segment candidates, and can never be returned by online recommendation. The exporter may still include the virtual token in `<DATASET>.item` so RecBole can resolve the training vocabulary; that file's presence does not make the token an online item.

## Candidate Artifacts And Gate

The Python stages write these candidate files under `ARTIFACT_DIR`:

```text
recbole_config.yaml
metrics.json
item_embeddings.csv
user_embeddings.csv
checkpoints/...
```

`metrics.json` contains the normalized RecBole metrics (`framework`, `algorithm`, `model_version`, Recall@20, NDCG@20, Hit@20, and Precision@20), numeric exporter statistics from `<DATASET>.export_stats.json`, and the count of filtered virtual item embeddings. The publish gate compares Recall@20 and NDCG@20 with the configured floors and the active baseline's relative NDCG. Its quality configuration also exposes `positive_rows >= 1` and `positive_users >= 1` through the CLI flags described above.

Current limitation: `publish_gate.evaluate` checks `positive_rows` and `positive_users` only when those keys exist in `metrics.json`; the current exporter writes knowledge-watch/filter statistics but does not yet emit those two keys. Consequently, the existing shell pipeline does not guarantee that the positive-data checks are triggered. Treat this as a data-quality follow-up, not as permission to add unsupported environment variables.

When the gate fails, the script exits before `import_recsys_embeddings --publish`. The previous active model remains active, and the complete candidate directory is retained for diagnosis. When the gate is disabled, the script still performs the file and numeric-ID checks before import; disabling it is an explicit operational choice.

## Embedding And Serving Boundary

The CSV contract is:

```text
item_embeddings.csv: video_segment_id,video_id,embedding,model_version
user_embeddings.csv: user_id,embedding,model_version
```

The importer accepts only numeric `video_segment_id` values and writes:

```text
recsys.recommend_model_version
recsys.recommend_user_embedding
recsys.recommend_item_embedding
```

Online serving reads ordinary numeric video-segment embeddings from `recsys` only. It does not read the training directory, virtual knowledge-video items, legacy public embedding tables, or Gorse storage as a replacement for the active RecBole model. Gorse can consume the HTTP service's internal external-candidate endpoint when that optional integration is enabled.

Legacy recommendation tables are not created by this pipeline. If a cleanup is intentionally scheduled, validate and back up first, then run the explicit tool from the HTTP service directory:

```bash
cd ../video-service
go run ./tools/drop_legacy_recommendation_tables --execute --confirm drop-legacy-recommendation-tables
```

## Dependencies And Verification

`requirements.txt` uses bounded version ranges, not a fully pinned lockfile (`numpy>=1.24,<2.0`, `pandas>=1.5,<2.2`, `torch>=2.0,<2.4`, and minimum versions for RecBole/PyYAML). Create a virtual environment and install those ranges before a local run:

```bash
python3 -m venv .venv
source .venv/bin/activate
python -m pip install -r requirements.txt
PYTHONPATH=src python -m unittest discover -s tests -p 'test_*.py'
```

The production image installs its own CPU-compatible PyTorch build, but the source requirements remain bounded ranges. Generated data and artifacts are runtime outputs, not API contracts; the active version and metrics stored in PostgreSQL define serving state. The HTTP service's `cmd/recboletrainer` invokes this script from its scheduler/container integration.
