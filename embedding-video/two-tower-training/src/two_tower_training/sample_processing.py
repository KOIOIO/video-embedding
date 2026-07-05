import csv
import math
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path


@dataclass(frozen=True)
class RawSample:
    user_id: int
    video_id: int
    segment_id: int
    label: float
    weight: float
    source: str
    reason: str
    event_time: datetime


def load_raw_samples(path: Path) -> list[RawSample]:
    rows: list[RawSample] = []
    with Path(path).open(newline="", encoding="utf-8") as handle:
        reader = csv.DictReader(handle)
        required = {"user_id", "video_id", "video_segment_id", "label", "weight", "source", "reason", "event_time"}
        missing = required.difference(reader.fieldnames or [])
        if missing:
            raise ValueError(f"sample csv missing columns: {', '.join(sorted(missing))}")
        for row in reader:
            sample = parse_raw_sample(row)
            if sample is not None:
                rows.append(sample)
    if not rows:
        raise ValueError("sample csv has no usable rows")
    return rows


def parse_raw_sample(row: dict[str, str]) -> RawSample | None:
    user_id = int(row["user_id"])
    video_id = int(row["video_id"])
    segment_id = int(row["video_segment_id"])
    label = float(row["label"])
    weight = float(row["weight"])
    if user_id <= 0 or video_id <= 0 or segment_id <= 0:
        return None
    if label not in (0.0, 1.0):
        return None
    if weight <= 0:
        return None
    return RawSample(
        user_id=user_id,
        video_id=video_id,
        segment_id=segment_id,
        label=label,
        weight=weight,
        source=row["source"].strip() or "unknown",
        reason=row["reason"].strip() or "unknown",
        event_time=parse_event_time(row["event_time"]),
    )


def parse_event_time(value: str) -> datetime:
    cleaned = value.strip()
    if not cleaned:
        raise ValueError("event_time is required")
    if cleaned.endswith("Z"):
        cleaned = cleaned[:-1] + "+00:00"
    parsed = datetime.fromisoformat(cleaned)
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=timezone.utc)
    return parsed


def time_decay(event_time: datetime, reference_time: datetime, half_life_days: float) -> float:
    if half_life_days <= 0:
        raise ValueError("half_life_days must be greater than 0")
    event_utc = event_time.astimezone(timezone.utc)
    reference_utc = reference_time.astimezone(timezone.utc)
    age_days = max(0.0, (reference_utc - event_utc).total_seconds() / 86400.0)
    return math.exp(-age_days / half_life_days)


def adjust_behavior_weight(source: str, reason: str, weight: float) -> float:
    source = source.strip()
    reason = reason.strip()
    multiplier = 1.0
    if reason == "double_like":
        multiplier = 1.25
    elif reason == "like":
        multiplier = 1.0
    elif reason == "dislike":
        multiplier = 1.5
    elif reason == "video_double_like":
        multiplier = 1.1
    elif reason == "video_like":
        multiplier = 0.9
    elif reason == "video_dislike":
        multiplier = 1.25
    elif reason == "exposure_no_click":
        multiplier = 0.5
    elif reason == "exposure_clicked":
        multiplier = 0.8
    elif reason == "exposure_watched":
        multiplier = 1.0
    elif reason in {"watched_long", "watched"}:
        multiplier = 1.1
    elif reason == "watched_short":
        multiplier = 0.5
    if source == "exposure" and reason == "unknown":
        multiplier = 0.5
    return max(0.01, weight * multiplier)


def aggregate_samples(
    rows: list[RawSample],
    reference_time: datetime,
    half_life_days: float,
    max_weight: float = 5.0,
) -> list[RawSample]:
    grouped: dict[tuple[int, int], list[RawSample]] = {}
    for row in rows:
        grouped.setdefault((row.user_id, row.segment_id), []).append(row)

    aggregated: list[RawSample] = []
    for events in grouped.values():
        newest = max(events, key=lambda item: item.event_time)
        pos_strength = 0.0
        neg_strength = 0.0
        reasons = set()
        for event in events:
            if event.reason:
                reasons.add(event.reason)
            adjusted = adjust_behavior_weight(event.source, event.reason, event.weight)
            decayed = adjusted * time_decay(event.event_time, reference_time, half_life_days)
            if event.label == 1.0:
                pos_strength += decayed
            else:
                neg_strength += decayed
        if pos_strength >= neg_strength:
            label = 1.0
            weight = pos_strength
        else:
            label = 0.0
            weight = neg_strength
        aggregated.append(
            RawSample(
                user_id=newest.user_id,
                video_id=newest.video_id,
                segment_id=newest.segment_id,
                label=label,
                weight=min(max_weight, max(0.05, weight)),
                source="aggregated",
                reason=f"pos={pos_strength:.3f};neg={neg_strength:.3f};events={len(events)};reasons={','.join(sorted(reasons))}",
                event_time=newest.event_time,
            )
        )
    return aggregated
