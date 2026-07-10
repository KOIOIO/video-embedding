import sys
import types
import unittest

from recbole_recommendation import train


class RecBoleTrainTest(unittest.TestCase):
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
