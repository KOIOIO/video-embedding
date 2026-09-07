# RecBole 推荐链路说明

## 当前定位

`GET /api/video-segments/random-play` 是个性化推荐展示入口。当前 `configs/video.yml` 与 `configs/video_prod.yml` 都使用 `Recommendation.Engine=recbole`，Go 服务直接从 `recsys` 读取 active RecBole user/item embedding 做 pgvector 召回。若切换为 `gorse`，Gorse 可以把 `/api/internal/recommendations/external/recbole` 配置为 external 候选源。

## 数据与训练

RecBole 离线训练目录是仓库根目录下的 `recbole-training/`。

流水线入口：

```bash
cd ../recbole-training
CONFIG_FILE=../video-service/configs/video.yml ./scripts/run_recbole_pipeline.sh
```

流水线阶段：

1. `tools/export_recbole_dataset` 从 PostgreSQL 导出 RecBole atomic files：`.inter`、`.item`、`.user`。
2. `.inter` 使用视频 reaction、观看、曝光和题目搜索后的推荐视频跟进事件。
3. `.user` 使用用户在整个系统里的学习行为，包括知识掌握、答题、反馈、搜题、专项练习、单词、英语阅读/听力/绘本和画像快照。
4. `recbole_recommendation.train` 训练 RecBole 模型，默认 `BPR`，输出 user/item embedding。
5. `recbole_recommendation.publish_gate` 对当前指标和上一版 active metrics 做门禁。
6. `tools/import_recsys_embeddings` 导入 `recsys` 并发布 active model version。

## 在线表

新链路只读写 schema `recsys`：

```text
recsys.recommend_model_version
recsys.recommend_user_embedding
recsys.recommend_item_embedding
```

旧 public embedding/version 表不再由服务启动或导入工具创建。确认新链路通过验证并完成备份后，使用显式清理工具删除：

```bash
cd video-service
go run ./tools/drop_legacy_recommendation_tables --execute --confirm drop-legacy-recommendation-tables
```

默认不加 `--execute` 时该工具只输出计划和行数。

## Gorse External

Gorse external 推荐地址应配置为：

```text
GET /api/internal/recommendations/external/recbole?user_id=<id>&n=<limit>
```

该接口只返回候选 `video_segment_id`，不写曝光、不写推荐记录；曝光和最终返回仍由 `random-play` 链路负责。
