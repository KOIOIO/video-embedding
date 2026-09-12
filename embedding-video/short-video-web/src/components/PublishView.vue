<script setup>
import { computed, onBeforeUnmount, ref } from 'vue'
import {
  publishVideo,
  getVideoStatus,
  validateVideoFile,
  formatDuration,
  STATUS_PROCESSING,
  STATUS_PUBLISHED,
  STATUS_FAILED,
  MAX_DURATION_SECONDS,
} from '../user/videoApi.js'

const emit = defineEmits(['back', 'published'])

const MAX_TITLE = 200
const MAX_DESC = 2000

const file = ref(null)
const fileError = ref('')
const title = ref('')
const description = ref('')
const previewUrl = ref('')
const videoDuration = ref(0)

const phase = ref('idle') // idle | uploading | processing | success | failed
const uploadProgress = ref(0)
const errorMsg = ref('')
const currentVideoId = ref(0)

let pollTimer = null

const titleCount = computed(() => Array.from(title.value).length)
const descCount = computed(() => Array.from(description.value).length)
const canPublish = computed(() => {
  return phase.value === 'idle'
    && file.value !== null
    && titleCount.value > 0
    && titleCount.value <= MAX_TITLE
    && descCount.value <= MAX_DESC
    && fileError.value === ''
})
const durationLabel = computed(() => videoDuration.value > 0 ? formatDuration(videoDuration.value) : '')
const durationExceeded = computed(() => videoDuration.value > MAX_DURATION_SECONDS)

function onBack() {
  stopPolling()
  emit('back')
}

function onFileSelected(event) {
  const f = event.target.files?.[0]
  if (f) handleFile(f)
  event.target.value = ''
}

function onDrop(event) {
  event.preventDefault()
  const f = event.dataTransfer?.files?.[0]
  if (f) handleFile(f)
}

function onDragOver(event) {
  event.preventDefault()
}

function handleFile(f) {
  const check = validateVideoFile(f)
  if (!check.valid) {
    fileError.value = check.reason
    file.value = null
    previewUrl.value = ''
    videoDuration.value = 0
    return
  }
  fileError.value = ''
  file.value = f
  if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
  previewUrl.value = URL.createObjectURL(f)
  videoDuration.value = 0
}

function onVideoLoadedMetadata(event) {
  videoDuration.value = Math.round(event.target.duration || 0)
}

async function onPublish() {
  if (!canPublish.value) return
  phase.value = 'uploading'
  uploadProgress.value = 0
  errorMsg.value = ''

  try {
    const result = await publishVideo({
      file: file.value,
      title: title.value.trim(),
      description: description.value,
    })
    currentVideoId.value = result.video_id
    phase.value = 'processing'
    startPolling(result.video_id)
  } catch (err) {
    phase.value = 'failed'
    errorMsg.value = err?.message || '上传失败，请重试'
  }
}

function startPolling(videoId) {
  stopPolling()
  pollTimer = setInterval(async () => {
    try {
      const status = await getVideoStatus(videoId)
      if (status.status === STATUS_PUBLISHED) {
        stopPolling()
        phase.value = 'success'
        setTimeout(() => {
          emit('published')
        }, 1500)
      } else if (status.status === STATUS_FAILED) {
        stopPolling()
        phase.value = 'failed'
        errorMsg.value = status.error_msg || '视频处理失败，请重试'
      }
      // STATUS_PROCESSING or STATUS_UPLOADED: keep polling
    } catch {
      // 网络抖动，继续轮询
    }
  }, 3000)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function onRetry() {
  phase.value = 'idle'
  errorMsg.value = ''
  currentVideoId.value = 0
}

onBeforeUnmount(() => {
  stopPolling()
  if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
})
</script>

<template>
  <div class="publish">
    <header class="topbar">
      <button class="back-btn" type="button" aria-label="返回" @click="onBack">←</button>
      <span class="topbar-title">发布视频</span>
      <span class="topbar-spacer"></span>
    </header>

    <div class="content">
      <!-- 文件选择区 -->
      <div
        v-if="!previewUrl"
        class="upload-zone"
        @click="$refs.fileInput.click()"
        @drop="onDrop"
        @dragover="onDragOver"
      >
        <div class="upload-icon">＋</div>
        <p class="upload-text">点击或拖拽视频到此处</p>
        <p class="upload-hint">支持 mp4 / mov / avi / webm，≤500MB</p>
        <input ref="fileInput" type="file" accept="video/mp4,video/quicktime,video/x-msvideo,video/webm,.mp4,.mov,.avi,.webm" class="hidden-input" @change="onFileSelected" />
      </div>

      <!-- 视频预览 -->
      <div v-else class="preview-wrap">
        <video
          :src="previewUrl"
          class="preview-video"
          controls
          muted
          @loadedmetadata="onVideoLoadedMetadata"
        />
        <div class="preview-meta">
          <span class="file-name">{{ file?.name }}</span>
          <span v-if="durationLabel" class="duration" :class="{ exceeded: durationExceeded }">
            时长 {{ durationLabel }}
          </span>
          <button class="change-btn" type="button" @click="$refs.fileInput.click()">更换</button>
          <input ref="fileInput" type="file" accept="video/mp4,video/quicktime,video/x-msvideo,video/webm,.mp4,.mov,.avi,.webm" class="hidden-input" @change="onFileSelected" />
        </div>
        <p v-if="durationExceeded" class="duration-warning">
          视频时长超出上传限制，当前 {{ durationLabel }}
        </p>
      </div>

      <p v-if="fileError" class="file-error">{{ fileError }}</p>

      <!-- 标题输入 -->
      <div class="field">
        <div class="field-label">
          <span>标题 <span class="required">*</span></span>
          <span class="counter" :class="{ over: titleCount > MAX_TITLE }">{{ titleCount }}/{{ MAX_TITLE }}</span>
        </div>
        <input
          v-model="title"
          type="text"
          class="title-input"
          placeholder="写个吸引人的标题吧…"
          maxlength="200"
          :disabled="phase !== 'idle' && phase !== 'failed'"
        />
      </div>

      <!-- 描述输入 -->
      <div class="field">
        <div class="field-label">
          <span>描述</span>
          <span class="counter" :class="{ over: descCount > MAX_DESC }">{{ descCount }}/{{ MAX_DESC }}</span>
        </div>
        <textarea
          v-model="description"
          class="desc-input"
          placeholder="添加视频描述（可选）…"
          maxlength="2000"
          rows="3"
          :disabled="phase !== 'idle' && phase !== 'failed'"
        />
      </div>

      <p class="duration-hint">视频时长不能超过3分钟</p>

      <!-- 状态提示区 -->
      <div v-if="phase === 'uploading'" class="status-box">
        <div class="spinner"></div>
        <p>正在上传视频…</p>
      </div>

      <div v-else-if="phase === 'processing'" class="status-box">
        <div class="spinner"></div>
        <p>视频转码中，请稍候…</p>
        <p class="status-sub">这通常需要几十秒，请勿关闭页面</p>
      </div>

      <div v-else-if="phase === 'success'" class="status-box success">
        <div class="success-icon">✓</div>
        <p>发布成功！</p>
        <p class="status-sub">即将跳转到主页…</p>
      </div>

      <div v-else-if="phase === 'failed'" class="status-box failed">
        <div class="fail-icon">✕</div>
        <p>{{ errorMsg || '发布失败' }}</p>
        <button class="retry-btn" type="button" @click="onRetry">重新发布</button>
      </div>

      <!-- 发布按钮 -->
      <button
        v-if="phase === 'idle'"
        class="publish-btn"
        type="button"
        :class="{ disabled: !canPublish || durationExceeded }"
        :disabled="!canPublish || durationExceeded"
        @click="onPublish"
      >
        发布
      </button>
    </div>
  </div>
</template>

<style scoped>
.publish {
  position: relative;
  height: 100%;
  width: 100%;
  background: #000;
  color: #fff;
  overflow-y: auto;
}

.topbar {
  position: sticky;
  top: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: calc(12px + env(safe-area-inset-top)) 16px 12px;
  background: linear-gradient(180deg, rgba(0, 0, 0, 0.85), rgba(0, 0, 0, 0.6));
  backdrop-filter: blur(10px);
}

.back-btn {
  width: 36px;
  height: 36px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.1);
  font-size: 18px;
  color: #fff;
  transition: background 0.15s;
}
.back-btn:hover { background: rgba(254, 44, 85, 0.6); }

.topbar-title {
  font-size: 16px;
  font-weight: 700;
}
.topbar-spacer { width: 36px; }

.content {
  padding: 16px 16px 40px;
}

.upload-zone {
  border: 2px dashed rgba(255, 255, 255, 0.2);
  border-radius: 16px;
  padding: 48px 20px;
  text-align: center;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
}
.upload-zone:hover {
  border-color: #fe2c55;
  background: rgba(254, 44, 85, 0.05);
}
.upload-icon {
  font-size: 48px;
  color: #fe2c55;
  font-weight: 300;
  line-height: 1;
  margin-bottom: 12px;
}
.upload-text {
  margin: 0 0 6px;
  font-size: 15px;
  font-weight: 600;
}
.upload-hint {
  margin: 0;
  font-size: 12px;
  color: var(--text-dim, #999);
}
.hidden-input { display: none; }

.preview-wrap {
  margin-bottom: 16px;
}
.preview-video {
  width: 100%;
  max-height: 50vh;
  border-radius: 12px;
  background: #111;
  object-fit: contain;
}
.preview-meta {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 8px;
  flex-wrap: wrap;
}
.file-name {
  font-size: 13px;
  color: var(--text-dim, #999);
  max-width: 50%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.duration {
  font-size: 13px;
  color: var(--text-dim, #999);
}
.duration.exceeded { color: #fe2c55; font-weight: 600; }
.change-btn {
  margin-left: auto;
  padding: 5px 14px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.1);
  color: #fff;
  font-size: 12px;
  font-weight: 600;
}
.change-btn:hover { background: rgba(254, 44, 85, 0.7); }
.duration-warning {
  margin: 8px 0 0;
  font-size: 13px;
  color: #fe2c55;
  font-weight: 600;
}

.file-error {
  margin: 8px 0;
  font-size: 13px;
  color: #fe2c55;
}

.field {
  margin-top: 20px;
}
.field-label {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
  font-size: 14px;
  font-weight: 600;
}
.required { color: #fe2c55; }
.counter {
  font-size: 12px;
  color: var(--text-dim, #666);
  font-weight: 400;
}
.counter.over { color: #fe2c55; }

.title-input,
.desc-input {
  width: 100%;
  padding: 12px 14px;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.06);
  color: #fff;
  font-size: 14px;
  outline: none;
  transition: border-color 0.15s;
  box-sizing: border-box;
}
.title-input:focus,
.desc-input:focus {
  border-color: #fe2c55;
}
.title-input::placeholder,
.desc-input::placeholder {
  color: var(--text-dim, #555);
}
.desc-input {
  resize: vertical;
  min-height: 80px;
  font-family: inherit;
}

.duration-hint {
  margin: 16px 0 0;
  font-size: 12px;
  color: var(--text-dim, #666);
}

.status-box {
  margin-top: 24px;
  padding: 24px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.04);
  text-align: center;
}
.status-box p {
  margin: 8px 0 0;
  font-size: 15px;
  font-weight: 600;
}
.status-sub {
  font-size: 12px !important;
  color: var(--text-dim, #999) !important;
  font-weight: 400 !important;
}
.status-box.success p { color: #4ade80; }
.status-box.failed p { color: #fe2c55; }

.spinner {
  width: 32px;
  height: 32px;
  margin: 0 auto;
  border: 3px solid rgba(255, 255, 255, 0.2);
  border-top-color: #fe2c55;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

.success-icon {
  width: 48px;
  height: 48px;
  margin: 0 auto;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: rgba(74, 222, 128, 0.15);
  color: #4ade80;
  font-size: 24px;
  font-weight: 700;
}
.fail-icon {
  width: 48px;
  height: 48px;
  margin: 0 auto;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: rgba(254, 44, 85, 0.15);
  color: #fe2c55;
  font-size: 24px;
  font-weight: 700;
}
.retry-btn {
  margin-top: 16px;
  padding: 10px 32px;
  border-radius: 999px;
  background: linear-gradient(90deg, #fe2c55, #ff5470);
  color: #fff;
  font-size: 14px;
  font-weight: 700;
}

.publish-btn {
  width: 100%;
  margin-top: 24px;
  padding: 14px;
  border-radius: 12px;
  background: linear-gradient(90deg, #fe2c55, #ff5470);
  color: #fff;
  font-size: 16px;
  font-weight: 700;
  transition: opacity 0.15s, transform 0.1s;
}
.publish-btn:active { transform: scale(0.98); }
.publish-btn.disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
</style>
