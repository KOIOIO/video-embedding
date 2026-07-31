# RecBole Random-Play Watch Precision Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Correct RecBole training semantics for random-play effective watching at a 60% segment-watch threshold and make offline publishing decisions reflect time-ordered behavior.

**Architecture:** Keep the existing BPR/embedding serving boundary. Make the Go exporter produce deterministic, semantically prioritized interaction rows and export quality statistics; make Python configure chronological evaluation and persist normalized metrics; extend the existing publish gate with minimum data-quality checks without changing the online API or database schema.

**Tech Stack:** Go, PostgreSQL query fixtures, Python 3, RecBole, unittest, existing shell pipeline.

---

### Task 1: Fix interaction event semantics and aggregation

**Files:**
- Modify: `video-service/tools/export_recbole_dataset/main.go`
- Test: `video-service/tools/export_recbole_dataset/main_test.go`

- [ ] **Step 1: Add failing unit tests for the 60% boundary and semantic priority.**

  Add table cases around `interactionFromEvent`: ratio `0.599` is weak/negative, ratio `0.600` is effective-watch positive; add aggregation cases proving a later exposure cannot replace an earlier effective watch or like, while a dislike is retained only when no higher-priority positive exists.

- [ ] **Step 2: Run the focused Go tests and verify the new cases fail.**

  Run: `cd video-service && go test ./tools/export_recbole_dataset -run 'Test(Interaction|BuildInteraction)' -count=1`

  Expected: FAIL because the current reducer only compares timestamps and the current threshold is 40%.

- [ ] **Step 3: Implement a named event class and deterministic reducer.**

  Add an internal class/priority helper used by `interactionFromEvent` and `buildInteractionRows`. Set effective watch at `ratio >= 0.60`; preserve positive classes over later lower-value events; retain source, timestamp, rating, and weight from the selected event. Keep the existing public output columns.

- [ ] **Step 4: Run the focused Go tests and the complete exporter package tests.**

  Run: `cd video-service && go test ./tools/export_recbole_dataset -count=1`

  Expected: PASS.

- [ ] **Step 5: Commit the exporter change.**

  Run: `git add video-service/tools/export_recbole_dataset/main.go video-service/tools/export_recbole_dataset/main_test.go && git commit -m "fix: prioritize effective watch events in recbole export"`

### Task 2: Add exporter quality statistics and correct SQL scan coverage

**Files:**
- Modify: `video-service/tools/export_recbole_dataset/main.go`
- Test: `video-service/tools/export_recbole_dataset/main_test.go`

- [ ] **Step 1: Add failing tests for statistics and SQL column/scan parity.**

  Test that exported rows report total rows, users, positive rows, negative/weak rows, and counts by source/class. Add a string-level SQL test that the interaction query has exactly the columns scanned by `loadInteractionEvents`, and that the item query does not duplicate a selected column.

- [ ] **Step 2: Run the focused tests and verify failure.**

  Run: `cd video-service && go test ./tools/export_recbole_dataset -count=1`

  Expected: FAIL on missing statistics and the existing query/scan mismatch.

- [ ] **Step 3: Implement statistics serialization and fix query/scan alignment.**

  Introduce a small JSON-serializable export summary returned by `buildInteractionRows` or an adjacent pure helper; print it as a stable summary line or write it under the dataset output directory as specified by the pipeline contract. Remove the duplicate item select expression and make interaction `rows.Scan` match the exact select list. Keep errors immediate on scan failure.

- [ ] **Step 4: Run Go formatting and tests.**

  Run: `gofmt -w video-service/tools/export_recbole_dataset/main.go video-service/tools/export_recbole_dataset/main_test.go && cd video-service && go test ./tools/export_recbole_dataset -count=1`

  Expected: PASS.

- [ ] **Step 5: Commit the exporter quality instrumentation.**

  Run: `git add video-service/tools/export_recbole_dataset/main.go video-service/tools/export_recbole_dataset/main_test.go && git commit -m "feat: record recbole export data quality"`

### Task 3: Configure chronological RecBole evaluation

**Files:**
- Modify: `recbole-training/src/recbole_recommendation/config.py`
- Test: `recbole-training/tests/test_recbole_config.py`

- [ ] **Step 1: Add failing configuration assertions.**

  Assert that the generated config has a fixed seed, chronological split/evaluation settings, explicit negative sampling, and the existing `NDCG@20` validation target. Keep embedding size, top-k, and model defaults backward compatible.

- [ ] **Step 2: Run Python config tests and verify failure.**

  Run: `cd recbole-training && python -m unittest tests.test_recbole_config -v`

  Expected: FAIL because the current config relies on RecBole defaults.

- [ ] **Step 3: Add the minimum explicit RecBole settings.**

  Set deterministic seed, `eval_args` with time ordering and held-out validation/test behavior, and explicit training negative sampling compatible with BPR. Do not load unused user/item feature fields into the BPR interaction model unless RecBole accepts them without changing the embedding export contract.

- [ ] **Step 4: Run Python tests and inspect the generated YAML.**

  Run: `cd recbole-training && python -m unittest tests.test_recbole_config -v && python -m unittest discover -s tests -p 'test_*.py'`

  Expected: PASS; generated YAML contains the asserted settings.

- [ ] **Step 5: Commit the chronological evaluation configuration.**

  Run: `git add recbole-training/src/recbole_recommendation/config.py recbole-training/tests/test_recbole_config.py && git commit -m "feat: make recbole evaluation chronological"`

### Task 4: Persist effective-watch metrics and enforce data-quality gates

**Files:**
- Modify: `recbole-training/src/recbole_recommendation/train.py`
- Modify: `recbole-training/src/recbole_recommendation/metrics.py`
- Modify: `recbole-training/src/recbole_recommendation/publish_gate.py`
- Test: `recbole-training/tests/test_recbole_train.py`
- Test: `recbole-training/tests/test_recbole_publish_gate.py`

- [ ] **Step 1: Add failing tests for normalized watch metrics and insufficient-data rejection.**

  Test that metrics normalization preserves RecBole ranking metrics plus `EffectiveWatch@20` and export counts when present. Test that a candidate with fewer than the configured minimum positive users/rows is rejected with a stable reason, while a sufficient candidate still applies the existing Recall/NDCG/baseline checks.

- [ ] **Step 2: Run the focused Python tests and verify failure.**

  Run: `cd recbole-training && python -m unittest tests.test_recbole_train tests.test_recbole_publish_gate -v`

  Expected: FAIL because watch metrics and data-quality gate fields are not handled.

- [ ] **Step 3: Implement metric normalization and gate configuration.**

  Preserve unknown numeric metrics, add stable canonical names for effective-watch and sample counts, and add explicit gate arguments/defaults for minimum positive rows and users. Keep gate failure non-mutating and retain the active baseline relative-drop check.

- [ ] **Step 4: Run the complete RecBole unit test suite.**

  Run: `cd recbole-training && python -m unittest discover -s tests -p 'test_*.py'`

  Expected: PASS.

- [ ] **Step 5: Commit the metric and gate changes.**

  Run: `git add recbole-training/src/recbole_recommendation/train.py recbole-training/src/recbole_recommendation/metrics.py recbole-training/src/recbole_recommendation/publish_gate.py recbole-training/tests/test_recbole_train.py recbole-training/tests/test_recbole_publish_gate.py && git commit -m "feat: gate recbole models on watch data quality"`

### Task 5: Verify the pipeline contract without requiring production services

**Files:**
- Modify: `recbole-training/tests/test_recbole_pipeline_script.py`
- Modify: `recbole-training/README.md` only if supported environment variables or output files change

- [ ] **Step 1: Add failing shell-script contract assertions.**

  Assert that the pipeline passes the export summary/artifact location through, invokes the updated gate, and leaves publish behavior unchanged when the gate fails.

- [ ] **Step 2: Run the script contract tests.**

  Run: `cd recbole-training && python -m unittest tests.test_recbole_pipeline_script -v`

  Expected: FAIL until the script and test fixtures agree on the new quality artifact.

- [ ] **Step 3: Make the smallest script/documentation update.**

  Pass only the new supported paths/arguments needed by the exporter and gate. Keep `MODEL_VERSION`, `SAMPLE_LIMIT`, `DAYS_BACK`, and existing publish gate behavior compatible.

- [ ] **Step 4: Run all repository-local RecBole tests and Go exporter tests.**

  Run: `cd recbole-training && python -m unittest discover -s tests -p 'test_*.py'`; then `cd ../video-service && go test ./tools/export_recbole_dataset`

  Expected: PASS. A real pipeline run remains environment-dependent on PostgreSQL, RecBole/PyTorch, and the configured embedding tables.

- [ ] **Step 5: Commit the verified pipeline contract.**

  Run: `git add recbole-training/tests/test_recbole_pipeline_script.py recbole-training/README.md recbole-training/scripts/run_recbole_pipeline.sh && git commit -m "test: verify recbole watch-quality pipeline"`
