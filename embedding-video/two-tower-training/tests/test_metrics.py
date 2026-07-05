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

    def test_evaluate_retrieval_reports_dislike_hits(self):
        dataset = train.Dataset(
            user_ids=[1],
            segment_ids=[101, 102],
            segment_video_ids={101: 10, 102: 10},
            samples=[
                train.Sample(1, 10, 101, 0, 0, 1.0, 1.0),
                train.Sample(1, 10, 102, 0, 1, 0.0, 1.0, reason="dislike"),
            ],
        )
        model = train.TwoTowerModel(
            user_embeddings=[[1.0, 0.0]],
            item_embeddings=[
                [0.7, 0.3],
                [1.0, 0.0],
            ],
        )

        result = metrics.evaluate_retrieval(dataset, model, eval_samples=dataset.samples, train_samples=[], k=1)

        self.assertEqual(result["negative_hit_rate_at_1"], 1.0)
        self.assertEqual(result["dislike_hit_rate_at_1"], 1.0)

    def test_evaluate_retrieval_reports_excess_negative_hits_over_candidate_share(self):
        dataset = train.Dataset(
            user_ids=[1],
            segment_ids=[101, 102, 103],
            segment_video_ids={101: 10, 102: 10, 103: 10},
            samples=[
                train.Sample(1, 10, 101, 0, 0, 1.0, 1.0),
                train.Sample(1, 10, 102, 0, 1, 0.0, 1.0, reason="dislike"),
                train.Sample(1, 10, 103, 0, 2, 1.0, 1.0),
            ],
        )
        model = train.TwoTowerModel(
            user_embeddings=[[1.0, 0.0]],
            item_embeddings=[
                [1.0, 0.0],
                [0.9, 0.1],
                [0.8, 0.2],
            ],
        )

        result = metrics.evaluate_retrieval(
            dataset=dataset,
            model=model,
            eval_samples=[dataset.samples[1], dataset.samples[2]],
            train_samples=[dataset.samples[0]],
            k=2,
        )

        self.assertEqual(result["negative_hit_rate_at_2"], 1.0)
        self.assertEqual(result["expected_negative_hit_rate_at_2"], 1.0)
        self.assertEqual(result["excess_negative_hit_rate_at_2"], 0.0)

    def test_evaluate_retrieval_many_reports_each_k(self):
        dataset = train.Dataset(
            user_ids=[1],
            segment_ids=[101, 102],
            segment_video_ids={101: 10, 102: 10},
            samples=[
                train.Sample(1, 10, 101, 0, 0, 1.0, 1.0),
                train.Sample(1, 10, 102, 0, 1, 0.0, 1.0),
            ],
        )
        model = train.TwoTowerModel(
            user_embeddings=[[1.0, 0.0]],
            item_embeddings=[
                [1.0, 0.0],
                [0.0, 1.0],
            ],
        )

        result = metrics.evaluate_retrieval_many(dataset, model, eval_samples=dataset.samples, train_samples=[], ks=[1, 2])

        self.assertIn("recall_at_1", result)
        self.assertIn("recall_at_2", result)
        self.assertIn("coverage_at_2", result)

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

    def test_evaluate_pointwise_applies_score_scale_to_loss_and_scores(self):
        samples = [
            train.Sample(1, 10, 101, 0, 0, 1.0, 1.0),
            train.Sample(1, 10, 102, 0, 1, 0.0, 1.0),
        ]
        model = train.TwoTowerModel(
            user_embeddings=[[1.0, 0.0]],
            item_embeddings=[
                [0.5, 0.0],
                [0.25, 0.0],
            ],
        )

        unscaled = metrics.evaluate_pointwise(model, samples, "eval")
        scaled = metrics.evaluate_pointwise(model, samples, "eval", score_scale=8.0)

        self.assertEqual(scaled["eval_positive_avg_score"], 4.0)
        self.assertEqual(scaled["eval_negative_avg_score"], 2.0)
        self.assertNotEqual(scaled["eval_loss"], unscaled["eval_loss"])


if __name__ == "__main__":
    unittest.main()
