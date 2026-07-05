import math


def dot(left: list[float], right: list[float]) -> float:
    return sum(a * b for a, b in zip(left, right))


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


def evaluate_pointwise(model, samples: list, split_name: str, score_scale: float = 1.0) -> dict[str, float]:
    losses = []
    pairs = []
    positives = []
    negatives = []
    for sample in samples:
        score = dot(model.user_embeddings[sample.user_index], model.item_embeddings[sample.segment_index]) * score_scale
        losses.append(weighted_bce(score, sample.label, sample.weight))
        pairs.append((score, sample.label))
        if sample.label == 1.0:
            positives.append(score)
        else:
            negatives.append(score)
    return {
        f"{split_name}_loss": average(losses),
        f"{split_name}_auc": auc(pairs),
        f"{split_name}_positive_avg_score": average(positives),
        f"{split_name}_negative_avg_score": average(negatives),
    }


def evaluate_retrieval(dataset, model, eval_samples: list, train_samples: list, k: int) -> dict[str, float]:
    if k <= 0:
        raise ValueError("k must be greater than 0")
    eval_positives: dict[int, set[int]] = {}
    eval_negatives: dict[int, set[int]] = {}
    eval_dislikes: dict[int, set[int]] = {}
    train_seen_positives: dict[int, set[int]] = {}

    for sample in eval_samples:
        if sample.label == 1.0:
            eval_positives.setdefault(sample.user_index, set()).add(sample.segment_index)
        else:
            eval_negatives.setdefault(sample.user_index, set()).add(sample.segment_index)
            if "dislike" in getattr(sample, "reason", ""):
                eval_dislikes.setdefault(sample.user_index, set()).add(sample.segment_index)
    for sample in train_samples:
        if sample.label == 1.0:
            train_seen_positives.setdefault(sample.user_index, set()).add(sample.segment_index)

    users = sorted(eval_positives)
    if not users:
        return {
            f"recall_at_{k}": 0.0,
            f"hit_rate_at_{k}": 0.0,
            f"ndcg_at_{k}": 0.0,
            f"coverage_at_{k}": 0.0,
            f"negative_hit_rate_at_{k}": 0.0,
            f"dislike_hit_rate_at_{k}": 0.0,
        }

    recall_values = []
    hit_values = []
    ndcg_values = []
    recommended_items: set[int] = set()
    negative_hits = 0
    negative_total = sum(len(values) for values in eval_negatives.values())
    dislike_hits = 0
    dislike_total = sum(len(values) for values in eval_dislikes.values())

    for user_index in users:
        positive_items = eval_positives[user_index]
        seen = train_seen_positives.get(user_index, set())
        ranked = rank_items_for_user(model, user_index, seen, k)
        recommended_items.update(ranked)
        hit_count = len(positive_items.intersection(ranked))
        recall_values.append(hit_count / len(positive_items))
        hit_values.append(1.0 if hit_count > 0 else 0.0)
        ndcg_values.append(ndcg_for_ranked_items(ranked, positive_items))
        for item_index in eval_negatives.get(user_index, set()):
            if item_index in ranked:
                negative_hits += 1
        for item_index in eval_dislikes.get(user_index, set()):
            if item_index in ranked:
                dislike_hits += 1

    return {
        f"recall_at_{k}": average(recall_values),
        f"hit_rate_at_{k}": average(hit_values),
        f"ndcg_at_{k}": average(ndcg_values),
        f"coverage_at_{k}": len(recommended_items) / max(1, len(dataset.segment_ids)),
        f"negative_hit_rate_at_{k}": negative_hits / negative_total if negative_total else 0.0,
        f"dislike_hit_rate_at_{k}": dislike_hits / dislike_total if dislike_total else 0.0,
    }


def evaluate_retrieval_many(dataset, model, eval_samples: list, train_samples: list, ks: list[int]) -> dict[str, float]:
    merged: dict[str, float] = {}
    for k in sorted(set(ks)):
        merged.update(evaluate_retrieval(dataset, model, eval_samples, train_samples, k))
    return merged


def rank_items_for_user(model, user_index: int, seen_item_indexes: set[int], k: int) -> list[int]:
    user_vec = model.user_embeddings[user_index]
    scored = []
    for item_index, item_vec in enumerate(model.item_embeddings):
        if item_index in seen_item_indexes:
            continue
        scored.append((dot(user_vec, item_vec), item_index))
    scored.sort(key=lambda item: (-item[0], item[1]))
    return [item_index for _, item_index in scored[:k]]


def ndcg_for_ranked_items(ranked_items: list[int], positive_items: set[int]) -> float:
    dcg = 0.0
    for rank, item_index in enumerate(ranked_items, start=1):
        if item_index in positive_items:
            dcg += 1.0 / math.log2(rank + 1)
    ideal_hits = min(len(positive_items), len(ranked_items))
    if ideal_hits == 0:
        return 0.0
    ideal = sum(1.0 / math.log2(rank + 1) for rank in range(1, ideal_hits + 1))
    return dcg / ideal
