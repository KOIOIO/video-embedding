# RecBole Algorithm Handoff

This document is the algorithm and artifact contract for the current RecBole integration. The operational command, environment defaults, and local setup live in [`README.md`](README.md); the HTTP-side data/export overview is [`video-service/docs/recbole-recommendation-pipeline.md`](../video-service/docs/recbole-recommendation-pipeline.md).

## Current Model

The checked-in service configurations select `Recommendation.Engine=recbole`. The production candidate is RecBole `BPR` with `embedding_size=64`, deterministic seed `20260730`, chronological per-user ordering, and full-sort evaluation. BPR's user/item dot-product embeddings match the pgvector retrieval contract. Gorse is an optional external-candidate consumer; it is not a substitute for the `recsys` model store.

## Inputs And Splits

`tools/export_recbole_dataset` reads the HTTP service database and writes a RecBole atomic dataset under `DATA_DIR`:

```text
<DATASET>.train.inter
<DATASET>.valid.inter
<DATASET>.test.inter
<DATASET>.item
<DATASET>.user
<DATASET>.export_stats.json
```

The three `.inter` files are mandatory. They contain `user_id`, `item_id`, `rating`, `timestamp`, `source`, and `weight`. Ordinary `item_id` values are numeric video-segment IDs. The exporter combines segment/video reactions, watch and exposure events, and question-search follow-up events, then deduplicates ordinary user/segment interactions. User features cover the broader learning profile exported by the service, while item features describe video segments and their content metadata.

The split is created before Python training, not by an implicit random split. For each user, ordinary interactions are ordered by timestamp (with a stable item-ID tie break): the latest row is test, the preceding row is validation, and earlier rows are train. The Python config loads `benchmark_filename=["train", "valid", "test"]`, groups evaluation by user, and uses chronological order. The shell script fails closed if either validation or test contains a virtual token.

## Knowledge-video Virtual Items

Watch sessions are aggregated by user, knowledge video, and session, then by user and knowledge video. An aggregate is effective when watched seconds reach at least `60%` of the video's duration. Each effective aggregate becomes one training interaction with an item token such as `knowledge_video:88`.

The virtual-item boundary is strict:

| Stage | Treatment of `knowledge_video:<id>` |
| --- | --- |
| Training interactions | Allowed as a training-only signal |
| Validation/test targets | Excluded by `benchmarkSplit`; the shell script also rejects leaks |
| Full-sort evaluation | Internal IDs are masked to `-inf` before ranking |
| Item embedding export | `usable_token` keeps only positive numeric IDs; virtual tokens are filtered |
| Database import | No virtual row can pass the numeric `video_segment_id` check |
| Online candidates | Never an ordinary video-segment candidate or recommendation |

The exporter can retain a virtual token in `<DATASET>.item` so RecBole's training vocabulary resolves it. That catalog row is not an online item, and no virtual item is written to `item_embeddings.csv` or `recsys`.

## Artifacts

The training wrapper writes a config, checkpoint, metrics, and two CSVs:

```text
item_embeddings.csv: video_segment_id,video_id,embedding,model_version
user_embeddings.csv: user_id,embedding,model_version
metrics.json: framework,algorithm,model_version,Recall@20,NDCG@20,Hit@20,Precision@20 plus numeric export/filter statistics
```

`train.py` merges numeric fields from `<DATASET>.export_stats.json` into `metrics.json` and records `filtered_virtual_item_embeddings`. Non-numeric values in the exporter statistics are ignored. The metrics file is copied into the model-version row when the importer publishes it, so it is the serving audit record rather than a replacement for the raw candidate directory.

## Publish Gate And Rollback Boundary

`recbole_recommendation.publish_gate` exposes these Python CLI flags:

```text
--min-recall-at-20       default 0.01
--min-ndcg-at-20         default 0.005
--max-relative-ndcg-drop default 0.20
--min-positive-rows      default 1
--min-positive-users     default 1
```

The gate compares Recall@20 and NDCG@20 with the floors and compares NDCG@20 with the active baseline. It also checks `positive_rows >= 1` and `positive_users >= 1` when those optional keys are present in `metrics.json`. Current limitation: the exporter emits knowledge-watch and filtering statistics but does not emit `positive_rows` or `positive_users`, so the existing shell pipeline does not guarantee that those two checks run. Do not represent the four threshold names (`MIN_RECALL_AT_20`, `MIN_NDCG_AT_20`, `MAX_RELATIVE_NDCG_DROP`, `ARTIFACT_RETENTION_DAYS`) as supported shell variables.

With `PUBLISH_GATE_ENABLED=true`, gate failure stops the pipeline before `import_recsys_embeddings --publish`. It leaves the active `recsys` model version untouched and retains the candidate artifacts for diagnosis. A successful gate is followed by numeric-ID validation, embedding import, and an atomic active-version publication. The importer then cleans only the configured old embedding versions; it does not turn a failed candidate into the active version.

## Online Contract

Online Go code reads only ordinary numeric video-segment and user embeddings from:

```text
recsys.recommend_model_version
recsys.recommend_user_embedding
recsys.recommend_item_embedding
```

Serving does not read `DATA_DIR`, `ARTIFACT_DIR`, the RecBole checkpoint, or virtual knowledge-video tokens. Any Gorse integration receives ordinary segment IDs through the HTTP internal external-candidate route and remains subject to the Go service's playable-candidate filtering.

## Pipeline Variables And Dependencies

The shell entrypoint consumes exactly these variables (defaults are defined in the script):

```text
SERVICE_DIR CONFIG_FILE MODEL_VERSION MODEL_NAME RECBOLE_MODEL DATASET DIM EPOCHS
SAMPLE_LIMIT DAYS_BACK PYTHON_BIN DATA_ROOT DATA_DIR ARTIFACT_DIR BASELINE_METRICS
PUBLISH_GATE_ENABLED
```

Gate thresholds are CLI flags, not environment overrides. `requirements.txt` uses installable version constraints/ranges (for example `numpy>=1.24,<2.0`, `pandas>=1.5,<2.2`, and `torch>=2.0,<2.4`) rather than a completely pinned dependency lock. Re-run the RecBole unit tests after changing the exporter, split semantics, virtual-item masking, embedding export, or gate behavior.

For a source checkout, run the tests with the `src/` layout on the import path:

```bash
PYTHONPATH=src python3 -m unittest discover -s tests -p 'test_*.py'
```
