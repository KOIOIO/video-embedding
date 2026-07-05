import unittest
from datetime import datetime

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

    def test_split_temporal_preserves_input_order_without_event_times(self):
        rows = [
            sample_processing.RawSample(3, 30, 301, 1.0, 1.0, "test", "test", None),
            sample_processing.RawSample(1, 10, 101, 1.0, 1.0, "test", "test", None),
            sample_processing.RawSample(2, 20, 201, 1.0, 1.0, "test", "test", None),
        ]

        train_rows, eval_rows = splitting.split_temporal(rows, eval_ratio=0.34)

        self.assertEqual([item.segment_id for item in train_rows], [301])
        self.assertEqual([item.segment_id for item in eval_rows], [101, 201])


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
