import hashlib
import math
import random

import torch
from torch import nn
from torch.nn import functional as F

from two_tower_training.train import Dataset, TwoTowerModel


ID_EMBEDDING_SCALE = 0.5
SCORE_SCALE = 8.0
RANDOM_NEGATIVE_WEIGHT = 0.35
HARD_NEGATIVE_WEIGHT = 0.25


class TorchTwoTower(nn.Module):
    def __init__(
        self,
        user_count: int,
        item_count: int,
        dim: int,
        user_feature_dim: int,
        item_feature_dim: int,
        id_embedding_scale: float = ID_EMBEDDING_SCALE,
    ):
        super().__init__()
        self.id_embedding_scale = id_embedding_scale
        self.user_embedding = nn.Embedding(user_count, dim)
        self.item_embedding = nn.Embedding(item_count, dim)
        self.user_feature_mlp = nn.Sequential(
            nn.Linear(user_feature_dim, dim),
            nn.ReLU(),
            nn.Linear(dim, dim),
        )
        self.item_feature_mlp = nn.Sequential(
            nn.Linear(item_feature_dim, dim),
            nn.ReLU(),
            nn.Linear(dim, dim),
        )

    def user_vector(self, user_index: torch.Tensor, user_features: torch.Tensor) -> torch.Tensor:
        id_vec = self.user_embedding(user_index) * self.id_embedding_scale
        feature_vec = self.user_feature_mlp(user_features)
        return F.normalize(id_vec + feature_vec, dim=-1)

    def item_vector(self, item_index: torch.Tensor, item_features: torch.Tensor) -> torch.Tensor:
        id_vec = self.item_embedding(item_index) * self.id_embedding_scale
        feature_vec = self.item_feature_mlp(item_features)
        return F.normalize(id_vec + feature_vec, dim=-1)


def train_torch_model(
    dataset: Dataset,
    dim: int,
    epochs: int,
    learning_rate: float,
    seed: int,
    l2: float,
    batch_size: int = 256,
    random_negatives: int = 2,
    hard_negatives: int = 2,
) -> tuple[TwoTowerModel, dict[str, float]]:
    if dim <= 0:
        raise ValueError("dim must be greater than 0")
    if epochs <= 0:
        raise ValueError("epochs must be greater than 0")
    torch.manual_seed(seed)
    rng = random.Random(seed)

    user_feature_matrix = build_user_feature_matrix(dataset)
    item_feature_matrix = build_item_feature_matrix(dataset)
    user_features = torch.tensor(user_feature_matrix, dtype=torch.float32)
    item_features = torch.tensor(item_feature_matrix, dtype=torch.float32)
    model = TorchTwoTower(
        len(dataset.user_ids),
        len(dataset.segment_ids),
        dim,
        len(user_feature_matrix[0]),
        len(item_feature_matrix[0]),
    )
    optimizer = torch.optim.AdamW(model.parameters(), lr=learning_rate, weight_decay=l2)
    samples = list(dataset.samples)
    losses: list[float] = []
    all_item_indexes = list(range(len(dataset.segment_ids)))
    known_positives_by_user = known_positive_items_by_user(samples)

    for _ in range(epochs):
        rng.shuffle(samples)
        for start in range(0, len(samples), max(1, batch_size)):
            batch = samples[start : start + batch_size]
            pairs = training_pairs(
                batch,
                all_item_indexes,
                rng,
                random_negatives,
                hard_negatives,
                known_positives_by_user,
            )
            if not pairs:
                continue
            user_index = torch.tensor([pair[0] for pair in pairs], dtype=torch.long)
            positive_item_index = torch.tensor([pair[1] for pair in pairs], dtype=torch.long)
            negative_item_index = torch.tensor([pair[2] for pair in pairs], dtype=torch.long)
            weights = torch.tensor([pair[3] for pair in pairs], dtype=torch.float32)
            user_vec = model.user_vector(user_index, user_features[user_index])
            positive_item_vec = model.item_vector(positive_item_index, item_features[positive_item_index])
            negative_item_vec = model.item_vector(negative_item_index, item_features[negative_item_index])
            positive_score = (user_vec * positive_item_vec).sum(dim=1)
            negative_score = (user_vec * negative_item_vec).sum(dim=1)
            loss = (F.softplus(-(positive_score - negative_score) * SCORE_SCALE) * weights).mean()
            optimizer.zero_grad()
            loss.backward()
            optimizer.step()
            losses.append(float(loss.detach().item()))

    with torch.no_grad():
        user_indexes = torch.arange(len(dataset.user_ids), dtype=torch.long)
        item_indexes = torch.arange(len(dataset.segment_ids), dtype=torch.long)
        user_embeddings = model.user_vector(user_indexes, user_features[user_indexes]).cpu().tolist()
        item_embeddings = model.item_vector(item_indexes, item_features[item_indexes]).cpu().tolist()

    return TwoTowerModel(user_embeddings=user_embeddings, item_embeddings=item_embeddings), {
        "backend": "torch",
        "loss_objective": "pairwise_softplus",
        "score_scale": SCORE_SCALE,
        "id_embedding_scale": ID_EMBEDDING_SCALE,
        "torch_train_loss": sum(losses) / len(losses) if losses else 0.0,
        "batch_size": float(batch_size),
        "random_negatives": float(random_negatives),
        "hard_negatives": float(hard_negatives),
    }


def known_positive_items_by_user(samples: list) -> dict[int, set[int]]:
    positives: dict[int, set[int]] = {}
    for sample in samples:
        if sample.label == 1.0:
            positives.setdefault(sample.user_index, set()).add(sample.segment_index)
    return positives


def known_negative_items_by_user(samples: list) -> dict[int, list[int]]:
    negatives: dict[int, list[int]] = {}
    for sample in samples:
        if sample.label == 0.0:
            negatives.setdefault(sample.user_index, []).append(sample.segment_index)
    return negatives


def training_pairs(
    batch: list,
    all_item_indexes: list[int],
    rng: random.Random,
    random_negatives: int,
    hard_negatives: int,
    known_positives_by_user: dict[int, set[int]],
) -> list[tuple[int, int, int, float]]:
    pairs: list[tuple[int, int, int, float]] = []
    batch_positive_items = [sample.segment_index for sample in batch if sample.label == 1.0]
    batch_negatives_by_user = known_negative_items_by_user(batch)
    for sample in batch:
        if sample.label != 1.0:
            continue
        blocked = known_positives_by_user.get(sample.user_index, set())
        used_negative_items: set[int] = set()
        for item_index in batch_negatives_by_user.get(sample.user_index, []):
            if item_index in blocked or item_index in used_negative_items:
                continue
            pairs.append((sample.user_index, sample.segment_index, item_index, sample.weight))
            used_negative_items.add(item_index)

        negative_candidates = [
            idx
            for idx in all_item_indexes
            if idx != sample.segment_index and idx not in blocked and idx not in used_negative_items
        ]
        rng.shuffle(negative_candidates)
        for item_index in negative_candidates[: max(0, random_negatives)]:
            pairs.append((sample.user_index, sample.segment_index, item_index, RANDOM_NEGATIVE_WEIGHT * sample.weight))
            used_negative_items.add(item_index)
        hard_added = 0
        for item_index in batch_positive_items:
            if item_index == sample.segment_index or item_index in blocked or item_index in used_negative_items:
                continue
            pairs.append((sample.user_index, sample.segment_index, item_index, HARD_NEGATIVE_WEIGHT * sample.weight))
            used_negative_items.add(item_index)
            hard_added += 1
            if hard_added >= hard_negatives:
                break
    return pairs


def training_triples(batch: list, all_item_indexes: list[int], rng: random.Random, random_negatives: int, hard_negatives: int) -> list[tuple[int, int, float, float]]:
    known_positives = known_positive_items_by_user(batch)
    pairs = training_pairs(batch, all_item_indexes, rng, random_negatives, hard_negatives, known_positives)
    triples: list[tuple[int, int, float, float]] = []
    for sample in batch:
        triples.append((sample.user_index, sample.segment_index, sample.label, sample.weight))
    for user_index, _positive_item, negative_item, weight in pairs:
        triples.append((user_index, negative_item, 0.0, weight))
    return triples


def build_item_feature_matrix(dataset: Dataset) -> list[list[float]]:
    return [item_feature_vector(dataset.item_features.get(segment_id) if dataset.item_features else None) for segment_id in dataset.segment_ids]


def build_user_feature_matrix(dataset: Dataset) -> list[list[float]]:
    return [user_feature_vector(dataset.user_features.get(user_id) if dataset.user_features else None) for user_id in dataset.user_ids]


def user_feature_vector(feature) -> list[float]:
    if feature is None:
        return [0.0] * 27
    text = " ".join(
        [
            feature.recent_knowledge_point_ids,
            feature.recent_subjects,
            feature.question_search_knowledge_text,
            feature.generated_feedback_knowledge_text,
        ]
    )
    buckets = text_buckets(text, bucket_count=8)
    numeric = [
        capped(feature.grade_id / 20.0),
        capped(feature.class_id / 100.0),
        capped(feature.user_type / 10.0),
        capped(feature.mastery_avg),
        capped(feature.mastery_min),
        log_scaled_count(feature.weak_knowledge_count),
        log_scaled_count(feature.strong_knowledge_count),
        log_scaled_count(feature.knowledge_correct_count),
        log_scaled_count(feature.knowledge_incorrect_count),
        log_scaled_count(feature.answer_count),
        log_scaled_count(feature.answer_correct_count),
        log_scaled_count(feature.answer_incorrect_count),
        capped(feature.avg_score_rate),
        min(1.0, math.log1p(max(0.0, feature.avg_cost_seconds)) / 8.0),
        log_scaled_count(feature.question_feedback_count),
        log_scaled_count(feature.generated_feedback_count),
        log_scaled_count(feature.generated_correct_count),
        capped(feature.generated_avg_score_rate),
        log_scaled_count(feature.question_search_count),
    ]
    return buckets + numeric


def item_feature_vector(feature) -> list[float]:
    if feature is None:
        return [0.0] * 16
    text = " ".join([feature.content_summary, feature.knowledge_tags, feature.video_title])
    buckets = text_buckets(text, bucket_count=8)
    numeric = [
        math.log1p(feature.segment_duration) / 10.0,
        math.log1p(feature.video_duration) / 10.0,
        math.log1p(feature.like_count) / 5.0,
        math.log1p(feature.double_like_count) / 5.0,
        math.log1p(feature.dislike_count) / 5.0,
        min(1.0, feature.segment_duration / max(1, feature.video_duration)) if feature.video_duration > 0 else 0.0,
        1.0 if feature.content_summary.strip() else 0.0,
        1.0 if feature.knowledge_tags.strip() else 0.0,
    ]
    return buckets + numeric


def text_buckets(text: str, bucket_count: int) -> list[float]:
    buckets = [0.0] * bucket_count
    for token in text.replace("|", " ").replace(",", " ").split():
        digest = hashlib.sha1(token.encode("utf-8")).digest()
        buckets[digest[0] % len(buckets)] += 1.0
    norm = math.sqrt(sum(value * value for value in buckets))
    if norm > 0:
        buckets = [value / norm for value in buckets]
    return buckets


def log_scaled_count(value: int) -> float:
    return min(1.0, math.log1p(max(0, value)) / 10.0)


def capped(value: float) -> float:
    return min(1.0, max(0.0, value))
