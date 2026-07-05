# Gorse 推荐引擎运行手册

## 部署边界

Gorse 和业务 HTTP/worker 解耦部署。默认 `docker-compose.yml` 不启动 Gorse；推荐引擎使用根目录的 `docker-compose.gorse.yml` 单独启动，形态和双塔训练链路一样独立。

业务服务仍是 API 和数据事实源。Gorse 只负责推荐用户、物品、反馈数据集、模型训练和推荐候选服务。

## 本机依赖

当前本机预期已有：

1. Redis，默认 `127.0.0.1:6379`。
2. PostgreSQL，默认复用主服务的 `hengshui-tablet` 数据库。
3. MinIO，可选。当前默认使用 Docker volume 保存 Gorse blob/model 文件；需要对象存储时再切到 MinIO/S3。

Gorse 容器通过 `host.docker.internal` 访问宿主机 Redis/PostgreSQL。

## PostgreSQL Schema 初始化

Gorse 默认和主服务使用同一个 PostgreSQL 实例和 database，但放在独立 schema 下，避免和业务表混在一起。

```sql
CREATE SCHEMA IF NOT EXISTS gorse;
GRANT USAGE, CREATE ON SCHEMA gorse TO postgres;
```

也可以直接执行根目录提供的初始化脚本：

```bash
psql "$POSTGRES_DSN" \
  -f gorse/init-postgres.sql
```

Gorse 连接串通过私有 `.env.local` / `.env.deploy` 注入：

```bash
GORSE_DATA_STORE=postgres://postgres:change-me@host.docker.internal:5432/hengshui-tablet?sslmode=disable&search_path=gorse,public
GORSE_CACHE_STORE=postgres://postgres:change-me@host.docker.internal:5432/hengshui-tablet?sslmode=disable&search_path=gorse,public
```

隔离方式有两层：

1. `search_path=gorse,public`：Gorse 建表优先进入 `gorse` schema。
2. `table_prefix = "hstv_gorse_"`：即使误进同 schema，也能从表名前缀识别。

如需修改 PostgreSQL 地址、账号、密码、库名或 schema，同步修改：

1. 私有 `.env.local` / `.env.deploy` 中的 `GORSE_DATA_STORE` 和 `GORSE_CACHE_STORE`。
2. 私有 `.env.local` / `.env.deploy` 中的 `GORSE_API_KEY` / `GORSE_SERVER_API_KEY`，如果 API key 也改了。

## 启动和停止

在仓库根目录运行：

```bash
cp .env.deploy.example .env.deploy
# 编辑 .env.deploy，填入真实 POSTGRES_DSN、GORSE_* 和 API key
docker compose -f docker-compose.gorse.yml up -d
docker compose -f docker-compose.gorse.yml ps
```

停止：

```bash
docker compose -f docker-compose.gorse.yml down
```

删除 Gorse 本地模型/blob/cache volume：

```bash
docker compose -f docker-compose.gorse.yml down -v
```

## 端口

| 服务 | 默认端口 | 用途 |
| --- | ---: | --- |
| `gorse` | `8087` | Go 服务调用的推荐 REST API |
| `gorse_master` | `8088` | Gorse dashboard/admin HTTP |
| `gorse_master` | `8086` | Gorse 内部 master gRPC |
| `gorse_worker` | `8089` | worker HTTP |

可通过环境变量覆盖端口：

```bash
GORSE_SERVER_PORT=18087 GORSE_DASHBOARD_PORT=18088 docker compose -f docker-compose.gorse.yml up -d
```

## Go 服务接入

本地配置已经默认指向宿主机端口：

```yaml
Gorse:
  Endpoint: "http://localhost:8087"
  APIKey: ""
```

`APIKey` 由私有 `.env.local` / `.env.deploy` 中的 `GORSE_API_KEY` 注入。

生产容器如果和 `docker-compose.gorse.yml` 使用同一个 Docker Compose project/network，可使用：

```yaml
Gorse:
  Endpoint: "http://gorse:8087"
```

若业务服务和 Gorse 分开部署在不同主机，则把 `Gorse.Endpoint` 改成 Gorse server 的实际内网地址。

启用 Gorse 主推荐链路前，至少设置：

```yaml
Recommendation:
  Engine: "gorse"

Gorse:
  SyncEnabled: true
  WriteBackEnabled: false
```

`WriteBackEnabled` 建议等端到端验证通过后再打开。周期同步仍以 PostgreSQL 为事实源，可以修复漏写的 Gorse 反馈。

## 数据同步

先 dry-run 检查同步数据量：

```bash
cd video-service
go run ./tools/sync_gorse_recommendation_data --config configs/video.yml --dry-run
```

确认用户、物品、反馈数量合理后执行真实同步：

```bash
cd video-service
go run ./tools/sync_gorse_recommendation_data --config configs/video.yml
```

启动 worker 且 `Gorse.SyncEnabled=true` 后，会由 `internal/worker/gorsesync` 周期性全量同步。

## 健康检查

```bash
curl -f -H "X-API-Key: ${GORSE_API_KEY}" 'http://localhost:8087/api/recommend/1?n=10'
```

如果用户 `1` 没有足够数据，返回空列表不一定表示服务异常。优先检查：

```bash
docker compose -f docker-compose.gorse.yml logs --tail=100 gorse_master
docker compose -f docker-compose.gorse.yml logs --tail=100 gorse
docker compose -f docker-compose.gorse.yml logs --tail=100 gorse_worker
```

## MinIO/S3 blob 存储

默认配置：

```toml
[blob]
uri = "/var/lib/gorse/blob"
```

需要切到本机 MinIO 时，改成类似：

```toml
[blob]
uri = "s3://gorse/blob"

[blob.s3]
endpoint = "http://host.docker.internal:9000"
access_key_id = "change-me"
secret_access_key = "change-me"
```

先确保 MinIO 中存在 bucket `gorse`。

## 回滚

业务回滚只需要关闭 Go 服务侧 Gorse：

```yaml
Recommendation:
  Engine: "knowledge_match"

Gorse:
  SyncEnabled: false
  WriteBackEnabled: false
```

然后重启业务 HTTP/worker。Gorse 服务可继续保留，也可单独停止：

```bash
docker compose -f docker-compose.gorse.yml down
```

这不会删除 PostgreSQL 业务数据，也不会影响双塔训练目录。
