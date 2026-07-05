<script setup>
import Hls from 'hls.js'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

const props = defineProps({
  src: { type: String, required: true },
  title: { type: String, default: '' },
  autoplay: { type: Boolean, default: true },
  startTimeSec: { type: Number, default: 0 },
  endTimeSec: { type: Number, default: 0 },
  watchContext: { type: Object, default: null },
})

const emit = defineEmits(['watch-progress'])

const videoRef = ref(null)
const errorText = ref('')
const isNativeHls = ref(false)
const levels = ref([])
const selectedLevel = ref(-1)
const currentLevelLabel = ref('')

let hls = null
let segmentInterval = null
let segmentOnTimeUpdate = null
let segmentOnLoadedMetadata = null
let watchStartedAt = 0
let watchedMs = 0
let lastEmittedSec = 0
let activeWatchContext = null

const segmentProgress = ref(0)
const segmentCurrentSec = ref(0)
const isPlaying = ref(false)
const playbackCurrentSec = ref(0)
const playbackDurationSec = ref(0)

const normalizedSrc = computed(() => {
  if (!props.src) return ''
  return props.src.startsWith('http') ? props.src : props.src
})

const isHlsSrc = computed(() => {
  const s = String(normalizedSrc.value || '').toLowerCase()
  return s.includes('.m3u8')
})

const isSegmentMode = computed(() => {
  const s = Number(props.startTimeSec || 0)
  const e = Number(props.endTimeSec || 0)
  return Number.isFinite(s) && Number.isFinite(e) && e > s
})

const segmentDurationSec = computed(() => {
  if (!isSegmentMode.value) return 0
  return Math.max(0.1, Number(props.endTimeSec) - Number(props.startTimeSec))
})
const playbackProgress = computed(() => {
  const duration = isSegmentMode.value ? segmentDurationSec.value : playbackDurationSec.value
  const current = isSegmentMode.value ? segmentCurrentSec.value : playbackCurrentSec.value
  if (!(duration > 0)) return 0
  return clamp(current / duration, 0, 1)
})
const progressCurrentLabel = computed(() => formatTime(isSegmentMode.value ? segmentCurrentSec.value : playbackCurrentSec.value))
const progressDurationLabel = computed(() => formatTime(isSegmentMode.value ? segmentDurationSec.value : playbackDurationSec.value))

function clamp(n, min, max) {
  return Math.min(max, Math.max(min, n))
}

function formatTime(sec) {
  const s = Math.max(0, Math.floor(Number(sec || 0)))
  const m = Math.floor(s / 60)
  const r = s % 60
  return `${String(m).padStart(2, '0')}:${String(r).padStart(2, '0')}`
}

function clearSegmentHandlers() {
  const video = videoRef.value
  if (video && segmentOnTimeUpdate) {
    video.removeEventListener('timeupdate', segmentOnTimeUpdate)
  }
  if (video && segmentOnLoadedMetadata) {
    video.removeEventListener('loadedmetadata', segmentOnLoadedMetadata)
  }
  segmentOnTimeUpdate = null
  segmentOnLoadedMetadata = null
  if (segmentInterval) {
    clearInterval(segmentInterval)
    segmentInterval = null
  }
  segmentProgress.value = 0
  segmentCurrentSec.value = 0
  isPlaying.value = false
  playbackCurrentSec.value = 0
  playbackDurationSec.value = 0
}

function emitWatchProgress(force = false) {
  const watchedSec = Math.floor(watchedMs / 1000)
  if (!force && watchedSec <= lastEmittedSec) return
  lastEmittedSec = watchedSec
  emit('watch-progress', {
    context: activeWatchContext,
    watchedSec,
    completed: Boolean(force),
  })
}

function stopWatchTimer(forceEmit = false) {
  if (watchStartedAt > 0) {
    watchedMs += Math.max(0, Date.now() - watchStartedAt)
    watchStartedAt = 0
  }
  emitWatchProgress(forceEmit)
}

function startWatchTimer() {
  if (watchStartedAt > 0) return
  watchStartedAt = Date.now()
}

function resetWatchTracking() {
  watchStartedAt = 0
  watchedMs = 0
  lastEmittedSec = 0
  activeWatchContext = null
}

function syncPlaybackUI() {
  const video = videoRef.value
  if (!video) return

  isPlaying.value = !video.paused && !video.ended
  playbackCurrentSec.value = Number.isFinite(video.currentTime) ? Math.max(0, video.currentTime) : 0
  playbackDurationSec.value = Number.isFinite(video.duration) ? Math.max(0, video.duration) : 0
  if (isPlaying.value) {
    startWatchTimer()
  } else {
    stopWatchTimer(false)
  }
  if (!isSegmentMode.value) return

  const start = Number(props.startTimeSec || 0)
  const end = Number(props.endTimeSec || 0)
  const dur = Math.max(0.1, end - start)

  if (video.currentTime >= end) {
    video.pause()
    try {
      video.currentTime = end
    } catch {
    }
    isPlaying.value = false
    stopWatchTimer(true)
  }

  const cur = clamp(video.currentTime - start, 0, dur)
  segmentCurrentSec.value = cur
  segmentProgress.value = clamp(cur / dur, 0, 1)
}

async function seekToSegmentStart() {
  const video = videoRef.value
  if (!video) return
  if (!isSegmentMode.value) return

  const start = Number(props.startTimeSec || 0)
  const end = Number(props.endTimeSec || 0)
  if (!(end > start)) return
  try {
    video.currentTime = start
  } catch {
  }
  syncPlaybackUI()
  if (props.autoplay) {
    try {
      await video.play()
    } catch {
    }
  }
}

async function togglePlay() {
  const video = videoRef.value
  if (!video) return
  if (video.paused) {
    try {
      await video.play()
    } catch {
    }
  } else {
    video.pause()
  }
  syncPlaybackUI()
}

function onSeekInput(e) {
  const video = videoRef.value
  if (!video) return
  const p = clamp(Number(e?.target?.value || 0), 0, 1)
  const start = isSegmentMode.value ? Number(props.startTimeSec || 0) : 0
  const dur = isSegmentMode.value ? segmentDurationSec.value : playbackDurationSec.value
  if (!(dur > 0)) return
  try {
    video.currentTime = start + p * dur
  } catch {
  }
  syncPlaybackUI()
}

function teardown() {
  stopWatchTimer(true)
  resetWatchTracking()
  clearSegmentHandlers()
  const video = videoRef.value
  if (video) {
    video.onplay = null
    video.onpause = null
    video.onended = null
    video.ontimeupdate = null
    video.onloadedmetadata = null
    video.ondurationchange = null
  }
  if (hls) {
    hls.destroy()
    hls = null
  }
  errorText.value = ''
  isNativeHls.value = false
  levels.value = []
  selectedLevel.value = -1
  currentLevelLabel.value = ''
}

function formatLevelLabel(level) {
  const parts = []
  if (level?.height) parts.push(`${level.height}p`)
  if (level?.bitrate) parts.push(`${Math.round(level.bitrate / 1000)}kbps`)
  if (level?.codecSet) parts.push(level.codecSet)
  if (!parts.length) return '未知'
  return parts.join(' · ')
}

function syncLevelsFromHls() {
  if (!hls) return
  const mapped = (hls.levels || []).map((lvl, idx) => ({
    index: idx,
    height: lvl?.height || 0,
    width: lvl?.width || 0,
    bitrate: lvl?.bitrate || 0,
    codecSet: lvl?.codecSet || '',
    label: formatLevelLabel(lvl),
  }))
  levels.value = mapped
}

function updateCurrentLevelLabel() {
  if (!hls) {
    currentLevelLabel.value = ''
    return
  }
  const idx = hls.currentLevel
  if (idx === -1) {
    currentLevelLabel.value = '自动'
    return
  }
  const lvl = levels.value.find((x) => x.index === idx)
  currentLevelLabel.value = lvl?.label || `${idx}`
}

function applySelectedLevel() {
  if (!hls) return
  if (selectedLevel.value === -1) {
    hls.currentLevel = -1
    hls.nextLevel = -1
    hls.loadLevel = -1
  } else {
    hls.currentLevel = selectedLevel.value
    hls.nextLevel = selectedLevel.value
    hls.loadLevel = selectedLevel.value
  }
  updateCurrentLevelLabel()
}

async function setup() {
  teardown()
  const video = videoRef.value
  if (!video) return
  if (!normalizedSrc.value) return
  activeWatchContext = props.watchContext

  video.onplay = () => {
    startWatchTimer()
    syncPlaybackUI()
  }
  video.onpause = () => {
    stopWatchTimer(false)
    syncPlaybackUI()
  }
  video.onended = () => {
    stopWatchTimer(true)
    syncPlaybackUI()
  }
  video.ontimeupdate = () => syncPlaybackUI()
  video.onloadedmetadata = () => syncPlaybackUI()
  video.ondurationchange = () => syncPlaybackUI()

  if (!isHlsSrc.value) {
    video.src = normalizedSrc.value
    if (isSegmentMode.value) {
      segmentOnLoadedMetadata = () => {
        void seekToSegmentStart()
      }
      segmentOnTimeUpdate = () => syncPlaybackUI()
      video.addEventListener('loadedmetadata', segmentOnLoadedMetadata)
      video.addEventListener('timeupdate', segmentOnTimeUpdate)
      segmentInterval = setInterval(syncPlaybackUI, 250)
      if (video.readyState >= 1) {
        void seekToSegmentStart()
      }
      return
    }
    if (props.autoplay) {
      try {
        await video.play()
      } catch {
      }
    }
    return
  }

  if (video.canPlayType('application/vnd.apple.mpegurl')) {
    isNativeHls.value = true
    video.src = normalizedSrc.value
    if (isSegmentMode.value) {
      segmentOnLoadedMetadata = () => {
        void seekToSegmentStart()
      }
      segmentOnTimeUpdate = () => syncPlaybackUI()
      video.addEventListener('loadedmetadata', segmentOnLoadedMetadata)
      video.addEventListener('timeupdate', segmentOnTimeUpdate)
      segmentInterval = setInterval(syncPlaybackUI, 250)
      if (video.readyState >= 1) {
        void seekToSegmentStart()
      }
      return
    }
    if (props.autoplay) {
      try {
        await video.play()
      } catch {
      }
    }
    return
  }

  if (!Hls.isSupported()) {
    errorText.value = '当前浏览器不支持 HLS 播放'
    return
  }

  hls = new Hls({
    enableWorker: true,
    lowLatencyMode: false,
  })

  hls.on(Hls.Events.ERROR, (_, data) => {
    const msg = data?.details || data?.type || 'unknown'
    errorText.value = `播放错误: ${msg}`
    if (data?.fatal) {
      teardown()
    }
  })

  hls.on(Hls.Events.MANIFEST_PARSED, () => {
    syncLevelsFromHls()
    updateCurrentLevelLabel()
  })
  hls.on(Hls.Events.LEVEL_SWITCHED, () => {
    updateCurrentLevelLabel()
  })

  hls.loadSource(normalizedSrc.value)
  hls.attachMedia(video)
  if (props.autoplay && !isSegmentMode.value) {
    hls.on(Hls.Events.MANIFEST_PARSED, async () => {
      try {
        await video.play()
      } catch {
      }
    })
  }

  if (isSegmentMode.value) {
    segmentOnLoadedMetadata = () => {
      void seekToSegmentStart()
    }
    segmentOnTimeUpdate = () => syncPlaybackUI()
    video.addEventListener('loadedmetadata', segmentOnLoadedMetadata)
    video.addEventListener('timeupdate', segmentOnTimeUpdate)
    segmentInterval = setInterval(syncPlaybackUI, 250)
    if (video.readyState >= 1) {
      void seekToSegmentStart()
    }
  }
}

onMounted(setup)
watch(() => [props.src, props.startTimeSec, props.endTimeSec, props.watchContext], setup)
watch(selectedLevel, applySelectedLevel)
onBeforeUnmount(teardown)
</script>

<template>
  <div class="player">
    <div v-if="title" class="player-title">{{ title }}</div>
    <div v-if="isHlsSrc" class="toolbar">
      <div class="toolbar-left">
        <div class="hint">分辨率</div>
        <select class="select" :disabled="isNativeHls || !levels.length" v-model.number="selectedLevel">
          <option :value="-1">自动</option>
          <option v-for="lvl in levels" :key="lvl.index" :value="lvl.index">
            {{ lvl.label }}
          </option>
        </select>
        <div v-if="isNativeHls" class="hint">Safari 原生 HLS 不支持手动切换</div>
      </div>
      <div class="toolbar-right">
        <div v-if="currentLevelLabel" class="hint">当前：{{ currentLevelLabel }}</div>
      </div>
    </div>
    <video ref="videoRef" class="video" controls playsinline preload="metadata"></video>
    <div class="segment-bar" :class="{ 'native-control-strip': !isSegmentMode }">
      <button class="seg-btn" @click="togglePlay">{{ isPlaying ? '暂停' : '播放' }}</button>
      <div class="seg-time">{{ progressCurrentLabel }} / {{ progressDurationLabel }}</div>
      <input
        class="seg-range native-range"
        type="range"
        min="0"
        max="1"
        step="0.001"
        :value="playbackProgress"
        :disabled="!isSegmentMode && !playbackDurationSec"
        @input="onSeekInput"
      />
      <div v-if="isSegmentMode" class="seg-meta">{{ Number(props.startTimeSec) }}s ~ {{ Number(props.endTimeSec) }}s</div>
      <div v-else class="seg-meta">完整视频进度</div>
    </div>
    <div v-if="errorText" class="error">{{ errorText }}</div>
  </div>
</template>

<style scoped>
.player {
  width: 100%;
  display: grid;
  gap: 12px;
  color: #f4fbfa;
}

.player-title {
  font-size: 15px;
  font-weight: 700;
  color: #f7fbfa;
}

.toolbar {
  display: flex;
  gap: 10px;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
}

.toolbar-left,
.toolbar-right {
  display: flex;
  gap: 10px;
  align-items: center;
  flex-wrap: wrap;
}

.hint {
  font-size: 12px;
  color: #b8c5c2;
}

.select {
  appearance: none;
  border: 1px solid rgba(223, 227, 225, 0.22);
  background: rgba(255, 255, 255, 0.08);
  color: #f7fbfa;
  border-radius: 8px;
  padding: 8px 12px;
}

.video {
  width: 100%;
  aspect-ratio: 16 / 9;
  max-height: min(58vh, 560px);
  background: #000;
  border-radius: 8px;
  display: block;
}

.segment-bar {
  display: grid;
  grid-template-columns: max-content max-content minmax(220px, 1fr) max-content;
  gap: 10px;
  align-items: center;
  padding: 10px 12px;
  border: 1px solid rgba(223, 244, 241, 0.18);
  border-radius: 8px;
  background: rgba(6, 14, 13, 0.92);
}

.native-control-strip {
  margin-top: -2px;
}

.seg-btn {
  min-height: 36px;
  border: 1px solid rgba(223, 244, 241, 0.24);
  background: #dff4f1;
  color: #0b5f59;
  border-radius: 7px;
  padding: 0 12px;
  cursor: pointer;
  font-weight: 760;
}

.seg-time,
.seg-meta {
  font-size: 12px;
  color: #c9d7d4;
  white-space: nowrap;
}

.seg-range {
  width: 100%;
  min-width: 0;
  accent-color: #21c7b7;
  cursor: pointer;
}

.seg-range:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.seg-range::-webkit-slider-runnable-track {
  height: 8px;
  border-radius: 999px;
  background: rgba(223, 244, 241, 0.24);
}

.seg-range::-webkit-slider-thumb {
  width: 18px;
  height: 18px;
  margin-top: -5px;
}

.seg-range::-moz-range-track {
  height: 8px;
  border-radius: 999px;
  background: rgba(223, 244, 241, 0.24);
}

.error {
  color: #b42318;
  font-size: 13px;
}

@media (max-width: 560px) {
  .segment-bar {
    grid-template-columns: 1fr;
    align-items: stretch;
  }

  .seg-btn {
    width: 100%;
    justify-content: center;
  }
}
</style>
