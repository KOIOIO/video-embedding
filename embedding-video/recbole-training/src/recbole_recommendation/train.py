from __future__ import annotations

import argparse
import sys
from pathlib import Path

from . import config as config_builder
from . import export_embeddings
from . import metrics as metrics_io
from .virtual_items import install_full_sort_mask


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Train a RecBole model and export embeddings.")
    parser.add_argument("--dataset-dir", required=True)
    parser.add_argument("--dataset", default="video_app")
    parser.add_argument("--output", required=True)
    parser.add_argument("--model-version", required=True)
    parser.add_argument("--model", default="BPR")
    parser.add_argument("--embedding-size", type=int, default=64)
    parser.add_argument("--epochs", type=int, default=20)
    return parser.parse_args()


def allow_trusted_torch_checkpoint_loads() -> None:
    import torch

    original_load = torch.load
    if getattr(original_load, "_recbole_weights_only_patched", False):
        return

    def load_with_trusted_default(*args, **kwargs):
        kwargs.setdefault("weights_only", False)
        return original_load(*args, **kwargs)

    load_with_trusted_default._recbole_weights_only_patched = True
    torch.load = load_with_trusted_default


def run_recbole_training(args: argparse.Namespace, cfg: dict) -> dict:
    allow_trusted_torch_checkpoint_loads()
    try:
        from recbole.config import Config
        from recbole.data import create_dataset, data_preparation
        from recbole.utils import get_model, get_trainer, init_seed
    except ImportError as exc:
        raise RuntimeError("RecBole is not installed; install requirements.txt in recbole-training") from exc

    original_argv = sys.argv
    try:
        sys.argv = [original_argv[0]]
        config = Config(model=args.model, dataset=args.dataset, config_dict=cfg)
        init_seed(config["seed"], config["reproducibility"])
        dataset = create_dataset(config)
        train_data, valid_data, test_data = data_preparation(config, dataset)
        model = get_model(config["model"])(config, train_data.dataset).to(config["device"])
        install_full_sort_mask(model, dataset)
        trainer = get_trainer(config["MODEL_TYPE"], config["model"])(config, model)
        _, best_valid_result = trainer.fit(train_data, valid_data, saved=True, show_progress=False)
        result = dict(trainer.evaluate(test_data, load_best_model=True, show_progress=False))
        result.update(best_valid_result or {})
        result["model_file"] = trainer.saved_model_file
    finally:
        sys.argv = original_argv
    if isinstance(result, dict):
        return result
    return {}


def train_and_export(args: argparse.Namespace) -> None:
    output = Path(args.output)
    output.mkdir(parents=True, exist_ok=True)
    cfg = config_builder.build_config(args.dataset_dir, args.dataset, args.model, args.embedding_size)
    cfg["epochs"] = args.epochs
    cfg["checkpoint_dir"] = str(output / "checkpoints")
    config_builder.write_config(output / "recbole_config.yaml", cfg)

    raw_metrics = run_recbole_training(args, cfg)
    normalized = metrics_io.normalize_metrics(raw_metrics, args.model_version, args.model)
    if "Recall@20" not in normalized:
        normalized["Recall@20"] = float(raw_metrics.get("recall@20", raw_metrics.get("Recall@20", 0.0)))
    if "NDCG@20" not in normalized:
        normalized["NDCG@20"] = float(raw_metrics.get("ndcg@20", raw_metrics.get("NDCG@20", 0.0)))
    stats_path = Path(args.dataset_dir) / f"{args.dataset}.export_stats.json"
    normalized = metrics_io.merge_export_stats(normalized, stats_path)
    items, _ = export_embeddings.read_atomic_ids(args.dataset_dir, args.dataset)
    normalized["filtered_virtual_item_embeddings"] = sum(
        1 for item_id, _ in items if not export_embeddings.usable_token(item_id)
    )
    metrics_io.write_metrics(output / "metrics.json", normalized)

    checkpoint = checkpoint_from_result(raw_metrics) or latest_checkpoint(Path(cfg["checkpoint_dir"]))
    if checkpoint is None:
        raise RuntimeError("RecBole training finished without a checkpoint; cannot export embeddings")
    export_embeddings.export_from_checkpoint(
        checkpoint,
        args.dataset_dir,
        args.dataset,
        output,
        args.model_version,
    )


def checkpoint_from_result(result: dict) -> str | None:
    for key in ("model_file", "checkpoint", "checkpoint_path"):
        value = result.get(key)
        if isinstance(value, str) and value.strip():
            return value
    return None


def latest_checkpoint(checkpoint_dir: Path) -> Path | None:
    if not checkpoint_dir.exists():
        return None
    files = sorted(checkpoint_dir.glob("*.pth"), key=lambda p: p.stat().st_mtime, reverse=True)
    if not files:
        return None
    return files[0]


def main() -> None:
    train_and_export(parse_args())


if __name__ == "__main__":
    main()
