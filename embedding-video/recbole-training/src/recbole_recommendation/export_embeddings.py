from __future__ import annotations

import csv
import hashlib
from pathlib import Path
from typing import Iterable, Sequence


def vector_text(values: Sequence[float]) -> str:
    return "[" + ",".join(f"{float(v):.6f}" for v in values) + "]"


def deterministic_vector(key: str, dim: int) -> list[float]:
    values: list[float] = []
    counter = 0
    while len(values) < dim:
        digest = hashlib.sha256(f"{key}:{counter}".encode("utf-8")).digest()
        for byte in digest:
            values.append((byte / 255.0) - 0.5)
            if len(values) == dim:
                break
        counter += 1
    return values


def read_atomic_ids(dataset_dir: str | Path, dataset: str) -> tuple[list[tuple[str, str]], list[str]]:
    root = Path(dataset_dir)
    items: list[tuple[str, str]] = []
    item_path = root / f"{dataset}.item"
    with item_path.open(newline="", encoding="utf-8") as f:
        reader = csv.DictReader(f)
        for row in reader:
            item_id = row.get("item_id:token") or row.get("item_id") or ""
            video_id = row.get("video_id:token") or row.get("video_id") or "0"
            if item_id:
                items.append((item_id, video_id))

    users: list[str] = []
    user_path = root / f"{dataset}.user"
    with user_path.open(newline="", encoding="utf-8") as f:
        reader = csv.DictReader(f)
        for row in reader:
            user_id = row.get("user_id:token") or row.get("user_id") or ""
            if user_id:
                users.append(user_id)
    return items, users


def write_embedding_csvs(
    output_dir: str | Path,
    item_embeddings: Iterable[tuple[str, str, Sequence[float]]],
    user_embeddings: Iterable[tuple[str, Sequence[float]]],
    model_version: str,
) -> None:
    root = Path(output_dir)
    root.mkdir(parents=True, exist_ok=True)
    with (root / "item_embeddings.csv").open("w", newline="", encoding="utf-8") as f:
        writer = csv.writer(f)
        writer.writerow(["video_segment_id", "video_id", "embedding", "model_version"])
        for segment_id, video_id, embedding in item_embeddings:
            writer.writerow([segment_id, video_id, vector_text(embedding), model_version])
    with (root / "user_embeddings.csv").open("w", newline="", encoding="utf-8") as f:
        writer = csv.writer(f)
        writer.writerow(["user_id", "embedding", "model_version"])
        for user_id, embedding in user_embeddings:
            writer.writerow([user_id, vector_text(embedding), model_version])


def export_from_checkpoint(
    model_file: str | Path,
    dataset_dir: str | Path,
    dataset: str,
    output_dir: str | Path,
    model_version: str,
) -> None:
    from recbole.quick_start import load_data_and_model

    _, model, recbole_dataset, _, _, _ = load_data_and_model(model_file=str(model_file))
    user_embedding = embedding_matrix(model, "user_embedding")
    item_embedding = embedding_matrix(model, "item_embedding")
    item_video = dict(read_atomic_ids(dataset_dir, dataset)[0])

    user_rows = []
    for internal_id, vector in enumerate(user_embedding):
        token = recbole_token(recbole_dataset, recbole_dataset.uid_field, internal_id)
        if not usable_token(token):
            continue
        user_rows.append((token, vector))

    item_rows = []
    for internal_id, vector in enumerate(item_embedding):
        token = recbole_token(recbole_dataset, recbole_dataset.iid_field, internal_id)
        if not usable_token(token):
            continue
        item_rows.append((token, item_video.get(token, "0"), vector))

    if not item_rows or not user_rows:
        raise RuntimeError("RecBole checkpoint did not expose usable user/item embeddings")
    write_embedding_csvs(output_dir, item_rows, user_rows, model_version)


def embedding_matrix(model: object, attr: str) -> list[list[float]]:
    layer = getattr(model, attr, None)
    weight = getattr(layer, "weight", None)
    if weight is None:
        raise RuntimeError(f"RecBole model does not expose {attr}.weight")
    values = weight.detach().cpu().numpy().tolist()
    return [[float(v) for v in row] for row in values]


def recbole_token(dataset: object, field: str, internal_id: int) -> str:
    value = dataset.id2token(field, [internal_id])[0]
    if hasattr(value, "item"):
        value = value.item()
    return str(value)


def usable_token(token: str) -> bool:
    token = token.strip()
    if not token or token == "[PAD]":
        return False
    return token.isdigit()


def export_from_atomic_files(
    dataset_dir: str | Path,
    dataset: str,
    output_dir: str | Path,
    model_version: str,
    embedding_size: int,
) -> None:
    items, users = read_atomic_ids(dataset_dir, dataset)
    item_embeddings = (
        (segment_id, video_id, deterministic_vector(f"item:{segment_id}", embedding_size))
        for segment_id, video_id in items
    )
    user_embeddings = (
        (user_id, deterministic_vector(f"user:{user_id}", embedding_size))
        for user_id in users
    )
    write_embedding_csvs(output_dir, item_embeddings, user_embeddings, model_version)
