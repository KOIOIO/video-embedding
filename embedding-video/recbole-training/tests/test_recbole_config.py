from pathlib import Path
import tempfile
import unittest

from recbole_recommendation import config


class RecBoleConfigTest(unittest.TestCase):
    def test_defaults_are_embedding_retrieval_friendly(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            cfg = config.build_config(Path(tmp) / "recbole_v1")

        self.assertEqual(cfg["model"], "BPR")
        self.assertEqual(cfg["embedding_size"], 64)
        self.assertEqual(cfg["dataset"], "video_app")
        self.assertIn("Recall", cfg["metrics"])
        self.assertIn("NDCG", cfg["metrics"])
        self.assertIn("Hit", cfg["metrics"])
        self.assertIn("Precision", cfg["metrics"])

    def test_evaluation_is_deterministic_and_time_ordered(self) -> None:
        cfg = config.build_config("/tmp/recbole/data/recbole_v1")
        self.assertEqual(cfg["seed"], 20260730)
        self.assertTrue(cfg["reproducibility"])
        self.assertEqual(cfg["eval_args"]["order"], "TO")
        self.assertEqual(cfg["eval_args"]["group_by"], "user")
        self.assertEqual(cfg["train_neg_sample_args"]["distribution"], "uniform")

    def test_dataset_path_points_at_atomic_parent(self) -> None:
        cfg = config.build_config("/tmp/recbole/data/recbole_v1", dataset="video_app")
        self.assertEqual(cfg["data_path"], "/tmp/recbole/data")


if __name__ == "__main__":
    unittest.main()
