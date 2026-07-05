# Two Tower User Feature Training Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add education-specific user learning features to two-tower training and schedule training every four hours from 00:00 Beijing time.

**Architecture:** Extend the existing Go sample exporter with an optional user feature catalog CSV. Extend Python training to load the CSV into `Dataset.user_features` and feed it into a PyTorch user feature MLP. Keep existing embedding artifact/import protocols unchanged.

**Tech Stack:** Go `database/sql` + pgx, Python unittest, PyTorch, shell pipeline script.

---

### Task 1: Export User Feature Catalog

**Files:**
- Modify: `video-service/tools/export_two_tower_samples/main.go`
- Modify: `video-service/tools/export_two_tower_samples/main_test.go`

- [ ] Add `--user-output` parse option and tests.
- [ ] Add `userFeatureRow` and `writeUserFeatureCSV`.
- [ ] Add `loadUserFeatureCatalog`, using optional aggregate CTEs for mastery, answer records, feedback, generated feedback, and search records.
- [ ] Emit neutral aggregate values when an optional source table is missing.
- [ ] Run `go test ./tools/export_two_tower_samples`.

### Task 2: Load User Features in Training

**Files:**
- Modify: `two-tower-training/src/two_tower_training/train.py`
- Modify: `two-tower-training/tests/test_train.py`

- [ ] Add `UserFeature` dataclass.
- [ ] Add `load_user_features(path)`.
- [ ] Add `user_features` to `Dataset`.
- [ ] Add `--user-features` CLI argument.
- [ ] Ensure dataset helpers preserve `user_features`.
- [ ] Run `PYTHONPATH=src python3 -m unittest tests.test_train`.

### Task 3: Use User Feature MLP in PyTorch

**Files:**
- Modify: `two-tower-training/src/two_tower_training/torch_model.py`
- Modify: `two-tower-training/tests/test_torch_model.py`

- [ ] Add user feature vector builder with hashed text buckets plus numeric features.
- [ ] Add `user_feature_mlp` to `TorchTwoTower`.
- [ ] Change `user_vector` to combine ID embedding and user feature vector.
- [ ] Keep zero-vector fallback for missing user features.
- [ ] Run `PYTHONPATH=src python3 -m unittest tests.test_torch_model`.

### Task 4: Wire Pipeline and Cleanup

**Files:**
- Modify: `two-tower-training/scripts/run_two_tower_pipeline.sh`
- Modify: `two-tower-training/tests/test_pipeline_cleanup.py`

- [ ] Add `USER_FILE="${TRAIN_DIR}/data/${MODEL_VERSION}_user_features.csv"`.
- [ ] Pass `--user-output "${USER_FILE}"` to Go exporter.
- [ ] Pass `--user-features "${USER_FILE}"` to Python trainer.
- [ ] Update fake Go in the pipeline test to write user feature CSV.
- [ ] Assert user feature CSV is cleaned.
- [ ] Run `PYTHONPATH=src python3 -m unittest tests.test_pipeline_cleanup`.

### Task 5: Change Trainer Schedule

**Files:**
- Modify: `video-service/internal/worker/twotowertrainer/scheduler.go`
- Modify: `video-service/internal/worker/twotowertrainer/scheduler_test.go`
- Modify docs mentioning fixed run times if present.

- [ ] Change default schedule to `00:00, 04:00, 08:00, 12:00, 16:00, 20:00`.
- [ ] Update scheduler tests.
- [ ] Run `go test ./internal/worker/twotowertrainer`.

### Task 6: Full Verification

**Files:**
- Verify all touched files.

- [ ] Run Python training tests: `cd two-tower-training && PYTHONPATH=src python3 -m unittest discover -s tests`.
- [ ] Run Go tests: `cd video-service && go test ./tools/export_two_tower_samples ./internal/worker/twotowertrainer`.
- [ ] Run `git diff --check`.
- [ ] Summarize behavior and any remaining risk.
