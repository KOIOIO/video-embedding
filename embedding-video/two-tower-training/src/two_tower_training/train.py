import argparse
import csv
import json
import math
import random
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path

from two_tower_training import metrics as training_metrics
from two_tower_training import sample_processing, splitting


@dataclass(frozen=True)
class Sample:
    user_id: int
    video_id: int
    segment_id: int
    user_index: int
    segment_index: int
    label: float
    weight: float
    source: str = "unknown"
    reason: str = "unknown"
    event_time: datetime | None = None


@dataclass(frozen=True)
class ItemFeature:
    segment_id: int
    video_id: int
    segment_duration: int = 1
    video_duration: int = 0
    like_count: int = 0
    double_like_count: int = 0
    dislike_count: int = 0
    content_summary: str = ""
    knowledge_tags: str = ""
    video_title: str = ""


@dataclass(frozen=True)
class UserFeature:
    user_id: int
    grade_id: int = 0
    class_id: int = 0
    user_type: int = 0
    mastery_avg: float = 0.0
    mastery_min: float = 0.0
    weak_knowledge_count: int = 0
    strong_knowledge_count: int = 0
    knowledge_correct_count: int = 0
    knowledge_incorrect_count: int = 0
    answer_count: int = 0
    answer_correct_count: int = 0
    answer_incorrect_count: int = 0
    avg_score_rate: float = 0.0
    avg_cost_seconds: float = 0.0
    question_feedback_count: int = 0
    generated_feedback_count: int = 0
    generated_correct_count: int = 0
    generated_avg_score_rate: float = 0.0
    question_search_count: int = 0
    recent_knowledge_point_ids: str = ""
    recent_subjects: str = ""
    question_search_knowledge_text: str = ""
    generated_feedback_knowledge_text: str = ""


@dataclass(frozen=True)
class Dataset:
    user_ids: list[int]
    segment_ids: list[int]
    segment_video_ids: dict[int, int]
    samples: list[Sample]
    item_features: dict[int, ItemFeature] | None = None
    user_features: dict[int, UserFeature] | None = None


@dataclass(frozen=True)
class TwoTowerModel:
    user_embeddings: list[list[float]]
    item_embeddings: list[list[float]]


def load_dataset(
    path: Path,
    half_life_days: float = 30.0,
    item_catalog: Path | None = None,
    user_features: Path | None = None,
) -> Dataset:
    raw_rows = sample_processing.load_raw_samples(path)
    reference_time = max(row.event_time for row in raw_rows).astimezone(timezone.utc)
    processed_rows = sample_processing.aggregate_samples(
        raw_rows,
        reference_time=reference_time,
        half_life_days=half_life_days,
    )
    return build_dataset(
        processed_rows,
        item_features=load_item_features(item_catalog) if item_catalog else None,
        user_features=load_user_features(user_features) if user_features else None,
    )


def load_item_features(path: Path) -> dict[int, ItemFeature]:
    features: dict[int, ItemFeature] = {}
    with Path(path).open(newline="", encoding="utf-8") as handle:
        reader = csv.DictReader(handle)
        required = {
            "video_segment_id",
            "video_id",
            "segment_duration",
            "video_duration",
            "like_count",
            "double_like_count",
            "dislike_count",
            "content_summary",
            "knowledge_tags",
            "video_title",
        }
        missing = required.difference(reader.fieldnames or [])
        if missing:
            raise ValueError(f"item catalog csv missing columns: {', '.join(sorted(missing))}")
        for row in reader:
            feature = ItemFeature(
                segment_id=int(row["video_segment_id"]),
                video_id=int(row["video_id"]),
                segment_duration=max(1, int(row["segment_duration"] or "1")),
                video_duration=max(0, int(row["video_duration"] or "0")),
                like_count=max(0, int(row["like_count"] or "0")),
                double_like_count=max(0, int(row["double_like_count"] or "0")),
                dislike_count=max(0, int(row["dislike_count"] or "0")),
                content_summary=row["content_summary"],
                knowledge_tags=row["knowledge_tags"],
                video_title=row["video_title"],
            )
            if feature.segment_id > 0 and feature.video_id > 0:
                features[feature.segment_id] = feature
    return features


def load_user_features(path: Path) -> dict[int, UserFeature]:
    features: dict[int, UserFeature] = {}
    with Path(path).open(newline="", encoding="utf-8") as handle:
        reader = csv.DictReader(handle)
        required = {
            "user_id",
            "grade_id",
            "class_id",
            "user_type",
            "mastery_avg",
            "mastery_min",
            "weak_knowledge_count",
            "strong_knowledge_count",
            "knowledge_correct_count",
            "knowledge_incorrect_count",
            "answer_count",
            "answer_correct_count",
            "answer_incorrect_count",
            "avg_score_rate",
            "avg_cost_seconds",
            "question_feedback_count",
            "generated_feedback_count",
            "generated_correct_count",
            "generated_avg_score_rate",
            "question_search_count",
            "recent_knowledge_point_ids",
            "recent_subjects",
            "question_search_knowledge_text",
            "generated_feedback_knowledge_text",
        }
        missing = required.difference(reader.fieldnames or [])
        if missing:
            raise ValueError(f"user feature csv missing columns: {', '.join(sorted(missing))}")
        for row in reader:
            feature = UserFeature(
                user_id=int(row["user_id"]),
                grade_id=int(row["grade_id"] or "0"),
                class_id=int(row["class_id"] or "0"),
                user_type=int(row["user_type"] or "0"),
                mastery_avg=float(row["mastery_avg"] or "0"),
                mastery_min=float(row["mastery_min"] or "0"),
                weak_knowledge_count=int(row["weak_knowledge_count"] or "0"),
                strong_knowledge_count=int(row["strong_knowledge_count"] or "0"),
                knowledge_correct_count=int(row["knowledge_correct_count"] or "0"),
                knowledge_incorrect_count=int(row["knowledge_incorrect_count"] or "0"),
                answer_count=int(row["answer_count"] or "0"),
                answer_correct_count=int(row["answer_correct_count"] or "0"),
                answer_incorrect_count=int(row["answer_incorrect_count"] or "0"),
                avg_score_rate=float(row["avg_score_rate"] or "0"),
                avg_cost_seconds=float(row["avg_cost_seconds"] or "0"),
                question_feedback_count=int(row["question_feedback_count"] or "0"),
                generated_feedback_count=int(row["generated_feedback_count"] or "0"),
                generated_correct_count=int(row["generated_correct_count"] or "0"),
                generated_avg_score_rate=float(row["generated_avg_score_rate"] or "0"),
                question_search_count=int(row["question_search_count"] or "0"),
                recent_knowledge_point_ids=row["recent_knowledge_point_ids"],
                recent_subjects=row["recent_subjects"],
                question_search_knowledge_text=row["question_search_knowledge_text"],
                generated_feedback_knowledge_text=row["generated_feedback_knowledge_text"],
            )
            if feature.user_id > 0:
                features[feature.user_id] = feature
    return features


def build_dataset(
    rows: list[sample_processing.RawSample],
    item_features: dict[int, ItemFeature] | None = None,
    user_features: dict[int, UserFeature] | None = None,
) -> Dataset:
    if not rows:
        raise ValueError("sample csv has no usable rows")

    user_ids = sorted({row.user_id for row in rows})
    segment_ids = sorted(set(row.segment_id for row in rows).union((item_features or {}).keys()))
    user_index = {user_id: idx for idx, user_id in enumerate(user_ids)}
    segment_index = {segment_id: idx for idx, segment_id in enumerate(segment_ids)}
    segment_video_ids = {}
    samples = []
    for row in rows:
        previous_video_id = segment_video_ids.get(row.segment_id)
        if previous_video_id is not None and previous_video_id != row.video_id:
            raise ValueError(f"segment {row.segment_id} maps to multiple videos")
        segment_video_ids[row.segment_id] = row.video_id
        samples.append(
            Sample(
                user_id=row.user_id,
                video_id=row.video_id,
                segment_id=row.segment_id,
                user_index=user_index[row.user_id],
                segment_index=segment_index[row.segment_id],
                label=row.label,
                weight=row.weight,
                source=row.source,
                reason=row.reason,
                event_time=row.event_time,
            )
        )
    if item_features:
        for segment_id, feature in item_features.items():
            previous_video_id = segment_video_ids.get(segment_id)
            if previous_video_id is not None and previous_video_id != feature.video_id:
                raise ValueError(f"segment {segment_id} maps to multiple videos")
            segment_video_ids[segment_id] = feature.video_id

    return Dataset(
        user_ids=user_ids,
        segment_ids=segment_ids,
        segment_video_ids=segment_video_ids,
        samples=samples,
        item_features=item_features or {},
        user_features=user_features or {},
    )


def train_model(
    dataset: Dataset,
    dim: int,
    epochs: int,
    learning_rate: float,
    seed: int,
    l2: float,
    eval_ratio: float = 0.2,
    retrieval_k: int = 20,
    retrieval_ks: list[int] | None = None,
) -> tuple[TwoTowerModel, dict[str, float]]:
    if dim <= 0:
        raise ValueError("dim must be greater than 0")
    if epochs <= 0:
        raise ValueError("epochs must be greater than 0")
    if learning_rate <= 0:
        raise ValueError("learning-rate must be greater than 0")
    rng = random.Random(seed)
    scale = 1.0 / math.sqrt(dim)
    user_embeddings = [
        [rng.uniform(-scale, scale) for _ in range(dim)]
        for _ in dataset.user_ids
    ]
    item_embeddings = [
        [rng.uniform(-scale, scale) for _ in range(dim)]
        for _ in dataset.segment_ids
    ]

    train_samples, eval_samples = splitting.split_temporal(dataset.samples, eval_ratio=eval_ratio)
    samples = list(train_samples)
    for _ in range(epochs):
        rng.shuffle(samples)
        for sample in samples:
            user_vec = user_embeddings[sample.user_index]
            item_vec = item_embeddings[sample.segment_index]
            score = dot(user_vec, item_vec)
            prediction = sigmoid(score)
            grad = sample.weight * (prediction - sample.label)
            old_user = list(user_vec)
            for i in range(dim):
                user_grad = grad * item_vec[i] + l2 * user_vec[i]
                item_grad = grad * old_user[i] + l2 * item_vec[i]
                user_vec[i] -= learning_rate * user_grad
                item_vec[i] -= learning_rate * item_grad
            normalize_in_place(user_vec)
            normalize_in_place(item_vec)

    model = TwoTowerModel(user_embeddings=user_embeddings, item_embeddings=item_embeddings)
    metrics_payload = training_metrics.evaluate_pointwise(model, train_samples, "train")
    metrics_payload.update(training_metrics.evaluate_pointwise(model, eval_samples, "eval"))
    metrics_payload.update(
        training_metrics.evaluate_retrieval_many(
            dataset=dataset,
            model=model,
            eval_samples=eval_samples,
            train_samples=train_samples,
            ks=retrieval_ks or [retrieval_k],
        )
    )
    primary_k = (retrieval_ks or [retrieval_k])[0]
    metrics_payload.update(
        {
            "loss": metrics_payload["eval_loss"],
            "auc": metrics_payload["eval_auc"],
            "positive_avg_score": metrics_payload["eval_positive_avg_score"],
            "negative_avg_score": metrics_payload["eval_negative_avg_score"],
            "dim": float(dim),
            "epochs": float(epochs),
            "learning_rate": float(learning_rate),
            "l2": float(l2),
            "eval_ratio": float(eval_ratio),
            "retrieval_k": float(primary_k),
            "retrieval_ks": ",".join(str(k) for k in (retrieval_ks or [retrieval_k])),
            "sample_count": float(len(dataset.samples)),
            "train_sample_count": float(len(train_samples)),
            "eval_sample_count": float(len(eval_samples)),
            "user_count": float(len(dataset.user_ids)),
            "item_count": float(len(dataset.segment_ids)),
        }
    )
    return model, metrics_payload


def dataset_with_samples(dataset: Dataset, samples: list[Sample]) -> Dataset:
    return Dataset(
        user_ids=dataset.user_ids,
        segment_ids=dataset.segment_ids,
        segment_video_ids=dataset.segment_video_ids,
        samples=samples,
        item_features=dataset.item_features,
        user_features=dataset.user_features,
    )


def train_backend(
    dataset: Dataset,
    backend: str,
    dim: int,
    epochs: int,
    learning_rate: float,
    seed: int,
    l2: float,
    eval_ratio: float,
    retrieval_ks: list[int],
    batch_size: int = 256,
    random_negatives: int = 2,
    hard_negatives: int = 2,
) -> tuple[TwoTowerModel, dict[str, float]]:
    train_samples, eval_samples = splitting.split_temporal(dataset.samples, eval_ratio=eval_ratio)
    if backend == "torch":
        from two_tower_training import torch_model

        model, backend_metrics = torch_model.train_torch_model(
            dataset=dataset_with_samples(dataset, train_samples),
            dim=dim,
            epochs=epochs,
            learning_rate=learning_rate,
            seed=seed,
            l2=l2,
            batch_size=batch_size,
            random_negatives=random_negatives,
            hard_negatives=hard_negatives,
        )
        metrics = evaluate_split_model(
            dataset=dataset,
            model=model,
            train_samples=train_samples,
            eval_samples=eval_samples,
            dim=dim,
            epochs=epochs,
            learning_rate=learning_rate,
            l2=l2,
            eval_ratio=eval_ratio,
            retrieval_ks=retrieval_ks,
            score_scale=float(backend_metrics.get("score_scale", 1.0)),
        )
        metrics.update(backend_metrics)
        return model, metrics
    if backend == "sgd":
        return train_model(
            dataset,
            dim=dim,
            epochs=epochs,
            learning_rate=learning_rate,
            seed=seed,
            l2=l2,
            eval_ratio=eval_ratio,
            retrieval_ks=retrieval_ks,
        )
    raise ValueError(f"unknown backend: {backend}")


def evaluate(dataset: Dataset, model: TwoTowerModel) -> dict[str, float]:
    losses = []
    positive_scores = []
    negative_scores = []
    pairs = []
    for sample in dataset.samples:
        score = dot(model.user_embeddings[sample.user_index], model.item_embeddings[sample.segment_index])
        loss = weighted_bce(score, sample.label, sample.weight)
        losses.append(loss)
        pairs.append((score, sample.label))
        if sample.label == 1.0:
            positive_scores.append(score)
        else:
            negative_scores.append(score)
    return {
        "loss": sum(losses) / len(losses),
        "auc": auc(pairs),
        "positive_avg_score": average(positive_scores),
        "negative_avg_score": average(negative_scores),
    }


def weighted_bce(score: float, label: float, weight: float) -> float:
    if score >= 0:
        base = score - score * label + math.log1p(math.exp(-score))
    else:
        base = -score * label + math.log1p(math.exp(score))
    return weight * base


def auc(scored_labels: list[tuple[float, float]]) -> float:
    positives = [score for score, label in scored_labels if label == 1.0]
    negatives = [score for score, label in scored_labels if label == 0.0]
    if not positives or not negatives:
        return 0.0
    wins = 0.0
    total = len(positives) * len(negatives)
    for pos in positives:
        for neg in negatives:
            if pos > neg:
                wins += 1.0
            elif pos == neg:
                wins += 0.5
    return wins / total


def average(values: list[float]) -> float:
    if not values:
        return 0.0
    return sum(values) / len(values)


def sigmoid(value: float) -> float:
    if value >= 0:
        z = math.exp(-value)
        return 1.0 / (1.0 + z)
    z = math.exp(value)
    return z / (1.0 + z)


def dot(left: list[float], right: list[float]) -> float:
    return sum(a * b for a, b in zip(left, right))


def normalize_in_place(values: list[float]) -> None:
    norm = math.sqrt(sum(value * value for value in values))
    if norm == 0:
        return
    for i, value in enumerate(values):
        values[i] = value / norm


def write_artifacts(
    output_dir: Path,
    dataset: Dataset,
    model: TwoTowerModel,
    metrics: dict[str, float],
    model_version: str,
) -> None:
    output_dir.mkdir(parents=True, exist_ok=True)
    write_json(output_dir / "user_id_map.json", {str(user_id): idx for idx, user_id in enumerate(dataset.user_ids)})
    write_json(output_dir / "segment_id_map.json", {str(segment_id): idx for idx, segment_id in enumerate(dataset.segment_ids)})
    metrics_payload = dict(metrics)
    metrics_payload["model_version"] = model_version
    write_json(output_dir / "metrics.json", metrics_payload)
    write_user_embeddings(output_dir / "user_embeddings.csv", dataset, model, model_version)
    write_item_embeddings(output_dir / "item_embeddings.csv", dataset, model, model_version)


def write_user_embeddings(path: Path, dataset: Dataset, model: TwoTowerModel, model_version: str) -> None:
    with path.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.writer(handle)
        writer.writerow(["user_id", "embedding", "model_version"])
        for user_id, embedding in zip(dataset.user_ids, model.user_embeddings):
            writer.writerow([user_id, format_embedding(embedding), model_version])


def write_item_embeddings(path: Path, dataset: Dataset, model: TwoTowerModel, model_version: str) -> None:
    with path.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.writer(handle)
        writer.writerow(["video_segment_id", "video_id", "embedding", "model_version"])
        for segment_id, embedding in zip(dataset.segment_ids, model.item_embeddings):
            writer.writerow([segment_id, dataset.segment_video_ids[segment_id], format_embedding(embedding), model_version])


def write_json(path: Path, payload) -> None:
    path.write_text(json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def format_embedding(values: list[float]) -> str:
    return "[" + ",".join(f"{value:.6f}" for value in values) + "]"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Train the first offline two-tower recommendation model.")
    parser.add_argument("--samples", type=Path, default=Path("data/two_tower_samples.csv"))
    parser.add_argument("--item-catalog", type=Path, default=None)
    parser.add_argument("--user-features", type=Path, default=None)
    parser.add_argument("--output", type=Path, default=Path("artifacts/two_tower_v1"))
    parser.add_argument("--model-version", default="two_tower_v1")
    parser.add_argument("--dim", type=int, default=128)
    parser.add_argument("--epochs", type=int, default=20)
    parser.add_argument("--learning-rate", type=float, default=0.05)
    parser.add_argument("--l2", type=float, default=0.0001)
    parser.add_argument("--eval-ratio", type=float, default=0.2)
    parser.add_argument("--retrieval-k", type=int, default=20)
    parser.add_argument("--retrieval-ks", default="")
    parser.add_argument("--half-life-days", type=float, default=30.0)
    parser.add_argument("--backend", choices=["torch", "sgd"], default="torch")
    parser.add_argument("--batch-size", type=int, default=256)
    parser.add_argument("--random-negatives", type=int, default=2)
    parser.add_argument("--hard-negatives", type=int, default=2)
    parser.add_argument("--seed", type=int, default=20260623)
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    retrieval_ks = parse_retrieval_ks(args.retrieval_ks, args.retrieval_k)
    dataset = load_dataset(
        args.samples,
        half_life_days=args.half_life_days,
        item_catalog=args.item_catalog,
        user_features=args.user_features,
    )
    model, metrics = train_backend(
        dataset,
        backend=args.backend,
        dim=args.dim,
        epochs=args.epochs,
        learning_rate=args.learning_rate,
        seed=args.seed,
        l2=args.l2,
        eval_ratio=args.eval_ratio,
        retrieval_ks=retrieval_ks,
        batch_size=args.batch_size,
        random_negatives=args.random_negatives,
        hard_negatives=args.hard_negatives,
    )
    write_artifacts(args.output, dataset, model, metrics, args.model_version)
    print(f"samples={int(metrics['sample_count'])}")
    print(f"users={int(metrics['user_count'])}")
    print(f"items={int(metrics['item_count'])}")
    print(f"loss={metrics['loss']:.6f}")
    print(f"auc={metrics['auc']:.6f}")
    print(f"eval_auc={metrics['eval_auc']:.6f}")
    primary_k = retrieval_ks[0]
    print(f"recall_at_{primary_k}={metrics[f'recall_at_{primary_k}']:.6f}")
    print(f"ndcg_at_{primary_k}={metrics[f'ndcg_at_{primary_k}']:.6f}")
    print(f"coverage_at_{primary_k}={metrics[f'coverage_at_{primary_k}']:.6f}")
    print(f"positive_avg_score={metrics['positive_avg_score']:.6f}")
    print(f"negative_avg_score={metrics['negative_avg_score']:.6f}")
    print(f"output={args.output}")


def parse_retrieval_ks(raw: str, fallback: int) -> list[int]:
    if not raw.strip():
        return [fallback]
    values = []
    for part in raw.split(","):
        value = int(part.strip())
        if value <= 0:
            raise ValueError("retrieval ks must be greater than 0")
        values.append(value)
    return sorted(set(values))


def evaluate_trained_model(
    dataset: Dataset,
    model: TwoTowerModel,
    dim: int,
    epochs: int,
    learning_rate: float,
    l2: float,
    eval_ratio: float,
    retrieval_ks: list[int],
) -> dict[str, float]:
    train_samples, eval_samples = splitting.split_temporal(dataset.samples, eval_ratio=eval_ratio)
    return evaluate_split_model(
        dataset=dataset,
        model=model,
        train_samples=train_samples,
        eval_samples=eval_samples,
        dim=dim,
        epochs=epochs,
        learning_rate=learning_rate,
        l2=l2,
        eval_ratio=eval_ratio,
        retrieval_ks=retrieval_ks,
    )


def evaluate_split_model(
    dataset: Dataset,
    model: TwoTowerModel,
    train_samples: list[Sample],
    eval_samples: list[Sample],
    dim: int,
    epochs: int,
    learning_rate: float,
    l2: float,
    eval_ratio: float,
    retrieval_ks: list[int],
    score_scale: float = 1.0,
) -> dict[str, float]:
    metrics_payload = training_metrics.evaluate_pointwise(model, train_samples, "train", score_scale=score_scale)
    metrics_payload.update(training_metrics.evaluate_pointwise(model, eval_samples, "eval", score_scale=score_scale))
    metrics_payload.update(
        training_metrics.evaluate_retrieval_many(
            dataset=dataset,
            model=model,
            eval_samples=eval_samples,
            train_samples=train_samples,
            ks=retrieval_ks,
        )
    )
    primary_k = retrieval_ks[0]
    metrics_payload.update(
        {
            "loss": metrics_payload["eval_loss"],
            "auc": metrics_payload["eval_auc"],
            "positive_avg_score": metrics_payload["eval_positive_avg_score"],
            "negative_avg_score": metrics_payload["eval_negative_avg_score"],
            "dim": float(dim),
            "epochs": float(epochs),
            "learning_rate": float(learning_rate),
            "l2": float(l2),
            "score_scale": float(score_scale),
            "eval_ratio": float(eval_ratio),
            "retrieval_k": float(primary_k),
            "retrieval_ks": ",".join(str(k) for k in retrieval_ks),
            "sample_count": float(len(dataset.samples)),
            "train_sample_count": float(len(train_samples)),
            "eval_sample_count": float(len(eval_samples)),
            "user_count": float(len(dataset.user_ids)),
            "item_count": float(len(dataset.segment_ids)),
        }
    )
    return metrics_payload


if __name__ == "__main__":
    main()
