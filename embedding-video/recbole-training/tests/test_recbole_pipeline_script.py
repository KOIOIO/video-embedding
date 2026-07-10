from pathlib import Path
import unittest


class RecBolePipelineScriptTest(unittest.TestCase):
    def test_script_uses_recbole_pipeline_commands(self) -> None:
        script = Path("scripts/run_recbole_pipeline.sh").read_text(encoding="utf-8")
        for fragment in [
            "go run ./tools/export_recbole_dataset",
            "go run ./tools/export_active_recsys_model_metrics",
            '"${PYTHON_BIN}" -m recbole_recommendation.train',
            '"${PYTHON_BIN}" -m recbole_recommendation.publish_gate',
            "go run ./tools/import_recsys_embeddings",
        ]:
            self.assertIn(fragment, script)

    def test_script_does_not_reference_legacy_commands(self) -> None:
        script = Path("scripts/run_recbole_pipeline.sh").read_text(encoding="utf-8")
        for legacy in [
            "export_" + "two_" + "tower_samples",
            "two_" + "tower_training.train",
            "import_" + "two_" + "tower_embeddings",
            "run_" + "two_" + "tower_pipeline",
        ]:
            self.assertNotIn(legacy, script)

    def test_script_exports_atomic_files_under_dataset_directory(self) -> None:
        script = Path("scripts/run_recbole_pipeline.sh").read_text(encoding="utf-8")
        self.assertIn('DATA_ROOT="${DATA_ROOT:-${TRAINING_DIR}/data/${MODEL_VERSION}}"', script)
        self.assertIn('DATA_DIR="${DATA_DIR:-${DATA_ROOT}/${DATASET}}"', script)

    def test_script_prefers_project_virtualenv_python(self) -> None:
        script = Path("scripts/run_recbole_pipeline.sh").read_text(encoding="utf-8")
        self.assertIn('VENV_PYTHON="${TRAINING_DIR}/.venv/bin/python"', script)
        self.assertIn('PYTHON_BIN="${PYTHON_BIN:-}"', script)
        self.assertIn('if [[ -z "${PYTHON_BIN}" && -x "${VENV_PYTHON}" ]]; then', script)
        self.assertIn('PYTHON_BIN="${VENV_PYTHON}"', script)
        self.assertIn('PYTHON_BIN="${PYTHON_BIN:-python3}"', script)


if __name__ == "__main__":
    unittest.main()
