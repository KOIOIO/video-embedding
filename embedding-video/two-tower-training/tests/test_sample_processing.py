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

    def test_aggregate_samples_preserves_first_pair_order(self):
        reference = datetime(2026, 6, 24, 0, 0, tzinfo=timezone.utc)
        rows = [
            sample_processing.RawSample(2, 20, 200, 1.0, 1.0, "test", "test", reference),
            sample_processing.RawSample(1, 10, 100, 1.0, 1.0, "test", "test", reference),
            sample_processing.RawSample(2, 10, 100, 1.0, 1.0, "test", "test", reference),
        ]

        aggregated = sample_processing.aggregate_samples(rows, reference_time=reference, half_life_days=30.0)

        self.assertEqual([(row.user_id, row.segment_id) for row in aggregated], [(2, 200), (1, 100), (2, 100)])

    def test_adjust_behavior_weight_uses_reason_semantics(self):
        self.assertGreater(
            sample_processing.adjust_behavior_weight("segment_reaction", "double_like", 2.0),
            sample_processing.adjust_behavior_weight("segment_reaction", "like", 2.0),
        )
        self.assertLess(
            sample_processing.adjust_behavior_weight("exposure", "exposure_no_click", 1.0),
            1.0,
        )
        self.assertGreater(
            sample_processing.adjust_behavior_weight("segment_reaction", "dislike", 2.0),
            2.0,
        )
        self.assertGreater(
            sample_processing.adjust_behavior_weight("watch", "watched_long", 1.0),
            sample_processing.adjust_behavior_weight("watch", "watched_short", 1.0),
        )

    def test_aggregate_samples_keeps_dislike_reason_for_strong_negative(self):
        reference = datetime(2026, 6, 24, 0, 0, tzinfo=timezone.utc)
        rows = [
            sample_processing.RawSample(
                user_id=7,
                video_id=11,
                segment_id=101,
                label=0.0,
                weight=2.0,
                source="segment_reaction",
                reason="dislike",
                event_time=datetime(2026, 6, 23, 0, 0, tzinfo=timezone.utc),
            ),
            sample_processing.RawSample(
                user_id=7,
                video_id=11,
                segment_id=101,
                label=1.0,
                weight=0.1,
                source="watch",
                reason="watched_short",
                event_time=datetime(2026, 6, 22, 0, 0, tzinfo=timezone.utc),
            ),
        ]

        aggregated = sample_processing.aggregate_samples(rows, reference_time=reference, half_life_days=30.0)

        self.assertEqual(aggregated[0].label, 0.0)
        self.assertIn("dislike", aggregated[0].reason)


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
