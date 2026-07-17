from __future__ import annotations

from pathlib import Path
from typing import Any


DEFAULT_METRICS = ["Recall", "NDCG", "Hit", "Precision"]


def build_config(
    dataset_dir: str | Path,
    dataset: str = "video_dataset",
    model: str = "BPR",
    embedding_size: int = 64,
) -> dict[str, Any]:
    dataset_path = Path(dataset_dir)
    return {
        "model": model,
        "dataset": dataset,
        "data_path": str(dataset_path.parent),
        "field_separator": ",",
        "USER_ID_FIELD": "user_id",
        "ITEM_ID_FIELD": "item_id",
        "RATING_FIELD": "rating",
        "TIME_FIELD": "timestamp",
        "load_col": {
            "inter": ["user_id", "item_id", "rating", "timestamp", "source", "weight"],
            "user": ["user_id"],
            "item": ["item_id", "video_id"],
        },
        "embedding_size": embedding_size,
        "metrics": DEFAULT_METRICS,
        "valid_metric": "NDCG@20",
        "topk": [10, 20],
        "epochs": 20,
        "train_batch_size": 2048,
        "eval_batch_size": 4096,
    }


def write_config(path: str | Path, config: dict[str, Any]) -> None:
    import yaml

    target = Path(path)
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(yaml.safe_dump(config, sort_keys=False), encoding="utf-8")
