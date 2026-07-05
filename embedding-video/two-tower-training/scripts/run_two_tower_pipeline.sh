#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
HTTP_DIR="${ROOT_DIR}/video-service"
TRAIN_DIR="${ROOT_DIR}/two-tower-training"

CONFIG_FILE="${CONFIG_FILE:-configs/video.yml}"
MODEL_VERSION="${MODEL_VERSION:-two_tower_$(date +%Y%m%d_%H%M%S)}"
SAMPLE_LIMIT="${SAMPLE_LIMIT:-10000}"
SEED_COUNT="${SEED_COUNT:-0}"
DIM="${DIM:-64}"
EPOCHS="${EPOCHS:-60}"
LEARNING_RATE="${LEARNING_RATE:-0.01}"
L2="${L2:-0.001}"
SEED="${SEED:-42}"
BACKEND="${BACKEND:-torch}"
BATCH_SIZE="${BATCH_SIZE:-128}"
RANDOM_NEGATIVES="${RANDOM_NEGATIVES:-3}"
HARD_NEGATIVES="${HARD_NEGATIVES:-2}"
PUBLISH_GATE_ENABLED="${PUBLISH_GATE_ENABLED:-true}"
MIN_EVAL_AUC="${MIN_EVAL_AUC:-0.60}"
MIN_RECALL_AT_20="${MIN_RECALL_AT_20:-0.15}"
MIN_COVERAGE_AT_20="${MIN_COVERAGE_AT_20:-0.05}"
MAX_NEGATIVE_HIT_RATE_AT_20="${MAX_NEGATIVE_HIT_RATE_AT_20:-0.40}"
MIN_EVAL_SAMPLE_COUNT="${MIN_EVAL_SAMPLE_COUNT:-100}"
MAX_AUC_DROP="${MAX_AUC_DROP:-0.03}"
MAX_RECALL_DROP="${MAX_RECALL_DROP:-0.05}"
MAX_COVERAGE_DROP="${MAX_COVERAGE_DROP:-0.05}"
MAX_NEGATIVE_HIT_RATE_INCREASE="${MAX_NEGATIVE_HIT_RATE_INCREASE:-0.05}"
MAX_DISLIKE_HIT_RATE_INCREASE="${MAX_DISLIKE_HIT_RATE_INCREASE:-0.05}"
EVAL_RATIO="${EVAL_RATIO:-0.15}"
RETRIEVAL_K="${RETRIEVAL_K:-20}"
RETRIEVAL_KS="${RETRIEVAL_KS:-10,20,50,100}"
HALF_LIFE_DAYS="${HALF_LIFE_DAYS:-7}"
ARTIFACT_RETENTION_DAYS="${ARTIFACT_RETENTION_DAYS:-7}"
CLEANUP_REFERENCE_TIME="${CLEANUP_REFERENCE_TIME:-}"
PUBLISHED=false

SAMPLE_FILE="${TRAIN_DIR}/data/${MODEL_VERSION}_samples.csv"
ITEM_FILE="${TRAIN_DIR}/data/${MODEL_VERSION}_items.csv"
USER_FILE="${TRAIN_DIR}/data/${MODEL_VERSION}_user_features.csv"
BASELINE_METRICS_FILE="${TRAIN_DIR}/data/${MODEL_VERSION}_baseline_metrics.json"
ARTIFACT_DIR="${TRAIN_DIR}/artifacts/${MODEL_VERSION}"
LEGACY_SAMPLE_FILE="${HTTP_DIR}/storage/two_tower_samples.csv"

mkdir -p "${TRAIN_DIR}/data" "${TRAIN_DIR}/artifacts"

echo "model_version=${MODEL_VERSION}"
echo "config=${CONFIG_FILE}"
echo "samples=${SAMPLE_FILE}"
echo "artifact_dir=${ARTIFACT_DIR}"

require_training_backend() {
  if [[ "${BACKEND}" != "torch" ]]; then
    return
  fi
  if ! (
    cd "${TRAIN_DIR}"
    PYTHONPATH=src python3 -c "import torch" >/dev/null 2>&1
  ); then
    echo "training_dependency_missing=torch install_command='cd ${TRAIN_DIR} && python3 -m pip install -r requirements.txt'" >&2
    return 1
  fi
}

cleanup_training_samples() {
  rm -f "${LEGACY_SAMPLE_FILE}"
  find "${TRAIN_DIR}/data" -maxdepth 1 -type f -name "*.csv" -delete
  find "${TRAIN_DIR}/data" -maxdepth 1 -type f -name "*_baseline_metrics.json" -delete
}

artifact_mtime_epoch() {
  local artifact_dir="$1"
  stat -c %Y "${artifact_dir}" 2>/dev/null || stat -f %m "${artifact_dir}"
}

cleanup_old_artifacts() {
  if [[ "${ARTIFACT_RETENTION_DAYS}" -le 0 ]]; then
    return
  fi
  local artifact_count
  artifact_count=$(find "${TRAIN_DIR}/artifacts" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l | tr -d ' ')
  if [[ "${PUBLISHED}" != "true" ]] && [[ "${artifact_count}" -le 1 ]]; then
    echo "skip_artifact_cleanup=no_new_publish_and_single_model"
    return
  fi
  local reference_epoch
  if [[ -n "${CLEANUP_REFERENCE_TIME}" ]]; then
    if [[ "${CLEANUP_REFERENCE_TIME}" =~ ^[0-9]+$ ]]; then
      reference_epoch="${CLEANUP_REFERENCE_TIME}"
    else
      reference_epoch="$(date -j -f "%Y-%m-%dT%H:%M:%S%z" "${CLEANUP_REFERENCE_TIME}" +%s 2>/dev/null || date -d "${CLEANUP_REFERENCE_TIME}" +%s)"
    fi
  else
    reference_epoch="$(date +%s)"
  fi
  local cutoff_epoch=$((reference_epoch - ARTIFACT_RETENTION_DAYS * 24 * 60 * 60))
  local artifact_dir artifact_epoch
  while IFS= read -r -d '' artifact_dir; do
    if [[ "${artifact_dir}" == "${ARTIFACT_DIR}" ]]; then
      continue
    fi
    artifact_epoch="$(artifact_mtime_epoch "${artifact_dir}")"
    if [[ "${artifact_epoch}" -lt "${cutoff_epoch}" ]]; then
      rm -rf "${artifact_dir}"
      echo "removed_old_artifact=${artifact_dir}"
    fi
  done < <(find "${TRAIN_DIR}/artifacts" -mindepth 1 -maxdepth 1 -type d -print0)
}

run_cleanup() {
  cleanup_training_samples
  cleanup_old_artifacts
}
trap run_cleanup EXIT

require_training_backend

(
  cd "${HTTP_DIR}"
  go run ./tools/export_two_tower_samples \
    --config "${CONFIG_FILE}" \
    --limit "${SAMPLE_LIMIT}" \
    --seed-count "${SEED_COUNT}" \
    --output "${SAMPLE_FILE}" \
    --item-output "${ITEM_FILE}" \
    --user-output "${USER_FILE}"
)

(
  cd "${TRAIN_DIR}"
  PYTHONPATH=src python3 -m two_tower_training.train \
    --samples "${SAMPLE_FILE}" \
    --item-catalog "${ITEM_FILE}" \
    --user-features "${USER_FILE}" \
    --output "${ARTIFACT_DIR}" \
    --model-version "${MODEL_VERSION}" \
    --dim "${DIM}" \
    --epochs "${EPOCHS}" \
    --learning-rate "${LEARNING_RATE}" \
    --l2 "${L2}" \
    --eval-ratio "${EVAL_RATIO}" \
    --retrieval-k "${RETRIEVAL_K}" \
    --retrieval-ks "${RETRIEVAL_KS}" \
    --half-life-days "${HALF_LIFE_DAYS}" \
    --backend "${BACKEND}" \
    --batch-size "${BATCH_SIZE}" \
    --random-negatives "${RANDOM_NEGATIVES}" \
    --hard-negatives "${HARD_NEGATIVES}" \
    --seed "${SEED}"
)

(
  cd "${HTTP_DIR}"
  go run ./tools/export_active_recommend_model_metrics \
    --config "${CONFIG_FILE}" \
    --model two_tower \
    --output "${BASELINE_METRICS_FILE}"
)

if [[ "${PUBLISH_GATE_ENABLED}" == "true" ]]; then
  (
    cd "${TRAIN_DIR}"
    PYTHONPATH=src python3 -m two_tower_training.publish_gate \
      --metrics "${ARTIFACT_DIR}/metrics.json" \
      --min-eval-auc "${MIN_EVAL_AUC}" \
      --min-recall-at-20 "${MIN_RECALL_AT_20}" \
      --min-coverage-at-20 "${MIN_COVERAGE_AT_20}" \
      --max-negative-hit-rate-at-20 "${MAX_NEGATIVE_HIT_RATE_AT_20}" \
      --min-eval-sample-count "${MIN_EVAL_SAMPLE_COUNT}" \
      --metric-k "${RETRIEVAL_K}" \
      --baseline-metrics "${BASELINE_METRICS_FILE}" \
      --max-auc-drop "${MAX_AUC_DROP}" \
      --max-recall-drop "${MAX_RECALL_DROP}" \
      --max-coverage-drop "${MAX_COVERAGE_DROP}" \
      --max-negative-hit-rate-increase "${MAX_NEGATIVE_HIT_RATE_INCREASE}" \
      --max-dislike-hit-rate-increase "${MAX_DISLIKE_HIT_RATE_INCREASE}"
  )
else
  echo "publish_gate=disabled"
fi

(
  cd "${HTTP_DIR}"
  go run ./tools/import_two_tower_embeddings \
    --config "${CONFIG_FILE}" \
    --artifact-dir "${ARTIFACT_DIR}" \
    --dim "${DIM}" \
    --publish
)
PUBLISHED=true

echo "published ${MODEL_VERSION}"
