# VectorWorker 分层分段向量化 Implementation Plan

> **Goal:** 上传视频后，先按固定时长粗分段做 ASR，再用 LLM 产出结构化“内容分段”，随后按结构化分段进行二次 ASR + embedding 并落库到 `edu_video_segment`。
>
> **Architecture:** `vector_worker` 增加一个 `mode=hierarchical` 的处理分支：粗分段（固定秒数、不中断）→ 粗段 ASR → LLM 结构化分段（落库 status=0）→ 细分段 ASR + embedding（更新 status=1）。
>
> **Tech Stack:** Go + GORM + Redis 队列 + FFmpeg(本机或 Docker) + DashScope(OpenAI Compatible) ASR/Embedding + LLM(Chat Completions) + Postgres(pgvector)

---

## 背景与现状对齐

- `vector_worker` 当前实现：按滑窗抽音频 → ASR → embedding → 写入 `edu_video_segment`  
  代码入口：[task.go](file:///c:/Users/xiaoy/Desktop/nlp-video-project/nlp-video-project/cmd/vector_worker/task.go#L85-L220)
- ASR/embedding 客户端现有实现：仅支持 `/audio/transcriptions` 与 `/embeddings`  
  代码：[client_openai.go](file:///c:/Users/xiaoy/Desktop/nlp-video-project/nlp-video-project/cmd/vector_worker/client_openai.go)
- FFmpeg 能力现有实现：`ExtractAudioSegment`、`ProbeDurationSeconds`、HLS 转码等  
  代码：[ffmpeg_transcoder.go](file:///c:/Users/xiaoy/Desktop/nlp-video-project/nlp-video-project/internal/infrastructure/transcode/ffmpeg_transcoder.go)
- 表结构（当前模型）：`EduVideoSegment` 已包含 `start_time/end_time/content_summary/embedding/knowledge_tags/status/deleted`  
  代码：[video.go](file:///c:/Users/xiaoy/Desktop/nlp-video-project/nlp-video-project/internal/model/video.go#L31-L49)

---

## 目标数据流（你提出的 4 步）

1) **粗分段**：上传的 mp4 先按时间分段（例如 60 秒一段，100 分钟→100段，中间不跳；写入配置）。  
2) **粗段 ASR**：每个粗段 mp4 过 ASR，得到粗粒度文本。  
3) **LLM 结构化**：把粗段 ASR 的文本交给 LLM，生成“某段时间讲什么内容”的结构化分段，落库到 `edu_video_segment`（建议 status=0）。  
4) **细分段二次处理**：根据结构化分段重新切片（可选：生成细分段 mp4 并上传），对每个细分段进行 ASR + embedding，更新 `edu_video_segment`（status=1，写 embedding）。

---

## 约定与校验规则（必须做）

- 所有 `start_time/end_time` 必须在 `[0, video_duration]` 范围内，且 `end_time > start_time`。
- 输出分段按 `start_time` 升序排列。
- 段落重叠：以先到为准裁剪后到段，或合并（建议裁剪，便于实现）。
- 过短段（小于 `RefineMinSegmentSec`）与邻近段合并；过长段（大于 `RefineMaxSegmentSec`）做二分或按粗段边界切开。
- 重跑同一个 `video_id`：先软删除旧段（`deleted=1`），再插入新段，避免多版本混杂。

---

## Task 1：把“粗分段秒数/LLM 参数”写入配置

**Files:**
- Modify: [config.go](file:///c:/Users/xiaoy/Desktop/nlp-video-project/nlp-video-project/internal/config/config.go)
- Modify: [video.yml](file:///c:/Users/xiaoy/Desktop/nlp-video-project/nlp-video-project/configs/video.yml)
- Modify: [app.go](file:///c:/Users/xiaoy/Desktop/nlp-video-project/nlp-video-project/cmd/vector_worker/app.go)

- [ ] Step 1：在 `internal/config/config.go` 的 `VectorWorker` 配置结构体增加字段
  - `CoarseSegmentSec int`
  - `RefineMinSegmentSec int`
  - `RefineMaxSegmentSec int`
  - `LLMModel string`
  - `LLMTimeoutMinutes int`

- [ ] Step 2：在 `configs/video.yml` 增加默认配置（示例）

```yml
VectorWorker:
  Mode: "hierarchical"
  CoarseSegmentSec: 60
  RefineMinSegmentSec: 20
  RefineMaxSegmentSec: 180
  LLMModel: "qwen-plus"
  LLMTimeoutMinutes: 3
```

- [ ] Step 3：在 `cmd/vector_worker/app.go` 读取上述配置，加入启动日志并传入 `handleVectorizeTask`

- [ ] Step 4：验证配置加载无误

Run:
```bash
go build ./...
```
Expected: build success

---

## Task 2：FFmpeg 增加“按时间切 mp4 段”能力

**Files:**
- Modify: [ffmpeg_transcoder.go](file:///c:/Users/xiaoy/Desktop/nlp-video-project/nlp-video-project/internal/infrastructure/transcode/ffmpeg_transcoder.go)

- [ ] Step 1：新增方法 `ClipVideoSegment(ctx, inputPath, outputPath, startSec, durationSec)`  
要求：
  - 优先 `-c copy`（快）
  - 失败 fallback 到 `fast` 编码参数（稳）
  - 同时支持本机 ffmpeg 与 docker ffmpeg

- [ ] Step 2：用一个本地样例视频做一次手工验证（可在 vector_worker 里临时调用或写小 main）
Expected:
  - 输出 mp4 可播放
  - 时长在误差允许范围内

---

## Task 3：OpenAI Compatible Client 增加 LLM（Chat）调用

**Files:**
- Modify: [client_openai.go](file:///c:/Users/xiaoy/Desktop/nlp-video-project/nlp-video-project/cmd/vector_worker/client_openai.go)

- [ ] Step 1：在 `openAICompatClient` 增加 chat 调用方法（建议名）
  - `ChatCompletions(ctx, model, prompt) (string, error)` 或 `ChatJSON(ctx, model, prompt) ([]byte, error)`
  - endpoint: `/chat/completions`
  - header: `Authorization: Bearer <apiKey>`

- [ ] Step 2：请求/响应格式（最小可用）

Request 示例：
```json
{
  "model": "qwen-plus",
  "messages": [
    { "role": "system", "content": "You are a helpful assistant that only outputs valid JSON." },
    { "role": "user", "content": "<PROMPT>" }
  ],
  "temperature": 0.2
}
```

Response 解析：读取 `choices[0].message.content`。

- [ ] Step 3：失败重试策略
  - 复用现有 `buildCandidateURLs`（`/v1` 有无）逻辑。
  - 超时时间使用 `LLMTimeoutMinutes`。

---

## Task 4：在 vector_worker 增加 `mode=hierarchical` 处理分支

**Files:**
- Modify: [task.go](file:///c:/Users/xiaoy/Desktop/nlp-video-project/nlp-video-project/cmd/vector_worker/task.go)
- Modify: [app.go](file:///c:/Users/xiaoy/Desktop/nlp-video-project/nlp-video-project/cmd/vector_worker/app.go)

### 4.1 粗分段（固定秒数、不跳）

- [ ] Step 1：在 `handleVectorizeTask` 中识别 `mode == "hierarchical"`，进入新流程。
- [ ] Step 2：计算粗分段：
  - `coarse := cfg.VectorWorker.CoarseSegmentSec`（默认 60）
  - for `start := 0; start < durationSec; start += coarse`
  - `end := min(start+coarse, durationSec)`

### 4.2 粗段 ASR（每段 mp4 过 ASR）

- [ ] Step 3：对每个粗段：
  - `ClipVideoSegment` 生成粗段 mp4 到 `tmpRoot`
  - `ExtractAudioSegment` 从粗段 mp4 提 wav
  - `client.Transcribe` 得到粗段文本
  - 收集数组 `coarseItems = []{start,end,text}`

### 4.3 LLM 结构化分段（落库 status=0）

- [ ] Step 4：拼接 Prompt（建议包含：视频总时长 + coarseItems + 约束规则），让 LLM 输出严格 JSON。

推荐输出 schema（必须包含时间）：
```json
{
  "segments": [
    {
      "segment_index": 0,
      "start_time": 0,
      "end_time": 75,
      "content_summary": "…",
      "knowledge_tags": ["…","…"]
    }
  ]
}
```

- [ ] Step 5：对 LLM 输出做校验与修正（按“约定与校验规则”）
- [ ] Step 6：落库：
  - `UPDATE edu_video_segment SET deleted=1 WHERE video_id=? AND deleted=0`
  - 批量插入新段：
    - `video_id, segment_index, start_time, end_time, content_summary, knowledge_tags, status=0, deleted=0`
    - embedding 为空

### 4.4 细分段二次 ASR + embedding（更新 status=1）

- [ ] Step 7：查询 `video_id=? AND deleted=0 AND status=0` 的段，逐段处理：
  - 从**原始视频文件**按 `start/end` extract wav（准确）
  - ASR 得 text
  - embedding 输入建议：`content_summary + "\n" + asr_text`
  - `client.Embed` 得向量
  - `UPDATE edu_video_segment SET embedding=?, status=1 WHERE id=?`

---

## Task 5：验证与回归检查

**验证 SQL**

- [ ] Step 1：验证结构化分段已落库（status=0）
```sql
select segment_index, start_time, end_time, status
from edu_video_segment
where video_id = <VIDEO_ID> and deleted = 0
order by segment_index;
```

- [ ] Step 2：验证二次 embedding 已写入（status=1）
```sql
select segment_index, status, (embedding is not null) as has_vec
from edu_video_segment
where video_id = <VIDEO_ID> and deleted = 0
order by segment_index;
```

**编译检查**
- [ ] Step 3：
```bash
go build ./...
```

---

## 建议提交顺序（每步可独立回滚）

1. `feat: add hierarchical vectorize configs`
2. `feat: add ffmpeg mp4 clip helper`
3. `feat: add llm chat client for segment structuring`
4. `feat: hierarchical vectorize pipeline (coarse->llm->refine asr/embed)`
5. `chore: add verification logging & sql notes`

