# 视频视频服务部署手册

## 交付模式

部署包提供三套拓扑：

- `standalone`：当前部署方式，API、worker、RecBole trainer 和前端在同一台服务器运行，PostgreSQL 与 Redis 使用当前地址。
- `cloud`：云服务器 `services.example.internal` 只运行 API，并连接同机已有的 PostgreSQL 和 Redis。
- `intranet`：当前服务器运行前端、worker 和 RecBole trainer，并连接云端 API、PostgreSQL 和 Redis。

部署包不包含 PostgreSQL、Redis 或 Gorse 镜像。推荐引擎使用 RecBole，部署配置已关闭 Gorse 同步和回写。

所有对象存储端点、Bucket 和转码参数来自 `config/video_prod.yml`。对象存储密钥只通过环境文件注入。

## 服务器要求

- Debian x86_64 / `linux/amd64`
- Docker Engine 24 或更高版本
- Docker Compose v2
- 至少 20 GB 可用磁盘；离线镜像和 RecBole 依赖占用较大
- 服务器时区建议设置为 `Asia/Shanghai`

## 离线镜像包

解压后进入 `video-deploy/deployment`：

```bash
./scripts/load-images.sh
./scripts/init-env.sh standalone
```

编辑 `env/standalone.env`，替换全部 `change-me`。不要把真实环境文件提交回 Git 或发送到公共渠道。

## 源码部署包

源码包包含 `source/` 构建上下文。进入 `video-deploy/deployment` 后执行：

```bash
./scripts/build-images.sh
./scripts/init-env.sh standalone
```

构建脚本固定生成 `linux/amd64` 镜像。

## 当前 standalone 部署

当前 `10.200.10.12` 环境先使用 standalone：

```bash
./scripts/compose.sh standalone config
./scripts/compose.sh standalone up -d
./scripts/compose.sh standalone ps
./scripts/healthcheck.sh standalone
```

当前默认端口为 API `8083`、前端 `1325`、Redis `16379`。根据实际数据库密码、Redis 密码、COS 密钥和 AI 密钥编辑环境文件。`video_prod.yml` 中不保存这些密钥。

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

启动前编辑 `env/cloud.env`，填写 `JWT_SECRET`、PostgreSQL 密码、Redis 密码、COS 密钥及所需 AI 密钥。模板已使用数据库 `video-app` 和用户 `video_app`。宿主机本地 Docker 链路默认使用 PostgreSQL `sslmode=disable`。

`JWT_SECRET` 必须至少 32 个字符，只注入云端 API。首次部署可在云服务器生成随机值：

```bash
openssl rand -hex 32
```

将输出的 64 字符字符串填入 `env/cloud.env`。升级时保持该值不变；修改后已有管理端登录令牌会全部失效。不要将真实 JWT 密钥写回模板或发送到公共渠道。

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
