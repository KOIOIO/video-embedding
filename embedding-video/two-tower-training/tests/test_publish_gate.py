import json
import tempfile
import unittest
from contextlib import redirect_stdout
from io import StringIO
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
                min_recall_at_k=0.30,
                min_coverage_at_k=0.20,
                max_negative_hit_rate_at_k=0.05,
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
                min_recall_at_k=0.30,
                min_coverage_at_k=0.20,
                max_negative_hit_rate_at_k=0.05,
                min_eval_sample_count=50,
            ),
        )

        self.assertFalse(result.passed)
        self.assertIn("eval_auc 0.600000 < 0.700000", result.failures)
        self.assertIn("recall_at_20 0.100000 < 0.300000", result.failures)
        self.assertIn("coverage_at_20 0.050000 < 0.200000", result.failures)
        self.assertIn("negative_hit_rate_at_20 0.200000 > 0.050000", result.failures)
        self.assertIn("eval_sample_count 3 < 50", result.failures)

    def test_gate_uses_configured_metric_k(self):
        result = publish_gate.evaluate_gate(
            metrics={
                "eval_auc": 0.72,
                "recall_at_50": 0.31,
                "coverage_at_50": 0.22,
                "negative_hit_rate_at_50": 0.03,
                "eval_sample_count": 100.0,
            },
            thresholds=publish_gate.Thresholds(
                min_eval_auc=0.70,
                min_recall_at_k=0.30,
                min_coverage_at_k=0.20,
                max_negative_hit_rate_at_k=0.05,
                min_eval_sample_count=50,
            ),
            metric_k=50,
        )

        self.assertTrue(result.passed)

    def test_gate_uses_excess_negative_hit_rate_when_available(self):
        result = publish_gate.evaluate_gate(
            metrics={
                "eval_auc": 0.72,
                "recall_at_20": 0.31,
                "coverage_at_20": 0.22,
                "negative_hit_rate_at_20": 0.90,
                "excess_negative_hit_rate_at_20": 0.03,
                "eval_sample_count": 57.0,
            },
            thresholds=publish_gate.Thresholds(
                min_eval_auc=0.70,
                min_recall_at_k=0.30,
                min_coverage_at_k=0.20,
                max_negative_hit_rate_at_k=0.05,
                min_eval_sample_count=50,
            ),
        )

        self.assertTrue(result.passed)

    def test_gate_compares_against_baseline_metrics(self):
        result = publish_gate.evaluate_gate(
            metrics={
                "eval_auc": 0.70,
                "recall_at_20": 0.20,
                "coverage_at_20": 0.18,
                "negative_hit_rate_at_20": 0.10,
                "dislike_hit_rate_at_20": 0.08,
                "eval_sample_count": 100.0,
            },
            thresholds=publish_gate.Thresholds(
                min_eval_auc=0.50,
                min_recall_at_k=0.05,
                min_coverage_at_k=0.02,
                max_negative_hit_rate_at_k=0.50,
                min_eval_sample_count=20,
                max_auc_drop=0.01,
                max_recall_drop=0.02,
                max_coverage_drop=0.02,
                max_negative_hit_rate_increase=0.01,
                max_dislike_hit_rate_increase=0.01,
            ),
            baseline={
                "eval_auc": 0.75,
                "recall_at_20": 0.25,
                "coverage_at_20": 0.20,
                "negative_hit_rate_at_20": 0.08,
                "dislike_hit_rate_at_20": 0.02,
            },
        )

        self.assertFalse(result.passed)
        self.assertIn("eval_auc drop 0.050000 > 0.010000", result.failures)
        self.assertIn("recall_at_20 drop 0.050000 > 0.020000", result.failures)
        self.assertIn("negative_hit_rate_at_20 increase 0.020000 > 0.010000", result.failures)
        self.assertIn("dislike_hit_rate_at_20 increase 0.060000 > 0.010000", result.failures)

    def test_cli_returns_nonzero_when_gate_fails(self):
        with tempfile.TemporaryDirectory() as tmp:
            metrics_path = Path(tmp) / "metrics.json"
            metrics_path.write_text(
                json.dumps(
                    {
                        "eval_auc": 0.1,
                        "recall_at_20": 0.1,
                        "coverage_at_20": 0.1,
                        "negative_hit_rate_at_20": 0.1,
                        "eval_sample_count": 10,
                    }
                ),
                encoding="utf-8",
            )

            with redirect_stdout(StringIO()):
                code = publish_gate.main(
                    [
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
                    ]
                )

        self.assertEqual(code, 1)


if __name__ == "__main__":
    unittest.main()
