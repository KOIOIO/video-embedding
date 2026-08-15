# RecBole 推荐链路说明

## 当前定位

当前 `configs/video.yml` 和 `configs/video_prod.yml` 将 `Recommendation.Engine` 设置为 `recbole`。`GET /api/video-segments/random-play` 的个性化路径由 Go 服务从 PostgreSQL 的 `recsys` schema 读取 active RecBole user/item embedding，并使用 pgvector 召回普通视频片段。Gorse 只在显式切换为 `gorse` 或配置 external 候选时参与，不是本链路的模型存储。

在线服务只接受普通数字视频片段 ID。训练目录、RecBole checkpoint、`knowledge_video:<id>` 虚拟 item 和旧 public embedding 表都不是在线候选来源。

## 入口与阶段

RecBole 代码位于仓库根目录的 `recbole-training/`。从仓库根目录运行：

```bash
cd recbole-training
CONFIG_FILE=../video-service/configs/video.yml ./scripts/run_recbole_pipeline.sh
```

从 HTTP 服务目录运行时，路径相应改为 `../recbole-training/`。脚本优先使用 PATH 中的 `export_recbole_dataset`、`export_active_recsys_model_metrics`、`import_recsys_embeddings` 预编译工具；找不到时回退到 HTTP 服务目录中的 `go run ./tools/...`。Python 使用 `PYTHON_BIN`，默认优先项目 `.venv/bin/python`，否则使用 `python3`。

流水线顺序固定为：

1. 从 PostgreSQL 导出 RecBole atomic 文件和导出统计。
2. 强制确认 `<DATASET>.train.inter`、`<DATASET>.valid.inter`、`<DATASET>.test.inter` 均存在，并拒绝 valid/test 目标中的 `knowledge_video:`。
3. 导出当前 active 模型的 baseline metrics。
4. 运行 RecBole 训练和评估，生成 `metrics.json`、checkpoint 与 embedding CSV。
5. `PUBLISH_GATE_ENABLED=true` 时运行 Python publish gate。
6. 确认 item embedding 第一列全部为正的数字视频片段 ID，再导入 `recsys` 并发布模型版本。

任一必需文件、虚拟 item 检查、训练、门禁或 embedding 校验失败时，不会执行最后的导入发布。

## 脚本支持的环境变量

`run_recbole_pipeline.sh` 只读取以下变量；门禁阈值不是环境变量：

| 变量 | 默认值 | 作用 |
| --- | --- | --- |
| `SERVICE_DIR` | 仓库中的 `video-service` | Go 工具和配置所在目录 |
| `CONFIG_FILE` | `${SERVICE_DIR}/configs/video.yml` | 传给导出/导入工具的服务配置 |
| `MODEL_VERSION` | `recbole_<时间戳>` | 本次候选模型版本和目录名 |
| `MODEL_NAME` | `recbole` | 写入 `recsys` 的线上模型名 |
| `RECBOLE_MODEL` | `BPR` | RecBole 模型类 |
| `DATASET` | `_video` | atomic 文件前缀 |
| `DIM` | `64` | user/item embedding 维度 |
| `EPOCHS` | `20` | 训练轮数 |
| `SAMPLE_LIMIT` | `10000` | 导出器查询的最大普通交互行数 |
| `DAYS_BACK` | `30` | 普通交互和知识点视频观看的回溯天数 |
| `PYTHON_BIN` | `.venv/bin/python`，否则 `python3` | 训练和门禁的 Python 可执行文件 |
| `DATA_ROOT` | `data/${MODEL_VERSION}` | 数据集父目录 |
| `DATA_DIR` | `${DATA_ROOT}/${DATASET}` | 本次 atomic 文件目录 |
| `ARTIFACT_DIR` | `artifacts/${MODEL_VERSION}` | checkpoint、指标和 embedding 产物目录 |
| `BASELINE_METRICS` | `${ARTIFACT_DIR}/baseline_metrics.json` | 训练前导出的 baseline 指标文件 |
| `PUBLISH_GATE_ENABLED` | `true` | 是否执行 publish gate |

以下名称不被 shell 脚本消费，不能写成环境配置项：`MIN_RECALL_AT_20`、`MIN_NDCG_AT_20`、`MAX_RELATIVE_NDCG_DROP`、`ARTIFACT_RETENTION_DAYS`。门禁阈值通过 Python CLI flags 传入：`--min-recall-at-20`、`--min-ndcg-at-20`、`--max-relative-ndcg-drop`、`--min-positive-rows`、`--min-positive-users`。

## 导出文件与切分

`tools/export_recbole_dataset` 在 `DATA_DIR` 生成：

```text
<DATASET>.train.inter
<DATASET>.valid.inter
<DATASET>.test.inter
<DATASET>.item
<DATASET>.user
<DATASET>.export_stats.json
```

三个 `.inter` 文件都有 `user_id`、`item_id`、`rating`、`timestamp`、`source`、`weight` 列。普通 item 是数字视频片段 ID；交互来源包括视频 reaction、观看/曝光，以及搜题后的推荐视频跟进。导出器按用户和视频片段去重，保留更高价值的交互。`.user` 汇总用户画像、掌握度、答题、反馈、搜索、专项练习、词汇和英语学习等特征；`.item` 提供视频片段和内容元数据。

切分在 Go 导出器中完成，不是 Python 端的随机切分。每个用户的普通交互按时间排序（同一时间按 item ID 稳定排序）：最后一条进入 test，倒数第二条进入 valid，更早的进入 train；只有一两条交互的用户不会凭空生成验证行。RecBole 配置使用 `benchmark_filename=["train", "valid", "test"]`、按用户分组、时间顺序和 full-sort 评估。

## 知识点视频训练信号

观看会话先按用户、知识点视频和 session 聚合，再按用户与知识点视频聚合。累计观看时长达到视频时长的 `60%` 才算有效观看，并转成训练期虚拟 item，例如 `knowledge_video:88`。

虚拟 item 的边界如下：

| 环节 | 处理 |
| --- | --- |
| train 交互 | 允许作为训练辅助信号 |
| valid/test 目标 | `benchmarkSplit` 排除，脚本也会检查泄漏 |
| full-sort 评估 | 解析出的内部 ID 被遮罩为 `-inf`，不参与排名 |
| item embedding CSV | 仅保留正数字 token，虚拟 token 被过滤 |
| 数据库导入 | 无法通过数字 `video_segment_id` 校验，不会写入 `recsys` |
| 在线候选 | 永远不是普通视频片段候选，不会被推荐 |

因此，`knowledge_video:<id>` 可以出现在 `<DATASET>.item` 训练词表中，但不得进入 valid/test 目标、full-sort 候选、`item_embeddings.csv`、数据库导入或在线推荐。

## 指标、质量门禁与失败处理

训练阶段把 RecBole 结果标准化为 `framework`、`algorithm`、`model_version`、Recall@20、NDCG@20、Hit@20、Precision@20，并把 `<DATASET>.export_stats.json` 中的数字导出/过滤统计合并进 `metrics.json`，另外记录被过滤的虚拟 item embedding 数量。

默认门禁阈值为 Recall@20 `>= 0.01`、NDCG@20 `>= 0.005`，相对 baseline 的 NDCG@20 下降不超过 `20%`。`publish_gate` 还暴露 `--min-positive-rows` 和 `--min-positive-users`，默认均为 `1`，即存在这些字段时要求 `positive_rows >= 1`、`positive_users >= 1`。

当前限制必须注意：`publish_gate.evaluate` 只有在 `metrics.json` 实际包含 `positive_rows` 或 `positive_users` 键时才执行对应检查；当前 Go exporter 的 `export_stats.json` 生成知识点观看和过滤统计，但尚未生成这两个键。因此现有脚本不会保证每次都触发正样本行数/用户数门禁，不能据此增加不存在的环境变量。

门禁失败时脚本在 `import_recsys_embeddings --publish` 之前退出，旧的 active model 保持不变，`ARTIFACT_DIR` 完整保留以便诊断。门禁关闭只跳过 Python gate，不跳过三份 split 文件、虚拟 item 泄漏和数字 embedding ID 校验。

## 在线表与导入边界

导入工具读取：

```text
item_embeddings.csv: video_segment_id,video_id,embedding,model_version
user_embeddings.csv: user_id,embedding,model_version
```

并写入以下 `recsys` 表：

```text
recsys.recommend_model_version
recsys.recommend_user_embedding
recsys.recommend_item_embedding
```

Go 在线推荐只从 `recsys` 读取普通数字视频片段 embedding；不读取训练目录或虚拟 item。Gorse external（如启用）只通过 `/api/internal/recommendations/external/recbole` 获取普通片段候选，最终可播放过滤和曝光仍由 Go 服务负责。

该 `internal` 路径当前直接注册在根 router 上，应用层没有 JWT 或 API key 校验；名称只表示调用约定，不构成安全边界。部署时必须通过反向代理 ACL、网络策略或防火墙只允许 Gorse/受控内部来源访问。

旧 public embedding/version 表不是该流水线的输出。若要清理，先完成备份和验证，再从 HTTP 服务目录运行显式清理工具：

```bash
cd video-service
go run ./tools/drop_legacy_recommendation_tables --execute --confirm drop-legacy-recommendation-tables
```

## 依赖与验证

`recbole-training/requirements.txt` 使用版本约束/范围而非完全锁定（例如 `numpy>=1.24,<2.0`、`pandas>=1.5,<2.2`、`torch>=2.0,<2.4`，RecBole 和 PyYAML 使用最低版本）。本地验证：

```bash
cd recbole-training
PYTHONPATH=src python3 -m unittest discover -s tests -p 'test_*.py'
```

更完整的脚本与算法约定见 [`recbole-training/README.md`](../../recbole-training/README.md) 和 [`ALGORITHM_HANDOFF.md`](../../recbole-training/ALGORITHM_HANDOFF.md)。
