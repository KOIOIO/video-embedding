import argparse
import json
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class Thresholds:
    min_eval_auc: float
    min_recall_at_k: float
    min_coverage_at_k: float
    max_negative_hit_rate_at_k: float
    min_eval_sample_count: int
    max_auc_drop: float = 0.0
    max_recall_drop: float = 0.0
    max_coverage_drop: float = 0.0
    max_negative_hit_rate_increase: float = 0.0
    max_dislike_hit_rate_increase: float = 0.0


@dataclass(frozen=True)
class GateResult:
    passed: bool
    failures: list[str]


def evaluate_gate(metrics: dict, thresholds: Thresholds, metric_k: int = 20, baseline: dict | None = None) -> GateResult:
    failures: list[str] = []
    eval_auc = float(metrics.get("eval_auc", 0.0))
    recall_name = f"recall_at_{metric_k}"
    coverage_name = f"coverage_at_{metric_k}"
    negative_hit_rate_name = f"negative_hit_rate_at_{metric_k}"
    recall = float(metrics.get(recall_name, 0.0))
    coverage = float(metrics.get(coverage_name, 0.0))
    negative_hit_rate = float(metrics.get(negative_hit_rate_name, 1.0))
    dislike_hit_rate_name = f"dislike_hit_rate_at_{metric_k}"
    dislike_hit_rate = float(metrics.get(dislike_hit_rate_name, 0.0))
    eval_samples = int(float(metrics.get("eval_sample_count", 0.0)))

    if eval_auc < thresholds.min_eval_auc:
        failures.append(f"eval_auc {eval_auc:.6f} < {thresholds.min_eval_auc:.6f}")
    if recall < thresholds.min_recall_at_k:
        failures.append(f"{recall_name} {recall:.6f} < {thresholds.min_recall_at_k:.6f}")
    if coverage < thresholds.min_coverage_at_k:
        failures.append(f"{coverage_name} {coverage:.6f} < {thresholds.min_coverage_at_k:.6f}")
    if negative_hit_rate > thresholds.max_negative_hit_rate_at_k:
        failures.append(f"{negative_hit_rate_name} {negative_hit_rate:.6f} > {thresholds.max_negative_hit_rate_at_k:.6f}")
    if eval_samples < thresholds.min_eval_sample_count:
        failures.append(f"eval_sample_count {eval_samples} < {thresholds.min_eval_sample_count}")
    if baseline:
        baseline_auc = float(baseline.get("eval_auc", 0.0))
        baseline_recall = float(baseline.get(recall_name, 0.0))
        baseline_coverage = float(baseline.get(coverage_name, 0.0))
        baseline_negative_hit_rate = float(baseline.get(negative_hit_rate_name, 0.0))
        baseline_dislike_hit_rate = float(baseline.get(dislike_hit_rate_name, 0.0))
        auc_drop = baseline_auc - eval_auc
        recall_drop = baseline_recall - recall
        coverage_drop = baseline_coverage - coverage
        negative_increase = negative_hit_rate - baseline_negative_hit_rate
        dislike_increase = dislike_hit_rate - baseline_dislike_hit_rate
        if auc_drop > thresholds.max_auc_drop:
            failures.append(f"eval_auc drop {auc_drop:.6f} > {thresholds.max_auc_drop:.6f}")
        if recall_drop > thresholds.max_recall_drop:
            failures.append(f"{recall_name} drop {recall_drop:.6f} > {thresholds.max_recall_drop:.6f}")
        if coverage_drop > thresholds.max_coverage_drop:
            failures.append(f"{coverage_name} drop {coverage_drop:.6f} > {thresholds.max_coverage_drop:.6f}")
        if negative_increase > thresholds.max_negative_hit_rate_increase:
            failures.append(f"{negative_hit_rate_name} increase {negative_increase:.6f} > {thresholds.max_negative_hit_rate_increase:.6f}")
        if dislike_increase > thresholds.max_dislike_hit_rate_increase:
            failures.append(f"{dislike_hit_rate_name} increase {dislike_increase:.6f} > {thresholds.max_dislike_hit_rate_increase:.6f}")

    return GateResult(passed=len(failures) == 0, failures=failures)


def load_metrics(path: Path) -> dict:
    return json.loads(Path(path).read_text(encoding="utf-8"))


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Check whether a two tower artifact can be published.")
    parser.add_argument("--metrics", type=Path, required=True)
    parser.add_argument("--min-eval-auc", type=float, default=0.55)
    parser.add_argument("--min-recall-at-20", type=float, default=0.05)
    parser.add_argument("--min-coverage-at-20", type=float, default=0.02)
    parser.add_argument("--max-negative-hit-rate-at-20", type=float, default=0.50)
    parser.add_argument("--min-eval-sample-count", type=int, default=20)
    parser.add_argument("--metric-k", type=int, default=20)
    parser.add_argument("--baseline-metrics", type=Path, default=None)
    parser.add_argument("--max-auc-drop", type=float, default=0.0)
    parser.add_argument("--max-recall-drop", type=float, default=0.0)
    parser.add_argument("--max-coverage-drop", type=float, default=0.0)
    parser.add_argument("--max-negative-hit-rate-increase", type=float, default=0.0)
    parser.add_argument("--max-dislike-hit-rate-increase", type=float, default=0.0)
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    thresholds = Thresholds(
        min_eval_auc=args.min_eval_auc,
        min_recall_at_k=args.min_recall_at_20,
        min_coverage_at_k=args.min_coverage_at_20,
        max_negative_hit_rate_at_k=args.max_negative_hit_rate_at_20,
        min_eval_sample_count=args.min_eval_sample_count,
        max_auc_drop=args.max_auc_drop,
        max_recall_drop=args.max_recall_drop,
        max_coverage_drop=args.max_coverage_drop,
        max_negative_hit_rate_increase=args.max_negative_hit_rate_increase,
        max_dislike_hit_rate_increase=args.max_dislike_hit_rate_increase,
    )
    baseline = load_metrics(args.baseline_metrics) if args.baseline_metrics else None
    result = evaluate_gate(load_metrics(args.metrics), thresholds, metric_k=args.metric_k, baseline=baseline)
    if result.passed:
        print("publish_gate=pass")
        return 0
    print("publish_gate=fail")
    for failure in result.failures:
        print(f"failure={failure}")
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
