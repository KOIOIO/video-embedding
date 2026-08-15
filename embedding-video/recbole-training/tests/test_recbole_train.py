import sys
import types
import unittest
from unittest import mock

from recbole_recommendation import train


class RecBoleTrainTest(unittest.TestCase):
    def test_hides_wrapper_args_from_recbole_and_restores_them(self) -> None:
        seen_argv: list[list[str]] = []
        class Config(dict):
            def __init__(self, **_kwargs):
                super().__init__(seed=1, reproducibility=True, model="BPR", MODEL_TYPE="GENERAL", device="cpu")
                seen_argv.append(sys.argv.copy())
        class Dataset:
            iid_field = "item_id"
            field2id_token = {"item_id": ["[PAD]", "101", "knowledge_video:88"]}
        class Model:
            def __init__(self, *_args): pass
            def to(self, _device): return self
            def full_sort_predict(self, _interaction): return [[0.0, 0.2, 0.9]]
        class Trainer:
            saved_model_file = "model.pth"
            def __init__(self, _config, model): self.model = model
            def fit(self, *_args, **_kwargs):
                self.assert_masked()
                return 0, {}
            def evaluate(self, *_args, **_kwargs):
                self.assert_masked()
                return {}
            def assert_masked(self):
                self_score = self.model.full_sort_predict(None)[0][2]
                if self_score != float("-inf"): raise AssertionError(self_score)
        dataset = Dataset()
        fake_modules = {
            "recbole": types.ModuleType("recbole"),
            "recbole.config": types.SimpleNamespace(Config=Config),
            "recbole.data": types.SimpleNamespace(create_dataset=lambda _cfg: dataset, data_preparation=lambda _cfg, ds: (types.SimpleNamespace(dataset=ds), object(), object())),
            "recbole.utils": types.SimpleNamespace(init_seed=lambda *_args: None, get_model=lambda _name: Model, get_trainer=lambda *_args: Trainer),
        }
        originals = {name: sys.modules.get(name) for name in fake_modules}
        original_argv = sys.argv
        wrapper_argv = ["train.py", "--dataset-dir", "data/example", "--epochs", "20"]
        sys.modules.update(fake_modules)
        sys.argv = wrapper_argv
        try:
            with mock.patch.object(train, "allow_trusted_torch_checkpoint_loads"):
                train.run_recbole_training(
                    types.SimpleNamespace(model="BPR", dataset="video_app"),
                    {"epochs": 20},
                )
            self.assertEqual(seen_argv, [["train.py"]])
            self.assertIs(sys.argv, wrapper_argv)
        finally:
            sys.argv = original_argv
            for name, original in originals.items():
                if original is None: sys.modules.pop(name, None)
                else: sys.modules[name] = original

    def test_allows_trusted_torch_checkpoint_loads_by_default(self) -> None:
        calls: list[dict] = []

        def fake_load(*_args, **kwargs):
            calls.append(kwargs)
            return {}

        fake_torch = types.SimpleNamespace(load=fake_load)
        original_torch = sys.modules.get("torch")
        sys.modules["torch"] = fake_torch
        try:
            train.allow_trusted_torch_checkpoint_loads()
            fake_torch.load("checkpoint.pth")
            fake_torch.load("checkpoint.pth", weights_only=True)
        finally:
            if original_torch is None:
                sys.modules.pop("torch", None)
            else:
                sys.modules["torch"] = original_torch

        self.assertEqual(calls[0]["weights_only"], False)
        self.assertEqual(calls[1]["weights_only"], True)


if __name__ == "__main__":
    unittest.main()
