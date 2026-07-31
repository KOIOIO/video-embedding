# 视频服务服务器部署包设计

## 目标

交付一套当前可直接部署、未来可平滑拆分到云端和内网的服务器部署包，同时提供源码构建包和 `linux/amd64` 离线镜像包。

本次部署继续使用当前 `video_prod.yml` 中的对象存储配置和现有 PostgreSQL、Redis 地址，不改变线上运行拓扑。部署材料不得包含仓库本地 `.env.deploy` 中的真实密钥。

## 部署拓扑

部署目录提供三套相互独立的 Compose 编排：

- `standalone`：本次实际使用。API、worker、RecBole trainer 和前端在同一 Docker 网络运行，沿用当前 PostgreSQL、Redis 与对象存储配置。
- `cloud`：未来云端使用。只运行 HTTP API，连接云端已有的 PostgreSQL 和 Redis。
- `intranet`：未来内网使用。运行 worker、RecBole trainer 和前端，通过环境变量连接云端 PostgreSQL、Redis 与 HTTP API。

部署包不包含 PostgreSQL、Redis 或 Gorse。推荐引擎保持 RecBole，部署配置关闭 Gorse 同步和回写。

HTTP API 与 worker 使用同一业务镜像但采用不同启动命令。对象存储仍由 `video_prod.yml` 配置，不在部署包中新增对象存储容器。

## 前端代理

前端镜像在容器启动时根据 `API_UPSTREAM` 生成 Nginx 配置。standalone 默认代理到 `http://api:8081`；未来 intranet 部署由环境文件将其设置为云服务器 HTTP API 地址。

Nginx 对 `/api`、`/videos`、`/swagger` 和 `/knowledge-video-media` 使用同一 upstream，并保留 Range、转发协议和客户端地址相关请求头。

## 配置与密钥

- `video_prod.yml` 随部署包提供，保持当前 OSS/COS 端点、Bucket 和业务参数。
- PostgreSQL DSN、Redis 地址和密码支持环境变量覆盖，避免未来拆分时改写镜像内配置。
- 提供 standalone、cloud、intranet 三份 `.env.example`，只包含占位符和说明。
- 实际 `.env` 文件、数据库密码、对象存储密钥和 AI API key 不进入 Git、源码包或镜像包。
- PostgreSQL 和 Redis 不包含在部署包中，由云端现有服务提供；API 和内网服务通过环境变量连接。

## 交付物

```text
video-deploy/
├── standalone/
├── cloud/
├── intranet/
├── config/video_prod.yml
├── env/
├── scripts/
├── source/
└── DEPLOYMENT.md
```

最终生成：

1. 源码部署压缩包，包含构建上下文、三套 Compose、配置模板、脚本和文档。
2. 离线镜像压缩包，包含部署目录以及 `linux/amd64` 的 API、前端、RecBole trainer 和所需第三方镜像归档。

## 脚本与操作

脚本提供环境初始化、镜像导入、配置校验、启动、停止和健康检查。脚本必须从自身所在部署目录解析路径，不能依赖打包机器的绝对路径。

部署文档分别给出：

- 当前 standalone 首次部署和升级步骤；
- 未来 cloud 节点启动顺序；
- 未来 intranet 节点连接云端服务的方法；
- 防火墙、数据库授权、Redis 保护和前端代理切换注意事项；
- 数据卷备份、回滚和健康检查命令。

## 验证标准

- 三套 Compose 均可通过 `docker compose config`。
- 前端镜像在不同 `API_UPSTREAM` 下生成正确代理配置。
- 业务镜像可分别启动 `httpapi`、`worker` 和 `recboletrainer` 命令。
- 所有自建镜像均为 `linux/amd64`。
- 离线镜像归档可由 `docker load` 读取。
- 两个压缩包可列出、校验并解压，且不包含真实 `.env.deploy` 或其他已知密钥文件。
