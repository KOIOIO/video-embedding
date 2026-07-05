# 双塔推荐算法对接协议

本文档面向负责优化双塔推荐模型的算法同事。目标是说明算法侧可以改什么、必须遵守哪些输入输出协议，以及如何把新模型接入现有 Go 服务。

## 1. 对接目标

当前项目已经跑通最小双塔链路：

```text
用户行为数据
  -> 导出训练样本 CSV
  -> Python 训练双塔模型
  -> 生成 item/user embedding 产物
  -> Go 工具导入数据库
  -> 发布 active model_version
  -> /api/video-segment/random-play 使用 active embedding 做推荐
```

算法同事主要负责优化 `two-tower-training/` 下的训练逻辑，包括样本处理、特征构造、模型结构、损失函数、负采样、评估指标等。只要保持本文档定义的输入输出协议不变，线上 Go 服务无需感知算法内部实现。

## 2. 当前目录边界

```text
two-tower-training/
  README.md                         # 训练目录说明
  ALGORITHM_HANDOFF.md              # 本对接协议
  Dockerfile                        # 训练容器环境：Go + Python
  scripts/run_two_tower_pipeline.sh # 训练发布流水线
  src/two_tower_training/           # Python 训练代码
  tests/                            # Python 训练测试
  data/                             # 临时样本文件，流水线成功后会清理
  artifacts/                        # 模型产物，默认保留 7 天
```

建议算法优化优先集中在：

- `src/two_tower_training/train.py`
- `tests/`
- 必要时新增 `src/two_tower_training/*.py`

不建议算法侧直接修改：

- `video-service/internal/`
- `video-service/tools/import_two_tower_embeddings`
- 数据库表结构
- 在线推荐接口协议

如果确实需要改动这些边界，需要先和后端确认。

## 3. 训练流水线协议

完整训练发布入口是：

```bash
cd two-tower-training
./scripts/run_two_tower_pipeline.sh
```

流水线步骤固定为：

```text
1. go run ./tools/export_two_tower_samples --item-output --user-output
2. python3 -m two_tower_training.train --user-features
3. go run ./tools/export_active_recommend_model_metrics
4. python3 -m two_tower_training.publish_gate
5. go run ./tools/import_two_tower_embeddings --publish
6. 清理 data/*.csv 和临时 baseline metrics
7. 清理 artifacts/ 下超过保留期的旧模型目录
```

可用环境变量：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `CONFIG_FILE` | `configs/video.yml` | Go 工具读取数据库配置 |
| `MODEL_VERSION` | `two_tower_yyyyMMdd_HHmmss` | 本次模型版本，必须贯穿所有产物 |
| `SAMPLE_LIMIT` | `10000` | 导出训练样本上限 |
| `SEED_COUNT` | `0` | 是否生成合理模拟行为样本 |
| `DIM` | `64` | 训练 embedding 维度；导入时必须与线上表维度一致 |
| `EPOCHS` | `60` | 训练轮数 |
| `LEARNING_RATE` | `0.01` | 学习率 |
| `L2` | `0.001` | L2 正则 |
| `SEED` | `42` | 随机种子 |
| `BACKEND` | `torch` | 训练后端，可选 `torch` 或兼容回退 `sgd` |
| `BATCH_SIZE` | `128` | PyTorch batch size |
| `RANDOM_NEGATIVES` | `3` | 每个正样本追加的随机负样本数 |
| `HARD_NEGATIVES` | `2` | 每个正样本追加的 batch 内 hard negatives 数 |
| `EVAL_RATIO` | `0.15` | 按时间留作评估集的样本比例 |
| `RETRIEVAL_K` | `20` | 召回评估使用的 Top K |
| `RETRIEVAL_KS` | `20,50` | 训练后输出的多个 Top K 指标 |
| `HALF_LIFE_DAYS` | `30` | 样本时间衰减半衰期天数 |
| `PUBLISH_GATE_ENABLED` | `true` | 是否在发布前执行指标门禁 |
| `MIN_EVAL_AUC` | `0.55` | 最低评估集 AUC |
| `MIN_RECALL_AT_20` | `0.05` | 最低 Recall@20 |
| `MIN_COVERAGE_AT_20` | `0.02` | 最低 Coverage@20 |
| `MAX_NEGATIVE_HIT_RATE_AT_20` | `0.50` | 最高负样本命中率 |
| `MIN_EVAL_SAMPLE_COUNT` | `20` | 最少评估样本数 |
| `MAX_AUC_DROP` | `0.02` | 相比上一版 active model 允许的最大 AUC 下降 |
| `MAX_RECALL_DROP` | `0.05` | 相比上一版 active model 允许的最大 Recall 下降 |
| `MAX_COVERAGE_DROP` | `0.05` | 相比上一版 active model 允许的最大 Coverage 下降 |
| `MAX_NEGATIVE_HIT_RATE_INCREASE` | `0.05` | 相比上一版 active model 允许的最大负样本命中率上升 |
| `MAX_DISLIKE_HIT_RATE_INCREASE` | `0.05` | 相比上一版 active model 允许的最大点踩命中率上升 |
| `ARTIFACT_RETENTION_DAYS` | `7` | 训练产物保留天数 |

`MIN_RECALL_AT_20`、`MIN_COVERAGE_AT_20` 和 `MAX_NEGATIVE_HIT_RATE_AT_20` 保留原变量名用于兼容；实际读取的指标后缀由 `RETRIEVAL_K` 决定。

生产训练容器会使用 `video_prod.yml`，并按北京时间 `00:00`、`04:00`、`08:00`、`12:00`、`16:00`、`20:00` 定时执行。

## 4. 输入样本协议

训练脚本输入文件由 Go 工具生成：

```bash
go run ./tools/export_two_tower_samples \
  --config "${CONFIG_FILE}" \
  --limit "${SAMPLE_LIMIT}" \
  --seed-count "${SEED_COUNT}" \
  --output "${SAMPLE_FILE}" \
  --item-output "${ITEM_FILE}" \
  --user-output "${USER_FILE}"
```

CSV 表头固定为：

```csv
user_id,video_id,video_segment_id,label,weight,source,reason,event_time
```

字段含义：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `user_id` | uint64 | 是 | 用户 ID，来自 `sys_user` |
| `video_id` | uint64 | 是 | 视频 ID，必须存在且未删除 |
| `video_segment_id` | uint64 | 是 | 片段 ID，来自未删除的 `edu_video_segment` |
| `label` | 0/1 | 是 | 训练标签，1 表示正样本，0 表示负样本 |
| `weight` | float | 是 | 样本权重，必须大于 0 |
| `source` | string | 是 | 行为来源 |
| `reason` | string | 是 | 标签或权重来源说明 |
| `event_time` | timestamp | 是 | 行为发生时间 |

当前 `source` 可能值：

| source | 来源表 | 典型含义 |
| --- | --- | --- |
| `segment_reaction` | `edu_user_reaction` | 用户对片段点赞、超级点赞、点踩 |
| `video_reaction` | `edu_video_user_reaction` | 用户对视频点赞、超级点赞、点踩 |
| `watch` | `edu_user_video_recommend` | 推荐后产生观看行为 |
| `exposure` | `edu_recommend_exposure` | 推荐曝光、点击、观看 |

算法侧可以基于 `source`、`reason`、`weight`、`event_time` 做更细粒度样本处理，例如：

- 按时间衰减样本权重
- 对 `double_like` 提高正样本权重
- 对曝光未点击构造弱负样本
- 对观看时长占片段时长较高的样本提高权重
- 对同一用户同一片段的多条行为做聚合

必须保持：

- 不丢失 `user_id`
- 不丢失 `video_id`
- 不丢失 `video_segment_id`
- 不生成数据库中不存在的用户、视频、片段 ID
- 不把已删除视频或片段作为最终 item embedding 导出目标

## 5. Item Catalog 协议

流水线会同时导出全量有效片段目录：

```bash
go run ./tools/export_two_tower_samples \
  --config "${CONFIG_FILE}" \
  --limit "${SAMPLE_LIMIT}" \
  --seed-count "${SEED_COUNT}" \
  --output "${SAMPLE_FILE}" \
  --item-output "${ITEM_FILE}" \
  --user-output "${USER_FILE}"
```

CSV 表头固定为：

```csv
video_segment_id,video_id,segment_duration,video_duration,like_count,double_like_count,dislike_count,content_summary,knowledge_tags,video_title
```

用途：

- 训练样本中未出现过的有效片段也会生成 item embedding，避免新片段完全无法被双塔召回。
- PyTorch item tower 会把 `segment_id embedding` 与 item feature MLP 相加后归一化。
- 内容摘要、知识标签、标题当前使用稳定 hash bucket 进入 feature vector；后续可以替换为真实文本 embedding，但输出协议不变。

## 6. User Feature 协议

流水线会同时导出有效用户的学习画像：

```bash
go run ./tools/export_two_tower_samples \
  --config "${CONFIG_FILE}" \
  --limit "${SAMPLE_LIMIT}" \
  --seed-count "${SEED_COUNT}" \
  --output "${SAMPLE_FILE}" \
  --item-output "${ITEM_FILE}" \
  --user-output "${USER_FILE}"
```

CSV 表头固定为：

```csv
user_id,grade_id,class_id,user_type,mastery_avg,mastery_min,weak_knowledge_count,strong_knowledge_count,knowledge_correct_count,knowledge_incorrect_count,answer_count,answer_correct_count,answer_incorrect_count,avg_score_rate,avg_cost_seconds,question_feedback_count,generated_feedback_count,generated_correct_count,generated_avg_score_rate,question_search_count,recent_knowledge_point_ids,recent_subjects,question_search_knowledge_text,generated_feedback_knowledge_text
```

用途：

- PyTorch user tower 会把 `user_id embedding` 与 user feature MLP 相加后归一化。
- 用户基础属性来自 `sys_user`，学习状态来自知识点掌握、答题记录、题目反馈、生成题反馈和搜题记录。
- 缺少某个可选教育表或用户没有学习画像时，训练会使用中性数值或零向量兜底，输出 embedding 协议不变。

## 7. 模型与损失

默认训练后端是 PyTorch：

```text
user tower: user_id embedding + user feature MLP
item tower: segment_id embedding + item feature MLP
loss: pairwise softplus ranking loss
negative sampling: 原始负样本 + 随机负样本 + batch 内其他正样本 hard negatives；过滤当前用户已知正样本
```

保留 `sgd` 后端只是为了回退和对照，不建议作为生产默认值。样本处理会基于 `source/reason/event_time` 做：

- 同一用户同一片段多行为聚合。
- 时间衰减。
- `double_like`、长观看提高权重。
- 曝光未点击作为弱负样本。
- 点踩、极短观看作为强负样本。

## 8. 输出产物协议

训练脚本必须把产物写入：

```text
two-tower-training/artifacts/${MODEL_VERSION}/
```

必需文件：

```text
item_embeddings.csv
user_embeddings.csv
metrics.json
```

可选文件：

```text
segment_id_map.json
user_id_map.json
其他算法调试文件
```

### 8.1 item_embeddings.csv

表头必须固定：

```csv
video_segment_id,video_id,embedding,model_version
```

示例：

```csv
101,20,"[0.012300,-0.045600,0.078900,...]",two_tower_20260623_003000
```

约束：

- `video_segment_id` 必须大于 0
- `video_id` 必须大于 0
- `embedding` 使用方括号包裹，逗号分隔
- 当前维度必须等于 `DIM`，生产默认是 64
- `model_version` 必须和本次 `MODEL_VERSION` 完全一致
- 同一 `model_version` 下，一个 `video_segment_id` 只能出现一次

导入后写入：

```text
edu_video_item_embedding.embedding
```

### 8.2 user_embeddings.csv

表头必须固定：

```csv
user_id,embedding,model_version
```

示例：

```csv
7,"[0.011100,0.022200,-0.033300,...]",two_tower_20260623_003000
```

约束：

- `user_id` 必须大于 0
- `embedding` 维度必须等于 `DIM`
- `model_version` 必须和本次 `MODEL_VERSION` 完全一致
- 同一 `model_version` 下，一个 `user_id` 只能出现一次

导入后写入：

```text
edu_user_tower_embedding.tower_vector
```

### 8.3 metrics.json

`metrics.json` 用于记录训练质量和排查信息。至少建议包含：

```json
{
  "model_version": "two_tower_20260623_003000",
  "dim": 64,
  "epochs": 60,
  "sample_count": 10000,
  "train_sample_count": 8000,
  "eval_sample_count": 2000,
  "user_count": 1000,
  "item_count": 5000,
  "train_loss": 0.41,
  "eval_loss": 0.48,
  "eval_auc": 0.71,
  "recall_at_20": 0.18,
  "hit_rate_at_20": 0.24,
  "ndcg_at_20": 0.16,
  "coverage_at_20": 0.31,
  "negative_hit_rate_at_20": 0.04,
  "dislike_hit_rate_at_20": 0.02,
  "recall_at_50": 0.29,
  "hit_rate_at_50": 0.38,
  "ndcg_at_50": 0.21,
  "coverage_at_50": 0.45,
  "negative_hit_rate_at_50": 0.05,
  "dislike_hit_rate_at_50": 0.03
}
```

算法侧可以增加指标，例如：

- `recall_at_20` / `recall_at_50`
- `hit_rate_at_20` / `hit_rate_at_50`
- `ndcg_at_20` / `ndcg_at_50`
- `coverage_at_20` / `coverage_at_50`
- `dislike_hit_rate_at_20` / `dislike_hit_rate_at_50`
- `train_start_time`
- `train_end_time`
- `feature_version`
- `negative_sampling`

导入工具会把 `metrics.json` 原文写入：

```text
edu_recommend_model_version.metrics_json
```

Release rule:

1. Training can generate artifacts even when metrics are weak.
2. Publishing active `model_version` requires the publish gate to pass.
3. If the gate fails, keep the artifact for review and leave the previous active model in place.
4. Raising or lowering gate thresholds must be done through pipeline environment variables and recorded in the release note.

## 8. 发布协议

导入命令：

```bash
cd video-service
go run ./tools/import_two_tower_embeddings \
  --config "${CONFIG_FILE}" \
  --artifact-dir "${ARTIFACT_DIR}" \
  --dim "${DIM}" \
  --publish
```

导入工具会做：

```text
1. 校验 item_embeddings.csv 和 user_embeddings.csv
2. 校验 embedding 维度
3. 校验产物中只有一个 model_version
4. upsert item embedding
5. upsert user embedding
6. 写入 edu_recommend_model_version
7. 将当前 model_version 标记为 two_tower active
```

线上推荐只读取 active 版本：

```text
edu_recommend_model_version
  model_name = 'two_tower'
  is_active = true
  status = 1
  deleted = 0
```

如果导入失败，不能发布新版本，线上继续使用旧 active 版本。

Publish gate 发布规则：

1. 绝对阈值必须通过：`eval_auc`、`recall_at_K`、`coverage_at_K`、`negative_hit_rate_at_K`、`eval_sample_count`。
2. 相对上一版 active model 的退化必须在允许范围内。
3. `dislike_hit_rate_at_K` 上升超过阈值时禁止发布。
4. Gate 失败时保留 artifact，跳过 `--publish`，线上继续使用上一版 active model。

## 9. 在线消费协议

`/api/video-segment/random-play` 是双塔推荐的主要展现路径。

在线推荐依赖：

```text
edu_user_tower_embedding
edu_video_item_embedding
edu_recommend_model_version
```

基本逻辑：

```text
1. 获取 active model_version
2. 获取当前 user_id 的 user tower embedding
3. 在同一 model_version 下召回 video_segment embedding
4. 按向量距离和业务规则排序
5. 写入 edu_recommend_exposure 曝光日志
6. 用户后续点击、观看、点赞、点踩会形成下一轮训练样本
```

算法侧不能改变线上接口返回结构。算法优化应通过 embedding 质量、覆盖率和排序质量体现。

## 10. 后续算法可优化方向

当前版本已经完成离线评估、发布门禁、样本聚合/时间衰减、全量 item catalog、PyTorch 后端、pairwise ranking loss、安全负采样、item tower 基础特征。后续建议继续优化：

```text
user tower: user_id embedding + 最近 N 个正负反馈 + 最近兴趣向量
item tower: segment_id embedding + 文本 embedding + knowledge tags + 学科/时长/热度
loss: sampled softmax / BPR / listwise objective
```

1. 样本层
   - 引入更长窗口的用户行为序列
   - 对不同学科、年级、活跃度分桶校准权重
   - 采样时控制热门内容占比，降低 popularity bias

2. 特征层
   - 用真实文本 embedding 替换当前稳定 hash bucket
   - 题目知识点或学科标签
   - 用户历史偏好统计和观看比例统计
   - 最近行为序列

3. 模型层
   - user tower MLP 或轻量 sequence encoder
   - user tower 融合行为序列
   - sampled softmax / BPR 与当前 BCE 对照实验

4. 评估层
   - 按用户分桶看冷启动、活跃用户、新用户
   - 按新内容、低曝光内容、学科分桶看 Recall@K 和 Coverage@K
   - 增加线上 A/B 指标和离线指标相关性回归

## 11. 强约束

以下约束不能破坏：

- embedding 维度必须和 `DIM` 一致，当前生产默认 64
- `item_embeddings.csv` 和 `user_embeddings.csv` 中的 `model_version` 必须一致
- `video_segment_id` 必须对应真实、未删除片段
- `video_id` 必须对应真实、未删除视频
- `user_id` 必须来自 `sys_user`
- 不允许把训练临时文件写到项目根目录
- 不允许改变 Go 导入工具要求的 CSV 表头
- 不允许在未验证指标时发布 active 版本
- 不允许直接修改生产数据库 embedding 表绕过导入工具

## 12. 本地验证清单

算法改动后至少执行：

```bash
cd two-tower-training
PYTHONPATH=src python3 -m unittest discover -s tests
```

用本地或测试库跑完整 pipeline：

```bash
CONFIG_FILE=../video-service/configs/video.yml \
MODEL_VERSION=two_tower_algo_test \
SAMPLE_LIMIT=2000 \
SEED_COUNT=500 \
DIM=64 \
EPOCHS=10 \
./scripts/run_two_tower_pipeline.sh
```

如果只验证产物格式，可以先训练后 dry-run 导入：

```bash
cd ../video-service
go run ./tools/import_two_tower_embeddings \
  --config configs/video.yml \
  --artifact-dir ../two-tower-training/artifacts/two_tower_algo_test \
  --dim 64 \
  --dry-run
```

通过标准：

- Python 单测通过
- 训练脚本能生成三个必需产物
- `item_embeddings.csv` 行数大于 0
- `user_embeddings.csv` 行数大于 0
- `metrics.json` 包含本次 `model_version`
- dry-run 导入通过
- 如果执行 publish，数据库 active model_version 更新为本次版本

## 13. 交付要求

算法同事每次提交新模型方案时，至少说明：

```text
model_version:
训练数据时间范围:
样本数:
用户数:
片段数:
embedding dim:
主要模型变化:
主要样本变化:
关键指标:
是否建议发布:
风险说明:
```

建议在 `metrics.json` 中同步写入这些信息，便于线上问题回溯。

## 14. 后端对接人需要关注

当算法侧需要以下变化时，需要后端配合：

- embedding 维度从 64 改为其他值
- 输出 CSV 字段变更
- 增加新的线上召回表
- 需要在推荐接口使用额外特征
- 需要改训练样本导出 SQL
- 需要把模型服务化，而不是只导出 embedding

除此之外，算法优化应尽量局限在 `two-tower-training/` 内完成。
