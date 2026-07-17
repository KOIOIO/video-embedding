import sys
import types
import unittest
from unittest import mock

from recbole_recommendation import train


class RecBoleTrainTest(unittest.TestCase):
    def test_hides_wrapper_args_from_recbole_and_restores_them(self) -> None:
        seen_argv: list[list[str]] = []

        def fake_run_recbole(**_kwargs):
            seen_argv.append(sys.argv.copy())
            return {}

        fake_quick_start = types.ModuleType("recbole.quick_start")
        fake_quick_start.run_recbole = fake_run_recbole
        original_quick_start = sys.modules.get("recbole.quick_start")
        original_argv = sys.argv
        wrapper_argv = ["train.py", "--dataset-dir", "data/example", "--epochs", "20"]
        sys.modules["recbole.quick_start"] = fake_quick_start
        sys.argv = wrapper_argv
        try:
            with mock.patch.object(train, "allow_trusted_torch_checkpoint_loads"):
                train.run_recbole_training(
                    types.SimpleNamespace(model="BPR", dataset="video_dataset"),
                    {"epochs": 20},
                )
            self.assertEqual(seen_argv, [["train.py"]])
            self.assertIs(sys.argv, wrapper_argv)
        finally:
            sys.argv = original_argv
            if original_quick_start is None:
                sys.modules.pop("recbole.quick_start", None)
            else:
                sys.modules["recbole.quick_start"] = original_quick_start

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
