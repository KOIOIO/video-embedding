from __future__ import annotations

import argparse
from dataclasses import dataclass
from pathlib import Path

from . import metrics as metrics_io


@dataclass(frozen=True)
class GateConfig:
    min_recall_at_20: float = 0.01
    min_ndcg_at_20: float = 0.005
    max_relative_ndcg_drop: float = 0.2


def metric_value(data: dict, name: str) -> float:
    for key in (name, name.lower(), name.replace("@", "_"), name.lower().replace("@", "_")):
        value = data.get(key)
        if isinstance(value, (int, float)):
            return float(value)
    return 0.0


def evaluate(metrics: dict, baseline: dict | None = None, config: GateConfig = GateConfig()) -> tuple[bool, list[str]]:
    baseline = baseline or {}
    reasons: list[str] = []
    recall = metric_value(metrics, "Recall@20")
    ndcg = metric_value(metrics, "NDCG@20")
    if recall < config.min_recall_at_20:
        reasons.append(f"Recall@20 {recall:.6f} below {config.min_recall_at_20:.6f}")
    if ndcg < config.min_ndcg_at_20:
        reasons.append(f"NDCG@20 {ndcg:.6f} below {config.min_ndcg_at_20:.6f}")
    baseline_ndcg = metric_value(baseline, "NDCG@20")
    if baseline_ndcg > 0 and ndcg < baseline_ndcg * (1 - config.max_relative_ndcg_drop):
        reasons.append(f"NDCG@20 {ndcg:.6f} drops more than {config.max_relative_ndcg_drop:.0%} from baseline {baseline_ndcg:.6f}")
    return len(reasons) == 0, reasons


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Check whether a RecBole artifact can be published.")
    parser.add_argument("--metrics", required=True)
    parser.add_argument("--baseline", default="")
    parser.add_argument("--min-recall-at-20", type=float, default=GateConfig.min_recall_at_20)
    parser.add_argument("--min-ndcg-at-20", type=float, default=GateConfig.min_ndcg_at_20)
    parser.add_argument("--max-relative-ndcg-drop", type=float, default=GateConfig.max_relative_ndcg_drop)
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    current = metrics_io.read_metrics(args.metrics)
    baseline = metrics_io.read_metrics(args.baseline) if args.baseline and Path(args.baseline).exists() else {}
    ok, reasons = evaluate(
        current,
        baseline,
        GateConfig(
            min_recall_at_20=args.min_recall_at_20,
            min_ndcg_at_20=args.min_ndcg_at_20,
            max_relative_ndcg_drop=args.max_relative_ndcg_drop,
        ),
    )
    if not ok:
        raise SystemExit("; ".join(reasons))


if __name__ == "__main__":
    main()
