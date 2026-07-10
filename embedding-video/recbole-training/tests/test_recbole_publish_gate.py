import unittest

from recbole_recommendation import metrics, publish_gate


class RecBolePublishGateTest(unittest.TestCase):
    def test_metrics_include_framework_and_algorithm(self) -> None:
        got = metrics.normalize_metrics({"Recall@20": 0.2, "NDCG@20": 0.1}, "recbole_v1", "BPR")
        self.assertEqual(got["framework"], "recbole")
        self.assertEqual(got["algorithm"], "BPR")
        self.assertEqual(got["model_version"], "recbole_v1")

    def test_metrics_normalize_nested_recbole_results(self) -> None:
        got = metrics.normalize_metrics(
            {
                "best_valid_result": {"recall@20": 0.4, "ndcg@20": 0.5},
                "test_result": {"recall@20": 0.3, "ndcg@20": 0.35},
            },
            "recbole_v1",
            "BPR",
        )
        self.assertEqual(got["Recall@20"], 0.3)
        self.assertEqual(got["NDCG@20"], 0.35)

    def test_gate_accepts_good_metrics(self) -> None:
        ok, reasons = publish_gate.evaluate({"Recall@20": 0.2, "NDCG@20": 0.1})
        self.assertTrue(ok)
        self.assertEqual(reasons, [])

    def test_gate_rejects_low_or_regressed_metrics(self) -> None:
        ok, reasons = publish_gate.evaluate(
            {"Recall@20": 0.2, "NDCG@20": 0.01},
            {"NDCG@20": 0.1},
        )
        self.assertFalse(ok)
        self.assertTrue(any("drops" in reason for reason in reasons))


if __name__ == "__main__":
    unittest.main()
