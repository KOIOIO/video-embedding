import csv
import json
import tempfile
import unittest
from datetime import datetime
from pathlib import Path
from unittest import mock

from two_tower_training import train


class TwoTowerTrainingTest(unittest.TestCase):
    def test_load_samples_builds_stable_id_maps(self):
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
                        "weight": "2.0",
                    },
                    {
                        "user_id": "1",
                        "video_id": "10",
                        "video_segment_id": "100",
                        "label": "0",
                        "weight": "0.2",
                    },
                    {
                        "user_id": "2",
                        "video_id": "10",
                        "video_segment_id": "100",
                        "label": "1",
                        "weight": "1.0",
                    },
                ],
            )

            dataset = train.load_dataset(path)

        self.assertEqual(dataset.user_ids, [1, 2])
        self.assertEqual(dataset.segment_ids, [100, 200])
        self.assertEqual(dataset.segment_video_ids, {100: 10, 200: 20})
        self.assertEqual(len(dataset.samples), 3)
        self.assertEqual(dataset.samples[0].user_index, 1)
        self.assertEqual(dataset.samples[0].segment_index, 1)

    def test_load_dataset_includes_catalog_items_without_behavior(self):
        with tempfile.TemporaryDirectory() as tmp:
            sample_path = Path(tmp) / "samples.csv"
            item_path = Path(tmp) / "items.csv"
            write_samples(
                sample_path,
                [
                    {
                        "user_id": "1",
                        "video_id": "10",
                        "video_segment_id": "100",
                        "label": "1",
                        "weight": "2.0",
                    }
                ],
            )
            write_items(
                item_path,
                [
                    {
                        "video_segment_id": "100",
                        "video_id": "10",
                        "segment_duration": "30",
                        "video_duration": "300",
                        "like_count": "2",
                        "double_like_count": "1",
                        "dislike_count": "0",
                        "content_summary": "函数单调性",
                        "knowledge_tags": "函数|导数",
                        "video_title": "高一数学",
                    },
                    {
                        "video_segment_id": "200",
                        "video_id": "20",
                        "segment_duration": "45",
                        "video_duration": "360",
                        "like_count": "0",
                        "double_like_count": "0",
                        "dislike_count": "0",
                        "content_summary": "三角函数",
                        "knowledge_tags": "三角函数",
                        "video_title": "高一数学",
                    },
                ],
            )

            dataset = train.load_dataset(sample_path, item_catalog=item_path)

        self.assertEqual(dataset.segment_ids, [100, 200])
        self.assertEqual(dataset.segment_video_ids, {100: 10, 200: 20})
        self.assertEqual(dataset.item_features[200].content_summary, "三角函数")
        self.assertEqual(len(dataset.samples), 1)

    def test_load_dataset_includes_user_features(self):
        with tempfile.TemporaryDirectory() as tmp:
            sample_path = Path(tmp) / "samples.csv"
            user_path = Path(tmp) / "users.csv"
            write_samples(
                sample_path,
                [
                    {
                        "user_id": "7",
                        "video_id": "10",
                        "video_segment_id": "100",
                        "label": "1",
                        "weight": "2.0",
                    }
                ],
            )
            write_user_features(
                user_path,
                [
                    {
                        "user_id": "7",
                        "grade_id": "3",
                        "class_id": "8",
                        "user_type": "1",
                        "mastery_avg": "0.72",
                        "mastery_min": "0.31",
                        "weak_knowledge_count": "2",
                        "strong_knowledge_count": "4",
                        "knowledge_correct_count": "10",
                        "knowledge_incorrect_count": "5",
                        "answer_count": "9",
                        "answer_correct_count": "7",
                        "answer_incorrect_count": "2",
                        "avg_score_rate": "0.8",
                        "avg_cost_seconds": "36",
                        "question_feedback_count": "2",
                        "generated_feedback_count": "3",
                        "generated_correct_count": "2",
                        "generated_avg_score_rate": "0.75",
                        "question_search_count": "4",
                        "recent_knowledge_point_ids": "101|102",
                        "recent_subjects": "math",
                        "question_search_knowledge_text": "函数 单调性",
                        "generated_feedback_knowledge_text": "导数",
                    }
                ],
            )

            dataset = train.load_dataset(sample_path, user_features=user_path)

        self.assertIn(7, dataset.user_features)
        self.assertEqual(dataset.user_features[7].grade_id, 3)
        self.assertEqual(dataset.user_features[7].recent_subjects, "math")

    def test_train_model_ranks_positive_pair_above_negative_pair(self):
        dataset = train.Dataset(
            user_ids=[1],
            segment_ids=[101, 102],
            segment_video_ids={101: 11, 102: 11},
            samples=[
                train.Sample(user_id=1, video_id=11, segment_id=101, user_index=0, segment_index=0, label=1.0, weight=3.0),
                train.Sample(user_id=1, video_id=11, segment_id=102, user_index=0, segment_index=1, label=0.0, weight=3.0),
                train.Sample(user_id=1, video_id=11, segment_id=101, user_index=0, segment_index=0, label=1.0, weight=3.0),
            ],
        )

        model, metrics = train.train_model(
            dataset,
            dim=8,
            epochs=80,
            learning_rate=0.2,
            seed=7,
            l2=0.0,
            eval_ratio=0.2,
        )

        positive_score = train.dot(model.user_embeddings[0], model.item_embeddings[0])
        negative_score = train.dot(model.user_embeddings[0], model.item_embeddings[1])
        self.assertGreater(positive_score, negative_score)
        self.assertGreaterEqual(metrics["train_auc"], 0.99)

    def test_write_artifacts_outputs_embeddings_and_metrics(self):
        dataset = train.Dataset(
            user_ids=[7],
            segment_ids=[701],
            segment_video_ids={701: 70},
            samples=[train.Sample(user_id=7, video_id=70, segment_id=701, user_index=0, segment_index=0, label=1.0, weight=1.0)],
        )
        model = train.TwoTowerModel(
            user_embeddings=[[0.3, 0.4]],
            item_embeddings=[[0.5, 0.6]],
        )

        with tempfile.TemporaryDirectory() as tmp:
            out = Path(tmp) / "two_tower_v1"
            train.write_artifacts(out, dataset, model, {"loss": 0.12, "auc": 1.0}, model_version="two_tower_v1")

            metrics = json.loads((out / "metrics.json").read_text(encoding="utf-8"))
            with (out / "item_embeddings.csv").open(encoding="utf-8") as handle:
                item_rows = list(csv.DictReader(handle))
            with (out / "user_embeddings.csv").open(encoding="utf-8") as handle:
                user_rows = list(csv.DictReader(handle))

        self.assertEqual(metrics["model_version"], "two_tower_v1")
        self.assertEqual(item_rows[0]["video_segment_id"], "701")
        self.assertEqual(item_rows[0]["video_id"], "70")
        self.assertEqual(item_rows[0]["model_version"], "two_tower_v1")
        self.assertEqual(item_rows[0]["embedding"], "[0.500000,0.600000]")
        self.assertEqual(user_rows[0]["user_id"], "7")
        self.assertEqual(user_rows[0]["embedding"], "[0.300000,0.400000]")

    def test_write_artifacts_outputs_catalog_item_embeddings(self):
        dataset = train.Dataset(
            user_ids=[7],
            segment_ids=[701, 702],
            segment_video_ids={701: 70, 702: 70},
            samples=[train.Sample(user_id=7, video_id=70, segment_id=701, user_index=0, segment_index=0, label=1.0, weight=1.0)],
        )
        model = train.TwoTowerModel(
            user_embeddings=[[0.3, 0.4]],
            item_embeddings=[[0.5, 0.6], [0.7, 0.8]],
        )

        with tempfile.TemporaryDirectory() as tmp:
            out = Path(tmp) / "two_tower_v1"
            train.write_artifacts(out, dataset, model, {"loss": 0.12, "auc": 1.0}, model_version="two_tower_v1")
            with (out / "item_embeddings.csv").open(encoding="utf-8") as handle:
                item_rows = list(csv.DictReader(handle))

        self.assertEqual([row["video_segment_id"] for row in item_rows], ["701", "702"])

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
            retrieval_ks=[1, 2],
        )

        self.assertEqual(len(model.user_embeddings), 2)
        self.assertIn("train_loss", metrics)
        self.assertIn("eval_loss", metrics)
        self.assertIn("recall_at_1", metrics)
        self.assertIn("recall_at_2", metrics)
        self.assertIn("coverage_at_2", metrics)
        self.assertIn("dislike_hit_rate_at_2", metrics)
        self.assertEqual(metrics["eval_sample_count"], 1.0)

    def test_train_backend_trains_only_on_temporal_train_samples(self):
        dataset = train.Dataset(
            user_ids=[1],
            segment_ids=[101, 102, 103, 104],
            segment_video_ids={101: 10, 102: 10, 103: 10, 104: 10},
            samples=[
                train.Sample(1, 10, 101, 0, 0, 1.0, 1.0, event_time=parse_time("2026-06-20T00:00:00+00:00")),
                train.Sample(1, 10, 102, 0, 1, 0.0, 1.0, event_time=parse_time("2026-06-21T00:00:00+00:00")),
                train.Sample(1, 10, 103, 0, 2, 1.0, 1.0, event_time=parse_time("2026-06-22T00:00:00+00:00")),
                train.Sample(1, 10, 104, 0, 3, 0.0, 1.0, event_time=parse_time("2026-06-23T00:00:00+00:00")),
            ],
        )
        captured_sample_count = None

        def fake_train_torch_model(dataset, **_kwargs):
            nonlocal captured_sample_count
            captured_sample_count = len(dataset.samples)
            return train.TwoTowerModel(user_embeddings=[[1.0, 0.0]], item_embeddings=[[1.0, 0.0]] * 4), {
                "backend": "torch",
                "score_scale": 8.0,
            }

        with mock.patch("two_tower_training.torch_model.train_torch_model", side_effect=fake_train_torch_model):
            _model, metrics = train.train_backend(
                dataset,
                backend="torch",
                dim=2,
                epochs=1,
                learning_rate=0.01,
                seed=7,
                l2=0.0,
                eval_ratio=0.5,
                retrieval_ks=[1],
                batch_size=2,
                random_negatives=0,
                hard_negatives=0,
            )

        self.assertEqual(captured_sample_count, 2)
        self.assertEqual(metrics["score_scale"], 8.0)
        self.assertEqual(metrics["eval_positive_avg_score"], 8.0)
        self.assertEqual(metrics["train_sample_count"], 2.0)
        self.assertEqual(metrics["eval_sample_count"], 2.0)


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
        for row in rows:
            merged = {
                "source": "test",
                "reason": "test",
                "event_time": "2026-06-23T00:00:00+08:00",
            }
            merged.update(row)
            writer.writerow(merged)


def write_items(path, rows):
    fieldnames = [
        "video_segment_id",
        "video_id",
        "segment_duration",
        "video_duration",
        "like_count",
        "double_like_count",
        "dislike_count",
        "content_summary",
        "knowledge_tags",
        "video_title",
    ]
    with path.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.DictWriter(handle, fieldnames=fieldnames)
        writer.writeheader()
        writer.writerows(rows)


def write_user_features(path, rows):
    fieldnames = [
        "user_id",
        "grade_id",
        "class_id",
        "user_type",
        "mastery_avg",
        "mastery_min",
        "weak_knowledge_count",
        "strong_knowledge_count",
        "knowledge_correct_count",
        "knowledge_incorrect_count",
        "answer_count",
        "answer_correct_count",
        "answer_incorrect_count",
        "avg_score_rate",
        "avg_cost_seconds",
        "question_feedback_count",
        "generated_feedback_count",
        "generated_correct_count",
        "generated_avg_score_rate",
        "question_search_count",
        "recent_knowledge_point_ids",
        "recent_subjects",
        "question_search_knowledge_text",
        "generated_feedback_knowledge_text",
    ]
    with path.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.DictWriter(handle, fieldnames=fieldnames)
        writer.writeheader()
        writer.writerows(rows)


def parse_time(value):
    return datetime.fromisoformat(value)


if __name__ == "__main__":
    unittest.main()
