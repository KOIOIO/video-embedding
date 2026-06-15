# 基于大语言模型与向量检索的智能教学视频分析与推荐系统

## 项目简介

本项目是《自然语言处理》课程大作业，实现了一个基于大语言模型（LLM）、自动语音识别（ASR）与向量检索技术的智能教学视频内容分析与推荐系统。系统能够对教学视频进行自动语音转写、智能内容分段、语义向量化存储，并基于题目内容实现视频片段的精准推荐。

核心技术栈包括：Go 语言 HTTP 后端服务、PostgreSQL + pgvector 向量数据库、Redis Streams 消息队列、FFmpeg 视频处理、大语言模型与 Embedding 服务。

## 系统架构

```
┌─────────────┐     ┌──────────────────────────────────────┐
│  前端/Vue3  │────▶│  HTTP API 服务 (Gin Router)          │
│  用户界面   │     │  /api/videos  /api/questions         │
└─────────────┘     │  /api/recommendations  /api/watch     │
                    └──────────┬───────────────────────────┘
                               │
            ┌──────────────────┼──────────────────┐
            ▼                  ▼                  ▼
    ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
    │ PostgreSQL   │  │   Redis      │  │ 对象存储(S3) │
    │ + pgvector   │  │   Streams    │  │ 视频/HLS文件 │
    └──────────────┘  └──────────────┘  └──────────────┘
            │                  │                  │
            └──────────────────┼──────────────────┘
                               │
                    ┌──────────▼───────────┐
                    │     Worker 服务      │
                    │  ┌────────────────┐  │
                    │  │ 转码 Worker    │  │
                    │  │ (FFmpeg HLS)   │  │
                    │  └────────────────┘  │
                    │  ┌────────────────┐  │
                    │  │ 向量化 Worker  │  │
                    │  │ (ASR/LLM/Emb)  │  │
                    │  └────────────────┘  │
                    └──────────┬───────────┘
                               │
                    ┌──────────▼───────────┐
                    │    外部 AI 服务      │
                    │  ASR | Embedding     │
                    │  LLM (Qwen)          │
                    └──────────────────────┘
```

## 核心功能模块

### 1. 视频管理与处理
- **视频上传**：支持普通上传与分片断点续传
- **HLS 转码**：自动将上传视频转为 HLS 流媒体格式
- **封面生成**：基于 FFmpeg 自动提取视频封面

### 2. 视频内容智能分析（NLP 核心）
- **语音转写 (ASR)**：自动将视频音频转为文本
- **智能分段**：基于 LLM 对转写文本进行语义分段，划分知识点片段
- **向量化存储**：使用 Embedding 模型将文本片段转为向量，存入 pgvector
- **分层级处理链路**：Prepare → Coarse → Refine → Finalize 四阶段向量化

### 3. 智能推荐与检索
- **题目关联推荐**：根据题目内容检索相关视频片段
- **语义相似检索**：基于向量相似度匹配最相关教学片段
- **降级保障**：AI 服务不可用时自动降级，返回本地 embedding 结果

### 4. 学习行为追踪
- **观看记录上报**：记录用户观看行为
- **视频反馈**：支持点赞/踩等交互反馈
- **题库管理**：题目 CRUD 与分页查询

## REST API 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/healthz` | 健康检查 |
| `POST` | `/api/videos` | 上传视频 |
| `GET` | `/api/videos` | 获取视频列表 |
| `DELETE` | `/api/videos/:id` | 删除视频 |
| `GET` | `/api/videos/:id/play` | 获取播放地址 |
| `GET` | `/api/videos/:id/similar` | 获取相似视频 |
| `POST` | `/api/recommendations/by-question` | 根据题目推荐视频片段 |
| `POST` | `/api/watch-records` | 上报观看记录 |
| `GET` | `/api/questions` | 查询题库 |

## 快速开始

### 环境要求
- Go 1.21+
- PostgreSQL 13+（需启用 pgvector 扩展）
- Redis 6+
- FFmpeg
- S3 兼容对象存储（MinIO 等）

### 启动 HTTP 服务

```bash
cd video-service
go run ./cmd/httpapi
```

服务默认监听 `:8081`，可通过环境变量 `HTTP_ADDR` 覆盖：
```bash
HTTP_ADDR=:8081 go run ./cmd/httpapi
```

### 启动 Worker 服务

```bash
cd video-service
go run ./cmd/worker
```

Worker 统一启动转码和向量化两条异步处理链路。

### 环境变量配置

| 环境变量 | 作用 |
|------|------|
| `HTTP_ADDR` | HTTP 监听地址 |
| `CONFIG_FILE` | 配置文件路径 |
| `DASHSCOPE_API_KEY` | LLM/Embedding API Key |
| `ASR_API_KEY` | ASR 服务 API Key |
| `EMBEDDING_API_KEY` | Embedding 服务 API Key |

## 项目结构

```text
.
├── README.md
├── video-service/      # HTTP 后端服务
│   ├── cmd/
│   │   ├── httpapi/                 # HTTP API 入口
│   │   └── worker/                  # Worker 入口
│   ├── configs/                     # 配置文件
│   ├── internal/
│   │   ├── http/handler/            # HTTP 请求处理
│   │   ├── http/router/             # 路由配置
│   │   ├── application/videoapp/    # 业务逻辑
│   │   ├── infrastructure/          # 基础设施层
│   │   └── worker/                  # Worker 实现
│   └── docs/swagger/                # API 文档
├── legacy-video/           # 历史遗留工程
└── hls-web/                         # Vue 3 + Vite 前端调试界面
```

## NLP 技术实现要点

### 视频向量化流程

1. **Prepare 阶段**：校验视频元数据，探测时长，生成粗分段计划
2. **Coarse 阶段**：按计划粗切分视频片段 → 上传片段 → 执行 coarse ASR
3. **Refine 阶段**：基于 coarse 文本调用 LLM 生成细分段 → refine ASR → 生成 Embedding 向量
4. **Finalize 阶段**：标记向量化完成，写入最终状态

### 推荐链路

- **正常模式**：题目文本 → Embedding 向量 → pgvector 相似检索 → 返回相关视频片段
- **降级模式**：外部 AI 服务不可用时，使用本地 embedding 兜底，返回降级结果

### 可靠性设计

- Redis Streams 消费者组 + ACK 机制确保任务不丢失
- 失败任务写入 DLQ（死信队列）便于排查
- 上传链路一致性补偿：对象上传后若记录失败则清理已上传对象
- AI 服务故障时自动降级，保证推荐接口可用

## 实验数据与评估

本系统在以下维度对推荐效果进行评估：
- 推荐准确率（Top-K 命中率）
- 推荐召回率
- 向量检索效率（查询延迟）
- 降级模式下的推荐质量对比

## 参考技术

- PostgreSQL pgvector: https://github.com/pgvector/pgvector
- FFmpeg: https://ffmpeg.org/
- Gin Web Framework: https://github.com/gin-gonic/gin
- Redis Streams: https://redis.io/docs/data-types/streams/
