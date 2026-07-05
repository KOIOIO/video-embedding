import unittest
import random

from two_tower_training import torch_model, train


class TorchModelTest(unittest.TestCase):
    def test_train_torch_model_uses_catalog_items_and_reports_loss(self):
        dataset = train.Dataset(
            user_ids=[1, 2],
            segment_ids=[101, 102, 201],
            segment_video_ids={101: 10, 102: 10, 201: 20},
            samples=[
                train.Sample(1, 10, 101, 0, 0, 1.0, 2.0),
                train.Sample(1, 10, 102, 0, 1, 0.0, 1.0, reason="dislike"),
                train.Sample(2, 20, 201, 1, 2, 1.0, 2.0),
                train.Sample(2, 10, 102, 1, 1, 0.0, 1.0, reason="dislike"),
            ],
            item_features={
                101: train.ItemFeature(101, 10, content_summary="函数", knowledge_tags="函数|导数", video_title="数学", like_count=2),
                102: train.ItemFeature(102, 10, content_summary="集合", knowledge_tags="集合", video_title="数学", dislike_count=1),
                201: train.ItemFeature(201, 20, content_summary="英语阅读", knowledge_tags="英语", video_title="英语", like_count=1),
            },
            user_features={
                1: train.UserFeature(user_id=1, grade_id=10, class_id=20, mastery_avg=0.4, recent_subjects="math", recent_knowledge_point_ids="101|102"),
                2: train.UserFeature(user_id=2, grade_id=11, class_id=21, mastery_avg=0.8, recent_subjects="english", recent_knowledge_point_ids="201"),
            },
        )

        model, metrics = torch_model.train_torch_model(
            dataset=dataset,
            dim=8,
            epochs=3,
            learning_rate=0.05,
            seed=7,
            l2=0.0,
            batch_size=4,
            random_negatives=1,
            hard_negatives=1,
        )

        self.assertEqual(len(model.user_embeddings), 2)
        self.assertEqual(len(model.item_embeddings), 3)
        self.assertIn("torch_train_loss", metrics)
        self.assertEqual(metrics["backend"], "torch")

    def test_user_feature_vector_changes_with_learning_features(self):
        low = train.UserFeature(user_id=1, grade_id=1, class_id=1, mastery_avg=0.2, recent_subjects="math", recent_knowledge_point_ids="101")
        high = train.UserFeature(user_id=1, grade_id=1, class_id=1, mastery_avg=0.9, recent_subjects="english", recent_knowledge_point_ids="201")

        low_vec = torch_model.user_feature_vector(low)
        high_vec = torch_model.user_feature_vector(high)

        self.assertEqual(len(low_vec), len(high_vec))
        self.assertNotEqual(low_vec, high_vec)

    def test_training_pairs_do_not_use_user_known_positives_as_negatives(self):
        samples = [
            train.Sample(1, 10, 101, 0, 0, 1.0, 2.0),
            train.Sample(1, 10, 102, 0, 1, 1.0, 2.0),
            train.Sample(1, 10, 103, 0, 2, 0.0, 1.0, reason="dislike"),
            train.Sample(2, 20, 201, 1, 3, 1.0, 2.0),
        ]
        known_positives = torch_model.known_positive_items_by_user(samples)

        pairs = torch_model.training_pairs(
            batch=samples,
            all_item_indexes=[0, 1, 2, 3],
            rng=random.Random(7),
            random_negatives=8,
            hard_negatives=8,
            known_positives_by_user=known_positives,
        )

        self.assertTrue(pairs)
        for user_index, _positive_item, negative_item, _weight in pairs:
            self.assertNotIn(negative_item, known_positives[user_index])

    def test_train_torch_model_reports_pairwise_objective(self):
        dataset = train.Dataset(
            user_ids=[1],
            segment_ids=[101, 102],
            segment_video_ids={101: 10, 102: 10},
            samples=[
                train.Sample(1, 10, 101, 0, 0, 1.0, 2.0),
                train.Sample(1, 10, 102, 0, 1, 0.0, 1.0, reason="dislike"),
            ],
        )

        _model, metrics = torch_model.train_torch_model(
            dataset=dataset,
            dim=8,
            epochs=2,
            learning_rate=0.01,
            seed=7,
            l2=0.0,
            batch_size=2,
            random_negatives=1,
            hard_negatives=1,
        )

        self.assertEqual(metrics["loss_objective"], "pairwise_softplus")
        self.assertEqual(metrics["id_embedding_scale"], 0.5)

    def test_pairwise_training_ranks_explicit_positive_above_negative(self):
        dataset = train.Dataset(
            user_ids=[1],
            segment_ids=[101, 102, 103],
            segment_video_ids={101: 10, 102: 10, 103: 10},
            samples=[
                train.Sample(1, 10, 101, 0, 0, 1.0, 3.0),
                train.Sample(1, 10, 102, 0, 1, 0.0, 2.0, reason="dislike"),
                train.Sample(1, 10, 103, 0, 2, 0.0, 1.0, reason="watched_short"),
            ],
            item_features={
                101: train.ItemFeature(101, 10, content_summary="函数 单调性", knowledge_tags="函数|导数", video_title="数学"),
                102: train.ItemFeature(102, 10, content_summary="英语 阅读", knowledge_tags="英语", video_title="英语"),
                103: train.ItemFeature(103, 10, content_summary="物理 力学", knowledge_tags="物理", video_title="物理"),
            },
        )

        model, _metrics = torch_model.train_torch_model(
            dataset=dataset,
            dim=8,
            epochs=20,
            learning_rate=0.02,
            seed=11,
            l2=0.0,
            batch_size=3,
            random_negatives=1,
            hard_negatives=1,
        )

        positive_score = train.dot(model.user_embeddings[0], model.item_embeddings[0])
        negative_score = train.dot(model.user_embeddings[0], model.item_embeddings[1])
        self.assertGreater(positive_score, negative_score)


if __name__ == "__main__":
    unittest.main()
