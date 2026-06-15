# hls-web 问题推荐分段列表 + 片段播放（保留完整播放）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `hls-web` 增加“问题检索推荐分段”模块：输入 `question_text` 调用后端 API 返回 Top3 分段列表（标题=content_summary、封面=视频封面），点击列表项进入“片段模式播放”（进度条仅显示片段、到 end 强制停止），同时保留原有“完整视频播放”能力。

**Architecture:** App.vue 负责：调用 API、渲染分段列表、选择播放模式（完整/片段）并将 `src + start/end` 传给播放器；HlsPlayer.vue 扩展支持 segment props，在 segment 模式下禁用原生 controls，使用自定义进度条映射 `currentTime` 到 `start~end` 区间并强制停止；在非 segment 模式保持现有逻辑与原生 controls（完整播放）。

**Tech Stack:** Vue 3 (script setup), Vite, hls.js, Fetch API.

---

## Files & Responsibilities

**Modify**
- `hls-web/src/App.vue`：新增“问题检索推荐分段”卡片、调用 `/api/video/recommend_by_question`、展示 Top3 列表、切换片段/完整播放。
- `hls-web/src/components/HlsPlayer.vue`：支持 `startTimeSec/endTimeSec` props；segment 模式自定义进度条、强制停止；非 segment 模式保持现有完整播放（HLS/MP4）。

---

### Task 1: App.vue 增加问题检索与分段列表 UI

**Files:**
- Modify: `hls-web/src/App.vue`

- [ ] **Step 1: 增加状态与请求方法**

在 `<script setup>` 增加：
- `questionText` 输入框状态
- `qaLoading/qaError/qaSegments` 推荐结果列表
- `segmentStart/segmentEnd` 当前播放片段范围（0 表示完整播放）
- `fetchRecommendByQuestion()`：POST `/api/video/recommend_by_question`，body 仅 `{question_text}`
- `playSegment(item)`：设置 `currentPlaySrc` 为 `item.play_url`，并设置 `segmentStart/segmentEnd`，标题使用 `item.title`
- `playFull()`：保留当前 `currentPlaySrc`，清空 `segmentStart/segmentEnd` 为 0，进入完整播放模式

- [ ] **Step 2: 增加模板区域**

在页面中新增一个 card：
- 输入框 + 查询按钮
- 列表：封面、标题、片段时间范围、播放按钮
- 当前播放模式提示：片段模式/完整模式，并提供“切换为完整播放”按钮

- [ ] **Step 3: 将播放器 props 传入 start/end**

把已有 `<HlsPlayer :src="currentPlaySrc" ...>` 改为：
- 额外传 `:start-time-sec="segmentStart"`、`:end-time-sec="segmentEnd"`

---

### Task 2: HlsPlayer.vue 支持片段模式进度条与强制停止（保留完整播放）

**Files:**
- Modify: `hls-web/src/components/HlsPlayer.vue`

- [ ] **Step 1: 增加 props 与 mode 判断**

新增 props：
- `startTimeSec` number default 0
- `endTimeSec` number default 0

计算：
- `isSegmentMode = endTimeSec > startTimeSec`
- `segmentDuration = max(0.1, end-start)`

- [ ] **Step 2: 在 segment 模式下使用自定义 controls**

模板：
- `<video ... :controls="!isSegmentMode">`
- 当 `isSegmentMode` 为 true 时渲染：
  - 播放/暂停按钮
  - 时间显示：`segmentCurrent / segmentDuration`
  - `<input type="range" min="0" max="1" step="0.001">` 作为进度条

- [ ] **Step 3: 片段播放逻辑**

事件与控制：
- `loadedmetadata/canplay` 后 `video.currentTime = startTimeSec`，若 `autoplay` 则播放
- `timeupdate` + `setInterval(250ms)` 兜底：
  - 若 `currentTime >= endTimeSec`：`pause()` 并 `currentTime = endTimeSec`
  - 更新进度条：`(currentTime-start)/segmentDuration`
- 用户拖动 range：`video.currentTime = start + p*segmentDuration`

- [ ] **Step 4: 非 segment 模式保持现有逻辑**

当 `!isSegmentMode`：
- 继续使用原生 controls
- HLS/MP4 处理逻辑不变

---

### Task 3: 本地验证

**Files:**
- (no repo changes required)

- [ ] **Step 1: 安装依赖并启动前端**

在 `hls-web` 目录：

```bash
npm install
npm run dev
```

- [ ] **Step 2: 验证功能点**

- 输入问题 -> Top3 列表渲染（封面/标题/时间）
- 点击列表项 -> 进入片段模式，进度条只显示片段、拖动只在片段内、到 end 自动暂停
- 点击“完整播放” -> 退出片段模式，恢复原生 controls 与完整时长进度条

---

## Self-Review

- Spec coverage：
  - “API 只传问题字符串”：Task 1
  - “Top3 列表 + 封面 + 标题”：Task 1
  - “片段进度条 + 强制停止”：Task 2
  - “保留完整播放能力”：Task 2 + Task 1 的模式切换
- Placeholder scan：无 TBD/TODO
- Type consistency：props 命名 `startTimeSec/endTimeSec` 在 App 与 Player 一致
