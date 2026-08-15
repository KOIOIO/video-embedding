# 衡桃视频服务部署手册

## 交付模式

部署包提供三套拓扑：

- `standalone`：当前部署方式，API、worker、RecBole trainer 和前端在同一台服务器运行，PostgreSQL 与 Redis 使用当前地址。
- `cloud`：云服务器 `services.example.internal` 只运行 API，并连接同机已有的 PostgreSQL 和 Redis。
- `intranet`：当前服务器运行前端、worker 和 RecBole trainer，并连接云端 API、PostgreSQL 和 Redis。

部署包不包含 PostgreSQL、Redis 或 Gorse 镜像。推荐引擎使用 RecBole，部署配置已关闭 Gorse 同步和回写。

所有对象存储端点、Bucket 和转码参数来自交付包中的 `config/video_prod.yml`。对象存储密钥只通过环境文件注入。

拓扑与敏感配置的归属：

| 拓扑 | 运行服务 | 需要 `JWT_SECRET` 的环境文件 | 说明 |
| --- | --- | --- | --- |
| `standalone` | API、worker、RecBole trainer、frontend | `env/standalone.env` | API 对外提供管理 JWT；worker 禁用 trainer，独立 trainer 设为 `RECBOLE_TRAINER_ENABLED=true` |
| `cloud` | API | `env/cloud.env` | 仅 API 注入 JWT；连接宿主机 PostgreSQL/Redis |
| `intranet` | worker、RecBole trainer、frontend | 不需要 | trainer 由 Compose 注入 `RECBOLE_TRAINER_ENABLED=true`，前端通过 `API_UPSTREAM` 访问 cloud API |

## 服务器要求

- Debian x86_64 / `linux/amd64` 是当前交付默认平台
- Docker Engine 24 或更高版本
- Docker Compose v2
- 至少 20 GB 可用磁盘；离线镜像和 RecBole 依赖占用较大
- 服务器时区建议设置为 `Asia/Shanghai`

## 离线镜像包

`SHA256SUMS` 由 `package.sh` 写在归档旁边，不在归档内部。解压前，把校验文件及其中列出的所有归档放在同一交付目录并执行：

```bash
cd /path/to/delivery-directory
sha256sum -c SHA256SUMS
```

macOS 没有 `sha256sum` 时使用 `shasum -a 256 -c SHA256SUMS`。缺少文件或校验失败时不要解压、加载或启动镜像。

校验通过后解压离线镜像归档，进入 `video-deploy/deployment`：

```bash
./scripts/load-images.sh
./scripts/init-env.sh standalone
```

编辑 `env/standalone.env`，替换全部 `change-me`。不要把真实环境文件提交回 Git 或发送到公共渠道。

## 源码部署包

源码包同样应先在归档旁验证 `SHA256SUMS`，再解压。包内包含 `source/` 构建上下文；进入 `video-deploy/deployment` 后执行：

```bash
./scripts/build-images.sh
./scripts/init-env.sh standalone
```

`build-images.sh` 默认生成 `linux/amd64` 镜像，也支持在有对应 Buildx builder 时覆盖：

```bash
VIDEO_APP_PLATFORM=linux/arm64 ./scripts/build-images.sh
```

`package.sh images` 的离线交付包仍固定检查并命名为 `linux/amd64`；需要其他平台时直接运行 `build-images.sh`，不要把非 amd64 镜像放进标准离线包。

`tests/validate-layout.sh` 是发布方在完整源仓库中运行的布局检查，它会读取仓库根目录的 Dockerfile，不能在只含 `source/` 子目录的源码包或无源码的离线镜像包中执行。从完整仓库根目录打包时先运行：

```bash
bash deployment/tests/validate-layout.sh
deployment/scripts/package.sh all
```

只生成一种包时把 `all` 替换为 `source` 或 `images`。`package.sh` 会在仓库根目录的 `outputs/server-deployment/` 写出归档和与归档并列的 `SHA256SUMS`；交付时两者必须一起分发。

## 当前 standalone 部署

当前 `10.200.10.12` 环境先使用 standalone：

```bash
./scripts/compose.sh standalone config
./scripts/compose.sh standalone up -d
./scripts/compose.sh standalone ps
./scripts/healthcheck.sh standalone
```

当前默认端口为 API `8083`、前端 `1325`、Redis `16379`。根据实际数据库密码、Redis 密码、COS 密钥和 AI 密钥编辑环境文件。`video_prod.yml` 中不保存这些密钥。standalone 的 API 使用 `JWT_SECRET`，worker 不需要该变量；RecBole trainer 由 Compose 设置 `RECBOLE_TRAINER_ENABLED=true`。

停止服务但保留数据卷：

```bash
./scripts/compose.sh standalone down
```

不要使用 `down -v`，除非已经确认要删除本地持久数据。

## 云服务器部署

云服务器为 Debian 13 x86_64，公网 IP 为 `services.example.internal`。PostgreSQL 监听宿主机端口 `15432`，Redis 监听宿主机端口 `6379`。

在云服务器执行：

```bash
./scripts/init-env.sh cloud
# 编辑 env/cloud.env
./scripts/compose.sh cloud config
./scripts/compose.sh cloud up -d api
./scripts/healthcheck.sh cloud
```

本部署包不安装或打包 PostgreSQL、Redis。云端 API 容器通过 Docker `host-gateway` 访问宿主机，模板中的连接地址为 PostgreSQL `host.docker.internal:15432` 和 Redis `host.docker.internal:6379`。不要把这两个地址改成 `127.0.0.1`，容器内的 `127.0.0.1` 指向容器自身。

启动前编辑 `env/cloud.env`，填写 `JWT_SECRET`、PostgreSQL 密码、Redis 密码、COS 密钥及所需 AI 密钥。模板已使用数据库 `video-app` 和用户 `video_app`。宿主机本地 Docker 链路默认使用 PostgreSQL `sslmode=disable`；这是当前交付配置，远程链路必须由 VPN 或严格来源 IP 白名单保护，不能视为生产 TLS 配置。

```bash
openssl rand -hex 32
```

将输出的 64 字符字符串填入 `env/cloud.env`。升级时保持该值不变；修改后已有管理端登录令牌会全部失效。不要将真实 JWT 密钥写回模板或发送到公共渠道。

`deployment/config/video_prod.yml` 没有设置 `Auth.JWTExpireHour`，所以交付 API 的 JWT 有效期使用代码默认值 8 小时；不要把 HTTP 服务示例配置中的 24 小时当成交付拓扑默认值。若需要其他期限，应显式扩展交付配置并重新验证。

云端防火墙的 API `8083`、PostgreSQL `15432` 和 Redis `6379` 只允许当前服务器的固定出口 IP 或 VPN 地址访问，不得对 `0.0.0.0/0` 开放。

Redis 必须使用高强度密码，PostgreSQL 必须启用网络访问控制和独立业务账户。

## 当前服务器部署前端、worker 和 RecBole trainer

确认当前服务器能够访问 `services.example.internal:8083`、`services.example.internal:15432` 和 `services.example.internal:6379` 后执行：

```bash
./scripts/init-env.sh intranet
# 编辑 env/intranet.env
./scripts/compose.sh intranet config
./scripts/compose.sh intranet up -d frontend worker recbole_trainer
./scripts/compose.sh intranet ps
./scripts/healthcheck.sh intranet
./scripts/compose.sh intranet logs --tail=200 frontend worker recbole_trainer
```

关键配置：

- `API_UPSTREAM`：使用 `http://services.example.internal:8083`，末尾不要带 `/`。
- `POSTGRES_DSN`：使用 `services.example.internal`、端口 `15432`，并填写真实凭据。
- `REDIS_ADDR`：使用 `services.example.internal:6379`。
- `REDIS_PASSWORD`：填写云端 Redis 的真实密码。

当前 PostgreSQL 连接参数没有启用 TLS，因此环境模板使用 `sslmode=disable`。当前服务器与云端 PostgreSQL 之间是远程链路，必须通过 VPN 或严格的来源 IP 白名单保护；后续启用 PostgreSQL TLS 后，应改为 `sslmode=verify-full` 并配置可信证书。

intranet 环境不运行 API，因此不注入 `JWT_SECRET`；它只运行 worker、RecBole trainer 和 frontend。trainer 的 `RECBOLE_TRAINER_ENABLED=true` 已写在 Compose，worker 明确为 `false`。

浏览器访问当前服务器的 `1325` 端口。前端 Nginx 将 `/api`、`/videos`、`/swagger` 和 `/knowledge-video-media` 代理到云端 API，因此浏览器不直接访问云端 `8083`，也不会产生跨域问题。

## 升级与回滚

升级前备份 PostgreSQL、Redis 和以下具名卷：

- RecBole 数据与 artifacts
- API/worker 临时存储（仍在处理的任务可能依赖）

记录当前镜像 ID：

```bash
docker image inspect video-service-api:latest --format '{{.Id}}'
```

导入新镜像后执行 `compose.sh <mode> up -d`。回滚时重新标记旧镜像，恢复数据库备份，再重建对应服务。API 和 worker 必须使用同一版本的业务镜像。

## 日志与诊断

```bash
./scripts/compose.sh standalone logs -f api worker
./scripts/compose.sh cloud logs -f api
./scripts/compose.sh intranet logs -f frontend worker recbole_trainer
```

API 健康检查为 `GET /healthz`，Swagger 为 `/swagger/index.html`。若 worker 持续报 Redis 错误，先从容器内验证 `REDIS_ADDR` 的 DNS、端口和密码，再检查云端防火墙规则。
