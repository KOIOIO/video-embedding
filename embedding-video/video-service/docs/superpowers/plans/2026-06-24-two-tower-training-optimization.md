# Two Tower Training Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade the offline two-tower training pipeline so new models are trained on cleaner behavior samples, evaluated with retrieval metrics on held-out recent behavior, compared with the previous active model, and published only when metrics pass explicit gates.

**Architecture:** Keep the existing Go import contract unchanged: training still writes `item_embeddings.csv`, `user_embeddings.csv`, and `metrics.json` with 64-dimensional embeddings by default. Extend the Go export tool to emit both behavior samples and a full eligible item catalog. Split Python training code into focused modules for sample processing, temporal splitting, retrieval metrics, publish gating, and a PyTorch backend. The pipeline gates publication before `go run ./tools/import_two_tower_embeddings --publish`, so a bad model artifact can be retained for debugging without becoming the active online model.

**Tech Stack:** Python 3, PyTorch, `unittest`, existing Go export/import tools, Bash pipeline script, PostgreSQL/pgvector through existing Go tools.

---

## Scope Check

This plan is one cohesive training-pipeline upgrade. It intentionally does not change the online recommendation API, database schema, embedding dimension, or Go importer CSV protocol.

Completed scope:

1. Preserve `source`, `reason`, and `event_time` in Python sample loading.
2. Aggregate duplicate user-segment behavior rows into one training signal.
3. Apply time decay to behavior weights.
4. Split training and evaluation samples by event time.
5. Add retrieval metrics: `recall_at_20`, `recall_at_50`, `hit_rate_at_K`, `ndcg_at_K`, `coverage_at_K`, `negative_hit_rate_at_K`, and `dislike_hit_rate_at_K`.
6. Add a publish gate that fails the pipeline before publishing when absolute thresholds or baseline comparison thresholds fail.
7. Export previous active model metrics from the database before publishing.
8. Export all eligible segment item features and generate item embeddings for catalog-only items.
9. Add PyTorch training with item feature MLP, random negatives, and in-batch hard negatives while preserving the pure-Python SGD fallback.
10. Update README and algorithm handoff docs so future model work uses the new workflow.

Still intentionally unchanged:

1. Online recommendation API and response shape.
2. Go importer CSV protocol and embedding table schema.
3. Production default embedding dimension of 64.

## File Structure

Create:

- `../two-tower-training/src/two_tower_training/sample_processing.py`
  - Parse exported CSV rows with metadata, apply time decay, and aggregate repeated user-segment events.
- `../two-tower-training/src/two_tower_training/splitting.py`
  - Create deterministic temporal train/eval splits.
- `../two-tower-training/src/two_tower_training/metrics.py`
  - Compute pointwise metrics and retrieval metrics.
- `../two-tower-training/src/two_tower_training/publish_gate.py`
  - CLI and library function for checking metrics before publication.
- `../two-tower-training/tests/test_sample_processing.py`
  - Unit tests for parsing, decay, and aggregation.
- `../two-tower-training/tests/test_splitting.py`
  - Unit tests for temporal split behavior.
- `../two-tower-training/tests/test_metrics.py`
  - Unit tests for retrieval metrics.
- `../two-tower-training/tests/test_publish_gate.py`
  - Unit tests for publish threshold decisions.

Modify:

- `../two-tower-training/src/two_tower_training/train.py`
  - Use new sample processing, train on temporal train samples, evaluate on held-out samples, and write richer metrics.
- `../two-tower-training/tests/test_train.py`
  - Update tests for metadata-aware loading and eval metrics.
- `../two-tower-training/scripts/run_two_tower_pipeline.sh`
  - Run the publish gate before the Go importer publishes a version.
- `../two-tower-training/tests/test_pipeline_cleanup.py`
  - Update fake Python behavior so the pipeline test covers gate execution.
- `../two-tower-training/README.md`
  - Document optimized training, metrics, and gate environment variables.
- `../two-tower-training/ALGORITHM_HANDOFF.md`
  - Document the new required metrics and release rule.

## Task 1: Add Sample Processing With Metadata, Time Decay, And Aggregation

**Files:**

- Create: `../two-tower-training/src/two_tower_training/sample_processing.py`
- Create: `../two-tower-training/tests/test_sample_processing.py`

- [ ] **Step 1: Write failing sample processing tests**

Create `../two-tower-training/tests/test_sample_processing.py`:

```python
import csv
import math
import tempfile
import unittest
from datetime import datetime, timezone
from pathlib import Path

from two_tower_training import sample_processing


class SampleProcessingTest(unittest.TestCase):
    def test_load_raw_samples_keeps_metadata(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "samples.csv"
            write_samples(
                path,
                [
                    {
                        "user_id": "7",
                        "video_id": "11",
                        "video_segment_id": "101",
                        "label": "1",
                        "weight": "3.0",
                        "source": "segment_reaction",
                        "reason": "double_like",
                        "event_time": "2026-06-23T12:00:00+08:00",
                    }
                ],
            )

            rows = sample_processing.load_raw_samples(path)

        self.assertEqual(len(rows), 1)
        self.assertEqual(rows[0].user_id, 7)
        self.assertEqual(rows[0].video_id, 11)
        self.assertEqual(rows[0].segment_id, 101)
        self.assertEqual(rows[0].label, 1.0)
        self.assertEqual(rows[0].weight, 3.0)
        self.assertEqual(rows[0].source, "segment_reaction")
        self.assertEqual(rows[0].reason, "double_like")
        self.assertEqual(rows[0].event_time.isoformat(), "2026-06-23T12:00:00+08:00")

    def test_time_decay_uses_half_life_days(self):
        reference = datetime(2026, 6, 24, 0, 0, tzinfo=timezone.utc)
        event = datetime(2026, 6, 17, 0, 0, tzinfo=timezone.utc)

        decay = sample_processing.time_decay(event, reference, half_life_days=7.0)

        self.assertTrue(math.isclose(decay, math.exp(-1.0), rel_tol=1e-9))

    def test_aggregate_samples_combines_duplicate_user_segment_rows(self):
        reference = datetime(2026, 6, 24, 0, 0, tzinfo=timezone.utc)
        rows = [
            sample_processing.RawSample(
                user_id=7,
                video_id=11,
                segment_id=101,
                label=1.0,
                weight=3.0,
                source="segment_reaction",
                reason="double_like",
                event_time=datetime(2026, 6, 23, 0, 0, tzinfo=timezone.utc),
            ),
            sample_processing.RawSample(
                user_id=7,
                video_id=11,
                segment_id=101,
                label=0.0,
                weight=1.0,
                source="exposure",
                reason="exposure_no_click",
                event_time=datetime(2026, 6, 22, 0, 0, tzinfo=timezone.utc),
            ),
        ]

        aggregated = sample_processing.aggregate_samples(rows, reference_time=reference, half_life_days=30.0)

        self.assertEqual(len(aggregated), 1)
        row = aggregated[0]
        self.assertEqual(row.user_id, 7)
        self.assertEqual(row.video_id, 11)
        self.assertEqual(row.segment_id, 101)
        self.assertEqual(row.label, 1.0)
        self.assertGreater(row.weight, 2.0)
        self.assertEqual(row.source, "aggregated")
        self.assertIn("events=2", row.reason)
        self.assertEqual(row.event_time.isoformat(), "2026-06-23T00:00:00+00:00")


def write_samples(path, rows):
    fieldnames = [
        "user_id",
        "video_id",
        "video_segment_id",
        "label",
        "weight",
        "source",
        "reason",
        "event_time",
    ]
    with path.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.DictWriter(handle, fieldnames=fieldnames)
        writer.writeheader()
        writer.writerows(rows)


if __name__ == "__main__":
    unittest.main()
```

- [ ] **Step 2: Run the tests and verify failure**

Run:

```bash
cd two-tower-training
PYTHONPATH=src python3 -m unittest tests.test_sample_processing -v
```

Expected: FAIL with `ImportError: cannot import name 'sample_processing'`.

- [ ] **Step 3: Create sample processing implementation**

Create `../two-tower-training/src/two_tower_training/sample_processing.py`:

```python
import csv
import math
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path


@dataclass(frozen=True)
class RawSample:
    user_id: int
    video_id: int
    segment_id: int
    label: float
    weight: float
    source: str
    reason: str
    event_time: datetime


def load_raw_samples(path: Path) -> list[RawSample]:
    rows: list[RawSample] = []
    with Path(path).open(newline="", encoding="utf-8") as handle:
        reader = csv.DictReader(handle)
        required = {"user_id", "video_id", "video_segment_id", "label", "weight", "source", "reason", "event_time"}
        missing = required.difference(reader.fieldnames or [])
        if missing:
            raise ValueError(f"sample csv missing columns: {', '.join(sorted(missing))}")
        for row in reader:
            sample = parse_raw_sample(row)
            if sample is not None:
                rows.append(sample)
    if not rows:
        raise ValueError("sample csv has no usable rows")
    return rows


def parse_raw_sample(row: dict[str, str]) -> RawSample | None:
    user_id = int(row["user_id"])
    video_id = int(row["video_id"])
    segment_id = int(row["video_segment_id"])
    label = float(row["label"])
    weight = float(row["weight"])
    if user_id <= 0 or video_id <= 0 or segment_id <= 0:
        return None
    if label not in (0.0, 1.0):
        return None
    if weight <= 0:
        return None
    return RawSample(
        user_id=user_id,
        video_id=video_id,
        segment_id=segment_id,
        label=label,
        weight=weight,
        source=row["source"].strip() or "unknown",
        reason=row["reason"].strip() or "unknown",
        event_time=parse_event_time(row["event_time"]),
    )


def parse_event_time(value: str) -> datetime:
    cleaned = value.strip()
    if not cleaned:
        raise ValueError("event_time is required")
    if cleaned.endswith("Z"):
        cleaned = cleaned[:-1] + "+00:00"
    parsed = datetime.fromisoformat(cleaned)
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=timezone.utc)
    return parsed


def time_decay(event_time: datetime, reference_time: datetime, half_life_days: float) -> float:
    if half_life_days <= 0:
        raise ValueError("half_life_days must be greater than 0")
    event_utc = event_time.astimezone(timezone.utc)
    reference_utc = reference_time.astimezone(timezone.utc)
    age_days = max(0.0, (reference_utc - event_utc).total_seconds() / 86400.0)
    return math.exp(-age_days / half_life_days)


def aggregate_samples(
    rows: list[RawSample],
    reference_time: datetime,
    half_life_days: float,
    max_weight: float = 5.0,
) -> list[RawSample]:
    grouped: dict[tuple[int, int], list[RawSample]] = {}
    for row in rows:
        grouped.setdefault((row.user_id, row.segment_id), []).append(row)

    aggregated: list[RawSample] = []
    for _, events in grouped.items():
        newest = max(events, key=lambda item: item.event_time)
        pos_strength = 0.0
        neg_strength = 0.0
        for event in events:
            decayed = event.weight * time_decay(event.event_time, reference_time, half_life_days)
            if event.label == 1.0:
                pos_strength += decayed
            else:
                neg_strength += decayed
        if pos_strength >= neg_strength:
            label = 1.0
            weight = pos_strength
        else:
            label = 0.0
            weight = neg_strength
        aggregated.append(
            RawSample(
                user_id=newest.user_id,
                video_id=newest.video_id,
                segment_id=newest.segment_id,
                label=label,
                weight=min(max_weight, max(0.05, weight)),
                source="aggregated",
                reason=f"pos={pos_strength:.3f};neg={neg_strength:.3f};events={len(events)}",
                event_time=newest.event_time,
            )
        )
    aggregated.sort(key=lambda item: (item.user_id, item.segment_id))
    return aggregated
```

- [ ] **Step 4: Run sample processing tests and verify pass**

Run:

```bash
cd two-tower-training
PYTHONPATH=src python3 -m unittest tests.test_sample_processing -v
```

Expected: PASS.

- [ ] **Step 5: Commit sample processing**

Run:

```bash
git add two-tower-training/src/two_tower_training/sample_processing.py two-tower-training/tests/test_sample_processing.py
git commit -m "feat: aggregate two tower training samples"
```

## Task 2: Add Temporal Train/Eval Split

**Files:**

- Create: `../two-tower-training/src/two_tower_training/splitting.py`
- Create: `../two-tower-training/tests/test_splitting.py`

- [ ] **Step 1: Write failing temporal split tests**

Create `../two-tower-training/tests/test_splitting.py`:

```python
import unittest
from datetime import datetime, timezone

from two_tower_training import sample_processing, splitting


class TemporalSplitTest(unittest.TestCase):
    def test_split_temporal_keeps_newest_rows_for_eval(self):
        rows = [
            row(1, 101, "2026-06-20T00:00:00+00:00"),
            row(1, 102, "2026-06-21T00:00:00+00:00"),
            row(2, 201, "2026-06-22T00:00:00+00:00"),
            row(2, 202, "2026-06-23T00:00:00+00:00"),
            row(3, 301, "2026-06-24T00:00:00+00:00"),
        ]

        train_rows, eval_rows = splitting.split_temporal(rows, eval_ratio=0.4)

        self.assertEqual([item.segment_id for item in train_rows], [101, 102, 201])
        self.assertEqual([item.segment_id for item in eval_rows], [202, 301])

    def test_split_temporal_requires_train_and_eval_rows(self):
        rows = [row(1, 101, "2026-06-20T00:00:00+00:00")]

        with self.assertRaisesRegex(ValueError, "at least two samples"):
            splitting.split_temporal(rows, eval_ratio=0.2)


def row(user_id, segment_id, event_time):
    return sample_processing.RawSample(
        user_id=user_id,
        video_id=segment_id * 10,
        segment_id=segment_id,
        label=1.0,
        weight=1.0,
        source="test",
        reason="test",
        event_time=datetime.fromisoformat(event_time),
    )


if __name__ == "__main__":
    unittest.main()
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
cd two-tower-training
PYTHONPATH=src python3 -m unittest tests.test_splitting -v
```

Expected: FAIL with `ImportError: cannot import name 'splitting'`.

- [ ] **Step 3: Create temporal split implementation**

Create `../two-tower-training/src/two_tower_training/splitting.py`:

```python
import math

from two_tower_training.sample_processing import RawSample


def split_temporal(rows: list[RawSample], eval_ratio: float) -> tuple[list[RawSample], list[RawSample]]:
    if len(rows) < 2:
        raise ValueError("temporal split requires at least two samples")
    if eval_ratio <= 0 or eval_ratio >= 1:
        raise ValueError("eval_ratio must be between 0 and 1")

    ordered = sorted(rows, key=lambda item: (item.event_time, item.user_id, item.segment_id))
    eval_count = max(1, math.ceil(len(ordered) * eval_ratio))
    train_count = len(ordered) - eval_count
    if train_count <= 0:
        train_count = len(ordered) - 1
        eval_count = 1
    return ordered[:train_count], ordered[train_count:]
```

- [ ] **Step 4: Run temporal split tests and verify pass**

Run:

```bash
cd two-tower-training
PYTHONPATH=src python3 -m unittest tests.test_splitting -v
```

Expected: PASS.

- [ ] **Step 5: Commit temporal split**

Run:

```bash
git add two-tower-training/src/two_tower_training/splitting.py two-tower-training/tests/test_splitting.py
git commit -m "feat: split two tower samples by event time"
```

## Task 3: Add Retrieval Metrics

**Files:**

- Create: `../two-tower-training/src/two_tower_training/metrics.py`
- Create: `../two-tower-training/tests/test_metrics.py`

- [ ] **Step 1: Write failing retrieval metric tests**

Create `../two-tower-training/tests/test_metrics.py`:

```python
import unittest

from two_tower_training import metrics, train


class RetrievalMetricsTest(unittest.TestCase):
    def test_evaluate_retrieval_reports_hits_and_coverage(self):
        dataset = train.Dataset(
            user_ids=[1, 2],
            segment_ids=[101, 102, 201],
            segment_video_ids={101: 10, 102: 10, 201: 20},
            samples=[
                train.Sample(1, 10, 101, 0, 0, 1.0, 1.0),
                train.Sample(1, 10, 102, 0, 1, 0.0, 1.0),
                train.Sample(2, 20, 201, 1, 2, 1.0, 1.0),
            ],
        )
        model = train.TwoTowerModel(
            user_embeddings=[
                [1.0, 0.0],
                [0.0, 1.0],
            ],
            item_embeddings=[
                [1.0, 0.0],
                [0.8, 0.2],
                [0.0, 1.0],
            ],
        )

        result = metrics.evaluate_retrieval(
            dataset=dataset,
            model=model,
            eval_samples=dataset.samples,
            train_samples=[],
            k=1,
        )

        self.assertEqual(result["recall_at_1"], 1.0)
        self.assertEqual(result["hit_rate_at_1"], 1.0)
        self.assertEqual(result["ndcg_at_1"], 1.0)
        self.assertEqual(result["coverage_at_1"], 2 / 3)
        self.assertEqual(result["negative_hit_rate_at_1"], 0.0)

    def test_evaluate_retrieval_excludes_seen_train_positives(self):
        dataset = train.Dataset(
            user_ids=[1],
            segment_ids=[101, 102],
            segment_video_ids={101: 10, 102: 10},
            samples=[
                train.Sample(1, 10, 101, 0, 0, 1.0, 1.0),
                train.Sample(1, 10, 102, 0, 1, 1.0, 1.0),
            ],
        )
        model = train.TwoTowerModel(
            user_embeddings=[[1.0, 0.0]],
            item_embeddings=[
                [1.0, 0.0],
                [0.9, 0.1],
            ],
        )

        result = metrics.evaluate_retrieval(
            dataset=dataset,
            model=model,
            eval_samples=[dataset.samples[1]],
            train_samples=[dataset.samples[0]],
            k=1,
        )

        self.assertEqual(result["recall_at_1"], 1.0)


if __name__ == "__main__":
    unittest.main()
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
cd two-tower-training
PYTHONPATH=src python3 -m unittest tests.test_metrics -v
```

Expected: FAIL with `ImportError: cannot import name 'metrics'`.

- [ ] **Step 3: Create retrieval metrics implementation**

Create `../two-tower-training/src/two_tower_training/metrics.py`:

```python
import math


def dot(left: list[float], right: list[float]) -> float:
    return sum(a * b for a, b in zip(left, right))


def weighted_bce(score: float, label: float, weight: float) -> float:
    if score >= 0:
        base = score - score * label + math.log1p(math.exp(-score))
    else:
        base = -score * label + math.log1p(math.exp(score))
    return weight * base


def auc(scored_labels: list[tuple[float, float]]) -> float:
    positives = [score for score, label in scored_labels if label == 1.0]
    negatives = [score for score, label in scored_labels if label == 0.0]
    if not positives or not negatives:
        return 0.0
    wins = 0.0
    total = len(positives) * len(negatives)
    for pos in positives:
        for neg in negatives:
            if pos > neg:
                wins += 1.0
            elif pos == neg:
                wins += 0.5
    return wins / total


def average(values: list[float]) -> float:
    if not values:
        return 0.0
    return sum(values) / len(values)


def evaluate_pointwise(model, samples: list, split_name: str) -> dict[str, float]:
    losses = []
    pairs = []
    positives = []
    negatives = []
    for sample in samples:
        score = dot(model.user_embeddings[sample.user_index], model.item_embeddings[sample.segment_index])
        losses.append(weighted_bce(score, sample.label, sample.weight))
        pairs.append((score, sample.label))
        if sample.label == 1.0:
            positives.append(score)
        else:
            negatives.append(score)
    return {
        f"{split_name}_loss": average(losses),
        f"{split_name}_auc": auc(pairs),
        f"{split_name}_positive_avg_score": average(positives),
        f"{split_name}_negative_avg_score": average(negatives),
    }


def evaluate_retrieval(dataset, model, eval_samples: list, train_samples: list, k: int) -> dict[str, float]:
    if k <= 0:
        raise ValueError("k must be greater than 0")
    eval_positives: dict[int, set[int]] = {}
    eval_negatives: dict[int, set[int]] = {}
    train_seen_positives: dict[int, set[int]] = {}

    for sample in eval_samples:
        if sample.label == 1.0:
            eval_positives.setdefault(sample.user_index, set()).add(sample.segment_index)
        else:
            eval_negatives.setdefault(sample.user_index, set()).add(sample.segment_index)
    for sample in train_samples:
        if sample.label == 1.0:
            train_seen_positives.setdefault(sample.user_index, set()).add(sample.segment_index)

    users = sorted(eval_positives)
    if not users:
        return {
            f"recall_at_{k}": 0.0,
            f"hit_rate_at_{k}": 0.0,
            f"ndcg_at_{k}": 0.0,
            f"coverage_at_{k}": 0.0,
            f"negative_hit_rate_at_{k}": 0.0,
        }

    recall_values = []
    hit_values = []
    ndcg_values = []
    recommended_items: set[int] = set()
    negative_hits = 0
    negative_total = sum(len(values) for values in eval_negatives.values())

    for user_index in users:
        positive_items = eval_positives[user_index]
        seen = train_seen_positives.get(user_index, set())
        ranked = rank_items_for_user(model, user_index, seen, k)
        recommended_items.update(ranked)
        hit_count = len(positive_items.intersection(ranked))
        recall_values.append(hit_count / len(positive_items))
        hit_values.append(1.0 if hit_count > 0 else 0.0)
        ndcg_values.append(ndcg_for_ranked_items(ranked, positive_items))
        for item_index in eval_negatives.get(user_index, set()):
            if item_index in ranked:
                negative_hits += 1

    return {
        f"recall_at_{k}": average(recall_values),
        f"hit_rate_at_{k}": average(hit_values),
        f"ndcg_at_{k}": average(ndcg_values),
        f"coverage_at_{k}": len(recommended_items) / max(1, len(dataset.segment_ids)),
        f"negative_hit_rate_at_{k}": negative_hits / negative_total if negative_total else 0.0,
    }


def rank_items_for_user(model, user_index: int, seen_item_indexes: set[int], k: int) -> list[int]:
    user_vec = model.user_embeddings[user_index]
    scored = []
    for item_index, item_vec in enumerate(model.item_embeddings):
        if item_index in seen_item_indexes:
            continue
        scored.append((dot(user_vec, item_vec), item_index))
    scored.sort(key=lambda item: (-item[0], item[1]))
    return [item_index for _, item_index in scored[:k]]


def ndcg_for_ranked_items(ranked_items: list[int], positive_items: set[int]) -> float:
    dcg = 0.0
    for rank, item_index in enumerate(ranked_items, start=1):
        if item_index in positive_items:
            dcg += 1.0 / math.log2(rank + 1)
    ideal_hits = min(len(positive_items), len(ranked_items))
    if ideal_hits == 0:
        return 0.0
    ideal = sum(1.0 / math.log2(rank + 1) for rank in range(1, ideal_hits + 1))
    return dcg / ideal
```

- [ ] **Step 4: Run retrieval metric tests and verify pass**

Run:

```bash
cd two-tower-training
PYTHONPATH=src python3 -m unittest tests.test_metrics -v
```

Expected: PASS.

- [ ] **Step 5: Commit retrieval metrics**

Run:

```bash
git add two-tower-training/src/two_tower_training/metrics.py two-tower-training/tests/test_metrics.py
git commit -m "feat: add two tower retrieval metrics"
```

## Task 4: Wire Processing, Splitting, And Metrics Into Training

**Files:**

- Modify: `../two-tower-training/src/two_tower_training/train.py`
- Modify: `../two-tower-training/tests/test_train.py`

- [ ] **Step 1: Add failing train tests for eval metrics**

Append these tests to `../two-tower-training/tests/test_train.py` before the `write_samples` helper:

```python
    def test_load_dataset_aggregates_duplicate_rows_and_preserves_latest_event(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "samples.csv"
            write_samples(
                path,
                [
                    {
                        "user_id": "2",
                        "video_id": "20",
                        "video_segment_id": "200",
                        "label": "1",
                        "weight": "3.0",
                        "event_time": "2026-06-21T00:00:00+00:00",
                    },
                    {
                        "user_id": "2",
                        "video_id": "20",
                        "video_segment_id": "200",
                        "label": "0",
                        "weight": "0.2",
                        "event_time": "2026-06-20T00:00:00+00:00",
                    },
                ],
            )

            dataset = train.load_dataset(path, half_life_days=30.0)

        self.assertEqual(len(dataset.samples), 1)
        self.assertEqual(dataset.samples[0].label, 1.0)
        self.assertEqual(dataset.samples[0].source, "aggregated")
        self.assertEqual(dataset.samples[0].event_time.isoformat(), "2026-06-21T00:00:00+00:00")

    def test_train_model_reports_train_eval_and_retrieval_metrics(self):
        dataset = train.Dataset(
            user_ids=[1, 2],
            segment_ids=[101, 102, 201, 202],
            segment_video_ids={101: 10, 102: 10, 201: 20, 202: 20},
            samples=[
                train.Sample(1, 10, 101, 0, 0, 1.0, 2.0),
                train.Sample(1, 10, 102, 0, 1, 0.0, 1.0),
                train.Sample(2, 20, 201, 1, 2, 1.0, 2.0),
                train.Sample(2, 20, 202, 1, 3, 0.0, 1.0),
            ],
        )

        model, metrics = train.train_model(
            dataset,
            dim=8,
            epochs=20,
            learning_rate=0.1,
            seed=7,
            l2=0.0,
            eval_ratio=0.25,
            retrieval_k=2,
        )

        self.assertEqual(len(model.user_embeddings), 2)
        self.assertIn("train_loss", metrics)
        self.assertIn("eval_loss", metrics)
        self.assertIn("recall_at_2", metrics)
        self.assertIn("coverage_at_2", metrics)
        self.assertEqual(metrics["eval_sample_count"], 1.0)
```

- [ ] **Step 2: Run train tests and verify failure**

Run:

```bash
cd two-tower-training
PYTHONPATH=src python3 -m unittest tests.test_train -v
```

Expected: FAIL because `load_dataset()` does not accept `half_life_days` and `train_model()` does not accept `eval_ratio` or `retrieval_k`.

- [ ] **Step 3: Update `Sample` and `load_dataset` in training**

Modify `../two-tower-training/src/two_tower_training/train.py`.

Add imports near the top:

```python
from datetime import timezone

from two_tower_training import metrics as training_metrics
from two_tower_training import sample_processing, splitting
```

Replace the `Sample` dataclass with:

```python
@dataclass(frozen=True)
class Sample:
    user_id: int
    video_id: int
    segment_id: int
    user_index: int
    segment_index: int
    label: float
    weight: float
    source: str = "unknown"
    reason: str = "unknown"
    event_time: object | None = None
```

Replace `load_dataset` with:

```python
def load_dataset(path: Path, half_life_days: float = 30.0) -> Dataset:
    raw_rows = sample_processing.load_raw_samples(path)
    reference_time = max(row.event_time for row in raw_rows).astimezone(timezone.utc)
    processed_rows = sample_processing.aggregate_samples(
        raw_rows,
        reference_time=reference_time,
        half_life_days=half_life_days,
    )
    return build_dataset(processed_rows)


def build_dataset(rows: list[sample_processing.RawSample]) -> Dataset:
    if not rows:
        raise ValueError("sample csv has no usable rows")
    user_ids = sorted({row.user_id for row in rows})
    segment_ids = sorted({row.segment_id for row in rows})
    user_index = {user_id: idx for idx, user_id in enumerate(user_ids)}
    segment_index = {segment_id: idx for idx, segment_id in enumerate(segment_ids)}
    segment_video_ids: dict[int, int] = {}
    samples: list[Sample] = []
    for row in rows:
        previous_video_id = segment_video_ids.get(row.segment_id)
        if previous_video_id is not None and previous_video_id != row.video_id:
            raise ValueError(f"segment {row.segment_id} maps to multiple videos")
        segment_video_ids[row.segment_id] = row.video_id
        samples.append(
            Sample(
                user_id=row.user_id,
                video_id=row.video_id,
                segment_id=row.segment_id,
                user_index=user_index[row.user_id],
                segment_index=segment_index[row.segment_id],
                label=row.label,
                weight=row.weight,
                source=row.source,
                reason=row.reason,
                event_time=row.event_time,
            )
        )
    return Dataset(
        user_ids=user_ids,
        segment_ids=segment_ids,
        segment_video_ids=segment_video_ids,
        samples=samples,
    )
```

- [ ] **Step 4: Update `train_model` to use split metrics**

In `../two-tower-training/src/two_tower_training/train.py`, replace `train_model` with:

```python
def train_model(
    dataset: Dataset,
    dim: int,
    epochs: int,
    learning_rate: float,
    seed: int,
    l2: float,
    eval_ratio: float = 0.2,
    retrieval_k: int = 20,
) -> tuple[TwoTowerModel, dict[str, float]]:
    if dim <= 0:
        raise ValueError("dim must be greater than 0")
    if epochs <= 0:
        raise ValueError("epochs must be greater than 0")
    if learning_rate <= 0:
        raise ValueError("learning-rate must be greater than 0")
    rng = random.Random(seed)
    scale = 1.0 / math.sqrt(dim)
    user_embeddings = [
        [rng.uniform(-scale, scale) for _ in range(dim)]
        for _ in dataset.user_ids
    ]
    item_embeddings = [
        [rng.uniform(-scale, scale) for _ in range(dim)]
        for _ in dataset.segment_ids
    ]

    train_samples, eval_samples = splitting.split_temporal(dataset.samples, eval_ratio=eval_ratio)
    samples = list(train_samples)
    for _ in range(epochs):
        rng.shuffle(samples)
        for sample in samples:
            user_vec = user_embeddings[sample.user_index]
            item_vec = item_embeddings[sample.segment_index]
            score = dot(user_vec, item_vec)
            prediction = sigmoid(score)
            grad = sample.weight * (prediction - sample.label)
            old_user = list(user_vec)
            for i in range(dim):
                user_grad = grad * item_vec[i] + l2 * user_vec[i]
                item_grad = grad * old_user[i] + l2 * item_vec[i]
                user_vec[i] -= learning_rate * user_grad
                item_vec[i] -= learning_rate * item_grad
            normalize_in_place(user_vec)
            normalize_in_place(item_vec)

    model = TwoTowerModel(user_embeddings=user_embeddings, item_embeddings=item_embeddings)
    metrics_payload = training_metrics.evaluate_pointwise(model, train_samples, "train")
    metrics_payload.update(training_metrics.evaluate_pointwise(model, eval_samples, "eval"))
    metrics_payload.update(
        training_metrics.evaluate_retrieval(
            dataset=dataset,
            model=model,
            eval_samples=eval_samples,
            train_samples=train_samples,
            k=retrieval_k,
        )
    )
    metrics_payload.update(
        {
            "loss": metrics_payload["eval_loss"],
            "auc": metrics_payload["eval_auc"],
            "positive_avg_score": metrics_payload["eval_positive_avg_score"],
            "negative_avg_score": metrics_payload["eval_negative_avg_score"],
            "dim": float(dim),
            "epochs": float(epochs),
            "learning_rate": float(learning_rate),
            "l2": float(l2),
            "eval_ratio": float(eval_ratio),
            "retrieval_k": float(retrieval_k),
            "sample_count": float(len(dataset.samples)),
            "train_sample_count": float(len(train_samples)),
            "eval_sample_count": float(len(eval_samples)),
            "user_count": float(len(dataset.user_ids)),
            "item_count": float(len(dataset.segment_ids)),
        }
    )
    return model, metrics_payload
```

- [ ] **Step 5: Update CLI arguments and main call**

In `../two-tower-training/src/two_tower_training/train.py`, add these arguments inside `parse_args()`:

```python
    parser.add_argument("--eval-ratio", type=float, default=0.2)
    parser.add_argument("--retrieval-k", type=int, default=20)
    parser.add_argument("--half-life-days", type=float, default=30.0)
```

In `main()`, replace dataset loading and model training with:

```python
    dataset = load_dataset(args.samples, half_life_days=args.half_life_days)
    model, metrics = train_model(
        dataset,
        dim=args.dim,
        epochs=args.epochs,
        learning_rate=args.learning_rate,
        seed=args.seed,
        l2=args.l2,
        eval_ratio=args.eval_ratio,
        retrieval_k=args.retrieval_k,
    )
```

Add these prints after the existing AUC print:

```python
    print(f"eval_auc={metrics['eval_auc']:.6f}")
    print(f"recall_at_{args.retrieval_k}={metrics[f'recall_at_{args.retrieval_k}']:.6f}")
    print(f"ndcg_at_{args.retrieval_k}={metrics[f'ndcg_at_{args.retrieval_k}']:.6f}")
    print(f"coverage_at_{args.retrieval_k}={metrics[f'coverage_at_{args.retrieval_k}']:.6f}")
```

- [ ] **Step 6: Run train tests and verify pass**

Run:

```bash
cd two-tower-training
PYTHONPATH=src python3 -m unittest tests.test_train -v
```

Expected: PASS.

- [ ] **Step 7: Run all Python tests**

Run:

```bash
cd two-tower-training
PYTHONPATH=src python3 -m unittest discover -s tests
```

Expected: PASS.

- [ ] **Step 8: Commit training integration**

Run:

```bash
git add two-tower-training/src/two_tower_training/train.py two-tower-training/tests/test_train.py
git commit -m "feat: evaluate two tower training on recent holdout samples"
```

## Task 5: Add Publish Gate

**Files:**

- Create: `../two-tower-training/src/two_tower_training/publish_gate.py`
- Create: `../two-tower-training/tests/test_publish_gate.py`

- [ ] **Step 1: Write failing publish gate tests**

Create `../two-tower-training/tests/test_publish_gate.py`:

```python
import json
import tempfile
import unittest
from pathlib import Path

from two_tower_training import publish_gate


class PublishGateTest(unittest.TestCase):
    def test_gate_passes_when_metrics_meet_thresholds(self):
        result = publish_gate.evaluate_gate(
            metrics={
                "eval_auc": 0.72,
                "recall_at_20": 0.31,
                "coverage_at_20": 0.22,
                "negative_hit_rate_at_20": 0.03,
                "eval_sample_count": 100.0,
            },
            thresholds=publish_gate.Thresholds(
                min_eval_auc=0.70,
                min_recall_at_20=0.30,
                min_coverage_at_20=0.20,
                max_negative_hit_rate_at_20=0.05,
                min_eval_sample_count=50,
            ),
        )

        self.assertTrue(result.passed)
        self.assertEqual(result.failures, [])

    def test_gate_fails_with_actionable_messages(self):
        result = publish_gate.evaluate_gate(
            metrics={
                "eval_auc": 0.60,
                "recall_at_20": 0.10,
                "coverage_at_20": 0.05,
                "negative_hit_rate_at_20": 0.20,
                "eval_sample_count": 3.0,
            },
            thresholds=publish_gate.Thresholds(
                min_eval_auc=0.70,
                min_recall_at_20=0.30,
                min_coverage_at_20=0.20,
                max_negative_hit_rate_at_20=0.05,
                min_eval_sample_count=50,
            ),
        )

        self.assertFalse(result.passed)
        self.assertIn("eval_auc 0.600000 < 0.700000", result.failures)
        self.assertIn("recall_at_20 0.100000 < 0.300000", result.failures)
        self.assertIn("coverage_at_20 0.050000 < 0.200000", result.failures)
        self.assertIn("negative_hit_rate_at_20 0.200000 > 0.050000", result.failures)
        self.assertIn("eval_sample_count 3 < 50", result.failures)

    def test_cli_returns_nonzero_when_gate_fails(self):
        with tempfile.TemporaryDirectory() as tmp:
            metrics_path = Path(tmp) / "metrics.json"
            metrics_path.write_text(json.dumps({"eval_auc": 0.1, "recall_at_20": 0.1, "coverage_at_20": 0.1, "negative_hit_rate_at_20": 0.1, "eval_sample_count": 10}), encoding="utf-8")

            code = publish_gate.main([
                "--metrics",
                str(metrics_path),
                "--min-eval-auc",
                "0.7",
                "--min-recall-at-20",
                "0.3",
                "--min-coverage-at-20",
                "0.2",
                "--max-negative-hit-rate-at-20",
                "0.05",
                "--min-eval-sample-count",
                "50",
            ])

        self.assertEqual(code, 1)


if __name__ == "__main__":
    unittest.main()
```

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
cd two-tower-training
PYTHONPATH=src python3 -m unittest tests.test_publish_gate -v
```

Expected: FAIL with `ImportError: cannot import name 'publish_gate'`.

- [ ] **Step 3: Create publish gate implementation**

Create `../two-tower-training/src/two_tower_training/publish_gate.py`:

```python
import argparse
import json
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class Thresholds:
    min_eval_auc: float
    min_recall_at_20: float
    min_coverage_at_20: float
    max_negative_hit_rate_at_20: float
    min_eval_sample_count: int


@dataclass(frozen=True)
class GateResult:
    passed: bool
    failures: list[str]


def evaluate_gate(metrics: dict, thresholds: Thresholds) -> GateResult:
    failures: list[str] = []
    eval_auc = float(metrics.get("eval_auc", 0.0))
    recall = float(metrics.get("recall_at_20", 0.0))
    coverage = float(metrics.get("coverage_at_20", 0.0))
    negative_hit_rate = float(metrics.get("negative_hit_rate_at_20", 1.0))
    eval_samples = int(float(metrics.get("eval_sample_count", 0.0)))

    if eval_auc < thresholds.min_eval_auc:
        failures.append(f"eval_auc {eval_auc:.6f} < {thresholds.min_eval_auc:.6f}")
    if recall < thresholds.min_recall_at_20:
        failures.append(f"recall_at_20 {recall:.6f} < {thresholds.min_recall_at_20:.6f}")
    if coverage < thresholds.min_coverage_at_20:
        failures.append(f"coverage_at_20 {coverage:.6f} < {thresholds.min_coverage_at_20:.6f}")
    if negative_hit_rate > thresholds.max_negative_hit_rate_at_20:
        failures.append(f"negative_hit_rate_at_20 {negative_hit_rate:.6f} > {thresholds.max_negative_hit_rate_at_20:.6f}")
    if eval_samples < thresholds.min_eval_sample_count:
        failures.append(f"eval_sample_count {eval_samples} < {thresholds.min_eval_sample_count}")

    return GateResult(passed=len(failures) == 0, failures=failures)


def load_metrics(path: Path) -> dict:
    return json.loads(Path(path).read_text(encoding="utf-8"))


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Check whether a two tower artifact can be published.")
    parser.add_argument("--metrics", type=Path, required=True)
    parser.add_argument("--min-eval-auc", type=float, default=0.55)
    parser.add_argument("--min-recall-at-20", type=float, default=0.05)
    parser.add_argument("--min-coverage-at-20", type=float, default=0.02)
    parser.add_argument("--max-negative-hit-rate-at-20", type=float, default=0.50)
    parser.add_argument("--min-eval-sample-count", type=int, default=20)
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    thresholds = Thresholds(
        min_eval_auc=args.min_eval_auc,
        min_recall_at_20=args.min_recall_at_20,
        min_coverage_at_20=args.min_coverage_at_20,
        max_negative_hit_rate_at_20=args.max_negative_hit_rate_at_20,
        min_eval_sample_count=args.min_eval_sample_count,
    )
    result = evaluate_gate(load_metrics(args.metrics), thresholds)
    if result.passed:
        print("publish_gate=pass")
        return 0
    print("publish_gate=fail")
    for failure in result.failures:
        print(f"failure={failure}")
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
```

- [ ] **Step 4: Run publish gate tests and verify pass**

Run:

```bash
cd two-tower-training
PYTHONPATH=src python3 -m unittest tests.test_publish_gate -v
```

Expected: PASS.

- [ ] **Step 5: Commit publish gate**

Run:

```bash
git add two-tower-training/src/two_tower_training/publish_gate.py two-tower-training/tests/test_publish_gate.py
git commit -m "feat: gate two tower model publication"
```

## Task 6: Wire Publish Gate Into Pipeline

**Files:**

- Modify: `../two-tower-training/scripts/run_two_tower_pipeline.sh`
- Modify: `../two-tower-training/tests/test_pipeline_cleanup.py`

- [ ] **Step 1: Update pipeline cleanup test to expect gate execution**

In `../two-tower-training/tests/test_pipeline_cleanup.py`, replace the fake `python3` script body with:

```python
            fake_python.write_text(
                """#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == *"two_tower_training.publish_gate"* ]]; then
  echo "publish_gate=pass"
  exit 0
fi
out=""
model_version="test_model"
while [[ $# -gt 0 ]]; do
  if [[ "$1" == "--output" ]]; then
    out="$2"
    shift 2
    continue
  fi
  if [[ "$1" == "--model-version" ]]; then
    model_version="$2"
    shift 2
    continue
  fi
  shift
done
mkdir -p "$out"
printf '{"model_version":"%s","eval_auc":0.8,"recall_at_20":0.4,"coverage_at_20":0.3,"negative_hit_rate_at_20":0.02,"eval_sample_count":100}\\n' "$model_version" > "$out/metrics.json"
printf 'video_segment_id,video_id,embedding,model_version\\n' > "$out/item_embeddings.csv"
printf 'user_id,embedding,model_version\\n' > "$out/user_embeddings.csv"
""",
                encoding="utf-8",
            )
```

- [ ] **Step 2: Run pipeline test and verify it still passes before script change**

Run:

```bash
cd two-tower-training
PYTHONPATH=src python3 -m unittest tests.test_pipeline_cleanup -v
```

Expected: PASS. This confirms the fake Python supports both training and gate calls before the real script invokes the gate.

- [ ] **Step 3: Add gate environment variables to pipeline script**

In `../two-tower-training/scripts/run_two_tower_pipeline.sh`, add these environment defaults after `SEED="${SEED:-42}"`:

```bash
PUBLISH_GATE_ENABLED="${PUBLISH_GATE_ENABLED:-true}"
MIN_EVAL_AUC="${MIN_EVAL_AUC:-0.55}"
MIN_RECALL_AT_20="${MIN_RECALL_AT_20:-0.05}"
MIN_COVERAGE_AT_20="${MIN_COVERAGE_AT_20:-0.02}"
MAX_NEGATIVE_HIT_RATE_AT_20="${MAX_NEGATIVE_HIT_RATE_AT_20:-0.50}"
MIN_EVAL_SAMPLE_COUNT="${MIN_EVAL_SAMPLE_COUNT:-20}"
```

- [ ] **Step 4: Pass new train parameters through pipeline**

In `../two-tower-training/scripts/run_two_tower_pipeline.sh`, add these defaults after the publish gate defaults:

```bash
EVAL_RATIO="${EVAL_RATIO:-0.2}"
RETRIEVAL_K="${RETRIEVAL_K:-20}"
HALF_LIFE_DAYS="${HALF_LIFE_DAYS:-30}"
```

Then add these arguments to the `python3 -m two_tower_training.train` invocation:

```bash
    --eval-ratio "${EVAL_RATIO}" \
    --retrieval-k "${RETRIEVAL_K}" \
    --half-life-days "${HALF_LIFE_DAYS}"
```

- [ ] **Step 5: Insert gate before Go import publish**

In `../two-tower-training/scripts/run_two_tower_pipeline.sh`, insert this block after the training subshell and before the Go import subshell:

```bash
if [[ "${PUBLISH_GATE_ENABLED}" == "true" ]]; then
  (
    cd "${TRAIN_DIR}"
    PYTHONPATH=src python3 -m two_tower_training.publish_gate \
      --metrics "${ARTIFACT_DIR}/metrics.json" \
      --min-eval-auc "${MIN_EVAL_AUC}" \
      --min-recall-at-20 "${MIN_RECALL_AT_20}" \
      --min-coverage-at-20 "${MIN_COVERAGE_AT_20}" \
      --max-negative-hit-rate-at-20 "${MAX_NEGATIVE_HIT_RATE_AT_20}" \
      --min-eval-sample-count "${MIN_EVAL_SAMPLE_COUNT}"
  )
else
  echo "publish_gate=disabled"
fi
```

- [ ] **Step 6: Run pipeline cleanup test**

Run:

```bash
cd two-tower-training
PYTHONPATH=src python3 -m unittest tests.test_pipeline_cleanup -v
```

Expected: PASS.

- [ ] **Step 7: Run all Python tests**

Run:

```bash
cd two-tower-training
PYTHONPATH=src python3 -m unittest discover -s tests
```

Expected: PASS.

- [ ] **Step 8: Commit pipeline gate integration**

Run:

```bash
git add two-tower-training/scripts/run_two_tower_pipeline.sh two-tower-training/tests/test_pipeline_cleanup.py
git commit -m "feat: run publish gate before two tower import"
```

## Task 7: Update Training Documentation

**Files:**

- Modify: `../two-tower-training/README.md`
- Modify: `../two-tower-training/ALGORITHM_HANDOFF.md`

- [ ] **Step 1: Update README training section**

In `../two-tower-training/README.md`, replace the "当前版本" section with:

```markdown
## 当前版本

当前训练版本仍保持线上协议不变：

```text
user_id -> user embedding
video_segment_id -> item embedding
score = dot(user_embedding, item_embedding)
loss = weighted BCEWithLogitsLoss(label, score, weight)
```

训练前会先处理样本：

1. 保留 `source`、`reason`、`event_time`。
2. 对同一 `user_id + video_segment_id` 的多条行为做聚合。
3. 按 `event_time` 对样本权重做时间衰减。
4. 按时间切分训练集和评估集。

训练后会输出训练集、评估集和召回指标。线上发布前会执行 publish gate，指标不达标时不会导入并发布 active model version。
```

- [ ] **Step 2: Update README command examples**

In `../two-tower-training/README.md`, update the training command example to:

```bash
cd two-tower-training
PYTHONPATH=src python3 -m two_tower_training.train \
  --samples data/two_tower_samples.csv \
  --output artifacts/two_tower_v1 \
  --model-version two_tower_v1 \
  --dim 64 \
  --epochs 50 \
  --learning-rate 0.05 \
  --eval-ratio 0.2 \
  --retrieval-k 20 \
  --half-life-days 30 \
  --seed 20260623
```

Add this paragraph after the command:

```markdown
`metrics.json` now includes `train_loss`, `eval_loss`, `eval_auc`, `recall_at_20`, `hit_rate_at_20`, `ndcg_at_20`, `coverage_at_20`, `negative_hit_rate_at_20`, `train_sample_count`, and `eval_sample_count`.
```

- [ ] **Step 3: Document gate environment variables**

In `../two-tower-training/README.md`, add this block after the pipeline environment variable example:

```markdown
Publish gate variables:

```bash
PUBLISH_GATE_ENABLED=true
MIN_EVAL_AUC=0.55
MIN_RECALL_AT_20=0.05
MIN_COVERAGE_AT_20=0.02
MAX_NEGATIVE_HIT_RATE_AT_20=0.50
MIN_EVAL_SAMPLE_COUNT=20
EVAL_RATIO=0.2
RETRIEVAL_K=20
HALF_LIFE_DAYS=30
```

When the gate fails, the pipeline exits before `go run ./tools/import_two_tower_embeddings --publish`. The artifact remains under `two-tower-training/artifacts/${MODEL_VERSION}` for inspection.
```

- [ ] **Step 4: Update algorithm handoff metric requirements**

In `../two-tower-training/ALGORITHM_HANDOFF.md`, replace the `metrics.json` example with:

```json
{
  "model_version": "two_tower_20260623_003000",
  "dim": 64,
  "epochs": 50,
  "sample_count": 10000,
  "train_sample_count": 8000,
  "eval_sample_count": 2000,
  "user_count": 1000,
  "item_count": 5000,
  "train_loss": 0.41,
  "eval_loss": 0.48,
  "eval_auc": 0.71,
  "recall_at_20": 0.18,
  "hit_rate_at_20": 0.24,
  "ndcg_at_20": 0.16,
  "coverage_at_20": 0.31,
  "negative_hit_rate_at_20": 0.04
}
```

Add this release rule after the example:

```markdown
Release rule:

1. Training can generate artifacts even when metrics are weak.
2. Publishing active `model_version` requires the publish gate to pass.
3. If the gate fails, keep the artifact for review and leave the previous active model in place.
4. Raising or lowering gate thresholds must be done through pipeline environment variables and recorded in the release note.
```

- [ ] **Step 5: Run docs grep for old metric wording**

Run:

```bash
cd two-tower-training
rg -n "当前第一波训练结果|AUC 只是基础指标|publish gate|recall_at_20" README.md ALGORITHM_HANDOFF.md
```

Expected: output includes the new `publish gate` and `recall_at_20` text. If `当前第一波训练结果` remains, rewrite that section so it states the old numbers only proved the minimal historical chain.

- [ ] **Step 6: Commit docs**

Run:

```bash
git add two-tower-training/README.md two-tower-training/ALGORITHM_HANDOFF.md
git commit -m "docs: document two tower training quality gates"
```

## Task 8: Final Verification

**Files:**

- Verify: `../two-tower-training/src/two_tower_training/*.py`
- Verify: `../two-tower-training/tests/*.py`
- Verify: `../two-tower-training/scripts/run_two_tower_pipeline.sh`
- Verify: `../two-tower-training/README.md`
- Verify: `../two-tower-training/ALGORITHM_HANDOFF.md`

- [ ] **Step 1: Run all Python tests**

Run:

```bash
cd two-tower-training
PYTHONPATH=src python3 -m unittest discover -s tests
```

Expected: PASS.

- [ ] **Step 2: Run a small local training command**

Run:

```bash
cd two-tower-training
PYTHONPATH=src python3 -m two_tower_training.train \
  --samples data/two_tower_samples.csv \
  --output artifacts/two_tower_plan_check \
  --model-version two_tower_plan_check \
  --dim 64 \
  --epochs 2 \
  --learning-rate 0.05 \
  --eval-ratio 0.2 \
  --retrieval-k 20 \
  --half-life-days 30 \
  --seed 20260624
```

Expected: command exits 0 and prints `eval_auc=`, `recall_at_20=`, `ndcg_at_20=`, and `coverage_at_20=`.

- [ ] **Step 3: Run publish gate against the small artifact**

Run:

```bash
cd two-tower-training
PYTHONPATH=src python3 -m two_tower_training.publish_gate \
  --metrics artifacts/two_tower_plan_check/metrics.json \
  --min-eval-auc 0.0 \
  --min-recall-at-20 0.0 \
  --min-coverage-at-20 0.0 \
  --max-negative-hit-rate-at-20 1.0 \
  --min-eval-sample-count 1
```

Expected: command exits 0 and prints `publish_gate=pass`.

- [ ] **Step 4: Dry-run Go importer against the small artifact**

Run:

```bash
cd video-service
go run ./tools/import_two_tower_embeddings \
  --config configs/video.yml \
  --artifact-dir ../two-tower-training/artifacts/two_tower_plan_check \
  --dim 64 \
  --dry-run
```

Expected: command exits 0 and prints `dry_run=true`.

- [ ] **Step 5: Run focused Go importer tests**

Run:

```bash
cd video-service
go test ./tools/import_two_tower_embeddings
```

Expected: PASS.

- [ ] **Step 6: Remove local verification artifact if it is untracked**

Run:

```bash
rm -rf two-tower-training/artifacts/two_tower_plan_check
```

Expected: `git status --short` does not show `two-tower-training/artifacts/two_tower_plan_check`.

- [ ] **Step 7: Review git diff**

Run:

```bash
git status --short
git diff --stat
```

Expected: changed files are limited to the Python training modules, tests, pipeline script, and training docs listed in this plan.

- [ ] **Step 8: Final commit**

Run:

```bash
git add two-tower-training/src/two_tower_training two-tower-training/tests two-tower-training/scripts/run_two_tower_pipeline.sh two-tower-training/README.md two-tower-training/ALGORITHM_HANDOFF.md
git commit -m "feat: optimize two tower training quality loop"
```

## Rollback Plan

If the pipeline gate blocks all models in production, set:

```bash
PUBLISH_GATE_ENABLED=false
```

This restores the previous train-and-publish behavior without reverting code. Use this only to recover a blocked release. The next follow-up should lower thresholds deliberately or fix the metric regression.

If training metrics are unstable because the sample count is too low, set:

```bash
MIN_EVAL_SAMPLE_COUNT=1
MIN_EVAL_AUC=0
MIN_RECALL_AT_20=0
MIN_COVERAGE_AT_20=0
MAX_NEGATIVE_HIT_RATE_AT_20=1
```

This still exercises the gate code path while allowing very small local datasets to publish in a non-production environment.

## Self-Review Notes

Spec coverage:

1. Data cleanup and time decay are covered by Task 1.
2. Time-based holdout evaluation is covered by Task 2 and Task 4.
3. Retrieval metrics are covered by Task 3 and Task 4.
4. Publication safety is covered by Task 5 and Task 6.
5. Documentation and handoff are covered by Task 7.
6. End-to-end verification is covered by Task 8.

Type consistency:

1. `RawSample` is defined in Task 1 and consumed by Tasks 2 and 4.
2. `Sample` keeps the original first eight positional fields, so existing tests that instantiate `train.Sample(...)` continue to work.
3. Metric names use `recall_at_20`, `hit_rate_at_20`, `ndcg_at_20`, `coverage_at_20`, and `negative_hit_rate_at_20` consistently across training, gate, pipeline, and docs.
