import os
import shutil
import stat
import subprocess
import tempfile
import unittest
from pathlib import Path


class TwoTowerPipelineCleanupTest(unittest.TestCase):
    def test_pipeline_uses_small_data_training_defaults(self):
        repo_root = Path(__file__).resolve().parents[2]
        script = repo_root / "two-tower-training" / "scripts" / "run_two_tower_pipeline.sh"

        with tempfile.TemporaryDirectory() as tmp:
            tmp_root = Path(tmp)
            work_root = tmp_root / "repo"
            shutil.copytree(repo_root / "two-tower-training", work_root / "two-tower-training")
            (work_root / "video-service" / "tools").mkdir(parents=True)
            (work_root / "video-service" / "storage").mkdir(parents=True)
            shutil.copy2(script, work_root / "two-tower-training" / "scripts" / "run_two_tower_pipeline.sh")

            fake_bin = tmp_root / "bin"
            fake_bin.mkdir()
            fake_go = fake_bin / "go"
            fake_go.write_text(
                """#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == *"export_two_tower_samples"* ]]; then
  out=""; item_out=""; user_out=""
  while [[ $# -gt 0 ]]; do
    if [[ "$1" == "--output" ]]; then out="$2"; shift 2; continue; fi
    if [[ "$1" == "--item-output" ]]; then item_out="$2"; shift 2; continue; fi
    if [[ "$1" == "--user-output" ]]; then user_out="$2"; shift 2; continue; fi
    shift
  done
  mkdir -p "$(dirname "$out")"
  printf 'user_id,video_id,video_segment_id,label,weight,source,reason,event_time\\n1,1,1,1,1,test,test,2026-06-23T00:00:00+08:00\\n1,1,2,0,1,test,dislike,2026-06-24T00:00:00+08:00\\n' > "$out"
  mkdir -p "$(dirname "$item_out")"
  printf 'video_segment_id,video_id,segment_duration,video_duration,like_count,double_like_count,dislike_count,content_summary,knowledge_tags,video_title\\n1,1,30,300,1,0,0,test,tag,title\\n2,1,30,300,0,0,1,test,tag,title\\n' > "$item_out"
  mkdir -p "$(dirname "$user_out")"
  printf 'user_id,grade_id,class_id,user_type,mastery_avg,mastery_min,weak_knowledge_count,strong_knowledge_count,knowledge_correct_count,knowledge_incorrect_count,answer_count,answer_correct_count,answer_incorrect_count,avg_score_rate,avg_cost_seconds,question_feedback_count,generated_feedback_count,generated_correct_count,generated_avg_score_rate,question_search_count,recent_knowledge_point_ids,recent_subjects,question_search_knowledge_text,generated_feedback_knowledge_text\\n1,1,1,1,0.8,0.5,1,2,5,1,6,5,1,0.8,30,1,1,1,0.9,1,101,math,test,test\\n' > "$user_out"
  exit 0
fi
if [[ "$*" == *"export_active_recommend_model_metrics"* ]]; then
  out=""
  while [[ $# -gt 0 ]]; do if [[ "$1" == "--output" ]]; then out="$2"; shift 2; continue; fi; shift; done
  mkdir -p "$(dirname "$out")"; printf '{}' > "$out"; exit 0
fi
if [[ "$*" == *"import_two_tower_embeddings"* ]]; then exit 0; fi
echo "unexpected go args: $*" >&2
exit 1
""",
                encoding="utf-8",
            )
            fake_python = fake_bin / "python3"
            fake_python.write_text(
                """#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "-c" ]]; then exit 0; fi
expect_arg() {
  local key="$1"
  local want="$2"
  shift 2
  local args="$*"
  while [[ $# -gt 0 ]]; do
    if [[ "$1" == "$key" && "${2:-}" == "$want" ]]; then
      return 0
    fi
    shift
  done
  echo "missing $key $want in: $args" >&2
  exit 42
}
if [[ "$*" == *"two_tower_training.publish_gate"* ]]; then
  expect_arg "--metric-k" "10" "$@"
  expect_arg "--min-eval-sample-count" "50" "$@"
  echo "publish_gate=pass"
  exit 0
fi
expect_arg "--epochs" "60" "$@"
expect_arg "--learning-rate" "0.01" "$@"
expect_arg "--batch-size" "128" "$@"
expect_arg "--random-negatives" "3" "$@"
expect_arg "--hard-negatives" "2" "$@"
expect_arg "--retrieval-k" "10" "$@"
out=""; model_version="test_model"
while [[ $# -gt 0 ]]; do
  if [[ "$1" == "--output" ]]; then out="$2"; shift 2; continue; fi
  if [[ "$1" == "--model-version" ]]; then model_version="$2"; shift 2; continue; fi
  shift
done
mkdir -p "$out"
printf '{"model_version":"%s","eval_auc":0.8,"recall_at_20":0.4,"coverage_at_20":0.3,"negative_hit_rate_at_20":0.02,"eval_sample_count":100}\\n' "$model_version" > "$out/metrics.json"
printf 'video_segment_id,video_id,embedding,model_version\\n' > "$out/item_embeddings.csv"
printf 'user_id,embedding,model_version\\n' > "$out/user_embeddings.csv"
""",
                encoding="utf-8",
            )
            fake_go.chmod(fake_go.stat().st_mode | stat.S_IXUSR)
            fake_python.chmod(fake_python.stat().st_mode | stat.S_IXUSR)

            env = os.environ.copy()
            env.update(
                {
                    "PATH": f"{fake_bin}:{env.get('PATH', '')}",
                    "MODEL_VERSION": "test_model",
                    "ARTIFACT_RETENTION_DAYS": "0",
                }
            )

            result = subprocess.run(
                [str(work_root / "two-tower-training" / "scripts" / "run_two_tower_pipeline.sh")],
                cwd=work_root / "two-tower-training",
                env=env,
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
            )

            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)

    def test_pipeline_reports_missing_torch_before_export(self):
        repo_root = Path(__file__).resolve().parents[2]
        script = repo_root / "two-tower-training" / "scripts" / "run_two_tower_pipeline.sh"

        with tempfile.TemporaryDirectory() as tmp:
            tmp_root = Path(tmp)
            work_root = tmp_root / "repo"
            shutil.copytree(repo_root / "two-tower-training", work_root / "two-tower-training")
            (work_root / "video-service" / "tools").mkdir(parents=True)
            (work_root / "video-service" / "storage").mkdir(parents=True)
            shutil.copy2(script, work_root / "two-tower-training" / "scripts" / "run_two_tower_pipeline.sh")

            fake_bin = tmp_root / "bin"
            fake_bin.mkdir()
            fake_go = fake_bin / "go"
            fake_go.write_text(
                """#!/usr/bin/env bash
echo "go should not run when torch is missing" >&2
exit 33
""",
                encoding="utf-8",
            )
            fake_python = fake_bin / "python3"
            fake_python.write_text(
                """#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "-c" ]]; then
  exit 1
fi
echo "python training should not run when torch is missing" >&2
exit 34
""",
                encoding="utf-8",
            )
            fake_go.chmod(fake_go.stat().st_mode | stat.S_IXUSR)
            fake_python.chmod(fake_python.stat().st_mode | stat.S_IXUSR)

            env = os.environ.copy()
            env.update(
                {
                    "PATH": f"{fake_bin}:{env.get('PATH', '')}",
                    "MODEL_VERSION": "test_model",
                    "ARTIFACT_RETENTION_DAYS": "0",
                }
            )

            result = subprocess.run(
                [str(work_root / "two-tower-training" / "scripts" / "run_two_tower_pipeline.sh")],
                cwd=work_root / "two-tower-training",
                env=env,
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
            )

            self.assertNotEqual(result.returncode, 0)
            self.assertIn("training_dependency_missing=torch", result.stderr)
            self.assertNotIn("go should not run", result.stderr)

    def test_pipeline_removes_samples_and_old_artifacts(self):
        repo_root = Path(__file__).resolve().parents[2]
        script = repo_root / "two-tower-training" / "scripts" / "run_two_tower_pipeline.sh"

        with tempfile.TemporaryDirectory() as tmp:
            tmp_root = Path(tmp)
            work_root = tmp_root / "repo"
            shutil.copytree(repo_root / "two-tower-training", work_root / "two-tower-training")
            (work_root / "video-service" / "tools").mkdir(parents=True)
            (work_root / "video-service" / "storage").mkdir(parents=True)
            shutil.copy2(script, work_root / "two-tower-training" / "scripts" / "run_two_tower_pipeline.sh")

            old_artifact = work_root / "two-tower-training" / "artifacts" / "old_model"
            old_artifact.mkdir(parents=True)
            (old_artifact / "metrics.json").write_text("{}", encoding="utf-8")
            old_time = 1_700_000_000
            os.utime(old_artifact / "metrics.json", (old_time, old_time))
            os.utime(old_artifact, (old_time, old_time))

            current_artifact = work_root / "two-tower-training" / "artifacts" / "keep_model"
            current_artifact.mkdir(parents=True)
            (current_artifact / "metrics.json").write_text("{}", encoding="utf-8")

            fake_bin = tmp_root / "bin"
            fake_bin.mkdir()
            fake_go = fake_bin / "go"
            fake_go.write_text(
                """#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == *"export_two_tower_samples"* ]]; then
  out=""
  item_out=""
  user_out=""
  while [[ $# -gt 0 ]]; do
    if [[ "$1" == "--output" ]]; then
      out="$2"
      shift 2
      continue
    fi
    if [[ "$1" == "--item-output" ]]; then
      item_out="$2"
      shift 2
      continue
    fi
    if [[ "$1" == "--user-output" ]]; then
      user_out="$2"
      shift 2
      continue
    fi
    shift
  done
  mkdir -p "$(dirname "$out")"
  printf 'user_id,video_id,video_segment_id,label,weight,source,reason,event_time\\n1,1,1,1,1,test,test,2026-06-23T00:00:00+08:00\\n' > "$out"
  if [[ -n "$item_out" ]]; then
    mkdir -p "$(dirname "$item_out")"
    printf 'video_segment_id,video_id,segment_duration,video_duration,like_count,double_like_count,dislike_count,content_summary,knowledge_tags,video_title\\n1,1,30,300,1,0,0,test,tag,title\\n' > "$item_out"
  fi
  if [[ -n "$user_out" ]]; then
    mkdir -p "$(dirname "$user_out")"
    printf 'user_id,grade_id,class_id,user_type,mastery_avg,mastery_min,weak_knowledge_count,strong_knowledge_count,knowledge_correct_count,knowledge_incorrect_count,answer_count,answer_correct_count,answer_incorrect_count,avg_score_rate,avg_cost_seconds,question_feedback_count,generated_feedback_count,generated_correct_count,generated_avg_score_rate,question_search_count,recent_knowledge_point_ids,recent_subjects,question_search_knowledge_text,generated_feedback_knowledge_text\\n1,1,1,1,0.8,0.5,1,2,5,1,6,5,1,0.8,30,1,1,1,0.9,1,101,math,test,test\\n' > "$user_out"
  fi
  mkdir -p storage
  printf 'legacy sample\\n' > storage/two_tower_samples.csv
  exit 0
fi
if [[ "$*" == *"import_two_tower_embeddings"* ]]; then
  exit 0
fi
if [[ "$*" == *"export_active_recommend_model_metrics"* ]]; then
  out=""
  while [[ $# -gt 0 ]]; do
    if [[ "$1" == "--output" ]]; then
      out="$2"
      shift 2
      continue
    fi
    shift
  done
  mkdir -p "$(dirname "$out")"
  printf '{}' > "$out"
  exit 0
fi
echo "unexpected go args: $*" >&2
exit 1
""",
                encoding="utf-8",
            )
            fake_python = fake_bin / "python3"
            fake_python.write_text(
                """#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "-c" ]]; then exit 0; fi
if [[ "$*" == *"two_tower_training.publish_gate"* ]]; then
  echo "publish_gate=pass"
  exit 0
fi
out=""
model_version="test_model"
while [[ $# -gt 0 ]]; do
  if [[ "$1" == "--output" ]]; then
    out="$2"
    shift 2
    continue
  fi
  if [[ "$1" == "--model-version" ]]; then
    model_version="$2"
    shift 2
    continue
  fi
  shift
done
mkdir -p "$out"
printf '{"model_version":"%s","eval_auc":0.8,"recall_at_20":0.4,"coverage_at_20":0.3,"negative_hit_rate_at_20":0.02,"eval_sample_count":100}\\n' "$model_version" > "$out/metrics.json"
printf 'video_segment_id,video_id,embedding,model_version\\n' > "$out/item_embeddings.csv"
printf 'user_id,embedding,model_version\\n' > "$out/user_embeddings.csv"
""",
                encoding="utf-8",
            )
            fake_go.chmod(fake_go.stat().st_mode | stat.S_IXUSR)
            fake_python.chmod(fake_python.stat().st_mode | stat.S_IXUSR)

            env = os.environ.copy()
            env.update(
                {
                    "PATH": f"{fake_bin}:{env.get('PATH', '')}",
                    "MODEL_VERSION": "test_model",
                    "ARTIFACT_RETENTION_DAYS": "7",
                    "CLEANUP_REFERENCE_TIME": "1782144000",
                }
            )

            result = subprocess.run(
                [str(work_root / "two-tower-training" / "scripts" / "run_two_tower_pipeline.sh")],
                cwd=work_root / "two-tower-training",
                env=env,
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertFalse((work_root / "two-tower-training" / "data" / "test_model_samples.csv").exists())
            self.assertFalse((work_root / "two-tower-training" / "data" / "test_model_items.csv").exists())
            self.assertFalse((work_root / "two-tower-training" / "data" / "test_model_user_features.csv").exists())
            self.assertFalse((work_root / "two-tower-training" / "data" / "test_model_baseline_metrics.json").exists())
            self.assertFalse((work_root / "video-service" / "storage" / "two_tower_samples.csv").exists())
            self.assertFalse(old_artifact.exists())
            self.assertTrue(current_artifact.exists())
            self.assertTrue((work_root / "two-tower-training" / "artifacts" / "test_model").exists())


    def test_cleanup_runs_on_failure_single_model(self):
        repo_root = Path(__file__).resolve().parents[2]
        script = repo_root / "two-tower-training" / "scripts" / "run_two_tower_pipeline.sh"

        with tempfile.TemporaryDirectory() as tmp:
            tmp_root = Path(tmp)
            work_root = tmp_root / "repo"
            shutil.copytree(repo_root / "two-tower-training", work_root / "two-tower-training")
            (work_root / "video-service" / "tools").mkdir(parents=True)
            (work_root / "video-service" / "storage").mkdir(parents=True)
            shutil.copy2(script, work_root / "two-tower-training" / "scripts" / "run_two_tower_pipeline.sh")

            # Remove all pre-existing artifacts from copytree, so training creates the only one
            for p in (work_root / "two-tower-training" / "artifacts").iterdir():
                if p.is_dir():
                    shutil.rmtree(p)

            fake_bin = tmp_root / "bin"
            fake_bin.mkdir()
            fake_go = fake_bin / "go"
            fake_go.write_text(
                """#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == *"export_two_tower_samples"* ]]; then
  out=""; item_out=""; user_out=""
  while [[ $# -gt 0 ]]; do
    if [[ "$1" == "--output" ]]; then out="$2"; shift 2; continue; fi
    if [[ "$1" == "--item-output" ]]; then item_out="$2"; shift 2; continue; fi
    if [[ "$1" == "--user-output" ]]; then user_out="$2"; shift 2; continue; fi
    shift
  done
  mkdir -p "$(dirname "$out")"
  printf 'user_id,video_id,video_segment_id,label,weight,source,reason,event_time\\n1,1,1,1,1,test,test,2026-06-23T00:00:00+08:00\\n' > "$out"
  if [[ -n "$item_out" ]]; then
    mkdir -p "$(dirname "$item_out")"
    printf 'video_segment_id,video_id,segment_duration,video_duration,like_count,double_like_count,dislike_count,content_summary,knowledge_tags,video_title\\n1,1,30,300,1,0,0,test,tag,title\\n' > "$item_out"
  fi
  if [[ -n "$user_out" ]]; then
    mkdir -p "$(dirname "$user_out")"
    printf 'user_id,grade_id,class_id,user_type,mastery_avg,mastery_min,weak_knowledge_count,strong_knowledge_count,knowledge_correct_count,knowledge_incorrect_count,answer_count,answer_correct_count,answer_incorrect_count,avg_score_rate,avg_cost_seconds,question_feedback_count,generated_feedback_count,generated_correct_count,generated_avg_score_rate,question_search_count,recent_knowledge_point_ids,recent_subjects,question_search_knowledge_text,generated_feedback_knowledge_text\\n1,1,1,1,0.8,0.5,1,2,5,1,6,5,1,0.8,30,1,1,1,0.9,1,101,math,test,test\\n' > "$user_out"
  fi
  mkdir -p storage
  printf 'legacy sample\\n' > storage/two_tower_samples.csv
  exit 0
fi
if [[ "$*" == *"import_two_tower_embeddings"* ]]; then
  exit 1
fi
if [[ "$*" == *"export_active_recommend_model_metrics"* ]]; then
  out=""
  while [[ $# -gt 0 ]]; do if [[ "$1" == "--output" ]]; then out="$2"; shift 2; continue; fi; shift; done
  mkdir -p "$(dirname "$out")"
  printf '{}' > "$out"
  exit 0
fi
echo "unexpected go args: $*" >&2
exit 1
""",
                encoding="utf-8",
            )
            fake_python = fake_bin / "python3"
            fake_python.write_text(
                """#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "-c" ]]; then exit 0; fi
if [[ "$*" == *"two_tower_training.publish_gate"* ]]; then echo "publish_gate=pass"; exit 0; fi
out=""; model_version="test_model"
while [[ $# -gt 0 ]]; do
  if [[ "$1" == "--output" ]]; then out="$2"; shift 2; continue; fi
  if [[ "$1" == "--model-version" ]]; then model_version="$2"; shift 2; continue; fi
  shift
done
mkdir -p "$out"
printf '{"model_version":"%s","eval_auc":0.8,"recall_at_20":0.4,"coverage_at_20":0.3,"negative_hit_rate_at_20":0.02,"eval_sample_count":100}\\n' "$model_version" > "$out/metrics.json"
printf 'video_segment_id,video_id,embedding,model_version\\n' > "$out/item_embeddings.csv"
printf 'user_id,embedding,model_version\\n' > "$out/user_embeddings.csv"
""",
                encoding="utf-8",
            )
            fake_go.chmod(fake_go.stat().st_mode | stat.S_IXUSR)
            fake_python.chmod(fake_python.stat().st_mode | stat.S_IXUSR)

            env = os.environ.copy()
            env.update(
                {
                    "PATH": f"{fake_bin}:{env.get('PATH', '')}",
                    "MODEL_VERSION": "test_model",
                    "ARTIFACT_RETENTION_DAYS": "7",
                    "CLEANUP_REFERENCE_TIME": "1782144000",
                }
            )

            result = subprocess.run(
                [str(work_root / "two-tower-training" / "scripts" / "run_two_tower_pipeline.sh")],
                cwd=work_root / "two-tower-training",
                env=env,
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
            )

            self.assertNotEqual(result.returncode, 0)
            self.assertIn("skip_artifact_cleanup=no_new_publish_and_single_model", result.stdout)
            self.assertFalse((work_root / "two-tower-training" / "data" / "test_model_samples.csv").exists())
            self.assertFalse((work_root / "video-service" / "storage" / "two_tower_samples.csv").exists())
            self.assertTrue((work_root / "two-tower-training" / "artifacts" / "test_model").exists())

    def test_cleanup_runs_on_failure_multiple_models(self):
        repo_root = Path(__file__).resolve().parents[2]
        script = repo_root / "two-tower-training" / "scripts" / "run_two_tower_pipeline.sh"

        with tempfile.TemporaryDirectory() as tmp:
            tmp_root = Path(tmp)
            work_root = tmp_root / "repo"
            shutil.copytree(repo_root / "two-tower-training", work_root / "two-tower-training")
            (work_root / "video-service" / "tools").mkdir(parents=True)
            (work_root / "video-service" / "storage").mkdir(parents=True)
            shutil.copy2(script, work_root / "two-tower-training" / "scripts" / "run_two_tower_pipeline.sh")

            # Remove all pre-existing artifacts from copytree
            for p in (work_root / "two-tower-training" / "artifacts").iterdir():
                if p.is_dir():
                    shutil.rmtree(p)

            old_artifact = work_root / "two-tower-training" / "artifacts" / "old_model"
            old_artifact.mkdir(parents=True)
            (old_artifact / "metrics.json").write_text("{}", encoding="utf-8")
            old_time = 1_700_000_000
            os.utime(old_artifact / "metrics.json", (old_time, old_time))
            os.utime(old_artifact, (old_time, old_time))

            recent_artifact = work_root / "two-tower-training" / "artifacts" / "recent_model"
            recent_artifact.mkdir(parents=True)
            (recent_artifact / "metrics.json").write_text("{}", encoding="utf-8")

            fake_bin = tmp_root / "bin"
            fake_bin.mkdir()
            fake_go = fake_bin / "go"
            fake_go.write_text(
                """#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == *"export_two_tower_samples"* ]]; then
  out=""; item_out=""; user_out=""
  while [[ $# -gt 0 ]]; do
    if [[ "$1" == "--output" ]]; then out="$2"; shift 2; continue; fi
    if [[ "$1" == "--item-output" ]]; then item_out="$2"; shift 2; continue; fi
    if [[ "$1" == "--user-output" ]]; then user_out="$2"; shift 2; continue; fi
    shift
  done
  mkdir -p "$(dirname "$out")"
  printf 'user_id,video_id,video_segment_id,label,weight,source,reason,event_time\\n1,1,1,1,1,test,test,2026-06-23T00:00:00+08:00\\n' > "$out"
  if [[ -n "$item_out" ]]; then
    mkdir -p "$(dirname "$item_out")"
    printf 'video_segment_id,video_id,segment_duration,video_duration,like_count,double_like_count,dislike_count,content_summary,knowledge_tags,video_title\\n1,1,30,300,1,0,0,test,tag,title\\n' > "$item_out"
  fi
  if [[ -n "$user_out" ]]; then
    mkdir -p "$(dirname "$user_out")"
    printf 'user_id,grade_id,class_id,user_type,mastery_avg,mastery_min,weak_knowledge_count,strong_knowledge_count,knowledge_correct_count,knowledge_incorrect_count,answer_count,answer_correct_count,answer_incorrect_count,avg_score_rate,avg_cost_seconds,question_feedback_count,generated_feedback_count,generated_correct_count,generated_avg_score_rate,question_search_count,recent_knowledge_point_ids,recent_subjects,question_search_knowledge_text,generated_feedback_knowledge_text\\n1,1,1,1,0.8,0.5,1,2,5,1,6,5,1,0.8,30,1,1,1,0.9,1,101,math,test,test\\n' > "$user_out"
  fi
  mkdir -p storage
  printf 'legacy sample\\n' > storage/two_tower_samples.csv
  exit 0
fi
if [[ "$*" == *"import_two_tower_embeddings"* ]]; then
  exit 1
fi
if [[ "$*" == *"export_active_recommend_model_metrics"* ]]; then
  out=""
  while [[ $# -gt 0 ]]; do if [[ "$1" == "--output" ]]; then out="$2"; shift 2; continue; fi; shift; done
  mkdir -p "$(dirname "$out")"
  printf '{}' > "$out"
  exit 0
fi
echo "unexpected go args: $*" >&2
exit 1
""",
                encoding="utf-8",
            )
            fake_python = fake_bin / "python3"
            fake_python.write_text(
                """#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "-c" ]]; then exit 0; fi
if [[ "$*" == *"two_tower_training.publish_gate"* ]]; then echo "publish_gate=pass"; exit 0; fi
out=""; model_version="test_model"
while [[ $# -gt 0 ]]; do
  if [[ "$1" == "--output" ]]; then out="$2"; shift 2; continue; fi
  if [[ "$1" == "--model-version" ]]; then model_version="$2"; shift 2; continue; fi
  shift
done
mkdir -p "$out"
printf '{"model_version":"%s","eval_auc":0.8,"recall_at_20":0.4,"coverage_at_20":0.3,"negative_hit_rate_at_20":0.02,"eval_sample_count":100}\\n' "$model_version" > "$out/metrics.json"
printf 'video_segment_id,video_id,embedding,model_version\\n' > "$out/item_embeddings.csv"
printf 'user_id,embedding,model_version\\n' > "$out/user_embeddings.csv"
""",
                encoding="utf-8",
            )
            fake_go.chmod(fake_go.stat().st_mode | stat.S_IXUSR)
            fake_python.chmod(fake_python.stat().st_mode | stat.S_IXUSR)
            fake_stat = fake_bin / "stat"
            fake_stat.write_text(
                """#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "-c" && "${2:-}" == "%Y" ]]; then
  if [[ "${3:-}" == *"old_model" ]]; then
    echo "1700000000"
  else
    echo "1782144000"
  fi
  exit 0
fi
if [[ "${1:-}" == "-f" ]]; then
  echo '  File: "simulated-gnu-statfs-output"'
  exit 0
fi
exit 1
""",
                encoding="utf-8",
            )
            fake_stat.chmod(fake_stat.stat().st_mode | stat.S_IXUSR)

            env = os.environ.copy()
            env.update(
                {
                    "PATH": f"{fake_bin}:{env.get('PATH', '')}",
                    "MODEL_VERSION": "test_model",
                    "ARTIFACT_RETENTION_DAYS": "7",
                    "CLEANUP_REFERENCE_TIME": "1782144000",
                }
            )

            result = subprocess.run(
                [str(work_root / "two-tower-training" / "scripts" / "run_two_tower_pipeline.sh")],
                cwd=work_root / "two-tower-training",
                env=env,
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
            )

            self.assertNotEqual(result.returncode, 0)
            self.assertNotIn("File: unbound variable", result.stderr + result.stdout)
            self.assertNotIn("skip_artifact_cleanup=no_new_publish_and_single_model", result.stdout)
            self.assertFalse((work_root / "two-tower-training" / "data" / "test_model_samples.csv").exists())
            self.assertFalse((work_root / "video-service" / "storage" / "two_tower_samples.csv").exists())
            self.assertFalse(old_artifact.exists())
            self.assertTrue(recent_artifact.exists())
            self.assertTrue((work_root / "two-tower-training" / "artifacts" / "test_model").exists())


if __name__ == "__main__":
    unittest.main()
