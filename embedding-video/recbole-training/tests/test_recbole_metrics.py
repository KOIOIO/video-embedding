import json
import tempfile
import unittest
from pathlib import Path

from recbole_recommendation.metrics import merge_export_stats


class RecBoleMetricsTest(unittest.TestCase):
    def test_merges_numeric_export_statistics(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "stats.json"
            path.write_text(json.dumps({"knowledge_watch_training_interactions": 3, "ignored": "x"}))
            result = merge_export_stats({"Recall@20": 0.4}, path)
            self.assertEqual(result["knowledge_watch_training_interactions"], 3)
            self.assertNotIn("ignored", result)
