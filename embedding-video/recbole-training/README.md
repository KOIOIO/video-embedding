# RecBole 推荐离线训练

本目录承载当前 HTTP 服务的离线 RecBole 流水线：从 `video-service` 导出交互数据，训练 embedding 模型，评估候选模型，执行发布门禁，并且只有在门禁通过后，才把数字型视频片段 embedding 和用户 embedding 导入 PostgreSQL 的 `recsys` schema。

## 运行流水线

从本目录运行。仓库内自带的本地服务配置只是示例；请使用指向待导出数据库的配置。

```bash
CONFIG_FILE=../video-service/configs/video.yml ./scripts/run_recbole_pipeline.sh
```

脚本会优先使用 `PATH` 上预编译好的 `export_recbole_dataset`、`export_active_recsys_model_metrics` 和 `import_recsys_embeddings` 二进制；找不到时，会从 `SERVICE_DIR` 运行对应的 `go run ./tools/...` 命令。Python 阶段使用 `PYTHON_BIN` 运行：优先使用可执行的 `./.venv/bin/python`，否则回退到 `python3`。

各阶段顺序固定为：

1. 从 PostgreSQL 导出 atomic 文件和导出统计。
2. 强制要求存在 `<DATASET>.train.inter`、`<DATASET>.valid.inter`、`<DATASET>.test.inter`；拒绝验证集或测试集中出现 `knowledge_video:` token。
3. 导出当前 active 模型的指标作为基线（baseline）。
4. 训练并评估 RecBole，然后写出 `metrics.json` 和 embedding CSV。
5. 当 `PUBLISH_GATE_ENABLED=true` 时运行 Python 发布门禁。
6. 校验每个 item embedding ID 都是正的十进制视频片段 ID，导入 `recsys` 并发布模型版本。

任一必需文件缺失、虚拟 item 校验失败、embedding 校验失败、训练失败或门禁（启用时）不通过时，都不会执行最后的导入发布。

## 支持的 shell 变量

以下是 `scripts/run_recbole_pipeline.sh` 消费的变量：

| 变量 | 默认值 | 作用 |
| --- | --- | --- |
| `SERVICE_DIR` | 仓库中的 `video-service` | 用于 Go 工具和配置查找的 HTTP 服务目录 |
| `CONFIG_FILE` | `${SERVICE_DIR}/configs/video.yml` | 传给 Go 工具的 PostgreSQL 与服务配置 |
| `MODEL_VERSION` | `recbole_<YYYYMMDD_HHMMSS>` | 候选版本与输出目录名 |
| `MODEL_NAME` | `recbole` | 写入 `recsys` 的线上模型名 |
| `RECBOLE_MODEL` | `BPR` | RecBole 模型类 |
| `DATASET` | `video_app` | RecBole 数据集前缀 |
| `DIM` | `64` | 用户/item embedding 维度 |
| `EPOCHS` | `20` | RecBole 训练轮数 |
| `SAMPLE_LIMIT` | `10000` | 导出器查询普通交互行的上限 |
| `DAYS_BACK` | `30` | 交互与知识点视频观看的回看窗口 |
| `PYTHON_BIN` | 优先项目 `.venv/bin/python`，否则 `python3` | 训练和门禁阶段的 Python 可执行文件 |
| `DATA_ROOT` | `data/${MODEL_VERSION}` | 导出数据集的父目录 |
| `DATA_DIR` | `${DATA_ROOT}/${DATASET}` | 存放本次运行 atomic 文件的目录 |
| `ARTIFACT_DIR` | `artifacts/${MODEL_VERSION}` | 候选 checkpoint、指标与 embedding |
| `BASELINE_METRICS` | `${ARTIFACT_DIR}/baseline_metrics.json` | 训练前导出的基线指标 |
| `PUBLISH_GATE_ENABLED` | `true` | 是否执行 `recbole_recommendation.publish_gate` |

`MIN_RECALL_AT_20`、`MIN_NDCG_AT_20`、`MAX_RELATIVE_NDCG_DROP` 和 `ARTIFACT_RETENTION_DAYS` 不是本脚本的 shell 输入。门禁阈值通过 Python 命令行参数传入：`--min-recall-at-20`、`--min-ndcg-at-20`、`--max-relative-ndcg-drop`、`--min-positive-rows` 和 `--min-positive-users`。默认指标阈值为 Recall@20 `>= 0.01`、NDCG@20 `>= 0.005`，以及相对上一版 active 基线的 NDCG@20 最多下降 `20%`；两个正样本阈值默认都为 `1`。

## 数据集契约

`tools/export_recbole_dataset` 会在 `DATA_DIR` 下写出以下文件：

```text
<DATASET>.train.inter
<DATASET>.valid.inter
<DATASET>.test.inter
<DATASET>.item
<DATASET>.user
<DATASET>.export_stats.json
```

交互文件包含 `user_id`、`item_id`、`rating`、`timestamp`、`source` 和 `weight` 列。普通 item 是数字型视频片段 ID。导出器合并视频 reaction、观看/曝光事件和题目搜索后的跟进事件，并对每个「用户/视频片段」对去重，保留价值更高的那次交互。`.user` 文件包含当前导出器使用的更广泛的学习画像特征；`.item` 包含视频片段/内容元数据。

benchmark 切分是确定性的、按用户时间排序的：最后一次普通交互作为测试集，前一次作为验证集，更早的作为训练集。只有一两次普通交互的用户由导出器处理，不会凭空构造验证行。RecBole 使用 `benchmark_filename=["train", "valid", "test"]`、按用户分组、按时间排序和全排序（full-sort）评估。

### 知识点视频信号

一次有效的知识点视频观看，是观看秒数达到视频时长至少 `60%` 的聚合观看会话。它只在训练阶段以带命名空间的虚拟 item 表示，例如 `knowledge_video:88`。

虚拟 item 被有意排除在验证集和测试集目标之外，在 RecBole 全排序打分中被屏蔽，并从 `item_embeddings.csv` 中过滤掉。因此它们不会导入 `recsys`，不是数据库中的视频片段候选，也永远不会被在线推荐返回。导出器仍可能在 `<DATASET>.item` 中包含该虚拟 token，以便 RecBole 解析训练词表；该文件里出现 token 不代表它是在线 item。

## 候选产物与门禁

Python 阶段会在 `ARTIFACT_DIR` 下写出这些候选文件：

```text
recbole_config.yaml
metrics.json
item_embeddings.csv
user_embeddings.csv
checkpoints/...
```

`metrics.json` 包含归一化后的 RecBole 指标（`framework`、`algorithm`、`model_version`、Recall@20、NDCG@20、Hit@20 和 Precision@20）、来自 `<DATASET>.export_stats.json` 的数字型导出统计，以及被过滤掉的虚拟 item embedding 数量。发布门禁会把 Recall@20 和 NDCG@20 与配置的下限以及 active 基线的相对 NDCG 进行对比。它的质量配置还会通过上面提到的 CLI 参数暴露 `positive_rows >= 1` 和 `positive_users >= 1`。

当前限制：`publish_gate.evaluate` 只会在 `metrics.json` 中存在这些键时检查 `positive_rows` 和 `positive_users`；当前导出器会写出知识点观看/过滤统计，但尚未输出这两个键。因此现有的 shell 流水线并不能保证正样本检查被触发。请把这一点当作数据质量方面的后续工作，而不是擅自新增不受支持的环境变量的理由。

门禁失败时，脚本会在 `import_recsys_embeddings --publish` 之前退出。上一版 active 模型继续生效，完整的候选目录会保留下来用于排查。门禁被禁用时，脚本在导入前仍会执行文件和数字 ID 校验；禁用门禁是一个明确的操作选择。

## Embedding 与在线服务边界

CSV 契约如下：

```text
item_embeddings.csv: video_segment_id,video_id,embedding,model_version
user_embeddings.csv: user_id,embedding,model_version
```

导入器只接受数字型的 `video_segment_id` 值，并写入：

```text
recsys.recommend_model_version
recsys.recommend_user_embedding
recsys.recommend_item_embedding
```

在线服务只从 `recsys` 读取普通数字型视频片段 embedding。它不会把训练目录、虚拟知识点视频 item、旧的 public embedding 表或 Gorse 存储当作 active RecBole 模型的替代品。启用可选集成时，Gorse 可以消费 HTTP 服务内部的 external-candidate 接口。

旧版推荐表不由本流水线创建。如果确实有计划清理，请先校验并备份，再从 HTTP 服务目录运行显式清理工具：

```bash
cd ../video-service
go run ./tools/drop_legacy_recommendation_tables --execute --confirm drop-legacy-recommendation-tables
```

## 依赖与验证

`requirements.txt` 使用有界版本区间，而不是完全锁定的 lockfile（`numpy>=1.24,<2.0`、`pandas>=1.5,<2.2`、`torch>=2.0,<2.4`，以及 RecBole/PyYAML 的最低版本）。本地运行前先创建虚拟环境并安装这些区间：

```bash
python3 -m venv .venv
source .venv/bin/activate
python -m pip install -r requirements.txt
PYTHONPATH=src python -m unittest discover -s tests -p 'test_*.py'
```

生产镜像会自行安装兼容 CPU 的 PyTorch 构建，但源码中的 requirements 仍是有界版本区间。生成的数据与模型产物是运行时输出，不是 API 契约；在线服务状态由存储在 PostgreSQL 中的 active 版本和指标决定。HTTP 服务的 `cmd/recboletrainer` 会从其调度器/容器集成中调用本脚本。
