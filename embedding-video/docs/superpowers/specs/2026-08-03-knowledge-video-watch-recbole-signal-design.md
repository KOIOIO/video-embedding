# 学科视频观看状态与 RecBole 辅助信号设计

## 目标

为学科视频提供可幂等上报的观看状态接口，记录用户跨播放会话累计的实际观看时长，并将达到视频时长 60% 的有效观看作为训练期辅助交互纳入现有 RecBole 全量迭代。线上仍只推荐普通视频片段，不直接推荐学科视频，也不改变 `GET /api/video-segments/random-play` 的返回契约。

目标仓库是公司项目 `legacy-video`。实现必须保留工作树中已有的部署与 RecBole 数据质量改动，并在其基础上增量修改。

## 已确认口径

- 学科视频指现有 `knowledgevideo` 模块及 `edu_knowledge_video` 数据。
- 学科视频行为只用于改善现有普通视频片段推荐。
- 观看时长采用播放器实际播放时间，不采用当前播放位置，拖动进度条不增加时长。
- 有效观看阈值为累计观看时长达到服务端视频时长的 60%。
- 同一用户对同一视频的多个播放会话累计，累计值封顶为视频总时长。
- 用户身份沿用请求体 `user_id`，服务端验证用户存在。
- 观看数据在下一次每小时 `:15` 的 RecBole 全量训练中生效，接口本身不触发训练。

## 现状与约束

现有 `POST /api/knowledge-videos/{knowledgeVideoId}/playbacks` 只接收 `user_id`，每次实际播放向 `edu_knowledge_video_play_record` 追加一条开始记录。该表只有用户、知识点、学科视频和创建时间，不能表达观看时长、跨会话进度或有效观看。

现有 RecBole item 固定为 `edu_video_segment.id`。训练产物、embedding 导入表和在线召回 SQL 都以 `video_segment_id` 为契约。`knowledge_video_id` 不能直接作为普通 item ID，否则可能与片段 ID 冲突，并且无法通过在线 JOIN 解析。当前 BPR 也不会使用 `.user` 中除 `user_id` 之外的业务特征，因此仅增加用户统计字段不能影响模型。

## 总体架构

系统沿用现有 HTTP 服务、PostgreSQL、RecBole 定时训练和 pgvector 在线召回边界：

1. 前端播放器为每个播放会话生成 UUID，并周期上报该会话的绝对累计观看秒数。
2. HTTP 服务校验用户和视频，按会话幂等保存进度，计算跨会话总时长和 60% 有效观看状态。
3. RecBole 数据导出器把有效学科视频观看转换为带命名空间的训练期虚拟 item，例如 `knowledge_video:88`。
4. 虚拟 item 只进入训练集；验证、测试、评估候选和发布的 item embedding 只允许普通视频片段。
5. 用户 embedding 可以吸收学科视频共同行为，在线 random-play 继续使用原有普通片段召回链路。

## 观看状态接口

### 请求

新增幂等接口：

```http
PUT /api/knowledge-videos/{knowledgeVideoId}/watch-sessions/{sessionId}
Content-Type: application/json

{
  "user_id": 6,
  "watched_seconds": 37
}
```

约束：

- `knowledgeVideoId` 和 `user_id` 必须为正整数。
- `sessionId` 必须为 16 至 64 位，仅允许字母、数字、`-` 和 `_`。前端优先使用 UUID。
- `watched_seconds` 必须为非负整数，表示当前会话截至本次请求的实际累计观看秒数。
- 客户端不提交视频总时长、观看比例或有效观看状态。这些值由服务端使用持久化视频时长计算。

### 响应

```json
{
  "success": true,
  "data": {
    "session_id": "8a6d9d33-78f3-4c45-9f74-38a7409f082b",
    "session_watched_seconds": 37,
    "total_watched_seconds": 73,
    "duration_seconds": 100,
    "progress_ratio": 0.73,
    "effective_watch": true
  }
}
```

`session_watched_seconds` 和 `total_watched_seconds` 均不超过视频总时长。`progress_ratio` 位于 `[0, 1]`，`effective_watch` 等价于 `total_watched_seconds * 100 >= duration_seconds * 60`，使用整数比较避免浮点边界误差。

### 幂等与并发

同一 `(user_id, knowledge_video_id, session_id)` 的上报使用 upsert。会话时长取数据库已有值和本次值的最大值，因此请求重试、并发请求和乱序到达不会降低或重复增加进度。跨会话总时长按每个会话的最大时长求和，并封顶为视频总时长。

现有 `POST /api/knowledge-videos/{knowledgeVideoId}/playbacks` 保持兼容，不改变请求和响应。旧接口产生的无会话、无时长记录不计入有效观看和 RecBole 辅助交互。

## 数据模型与迁移

在现有 `edu_knowledge_video_play_record` 上追加：

- `session_id VARCHAR(64)`：新观看会话标识；旧记录保持为空。
- `watch_duration INT NOT NULL DEFAULT 0`：该会话已确认的最大实际观看秒数。
- `update_time TIMESTAMP`：最近一次成功进度上报时间。

新增部分唯一索引：

```sql
CREATE UNIQUE INDEX IF NOT EXISTS uk_knowledge_video_play_session
ON edu_knowledge_video_play_record(user_id, knowledge_video_id, session_id)
WHERE session_id IS NOT NULL AND session_id <> '';
```

迁移为追加式变更，不删除、不重写旧记录。应用层在保存前验证 `sys_user` 中存在该用户，并确认学科视频存在、未删除、状态为 ready 且 `duration > 0`。

## 前端上报

学科视频工作区为每个播放器实例和视频创建独立观看上下文，包含 `knowledgeVideoId`、`sessionId` 和最近一次成功上报秒数。

播放器按实际处于播放状态的墙钟时间累计观看时长。以下时机触发上报：

- 连续播放期间每 15 秒一次。
- 暂停时。
- 播放结束时。
- 播放器组件卸载时，使用 `keepalive` 做最后一次尽力上报。

只有请求成功后才推进前端的最近成功上报秒数；失败后下一次心跳继续上报更大的绝对值。服务端 `GREATEST` 语义负责处理重试和乱序。拖动、跳转和改变播放位置只更新 UI，不增加实际观看时长。

## RecBole 数据接入

### 虚拟 Item

导出器聚合用户对学科视频的观看会话。仅当跨会话累计观看达到 60% 时，生成一条正交互：

```text
user_id=<user>
item_id=knowledge_video:<knowledge_video_id>
source=knowledge_video_watch
timestamp=<latest session update time>
```

短观看不写入 BPR 交互文件，只进入导出统计。原因是当前 BPR 把出现于交互文件中的记录视为正反馈，不能依赖 `rating` 或 `weight` 把短观看表达为负样本。

每个用户和学科视频最多生成一条聚合交互。只有最近一次会话更新时间仍在当前 `DAYS_BACK` 窗口内的聚合记录进入本次训练。

### 预切分

数据集改为 RecBole 支持的 benchmark 预切分文件：

```text
<dataset>.train.inter
<dataset>.valid.inter
<dataset>.test.inter
```

普通视频片段交互继续按用户和时间顺序切分。学科视频虚拟交互只追加到 train 文件，不得成为 valid 或 test 的目标。RecBole 配置使用 `benchmark_filename: [train, valid, test]`，保持固定种子和现有排名指标。

### 评估候选

即使虚拟 item 只作为训练目标，RecBole 的全量排序默认仍可能把它作为评估候选。训练代码必须在 full-sort 评估时屏蔽所有 `knowledge_video:` token，使其得分不可进入 Top-K。发布门禁使用屏蔽后的普通视频片段指标，避免辅助 item 改变线上指标口径。

### 产物过滤

用户 embedding 按现有格式全部导出。item embedding 只接受可严格解析为正整数的普通 `video_segment_id`；所有 `knowledge_video:` token 必须被过滤。最终 `item_embeddings.csv`、embedding 导入器、`recsys` 表和线上 SQL 均保持现有契约。

## 统计与发布门禁

导出和 `metrics.json` 增加以下统计：

- 学科视频会话行数。
- 有效观看与短观看的用户-视频聚合数。
- 产生有效观看的用户数和学科视频数。
- 加入训练集的虚拟 item 交互数。
- 从 item embedding 产物中成功过滤的虚拟 item 数。

没有学科视频有效观看时，流水线退化为原有行为，不额外阻止发布。以下情况必须使本次流水线失败，并保持 active model 不变：

- 预切分文件缺失或顺序错误。
- `knowledge_video:` 出现在 valid/test 目标中。
- 评估候选未屏蔽虚拟 item。
- 最终 item embedding CSV 中仍存在虚拟 token。
- 现有 Recall/NDCG、active baseline 或数据质量门禁失败。

## 错误处理

- 非法路径参数、会话 ID、用户 ID 或负观看时长返回 `400`。
- 用户或视频不存在返回 `404`。
- 视频未 ready 或持久化时长无效返回 `409`。
- 数据库和内部错误返回 `500`。
- 观看上报失败不阻塞视频播放；前端保留待重试的绝对进度。
- RecBole 导出、训练、评估或过滤失败时不导入候选产物，不更新 active model。

## 测试策略

### Go HTTP 与应用层

- 合法 PUT 请求和完整响应映射。
- 非法 session ID、负时长、用户不存在、视频不存在和视频未就绪。
- 60% 整数边界、视频时长封顶和跨会话累计。
- 同会话重复与乱序请求保持最大值。

### Go 持久化与导出器

- 部分唯一索引不影响旧空 session 记录。
- 同会话 upsert、不同会话求和和并发更新语义。
- 59.9% 不导出，60% 导出。
- 虚拟 token 格式、每用户视频去重、时间窗口和统计计数。
- 虚拟交互只存在于 train 文件，普通交互按时间进入 train/valid/test。

### 前端

- API URL、请求体和 keepalive 配置。
- 每个播放器会话生成独立 UUID。
- 15 秒心跳、暂停、结束和卸载上报。
- 播放时间累计与 seek 隔离。
- 请求失败后能够使用更大的绝对值重试。

### Python 与流水线

- benchmark 预切分配置。
- full-sort 虚拟 item 屏蔽。
- item embedding 导出过滤与严格数字 token 校验。
- 统计字段写入 `metrics.json`。
- 虚拟 item 泄漏时流水线失败，发布门禁失败时 active model 不变。

## 验收场景

对于时长 100 秒的学科视频，同一用户在两个会话中分别累计观看 20 秒和 40 秒。重复和乱序上报后，服务端返回总观看时长 60 秒、进度 0.6、`effective_watch=true`。下一次定时训练在 train 文件中生成一个 `knowledge_video:<id>` 正交互；valid/test、评估 Top-K 和最终 item embedding CSV 中均不存在该 token。线上 random-play 仍只返回普通视频片段。

## 非目标

- 不直接推荐学科视频。
- 不新增学科视频在线召回或返回 DTO。
- 不把知识点名称或文本相似度强行映射为普通片段正反馈。
- 不改为实时训练、增量学习或上报即训练。
- 不在本次工作中替换 RecBole 模型或实现通用多任务推荐框架。
