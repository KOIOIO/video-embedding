# VectorWorker 二次 ASR 并发优化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 hierarchical 模式下“二次 ASR + embedding”从串行处理改为并发处理，显著降低单视频处理耗时。

**Architecture:** 在 `refineSegmentsASRAndEmbed` 中引入 worker pool：并发执行“抽音频 + ASR”，收集文本后按 batch 调 embedding，再批量更新数据库（embedding/status）。

**Tech Stack:** Go + context + goroutine/chan + FFmpeg + DashScope(OpenAI compatible) + GORM + Postgres(pgvector)

---

## 现状

- 当前 `refineSegmentsASRAndEmbed` 对每个分段串行执行：
  1) `ffmpeg ExtractAudioSegment`
  2) `ASR Transcribe`
  3) `Embed`（按 batch）
  4) 逐条 `UPDATE` 写 embedding/status
- 串行导致总耗时与分段数近似线性增长，且 ASR 远程调用是主要瓶颈。

目标是把步骤 (1)(2) 并行化，让整体耗时接近 “最慢的几段 + embedding 批次时间”。

---

## 文件结构

**修改文件：**
- Modify: `c:\Users\xiaoy\Desktop\nlp-video-project\nlp-video-project\cmd\vector_worker\task.go`

不新增文件（保持改动集中且易回滚）。

---

### Task 1: 并发化二次 ASR（抽音频 + ASR）

**Files:**
- Modify: `...\cmd\vector_worker\task.go:refineSegmentsASRAndEmbed`

- [ ] Step 1: 定义并发参数
  - `workers := asrWorkers`（复用现有配置，若 <=0 则默认 2）
  - 引入 `maxInFlight`（可选，默认 `workers*2`）用于限制队列长度

- [ ] Step 2: 定义 job/result 结构体（在函数内即可）

```go
type job struct {
    ID       uint64
    StartSec int
    EndSec   int
    Summary  string
}

type result struct {
    ID      uint64
    Input   string // summary + "\n" + asr_text
    Err     error
}
```

- [ ] Step 3: 建立 worker pool
  - `jobs := make(chan job, workers*2)`
  - `results := make(chan result, workers*2)`
  - `sync.WaitGroup` 启动 `workers` 个 goroutine：
    - 对每个 job：
      1) `ExtractAudioSegment(localVideo, audioPath, startSec, dur)`
      2) `Transcribe(audioPath)`
      3) 组合 `input := strings.TrimSpace(summary + "\n" + text)`
      4) `results <- result{ID: id, Input: input}`
    - 清理临时 wav

- [ ] Step 4: 发送全部 job 并等待汇总
  - producer：遍历 `segs` 推送到 `jobs`，close(jobs)
  - closer：`wg.Wait(); close(results)`
  - consumer：读取 `results`，构造 `ids []uint64` 与 `inputs []string`（保持同序）
  - 若出现 `Err`：直接返回（保持失败可见性）

- [ ] Step 5: 增加关键日志
  - refine start：分段数、workers
  - per job start（可保留现有日志，但避免太多）
  - asr batch done：统计耗时、成功数

---

### Task 2: 批量 embedding 与批量更新 DB

**Files:**
- Modify: `...\cmd\vector_worker\task.go:refineSegmentsASRAndEmbed`

- [ ] Step 1: 按 `embedBatch` 切分 inputs 调用 `client.Embed`
  - 复用现有 embeddingDim=1536 与 `normalizeEmbeddingDim`
  - 确保 `len(vecs)==len(inputs)`

- [ ] Step 2: 批量更新 DB（优先使用事务）
  - 在一个事务内，对每个 id 更新 embedding/status
  - 若性能仍不够，再升级为单条 SQL `UPDATE ... FROM (VALUES ...)`（可选）

- [ ] Step 3: 增加更新统计日志
  - updated rows count
  - embedding batch cost

---

### Task 3: 验证与回归

**Files:**
- None

- [ ] Step 1: 编译检查
```bash
go build ./...
```
Expected: build success

- [ ] Step 2: 数据库验证（示例）
```sql
select id, segment_index, status, (embedding is null) as emb_null
from edu_video_segment
where video_id = <VIDEO_ID> and deleted = 0
order by segment_index;
```
Expected:
- `emb_null = false`
- `status = 1`

- [ ] Step 3: 性能观察
  - 对同一视频，记录并发前/并发后的总耗时对比（以日志时间戳为准）

