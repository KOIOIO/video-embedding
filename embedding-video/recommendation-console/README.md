# Recommendation Console

业务推荐控制台前端工程。当前已接入 `/api/admin/recommendation/*` 的只读管理能力：推荐概览、数据源健康、命中效果变化、链路 Trace、Redis 播放桶状态、random-play 预览、题目推荐预览。

## Scripts

```bash
npm install
npm run dev
npm run build
npm run test
```

默认开发端口是 `5174`，`/api` 和 `/videos` 会通过 Vite proxy 转发到 `VITE_PROXY_TARGET`，未设置时为 `http://localhost:8081`。

## 模块

- `src/App.vue`：控制台主布局、概览加载、数据源视图、命中效果视图、链路 Trace、Redis 状态、预览表单。
- `src/components/PreviewTable.vue`：推荐预览结果表。
- `src/config/navigation.js`：页面导航和默认链路行。
- `src/api/http.js`：JSON 请求 helper。
- `src/api/recommendationConsole.js`：推荐控制台 API helper。

## 已接入接口

- `GET /api/admin/recommendation/overview`
- `GET /api/admin/recommendation/datasources`
- `GET /api/admin/recommendation/effects?days=14`
- `GET /api/admin/recommendation/trace/random-play?user_id=7&limit=5`
- `POST /api/admin/recommendation/trace/by-question`
- `GET /api/admin/recommendation/redis-state?user_id=7`
- `GET /api/admin/recommendation/preview/random-play?user_id=7&limit=5`
- `POST /api/admin/recommendation/preview/by-question`

## 后续接入方向

- 链路 Trace：继续下钻候选召回、过滤过程、命中原因。
- 用户调试：曝光、观看、reaction 明细。
- 管理动作：Gorse dry-run/sync、Redis 桶清理、模型版本查看。
