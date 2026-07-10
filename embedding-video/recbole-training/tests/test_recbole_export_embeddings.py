import csv
from pathlib import Path
import tempfile
import unittest

from recbole_recommendation import export_embeddings


class RecBoleExportEmbeddingsTest(unittest.TestCase):
    def test_writes_embedding_csv_contract(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            export_embeddings.write_embedding_csvs(
                tmp,
                [("101", "11", [0.1, 0.2])],
                [("7", [0.3, 0.4])],
                "recbole_v1",
            )
            with (Path(tmp) / "item_embeddings.csv").open() as f:
                item_rows = list(csv.reader(f))
            with (Path(tmp) / "user_embeddings.csv").open() as f:
                user_rows = list(csv.reader(f))

        self.assertEqual(item_rows[0], ["video_segment_id", "video_id", "embedding", "model_version"])
        self.assertEqual(item_rows[1], ["101", "11", "[0.100000,0.200000]", "recbole_v1"])
        self.assertEqual(user_rows[0], ["user_id", "embedding", "model_version"])
        self.assertEqual(user_rows[1], ["7", "[0.300000,0.400000]", "recbole_v1"])

    def test_export_from_atomic_files_uses_atomic_ids(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            data = root / "data"
            out = root / "artifacts"
            data.mkdir()
            (data / "hengshui_video.item").write_text(
                "item_id:token,video_id:token\n101,11\n", encoding="utf-8"
            )
            (data / "hengshui_video.user").write_text(
                "user_id:token,grade_id:token\n7,10\n", encoding="utf-8"
            )
            export_embeddings.export_from_atomic_files(data, "hengshui_video", out, "recbole_v1", 2)
            with (out / "item_embeddings.csv").open() as f:
                item_rows = list(csv.DictReader(f))
            with (out / "user_embeddings.csv").open() as f:
                user_rows = list(csv.DictReader(f))

        self.assertEqual(item_rows[0]["video_segment_id"], "101")
        self.assertEqual(item_rows[0]["video_id"], "11")
        self.assertEqual(user_rows[0]["user_id"], "7")


if __name__ == "__main__":
    unittest.main()
