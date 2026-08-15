#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TRAINING_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${TRAINING_DIR}/.." && pwd)"
SERVICE_DIR="${SERVICE_DIR:-${REPO_ROOT}/video-service}"

MODEL_VERSION="${MODEL_VERSION:-recbole_$(date +%Y%m%d_%H%M%S)}"
MODEL_NAME="${MODEL_NAME:-recbole}"
RECBOLE_MODEL="${RECBOLE_MODEL:-BPR}"
DATASET="${DATASET:-video_app}"
DIM="${DIM:-64}"
EPOCHS="${EPOCHS:-20}"
SAMPLE_LIMIT="${SAMPLE_LIMIT:-10000}"
DAYS_BACK="${DAYS_BACK:-30}"
PUBLISH_GATE_ENABLED="${PUBLISH_GATE_ENABLED:-true}"
CONFIG_FILE="${CONFIG_FILE:-${SERVICE_DIR}/configs/video.yml}"
VENV_PYTHON="${TRAINING_DIR}/.venv/bin/python"
PYTHON_BIN="${PYTHON_BIN:-}"
if [[ -z "${PYTHON_BIN}" && -x "${VENV_PYTHON}" ]]; then
  PYTHON_BIN="${VENV_PYTHON}"
fi
PYTHON_BIN="${PYTHON_BIN:-python3}"

if command -v export_recbole_dataset >/dev/null 2>&1; then
  EXPORT_DATASET_COMMAND=(export_recbole_dataset)
else
  EXPORT_DATASET_COMMAND=(go run ./tools/export_recbole_dataset)
fi
if command -v export_active_recsys_model_metrics >/dev/null 2>&1; then
  EXPORT_METRICS_COMMAND=(export_active_recsys_model_metrics)
else
  EXPORT_METRICS_COMMAND=(go run ./tools/export_active_recsys_model_metrics)
fi
if command -v import_recsys_embeddings >/dev/null 2>&1; then
  IMPORT_EMBEDDINGS_COMMAND=(import_recsys_embeddings)
else
  IMPORT_EMBEDDINGS_COMMAND=(go run ./tools/import_recsys_embeddings)
fi

DATA_ROOT="${DATA_ROOT:-${TRAINING_DIR}/data/${MODEL_VERSION}}"
DATA_DIR="${DATA_DIR:-${DATA_ROOT}/${DATASET}}"
ARTIFACT_DIR="${ARTIFACT_DIR:-${TRAINING_DIR}/artifacts/${MODEL_VERSION}}"
BASELINE_METRICS="${BASELINE_METRICS:-${ARTIFACT_DIR}/baseline_metrics.json}"

mkdir -p "${DATA_DIR}" "${ARTIFACT_DIR}"

(
  cd "${SERVICE_DIR}"
  "${EXPORT_DATASET_COMMAND[@]}" \
    --config "${CONFIG_FILE}" \
    --output-dir "${DATA_DIR}" \
    --dataset "${DATASET}" \
    --limit "${SAMPLE_LIMIT}" \
    --days-back "${DAYS_BACK}"
)

for split in train valid test; do
  split_file="${DATA_DIR}/${DATASET}.${split}.inter"
  if [[ ! -f "${split_file}" ]]; then
    echo "missing benchmark split: ${split_file}" >&2
    exit 1
  fi
done
if grep -q "knowledge_video:" "${DATA_DIR}/${DATASET}.valid.inter" "${DATA_DIR}/${DATASET}.test.inter"; then
  echo "virtual item leaked into validation/test targets" >&2
  exit 1
fi

(
  cd "${SERVICE_DIR}"
  "${EXPORT_METRICS_COMMAND[@]}" \
    --config "${CONFIG_FILE}" \
    --model "${MODEL_NAME}" \
    --output "${BASELINE_METRICS}"
)

(
  cd "${TRAINING_DIR}"
  PYTHONPATH=src "${PYTHON_BIN}" -m recbole_recommendation.train \
    --dataset-dir "${DATA_DIR}" \
    --dataset "${DATASET}" \
    --output "${ARTIFACT_DIR}" \
    --model-version "${MODEL_VERSION}" \
    --model "${RECBOLE_MODEL}" \
    --embedding-size "${DIM}" \
    --epochs "${EPOCHS}"
)

if [[ "${PUBLISH_GATE_ENABLED}" == "true" ]]; then
  (
    cd "${TRAINING_DIR}"
    PYTHONPATH=src "${PYTHON_BIN}" -m recbole_recommendation.publish_gate \
      --metrics "${ARTIFACT_DIR}/metrics.json" \
      --baseline "${BASELINE_METRICS}"
  )
fi

ITEM_EMBEDDINGS="${ARTIFACT_DIR}/item_embeddings.csv"
if [[ ! -s "${ITEM_EMBEDDINGS}" ]]; then
  echo "missing item embeddings: ${ITEM_EMBEDDINGS}" >&2
  exit 1
fi
if ! awk -F, 'NR == 1 { next } $1 !~ /^[1-9][0-9]*$/ { exit 1 }' "${ITEM_EMBEDDINGS}"; then
  echo "item embeddings contain a nonnumeric or virtual item id" >&2
  exit 1
fi

(
  cd "${SERVICE_DIR}"
  "${IMPORT_EMBEDDINGS_COMMAND[@]}" \
    --config "${CONFIG_FILE}" \
    --artifact-dir "${ARTIFACT_DIR}" \
    --model "${MODEL_NAME}" \
    --dim "${DIM}" \
    --publish
)
