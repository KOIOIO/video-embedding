from pathlib import Path
import unittest


class RecBolePipelineScriptTest(unittest.TestCase):
    def test_script_uses_recbole_pipeline_commands(self) -> None:
        script = Path("scripts/run_recbole_pipeline.sh").read_text(encoding="utf-8")
        for fragment in [
            '"${EXPORT_DATASET_COMMAND[@]}"',
            '"${EXPORT_METRICS_COMMAND[@]}"',
            '"${PYTHON_BIN}" -m recbole_recommendation.train',
            '"${PYTHON_BIN}" -m recbole_recommendation.publish_gate',
            '"${IMPORT_EMBEDDINGS_COMMAND[@]}"',
        ]:
            self.assertIn(fragment, script)

    def test_script_prefers_prebuilt_go_tools(self) -> None:
        script = Path("scripts/run_recbole_pipeline.sh").read_text(encoding="utf-8")
        for tool in [
            "export_recbole_dataset",
            "export_active_recsys_model_metrics",
            "import_recsys_embeddings",
        ]:
            self.assertIn(f'command -v {tool}', script)
            self.assertIn(f'go run ./tools/{tool}', script)

    def test_production_image_contains_prebuilt_go_tools(self) -> None:
        dockerfile = Path("Dockerfile.prod").read_text(encoding="utf-8")
        for tool in [
            "recboletrainer",
            "export_recbole_dataset",
            "export_active_recsys_model_metrics",
            "import_recsys_embeddings",
        ]:
            self.assertIn(f"-o /out/{tool}", dockerfile)
            self.assertIn(f"/out/{tool} /usr/local/bin/{tool}", dockerfile)
        self.assertNotIn("COPY --from=go-build /usr/local/go", dockerfile)

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
