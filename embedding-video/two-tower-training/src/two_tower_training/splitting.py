import math

from two_tower_training.sample_processing import RawSample


def split_temporal(rows: list[RawSample], eval_ratio: float) -> tuple[list[RawSample], list[RawSample]]:
    if len(rows) < 2:
        raise ValueError("temporal split requires at least two samples")
    if eval_ratio <= 0 or eval_ratio >= 1:
        raise ValueError("eval_ratio must be between 0 and 1")

    if all(item.event_time is not None for item in rows):
        ordered = sorted(rows, key=lambda item: (item.event_time, item.user_id, item.segment_id))
    else:
        ordered = list(rows)
    eval_count = max(1, math.ceil(len(ordered) * eval_ratio))
    train_count = len(ordered) - eval_count
    if train_count <= 0:
        train_count = len(ordered) - 1
    return ordered[:train_count], ordered[train_count:]
