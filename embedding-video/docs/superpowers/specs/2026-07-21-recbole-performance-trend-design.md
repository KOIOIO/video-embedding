# RecBole 推荐性能趋势迁移设计

## 目标

将推荐控制台中的“推荐性能趋势”从 Gorse Dashboard 数据迁移到 RecBole 离线训练评估数据。迁移后 API 服务只从 PostgreSQL 的 `recsys.recommend_model_version` 读取趋势，不依赖 Gorse Dashboard 或训练容器文件系统。

## 数据口径

- 数据源：`recsys.recommend_model_version`。
- 模型范围：`model_name = 'recbole'`、`framework = 'recbole'`、`status = 1`、`deleted = 0`。
- 时间：优先使用 `published_at`，为空时使用 `create_time`。
- 指标：`Recall@20`、`NDCG@20`、`Hit@20`、`Precision@20`。
- 默认指标：`NDCG@20`。
- 指标值：从 `metrics_json` 读取；缺少所选指标或值不是数字的模型不形成数据点。
- 排序：按时间升序；时间为空的模型不形成数据点。

## 后端设计

新增受保护的 REST 接口：

```text
GET /api/admin/recommendation/recbole/performance?metric=NDCG%4020&begin=<RFC3339>&end=<RFC3339>
```

响应保持现有图表易于消费的结构：

```json
{
  "code": 0,
  "data": {
    "metric": "NDCG@20",
    "label": "NDCG@20",
    "available_metrics": [
      { "value": "Recall@20", "label": "Recall@20" },
      { "value": "NDCG@20", "label": "NDCG@20" },
      { "value": "Hit@20", "label": "Hit@20" },
      { "value": "Precision@20", "label": "Precision@20" }
    ],
    "points": [
      {
        "timestamp": "2026-07-21T09:15:00Z",
        "value": 0.35,
        "model_version": "recbole_20260721_171500"
      }
    ]
  }
}
```

应用服务负责默认指标和白名单校验。Repository 负责时间范围过滤、JSONB 指标读取和稳定排序。Handler 继续要求 `begin`、`end` 为 RFC3339 且 `begin <= end`。非法参数返回 400，持久化失败返回 500，无匹配数据返回 200 和空 `points`。

数据点包含 `model_version`，前端悬浮提示可明确一次评估对应的模型，避免同一天多次训练时无法区分。

## 前端设计

- 将组件重命名为 `RecBolePerformanceChart`，保留现有日期范围、指标选择、刷新、加载、空状态、错误重试和折线图交互。
- 默认选择 `NDCG@20`，指标列表与后端固定白名单一致，响应中的 `available_metrics` 仍作为最终展示来源。
- 将 API 方法和 endpoint 改为 RecBole 命名并请求 `/api/admin/recommendation/recbole/performance`。
- 将 helper、CSS 类名、ARIA 标签、加载/错误文案和测试中的 Gorse 命名完整迁移为 RecBole。
- 悬浮提示展示指标值、模型版本和评估时间；摘要继续展示当前值、最高值、数据点数和更新时间。
- 不改变推荐控制台其他区域的布局或交互。

## 删除范围

删除只服务于 Gorse 性能趋势的代码：

- `/api/admin/recommendation/gorse/performance` 路由、Handler、DTO 和应用服务类型/方法。
- Gorse Dashboard client 及其测试。
- 只为 Dashboard 查询增加的配置字段、环境变量和部署配置。
- 前端 `GorsePerformanceChart`、Gorse performance API/helper、对应测试和专用 CSS 命名。
- README、运行手册和 Swagger 路由测试中的旧趋势说明。

保留 Gorse 推荐召回、同步、管理控制台概览及部署所需的其他 Gorse 配置。

## 测试与验收

后端按测试先行实现：

1. Repository 测试覆盖模型过滤、时间回退、JSONB 数值提取、缺失指标跳过和升序结果。
2. 应用服务测试覆盖默认指标、四项白名单和非法指标拒绝。
3. Handler 测试覆盖成功响应、模型版本字段、时间参数校验、非法指标和内部错误。
4. 路由测试确认新接口已注册、旧接口不再注册。

前端测试覆盖：

1. endpoint 与查询参数编码。
2. 数据点规范化时保留 `model_version`。
3. 新组件命名、ARIA、RecBole 文案、默认指标和工作区挂载顺序。
4. 源码中不再引用 Gorse 性能趋势组件、helper 或 endpoint。

最终验证运行后端相关包测试、前端单元测试和前端构建；条件允许时运行 HTTP 服务全量 Go 测试。
