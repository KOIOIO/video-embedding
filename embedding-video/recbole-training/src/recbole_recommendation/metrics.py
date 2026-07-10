from __future__ import annotations

import json
from collections.abc import Mapping
from pathlib import Path
from typing import Any


METRIC_NAMES = {
    "recall": "Recall",
    "ndcg": "NDCG",
    "hit": "Hit",
    "precision": "Precision",
}


def normalize_metrics(raw: dict[str, Any], model_version: str, algorithm: str) -> dict[str, Any]:
    result: dict[str, Any] = {
        "framework": "recbole",
        "algorithm": algorithm,
        "model_version": model_version,
    }
    for key, value in raw.items():
        if isinstance(value, (int, float)):
            result[canonical_metric_key(key)] = float(value)
        elif isinstance(value, Mapping):
            for nested_key, nested_value in value.items():
                if isinstance(nested_value, (int, float)):
                    result[canonical_metric_key(str(nested_key))] = float(nested_value)
    return result


def canonical_metric_key(key: str) -> str:
    name, sep, suffix = key.partition("@")
    normalized = METRIC_NAMES.get(name.lower(), name)
    if sep:
        return normalized + sep + suffix
    return normalized


def read_metrics(path: str | Path) -> dict[str, Any]:
    p = Path(path)
    if not p.exists():
        return {}
    text = p.read_text(encoding="utf-8").strip()
    if not text:
        return {}
    return json.loads(text)


def write_metrics(path: str | Path, metrics: dict[str, Any]) -> None:
    p = Path(path)
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(json.dumps(metrics, ensure_ascii=False, sort_keys=True) + "\n", encoding="utf-8")
