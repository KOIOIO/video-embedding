<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import HlsPlayer from './components/HlsPlayer.vue'
import { uploadVideoInChunks } from './chunkedUpload.js'
import { fetchRandomPlayableSegment } from './randomSegment.js'
import { normalizeSegmentReactionCounts, segmentIdOf, submitSegmentReaction as submitSegmentReactionRequest } from './segmentReaction.js'
import { buildWatchContext, reportWatchProgress as submitWatchProgress } from './watchProgress.js'

const API_BASE = '/api'
const DEBUG_USER_ID = 1
const REACTION_TYPES = {
  LIKE: 'like',
  DOUBLE_LIKE: 'double_like',
  DISLIKE: 'dislike',
}

const activeTab = ref('overview')

const serviceStatus = ref({ ok: false, checked: false, error: '' })
const swaggerUrl = import.meta.env.VITE_SWAGGER_URL || '/swagger/index.html'
const systemMetrics = ref(null)
const systemMetricsLoading = ref(false)
const systemMetricsError = ref('')
const cpuHistory = ref([])
const memoryHistory = ref([])
const processMemoryHistory = ref([])
const MAX_HISTORY_POINTS = 60
let systemMetricsTimer = null

const selectedFile = ref(null)
const uploadMode = ref('single')
const uploadTitle = ref('')
const uploadDescription = ref('')
const uploading = ref(false)
const uploadProgress = ref(0)
const uploadError = ref('')
const uploadResult = ref(null)

const transcodeStatus = ref(null)
const transcodeLoading = ref(false)
const transcodeError = ref('')
let statusTimer = null

const videosLoading = ref(false)
const videosError = ref('')
const videos = ref([])
const reactionCountsByVideoId = ref({})
const reactionActiveByVideoId = ref({})
const reactionLoadingByVideoId = ref({})
const reactionErrorByVideoId = ref({})
const segmentReactionCountsById = ref({})
const segmentReactionActiveById = ref({})
const segmentReactionLoadingById = ref({})
const segmentReactionErrorById = ref({})

const editorVideoId = ref(0)
const editorTitle = ref('')
const editorDescription = ref('')
const editorSaving = ref(false)

const similarLoading = ref(false)
const similarError = ref('')
const similarVideos = ref([])

const currentVideoId = ref(0)
const currentPlaySrc = ref('')
const currentPlayTitle = ref('')
const currentSegmentStart = ref(0)
const currentSegmentEnd = ref(0)
const currentWatchContext = ref(null)
const randomSegmentLoading = ref(false)
const randomSegmentError = ref('')
const currentRandomSegment = ref(null)

const questionListLoading = ref(false)
const questionListError = ref('')
const questionPage = ref(1)
const questionPageSize = 20
const questionTotal = ref(0)
const questions = ref([])

const expandedQuestionId = ref(0)
const currentQuestion = ref(null)
const questionDetailLoading = ref(false)

const recommendDraft = ref('')
const recommendLoading = ref(false)
const recommendError = ref('')
const recommendItems = ref([])

const questionRecommendLoading = ref(false)
const questionRecommendError = ref('')
const questionRecommendItems = ref([])

const questionPlayerSrc = ref('')
const questionPlayerTitle = ref('')
const questionPlayerStart = ref(0)
const questionPlayerEnd = ref(0)
const questionPlayerQuestionId = ref(0)
const questionWatchContext = ref(null)

const canPlay = computed(() => Boolean(currentPlaySrc.value))
const lastTaskId = computed(() => uploadResult.value?.task_id || '')
const lastVideoId = computed(() => Number(uploadResult.value?.video_id || 0) || 0)
const isArchiveUpload = computed(() => uploadMode.value === 'archive')
const uploadFileAccept = computed(() => (isArchiveUpload.value ? '.zip,application/zip,application/x-zip-compressed' : 'video/*'))
const uploadFileLabel = computed(() => (isArchiveUpload.value ? '选择 ZIP 压缩包' : '选择视频文件'))
const VIDEO_UPLOAD_CHUNK_SIZE = 8 * 1024 * 1024
const uploadButtonLabel = computed(() => {
  if (uploading.value) return '上传中…'
  return isArchiveUpload.value ? '上传 ZIP 批量导入' : '上传到 HTTP 后端'
})
const archiveVideos = computed(() => (Array.isArray(uploadResult.value?.videos) ? uploadResult.value.videos : []))
const archiveErrors = computed(() => (Array.isArray(uploadResult.value?.errors) ? uploadResult.value.errors : []))
const archiveSkippedFiles = computed(() => (Array.isArray(uploadResult.value?.skipped_files) ? uploadResult.value.skipped_files : []))
const questionTotalPages = computed(() => {
  const total = Number(questionTotal.value || 0)
  return Math.max(1, Math.ceil(total / questionPageSize))
})
const expandedQuestion = computed(() => {
  const id = Number(expandedQuestionId.value || 0)
  const q = currentQuestion.value
  return q && Number(q.id || 0) === id ? q : null
})

function escapeHtml(raw) {
  return String(raw || '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function previewQuestion(raw) {
  return String(raw || '').replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim()
}

function normalizeVideo(item) {
  if (!item || typeof item !== 'object') return null
  const videoId = Number(item.video_id || item.id || 0) || 0
  const title = String(item.title || item.name || '').trim()
  const rawUrl = String(item.raw_url || '')
  const hlsUrl = String(item.hls_url || '')
  return {
    ...item,
    id: videoId,
    video_id: videoId,
    name: title,
    title,
    raw_url: rawUrl,
    hls_url: hlsUrl,
    is_hls: Boolean(hlsUrl),
    is_published: Boolean(item.is_published),
    is_recommend: Boolean(item.is_recommend),
    view_count: Number(item.view_count || 0) || 0,
    like_count: Number(item.like_count || 0) || 0,
    double_like_count: Number(item.double_like_count || 0) || 0,
    has_reaction_counts: Object.prototype.hasOwnProperty.call(item, 'like_count')
      || Object.prototype.hasOwnProperty.call(item, 'double_like_count'),
    created_at_unix: Number(item.created_at_unix || 0) || 0,
    updated_at_unix: Number(item.updated_at_unix || 0) || 0,
    cover_url: String(item.cover_url || ''),
    description: String(item.description || ''),
  }
}

function normalizeVideos(list) {
  return Array.isArray(list) ? list.map(normalizeVideo).filter(Boolean) : []
}

async function requestJson(url, init) {
  const res = await fetch(url, init)
  const payload = await res.json().catch(() => null)
  const message = payload?.error?.message || payload?.message || `HTTP ${res.status}`
  if (!res.ok || payload?.success === false) {
    throw new Error(message)
  }
  return payload?.data ?? payload
}

function videoIdOf(video) {
  return Number(video?.id || video?.video_id || video || 0) || 0
}

function normalizeReactionCounts(data) {
  return {
    like_count: Number(data?.like_count || 0) || 0,
    double_like_count: Number(data?.double_like_count || 0) || 0,
  }
}

function setReactionCounts(videoId, counts) {
  const id = videoIdOf(videoId)
  if (!id) return
  reactionCountsByVideoId.value = {
    ...reactionCountsByVideoId.value,
    [id]: normalizeReactionCounts(counts),
  }
}

function mergeReactionCountsFromVideos(list) {
  const next = { ...reactionCountsByVideoId.value }
  let changed = false
  for (const video of list || []) {
    const id = videoIdOf(video)
    if (!id || !video?.has_reaction_counts) continue
    next[id] = normalizeReactionCounts(video)
    changed = true
  }
  if (changed) reactionCountsByVideoId.value = next
}

function reactionCountsFor(video) {
  const id = videoIdOf(video)
  return reactionCountsByVideoId.value[id] || normalizeReactionCounts(video)
}

function activeReactionFor(video) {
  return reactionActiveByVideoId.value[videoIdOf(video)] || ''
}

function isReactionLoading(video) {
  return Boolean(reactionLoadingByVideoId.value[videoIdOf(video)])
}

function reactionErrorFor(video) {
  return reactionErrorByVideoId.value[videoIdOf(video)] || ''
}

function setSegmentReactionCounts(segmentId, counts) {
  const id = segmentIdOf(segmentId)
  if (!id) return
  segmentReactionCountsById.value = {
    ...segmentReactionCountsById.value,
    [id]: normalizeSegmentReactionCounts(counts),
  }
}

function segmentReactionCountsFor(item) {
  const id = segmentIdOf(item)
  return segmentReactionCountsById.value[id] || normalizeSegmentReactionCounts(item)
}

function activeSegmentReactionFor(item) {
  return segmentReactionActiveById.value[segmentIdOf(item)] || ''
}

function isSegmentReactionLoading(item) {
  return Boolean(segmentReactionLoadingById.value[segmentIdOf(item)])
}

function segmentReactionErrorFor(item) {
  return segmentReactionErrorById.value[segmentIdOf(item)] || ''
}

async function fetchSegmentReactionCounts(item) {
  const id = segmentIdOf(item)
  if (!id) return null
  const data = await requestJson(`${API_BASE}/video-segments/${encodeURIComponent(String(id))}/reaction-counts`)
  setSegmentReactionCounts(id, data)
  return segmentReactionCountsById.value[id]
}

async function refreshSegmentReactionCountsForItems(list) {
  const ids = Array.from(new Set((list || []).map(segmentIdOf).filter(Boolean)))
  await Promise.all(ids.map(async (id) => {
    try {
      await fetchSegmentReactionCounts(id)
    } catch {
    }
  }))
}

async function fetchVideoReactionCounts(videoId) {
  const id = videoIdOf(videoId)
  if (!id) return null
  const data = await requestJson(`${API_BASE}/videos/${encodeURIComponent(String(id))}/reaction-counts`)
  setReactionCounts(id, data)
  return reactionCountsByVideoId.value[id]
}

async function refreshReactionCountsForVideos(list) {
  const ids = Array.from(new Set((list || []).map(videoIdOf).filter(Boolean)))
  await Promise.all(ids.map(async (id) => {
    try {
      await fetchVideoReactionCounts(id)
    } catch {
    }
  }))
}

async function submitVideoReaction(video, reactionType) {
  const id = videoIdOf(video)
  if (!id || isReactionLoading(video)) return
  reactionLoadingByVideoId.value = { ...reactionLoadingByVideoId.value, [id]: true }
  reactionErrorByVideoId.value = { ...reactionErrorByVideoId.value, [id]: '' }
  try {
    const data = await requestJson(`${API_BASE}/videos/${encodeURIComponent(String(id))}/reactions`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        user_id: DEBUG_USER_ID,
        reaction_type: reactionType,
      }),
    })
    reactionActiveByVideoId.value = {
      ...reactionActiveByVideoId.value,
      [id]: data?.active ? String(data.reaction_type || reactionType) : '',
    }
    await fetchVideoReactionCounts(id)
  } catch (error) {
    reactionErrorByVideoId.value = {
      ...reactionErrorByVideoId.value,
      [id]: String(error?.message || error),
    }
  } finally {
    reactionLoadingByVideoId.value = { ...reactionLoadingByVideoId.value, [id]: false }
  }
}

async function submitSegmentReaction(item, reactionType) {
  const id = segmentIdOf(item)
  if (!id || isSegmentReactionLoading(item)) return
  segmentReactionLoadingById.value = { ...segmentReactionLoadingById.value, [id]: true }
  segmentReactionErrorById.value = { ...segmentReactionErrorById.value, [id]: '' }
  try {
    const result = await submitSegmentReactionRequest({
      apiBase: API_BASE,
      item,
      reactionType,
      userId: DEBUG_USER_ID,
      requestJson,
    })
    segmentReactionActiveById.value = {
      ...segmentReactionActiveById.value,
      [id]: result.active ? result.reactionType : '',
    }
    setSegmentReactionCounts(id, result.counts)
  } catch (error) {
    segmentReactionErrorById.value = {
      ...segmentReactionErrorById.value,
      [id]: String(error?.message || error),
    }
  } finally {
    segmentReactionLoadingById.value = { ...segmentReactionLoadingById.value, [id]: false }
  }
}

function onFileChange(event) {
  selectedFile.value = event.target.files?.[0] || null
  uploadError.value = ''
}

function setUploadMode(mode) {
  uploadMode.value = mode
  selectedFile.value = null
  uploadError.value = ''
  uploadResult.value = null
  uploadProgress.value = 0
  transcodeStatus.value = null
  transcodeError.value = ''
}

function stopPolling() {
  if (statusTimer) {
    clearInterval(statusTimer)
    statusTimer = null
  }
}

function stopSystemMetricsPolling() {
  if (systemMetricsTimer) {
    clearInterval(systemMetricsTimer)
    systemMetricsTimer = null
  }
}

function formatBytes(bytes) {
  const value = Number(bytes || 0)
  if (!value) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = value
  let idx = 0
  while (size >= 1024 && idx < units.length - 1) {
    size /= 1024
    idx += 1
  }
  return `${size.toFixed(size >= 10 || idx === 0 ? 0 : 1)} ${units[idx]}`
}

function pushMetricPoint(target, value) {
  target.value.push(Number(value || 0))
  if (target.value.length > MAX_HISTORY_POINTS) {
    target.value.shift()
  }
}

function buildSparklinePoints(points, maxValue = 100) {
  if (!points.length) return ''
  const width = 100
  const height = 40
  const safeMax = Math.max(maxValue, 1)
  return points
    .map((point, index) => {
      const x = points.length === 1 ? 0 : (index / (points.length - 1)) * width
      const y = height - (Math.max(0, Math.min(point, safeMax)) / safeMax) * height
      return `${x},${y}`
    })
    .join(' ')
}

const cpuPolyline = computed(() => buildSparklinePoints(cpuHistory.value, 100))
const memoryPolyline = computed(() => buildSparklinePoints(memoryHistory.value, 100))
const processMemoryMax = computed(() => {
  const max = Math.max(...processMemoryHistory.value, 0)
  return max > 0 ? max : 1
})
const processMemoryPolyline = computed(() => buildSparklinePoints(processMemoryHistory.value, processMemoryMax.value))

async function fetchSystemMetrics() {
  systemMetricsLoading.value = true
  systemMetricsError.value = ''
  try {
    const data = await requestJson(`${API_BASE}/system/metrics`)
    systemMetrics.value = data
    pushMetricPoint(cpuHistory, data.cpu_percent)
    pushMetricPoint(memoryHistory, data.memory_used_percent)
    pushMetricPoint(processMemoryHistory, data.process_memory_bytes)
  } catch (error) {
    systemMetricsError.value = String(error?.message || error)
  } finally {
    systemMetricsLoading.value = false
  }
}

function startSystemMetricsPolling() {
  stopSystemMetricsPolling()
  void fetchSystemMetrics()
  systemMetricsTimer = setInterval(() => {
    void fetchSystemMetrics()
  }, 3000)
}

function formatUnixSeconds(value) {
  const sec = Number(value || 0)
  if (!sec) return '-'
  const date = new Date(sec * 1000)
  const pad = (n) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

function formatSegmentRange(startSec, endSec) {
  const start = Number(startSec || 0)
  const end = Number(endSec || 0)
  if (!(end > start)) return '-'
  return `${start}s - ${end}s`
}

function setFullPlayer(src, title) {
  currentPlaySrc.value = src || ''
  currentPlayTitle.value = title || ''
  currentSegmentStart.value = 0
  currentSegmentEnd.value = 0
  currentWatchContext.value = null
  currentRandomSegment.value = null
  activeTab.value = 'overview'
}

function setSegmentPlayer(src, title, startSec, endSec, watchContext = null) {
  currentPlaySrc.value = src || ''
  currentPlayTitle.value = title || ''
  currentSegmentStart.value = Number(startSec || 0)
  currentSegmentEnd.value = Number(endSec || 0)
  currentWatchContext.value = watchContext
  currentRandomSegment.value = null
  activeTab.value = 'overview'
}

async function playRandomSegment() {
  if (randomSegmentLoading.value) return
  randomSegmentLoading.value = true
  randomSegmentError.value = ''
  try {
    const item = await fetchRandomPlayableSegment({
      apiBase: API_BASE,
      requestJson,
    })
    currentVideoId.value = Number(item.video_id || 0) || 0
    setSegmentPlayer(item.play_url, item.title || '', item.start_time_sec, item.end_time_sec, buildWatchContext(item, 0))
    currentRandomSegment.value = item
    await fetchSegmentReactionCounts(item).catch(() => null)
  } catch (error) {
    randomSegmentError.value = String(error?.message || error)
  } finally {
    randomSegmentLoading.value = false
  }
}

function setQuestionPlayer(src, title, questionId, startSec = 0, endSec = 0, watchContext = null) {
  questionPlayerSrc.value = src || ''
  questionPlayerTitle.value = title || ''
  questionPlayerQuestionId.value = Number(questionId || 0) || 0
  questionPlayerStart.value = Number(startSec || 0)
  questionPlayerEnd.value = Number(endSec || 0)
  questionWatchContext.value = watchContext
}

async function fetchHealth() {
  serviceStatus.value = { ok: false, checked: false, error: '' }
  try {
    const res = await fetch(`${API_BASE}/healthz`)
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const data = await res.json().catch(() => null)
    serviceStatus.value = { ok: data?.status === 'ok', checked: true, error: '' }
  } catch (error) {
    serviceStatus.value = { ok: false, checked: true, error: String(error?.message || error) }
  }
}

async function fetchVideos() {
  videosLoading.value = true
  videosError.value = ''
  try {
    const data = await requestJson(`${API_BASE}/videos?type=ALL`)
    videos.value = normalizeVideos(data.videos)
    mergeReactionCountsFromVideos(videos.value)
    void refreshReactionCountsForVideos(videos.value)
  } catch (error) {
    videosError.value = String(error?.message || error)
  } finally {
    videosLoading.value = false
  }
}

async function fetchTranscodeStatus(taskId) {
  if (!taskId) return
  transcodeLoading.value = true
  transcodeError.value = ''
  try {
    const data = await requestJson(`${API_BASE}/transcode-tasks/${encodeURIComponent(taskId)}`)
    transcodeStatus.value = { message: '', ...data }
    if (data?.status === 'DONE' || data?.status === 'FAILED') {
      stopPolling()
      await fetchVideos()
    }
  } catch (error) {
    transcodeError.value = String(error?.message || error)
  } finally {
    transcodeLoading.value = false
  }
}

function startPolling(taskId) {
  stopPolling()
  void fetchTranscodeStatus(taskId)
  statusTimer = setInterval(() => {
    void fetchTranscodeStatus(taskId)
  }, 1500)
}

function handleUploadSuccess(data) {
  uploadResult.value = data
  const firstArchiveVideo = Array.isArray(data?.videos) ? data.videos[0] : null
  if (data?.task_id) {
    startPolling(data.task_id)
  } else if (firstArchiveVideo?.task_id) {
    startPolling(firstArchiveVideo.task_id)
  }
  if (data?.video_id) {
    void playVideo(data.video_id)
  } else if (firstArchiveVideo?.video_id) {
    void playVideo(firstArchiveVideo.video_id)
  }
  void fetchVideos()
}

async function uploadVideo() {
  uploadError.value = ''
  uploadProgress.value = 0
  uploadResult.value = null
  transcodeStatus.value = null
  transcodeError.value = ''
  if (!selectedFile.value) {
    uploadError.value = isArchiveUpload.value ? '请选择一个 ZIP 压缩包' : '请选择一个视频文件'
    return
  }

  uploading.value = true
  try {
    const data = await uploadVideoInChunks({
      apiBase: API_BASE,
      kind: isArchiveUpload.value ? 'archive' : 'video',
      file: selectedFile.value,
      title: uploadTitle.value,
      description: uploadDescription.value,
      chunkSize: VIDEO_UPLOAD_CHUNK_SIZE,
      requestJson,
      onProgress: (percent) => {
        uploadProgress.value = percent
      },
    })
    uploadProgress.value = 100
    handleUploadSuccess(data)
  } catch (error) {
    uploadError.value = String(error?.message || error)
  } finally {
    uploading.value = false
  }
}

async function playVideo(videoId) {
  const id = Number(videoId || 0)
  if (!id) return
  try {
    const data = await requestJson(`${API_BASE}/videos/${encodeURIComponent(String(id))}/play`)
    const video = normalizeVideo(data.video)
    currentVideoId.value = Number(video?.id || id) || id
    setFullPlayer(data.play_url || '', video?.title || '')
    await fetchSimilar(currentVideoId.value)
    void fetchVideos()
  } catch (error) {
    videosError.value = String(error?.message || error)
  }
}

async function fetchSimilar(videoId) {
  const id = Number(videoId || 0)
  if (!id) return
  similarLoading.value = true
  similarError.value = ''
  try {
    const data = await requestJson(`${API_BASE}/videos/${encodeURIComponent(String(id))}/similar?limit=6`)
    similarVideos.value = normalizeVideos(data.videos)
    mergeReactionCountsFromVideos(similarVideos.value)
    void refreshReactionCountsForVideos(similarVideos.value)
  } catch (error) {
    similarError.value = String(error?.message || error)
  } finally {
    similarLoading.value = false
  }
}

function startEdit(video) {
  editorVideoId.value = Number(video?.id || 0) || 0
  editorTitle.value = String(video?.title || '')
  editorDescription.value = String(video?.description || '')
}

function cancelEdit() {
  editorVideoId.value = 0
  editorTitle.value = ''
  editorDescription.value = ''
  editorSaving.value = false
}

function isEditing(video) {
  return Number(editorVideoId.value || 0) === Number(video?.id || 0)
}

async function saveVideo(video) {
  const id = Number(video?.id || 0)
  if (!id || editorSaving.value) return
  const title = editorTitle.value.trim()
  if (!title) {
    videosError.value = '标题不能为空'
    return
  }
  editorSaving.value = true
  videosError.value = ''
  try {
    await requestJson(`${API_BASE}/videos/${encodeURIComponent(String(id))}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title, description: editorDescription.value }),
    })
    cancelEdit()
    await fetchVideos()
    if (currentVideoId.value === id) {
      await fetchSimilar(id)
    }
  } catch (error) {
    videosError.value = String(error?.message || error)
  } finally {
    editorSaving.value = false
  }
}

async function deleteVideo(video) {
  const id = Number(video?.id || 0)
  if (!id) return
  try {
    await requestJson(`${API_BASE}/videos/${encodeURIComponent(String(id))}`, { method: 'DELETE' })
    if (currentVideoId.value === id) {
      currentVideoId.value = 0
      setFullPlayer('', '')
      similarVideos.value = []
    }
    await fetchVideos()
  } catch (error) {
    videosError.value = String(error?.message || error)
  }
}

async function togglePublished(video) {
  const id = Number(video?.id || 0)
  if (!id) return
  try {
    await requestJson(`${API_BASE}/videos/${encodeURIComponent(String(id))}/publish`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ is_published: !Boolean(video?.is_published) }),
    })
    await fetchVideos()
    if (currentVideoId.value === id) {
      await fetchSimilar(id)
    }
  } catch (error) {
    videosError.value = String(error?.message || error)
  }
}

async function toggleRecommend(video) {
  const id = Number(video?.id || 0)
  if (!id) return
  try {
    await requestJson(`${API_BASE}/videos/${encodeURIComponent(String(id))}/recommend`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        is_recommend: !Boolean(video?.is_recommend),
        user_id: 1,
        recommend_level: 1,
        recommend_score: 0.99,
      }),
    })
    await fetchVideos()
  } catch (error) {
    videosError.value = String(error?.message || error)
  }
}

async function uploadCover(video, event) {
  const id = Number(video?.id || 0)
  const file = event?.target?.files?.[0]
  if (!id || !file) return

  try {
    const form = new FormData()
    form.append('file', file)
    const response = await fetch(`${API_BASE}/videos/${encodeURIComponent(String(id))}/cover`, {
      method: 'POST',
      body: form,
    })
    const payload = await response.json().catch(() => null)
    if (!response.ok || payload?.success === false) {
      throw new Error(payload?.error?.message || payload?.message || `HTTP ${response.status}`)
    }
    await fetchVideos()
    if (currentVideoId.value === id) {
      await fetchSimilar(id)
    }
  } catch (error) {
    videosError.value = String(error?.message || error)
  } finally {
    if (event?.target) event.target.value = ''
  }
}

async function fetchQuestions(page = questionPage.value) {
  questionListLoading.value = true
  questionListError.value = ''
  try {
    const data = await requestJson(`${API_BASE}/questions?page=${encodeURIComponent(String(page))}&page_size=${questionPageSize}`)
    questionPage.value = Number(data.page || page) || 1
    questionTotal.value = Number(data.total || 0) || 0
    questions.value = Array.isArray(data.list) ? data.list : []
  } catch (error) {
    questionListError.value = String(error?.message || error)
  } finally {
    questionListLoading.value = false
  }
}

function goQuestionPage(step) {
  const next = Number(questionPage.value || 1) + Number(step || 0)
  if (next < 1 || next > questionTotalPages.value) return
  void fetchQuestions(next)
}

async function openQuestion(question) {
  const questionId = Number(question?.id || 0)
  if (!questionId) return
  if (expandedQuestionId.value === questionId) {
    expandedQuestionId.value = 0
    currentQuestion.value = null
    questionRecommendItems.value = []
    questionRecommendError.value = ''
    setQuestionPlayer('', '', 0)
    return
  }

  expandedQuestionId.value = questionId
  currentQuestion.value = null
  questionRecommendItems.value = []
  questionRecommendError.value = ''
  questionDetailLoading.value = true
  setQuestionPlayer('', '', 0)
  try {
    const data = await requestJson(`${API_BASE}/questions/${encodeURIComponent(String(questionId))}`)
    currentQuestion.value = data.question || null
  } catch (error) {
    questionRecommendError.value = String(error?.message || error)
  } finally {
    questionDetailLoading.value = false
  }
}

async function fetchRecommendByDraft() {
  recommendError.value = ''
  const questionText = recommendDraft.value.trim()
  if (!questionText) {
    recommendError.value = '请输入问题'
    return
  }
  recommendLoading.value = true
  try {
    const data = await requestJson(`${API_BASE}/recommendations/by-question`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ question_text: questionText, limit: 3 }),
    })
    recommendItems.value = Array.isArray(data.items) ? data.items : []
    void refreshSegmentReactionCountsForItems(recommendItems.value)
  } catch (error) {
    recommendError.value = String(error?.message || error)
  } finally {
    recommendLoading.value = false
  }
}

async function fetchRecommendForQuestion() {
  if (!currentQuestion.value?.id) {
    questionRecommendError.value = '请先选择题目'
    return
  }
  questionRecommendError.value = ''
  questionRecommendLoading.value = true
  try {
    const data = await requestJson(`${API_BASE}/recommendations/by-question`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        question_id: currentQuestion.value.id,
        question_text: String(currentQuestion.value.content || '').trim(),
        user_id: 1,
        limit: 3,
      }),
    })
    questionRecommendItems.value = Array.isArray(data.items) ? data.items : []
    void refreshSegmentReactionCountsForItems(questionRecommendItems.value)
  } catch (error) {
    questionRecommendError.value = String(error?.message || error)
  } finally {
    questionRecommendLoading.value = false
  }
}

async function reportWatchProgress(context, watchedSec, isWatched = true) {
  try {
    await submitWatchProgress({
      apiBase: API_BASE,
      context,
      watchedSec,
      isWatched,
      userId: DEBUG_USER_ID,
      requestJson,
    })
  } catch (error) {
    questionRecommendError.value = String(error?.message || error)
  }
}

function onCurrentWatchProgress(event) {
  void reportWatchProgress(event?.context || currentWatchContext.value, event?.watchedSec, Boolean(event?.completed))
}

function onQuestionWatchProgress(event) {
  void reportWatchProgress(event?.context || questionWatchContext.value, event?.watchedSec, Boolean(event?.completed))
}

async function markLearned(item) {
  const context = buildWatchContext(item, currentQuestion.value?.id)
  const fallbackDuration = Math.max(1, Number(item?.end_time_sec || 0) - Number(item?.start_time_sec || 0))
  await reportWatchProgress(context, fallbackDuration, true)
}

function playRecommendationSegment(item) {
  if (!item?.play_url) return
  currentVideoId.value = Number(item?.video_id || 0) || 0
  setSegmentPlayer(item.play_url, item.title || '', item.start_time_sec, item.end_time_sec, buildWatchContext(item, item.question_id))
}

function playRecommendationFull(item) {
  if (!item?.video_id) return
  void playVideo(item.video_id)
}

function playQuestionRecommendationSegment(item) {
  if (!item?.play_url || !currentQuestion.value?.id) return
  setQuestionPlayer(
    item.play_url,
    item.title || '',
    currentQuestion.value.id,
    item.start_time_sec,
    item.end_time_sec,
    buildWatchContext(item, currentQuestion.value.id),
  )
}

function playQuestionRecommendationFull(item) {
  if (!item?.play_url || !currentQuestion.value?.id) return
  setQuestionPlayer(
    item.play_url,
    item.title || '',
    currentQuestion.value.id,
    0,
    0,
    buildWatchContext(item, currentQuestion.value.id),
  )
}

onMounted(() => {
  void fetchHealth()
  void fetchVideos()
  void fetchQuestions(1)
  startSystemMetricsPolling()
})

onBeforeUnmount(() => {
  stopPolling()
  stopSystemMetricsPolling()
})
</script>

<template>
  <div class="console-page">
    <header class="hero-shell">
      <div class="hero-decor" aria-hidden="true">
        <span class="hero-bubble hero-bubble-a"></span>
        <span class="hero-bubble hero-bubble-b"></span>
        <span class="hero-bubble hero-bubble-c"></span>
      </div>
      <div class="hero-copy">
        <p class="eyebrow">HTTP Backend Test Console</p>
        <h1>智能教学视频服务 联调面板</h1>
        <p class="hero-text">
          面向智能教学视频分析与推荐系统的轻卡通联调工作台，覆盖上传、转码、播放、题库、推荐与观看记录上报。
        </p>
        <div class="hero-metrics" aria-label="console overview">
          <div class="hero-pill">
            <span class="hero-pill-label">视频资源</span>
            <strong>{{ videos.length }}</strong>
          </div>
          <div class="hero-pill">
            <span class="hero-pill-label">题库总数</span>
            <strong>{{ questionTotal }}</strong>
          </div>
          <div class="hero-pill">
            <span class="hero-pill-label">当前播放</span>
            <strong>{{ canPlay ? '进行中' : '待选择' }}</strong>
          </div>
        </div>
      </div>
      <div class="hero-actions">
        <a class="ghost-link" :href="swaggerUrl" target="_blank" rel="noreferrer">打开 Swagger</a>
        <button class="secondary-btn" @click="fetchHealth">刷新健康状态</button>
      </div>
      <div class="status-ribbon" :class="serviceStatus.ok ? 'ok' : serviceStatus.checked ? 'bad' : 'idle'">
        <span class="dot"></span>
        <span v-if="!serviceStatus.checked">未检测</span>
        <span v-else-if="serviceStatus.ok">HTTP 服务可达</span>
        <span v-else>服务异常：{{ serviceStatus.error || 'healthz 不可用' }}</span>
      </div>
    </header>

    <nav class="tab-nav" role="tablist" aria-label="功能导航">
      <button type="button" class="tab-btn" :class="{ active: activeTab === 'overview' }" @click="activeTab = 'overview'">概览</button>
      <button type="button" class="tab-btn" :class="{ active: activeTab === 'videos' }" @click="activeTab = 'videos'">视频管理</button>
      <button type="button" class="tab-btn" :class="{ active: activeTab === 'questions' }" @click="activeTab = 'questions'">题库推荐</button>
    </nav>

    <div v-if="activeTab === 'overview'" class="tab-content">
      <section class="panel metrics-panel">
        <div class="panel-head">
          <div>
            <p class="panel-tag">System Metrics</p>
            <h2>服务器 CPU / 内存监控</h2>
          </div>
          <div class="panel-actions">
            <button class="secondary-btn" :disabled="systemMetricsLoading" @click="fetchSystemMetrics">刷新监控</button>
          </div>
        </div>
        <p class="muted-line section-intro">展示后端所在机器的整机 CPU / 内存状态，以及当前 Go 进程的内存与 goroutine 数量。</p>
        <p v-if="systemMetricsError" class="feedback error">{{ systemMetricsError }}</p>
        <div v-if="systemMetrics" class="metrics-grid chart-grid">
          <article class="metric-card chart-card">
            <div class="metric-head">
              <span class="metric-label">CPU 使用率</span>
              <strong class="metric-value">{{ Number(systemMetrics.cpu_percent || 0).toFixed(1) }}%</strong>
            </div>
            <svg class="sparkline" viewBox="0 0 100 40" preserveAspectRatio="none">
              <polyline :points="cpuPolyline" />
            </svg>
          </article>
          <article class="metric-card chart-card">
            <div class="metric-head">
              <span class="metric-label">内存使用率</span>
              <strong class="metric-value">{{ Number(systemMetrics.memory_used_percent || 0).toFixed(1) }}%</strong>
            </div>
            <svg class="sparkline" viewBox="0 0 100 40" preserveAspectRatio="none">
              <polyline :points="memoryPolyline" />
            </svg>
          </article>
          <article class="metric-card chart-card">
            <div class="metric-head">
              <span class="metric-label">Go 进程内存</span>
              <strong class="metric-value">{{ formatBytes(systemMetrics.process_memory_bytes) }}</strong>
            </div>
            <svg class="sparkline" viewBox="0 0 100 40" preserveAspectRatio="none">
              <polyline :points="processMemoryPolyline" />
            </svg>
          </article>
          <article class="metric-card">
            <span class="metric-label">整机内存</span>
            <strong class="metric-value">{{ formatBytes(systemMetrics.memory_used_bytes) }} / {{ formatBytes(systemMetrics.memory_total_bytes) }}</strong>
          </article>
          <article class="metric-card">
            <span class="metric-label">Goroutine 数</span>
            <strong class="metric-value">{{ systemMetrics.goroutines }}</strong>
          </article>
          <article class="metric-card">
            <span class="metric-label">最近更新时间</span>
            <strong class="metric-value">{{ systemMetrics.timestamp || '-' }}</strong>
          </article>
        </div>
        <div v-if="systemMetrics?.active_counts" class="metrics-grid active-grid">
          <article class="metric-card">
            <span class="metric-label">转码任务并发</span>
            <strong class="metric-value">{{ systemMetrics.active_counts.transcode_tasks_active || 0 }}</strong>
          </article>
          <article class="metric-card">
            <span class="metric-label">向量化任务并发</span>
            <strong class="metric-value">{{ systemMetrics.active_counts.vector_tasks_active || 0 }}</strong>
          </article>
          <article class="metric-card">
            <span class="metric-label">粗切分并发</span>
            <strong class="metric-value">{{ systemMetrics.active_counts.vector_coarse_clip_active || 0 }}</strong>
          </article>
          <article class="metric-card">
            <span class="metric-label">粗上传并发</span>
            <strong class="metric-value">{{ systemMetrics.active_counts.vector_coarse_upload_active || 0 }}</strong>
          </article>
          <article class="metric-card">
            <span class="metric-label">粗 ASR 并发</span>
            <strong class="metric-value">{{ systemMetrics.active_counts.vector_coarse_asr_active || 0 }}</strong>
          </article>
          <article class="metric-card">
            <span class="metric-label">Refine ASR 并发</span>
            <strong class="metric-value">{{ systemMetrics.active_counts.vector_refine_asr_active || 0 }}</strong>
          </article>
        </div>
        <div v-else-if="systemMetricsLoading" class="feedback muted">监控指标加载中…</div>
        <div v-else class="empty-state small">暂无监控数据</div>
      </section>

      <section class="panel wide-panel">
        <div class="panel-head">
          <div>
            <p class="panel-tag">Playback</p>
            <h2>视频播放器</h2>
          </div>
          <div class="panel-actions">
            <button class="primary-btn" :disabled="randomSegmentLoading" @click="playRandomSegment">
              {{ randomSegmentLoading ? '随机中…' : '随机播放片段' }}
            </button>
            <button class="secondary-btn" :disabled="!lastVideoId" @click="playVideo(lastVideoId)">播放最新上传</button>
          </div>
        </div>
        <p class="muted-line section-intro">优先展示当前选中的完整视频或推荐片段，方便你在联调时快速验证播放链路。</p>
        <p v-if="randomSegmentError" class="feedback error">{{ randomSegmentError }}</p>

        <div v-if="canPlay" class="player-frame">
          <HlsPlayer
            :src="currentPlaySrc"
            :title="currentPlayTitle"
            :start-time-sec="currentSegmentStart"
            :end-time-sec="currentSegmentEnd"
            :watch-context="currentWatchContext"
            @watch-progress="onCurrentWatchProgress"
          />
        </div>
        <div v-else class="empty-state">选择视频或推荐片段后开始播放。</div>

        <div v-if="currentRandomSegment" class="random-segment-strip">
          <div>
            <div class="random-segment-title">{{ currentRandomSegment.title || '-' }}</div>
            <div class="recommend-meta">
              随机片段 {{ formatSegmentRange(currentRandomSegment.start_time_sec, currentRandomSegment.end_time_sec) }}
              · Segment {{ currentRandomSegment.video_segment_id }}
            </div>
          </div>
          <div class="reaction-strip compact-reactions segment-reactions">
            <button
              type="button"
              class="reaction-btn"
              :class="{ active: activeSegmentReactionFor(currentRandomSegment) === REACTION_TYPES.LIKE }"
              :disabled="isSegmentReactionLoading(currentRandomSegment)"
              :aria-pressed="activeSegmentReactionFor(currentRandomSegment) === REACTION_TYPES.LIKE"
              @click="submitSegmentReaction(currentRandomSegment, REACTION_TYPES.LIKE)"
            >
              赞 {{ segmentReactionCountsFor(currentRandomSegment).like_count }}
            </button>
            <button
              type="button"
              class="reaction-btn"
              :class="{ active: activeSegmentReactionFor(currentRandomSegment) === REACTION_TYPES.DOUBLE_LIKE }"
              :disabled="isSegmentReactionLoading(currentRandomSegment)"
              :aria-pressed="activeSegmentReactionFor(currentRandomSegment) === REACTION_TYPES.DOUBLE_LIKE"
              @click="submitSegmentReaction(currentRandomSegment, REACTION_TYPES.DOUBLE_LIKE)"
            >
              双赞 {{ segmentReactionCountsFor(currentRandomSegment).double_like_count }}
            </button>
            <button
              type="button"
              class="reaction-btn dislike"
              :class="{ active: activeSegmentReactionFor(currentRandomSegment) === REACTION_TYPES.DISLIKE }"
              :disabled="isSegmentReactionLoading(currentRandomSegment)"
              :aria-pressed="activeSegmentReactionFor(currentRandomSegment) === REACTION_TYPES.DISLIKE"
              @click="submitSegmentReaction(currentRandomSegment, REACTION_TYPES.DISLIKE)"
            >
              倒赞
            </button>
          </div>
          <p v-if="segmentReactionErrorFor(currentRandomSegment)" class="feedback error reaction-error">{{ segmentReactionErrorFor(currentRandomSegment) }}</p>
        </div>

        <div class="subpanel">
          <div class="subpanel-head">
            <h3>相似视频</h3>
            <button class="secondary-btn" :disabled="!currentVideoId || similarLoading" @click="fetchSimilar(currentVideoId)">刷新相似视频</button>
          </div>
          <p v-if="similarError" class="feedback error">{{ similarError }}</p>
          <div v-if="similarLoading" class="feedback muted">加载中…</div>
          <div v-else-if="similarVideos.length" class="video-list compact-list">
            <article v-for="video in similarVideos" :key="`similar-${video.id}`" class="video-row">
              <div class="video-cover">
                <img v-if="video.cover_url" :src="video.cover_url" alt="cover" />
                <span v-else>NO COVER</span>
              </div>
              <div class="video-meta">
                <div class="video-title">{{ video.title || '-' }}</div>
                <div class="muted-line">view_count {{ video.view_count || 0 }}</div>
                <div class="reaction-strip compact-reactions">
                  <button
                    type="button"
                    class="reaction-btn"
                    :class="{ active: activeReactionFor(video) === REACTION_TYPES.LIKE }"
                    :disabled="isReactionLoading(video)"
                    :aria-pressed="activeReactionFor(video) === REACTION_TYPES.LIKE"
                    @click="submitVideoReaction(video, REACTION_TYPES.LIKE)"
                  >
                    赞 {{ reactionCountsFor(video).like_count }}
                  </button>
                  <button
                    type="button"
                    class="reaction-btn"
                    :class="{ active: activeReactionFor(video) === REACTION_TYPES.DOUBLE_LIKE }"
                    :disabled="isReactionLoading(video)"
                    :aria-pressed="activeReactionFor(video) === REACTION_TYPES.DOUBLE_LIKE"
                    @click="submitVideoReaction(video, REACTION_TYPES.DOUBLE_LIKE)"
                  >
                    双赞 {{ reactionCountsFor(video).double_like_count }}
                  </button>
                  <button
                    type="button"
                    class="reaction-btn dislike"
                    :class="{ active: activeReactionFor(video) === REACTION_TYPES.DISLIKE }"
                    :disabled="isReactionLoading(video)"
                    :aria-pressed="activeReactionFor(video) === REACTION_TYPES.DISLIKE"
                    @click="submitVideoReaction(video, REACTION_TYPES.DISLIKE)"
                  >
                    倒赞
                  </button>
                </div>
              </div>
              <div class="mini-actions">
                <button class="tiny-btn" @click="playVideo(video.id)">播放</button>
              </div>
            </article>
          </div>
          <div v-else class="empty-state small">暂无相似视频</div>
        </div>
      </section>
    </div>

    <div v-else-if="activeTab === 'videos'" class="tab-content">
      <main class="dashboard-grid">
        <section class="panel upload-panel">
          <div class="panel-head">
            <div>
              <p class="panel-tag">Upload</p>
              <h2>上传视频与转码跟踪</h2>
            </div>
          </div>

          <div class="stack">
            <div class="segmented-control" role="tablist" aria-label="上传模式">
              <button type="button" :class="{ active: !isArchiveUpload }" @click="setUploadMode('single')">
                单个视频
              </button>
              <button type="button" :class="{ active: isArchiveUpload }" @click="setUploadMode('archive')">
                ZIP 批量
              </button>
            </div>
            <label class="field-stack">
              <span class="field-label">{{ uploadFileLabel }}</span>
              <input class="field file-field" type="file" :accept="uploadFileAccept" @change="onFileChange" />
            </label>
            <label class="field-stack" v-if="!isArchiveUpload">
              <span class="field-label">视频标题</span>
              <input class="field" v-model="uploadTitle" placeholder="给视频起一个好辨认的标题（可选）" />
            </label>
            <label class="field-stack">
              <span class="field-label">{{ isArchiveUpload ? '批量描述' : '视频描述' }}</span>
              <textarea class="field textarea" v-model="uploadDescription" rows="3" placeholder="补充来源、主题或测试说明（可选）"></textarea>
            </label>
            <div class="button-row">
              <button class="primary-btn" :disabled="uploading || !selectedFile" @click="uploadVideo">
                {{ uploadButtonLabel }}
              </button>
              <button class="secondary-btn" :disabled="!lastTaskId || transcodeLoading" @click="fetchTranscodeStatus(lastTaskId)">
                刷新转码状态
              </button>
            </div>
          </div>

          <div v-if="uploading" class="progress-track">
            <div class="progress-fill" :style="{ width: `${uploadProgress}%` }"></div>
          </div>
          <p v-if="uploadError" class="feedback error">{{ uploadError }}</p>

          <div class="kv-grid" v-if="uploadResult && !isArchiveUpload">
            <div>video_id</div>
            <div>{{ uploadResult.video_id || '-' }}</div>
            <div>task_id</div>
            <div>{{ uploadResult.task_id || '-' }}</div>
            <div>raw_url</div>
            <div><a class="inline-link" :href="uploadResult.raw_url" target="_blank">{{ uploadResult.raw_url || '-' }}</a></div>
            <div>hls_url</div>
            <div><a class="inline-link" :href="uploadResult.hls_url" target="_blank">{{ uploadResult.hls_url || '-' }}</a></div>
          </div>

          <div class="subpanel" v-if="uploadResult && isArchiveUpload">
            <h3>ZIP 导入结果</h3>
            <div class="kv-grid compact">
              <div>total</div>
              <div>{{ uploadResult.total ?? '-' }}</div>
              <div>uploaded</div>
              <div>{{ uploadResult.uploaded ?? 0 }}</div>
              <div>failed</div>
              <div>{{ uploadResult.failed ?? 0 }}</div>
              <div>skipped</div>
              <div>{{ uploadResult.skipped ?? 0 }}</div>
            </div>

            <div class="archive-list" v-if="archiveVideos.length">
              <div class="archive-row" v-for="video in archiveVideos" :key="video.video_id || video.task_id">
                <strong>{{ video.file_name || video.video_id }}</strong>
                <span>video_id {{ video.video_id }}</span>
                <a class="inline-link" :href="video.raw_url" target="_blank">raw</a>
              </div>
            </div>

            <p v-if="archiveSkippedFiles.length" class="feedback muted">
              已跳过：{{ archiveSkippedFiles.join('、') }}
            </p>
            <p v-for="item in archiveErrors" :key="item.file_name" class="feedback error">
              {{ item.file_name }}：{{ item.error }}
            </p>
          </div>

          <div class="subpanel" v-if="transcodeStatus || transcodeError">
            <h3>转码状态</h3>
            <p v-if="transcodeError" class="feedback error">{{ transcodeError }}</p>
            <div v-else-if="transcodeStatus" class="kv-grid compact">
              <div>status</div>
              <div>{{ transcodeStatus.status || '-' }}</div>
              <div>message</div>
              <div>{{ transcodeStatus.message || '-' }}</div>
              <div>hls_url</div>
              <div><a class="inline-link" :href="transcodeStatus.hls_url" target="_blank">{{ transcodeStatus.hls_url || '-' }}</a></div>
            </div>
          </div>
        </section>

        <section class="panel wide-panel">
          <div class="panel-head">
            <div>
              <p class="panel-tag">Videos</p>
              <h2>视频资源列表</h2>
            </div>
            <div class="panel-actions">
              <button class="secondary-btn" :disabled="videosLoading" @click="fetchVideos">刷新列表</button>
            </div>
          </div>
          <p class="muted-line section-intro">封面、发布、推荐和元数据编辑集中在同一张卡片里，减少来回跳转。</p>

          <p v-if="videosError" class="feedback error">{{ videosError }}</p>
          <div v-if="videosLoading" class="feedback muted">正在加载视频列表…</div>

          <div v-if="videos.length" class="video-list">
            <article v-for="video in videos" :key="video.id || video.video_id" class="video-card">
              <label class="video-cover uploader">
                <img v-if="video.cover_url" :src="video.cover_url" alt="cover" />
                <span v-else>UPLOAD COVER</span>
                <input type="file" accept="image/*" hidden @change="(event) => uploadCover(video, event)" />
              </label>

              <div class="video-body">
                <template v-if="isEditing(video)">
                  <label class="field-stack">
                    <span class="field-label">标题</span>
                    <input class="field small-field" v-model="editorTitle" maxlength="200" placeholder="标题" />
                  </label>
                  <label class="field-stack">
                    <span class="field-label">描述</span>
                    <textarea class="field textarea small-field" v-model="editorDescription" rows="3" maxlength="5000" placeholder="描述"></textarea>
                  </label>
                </template>
                <template v-else>
                  <div class="video-title">{{ video.title || '-' }}</div>
                  <div class="video-desc">{{ video.description || '暂无描述' }}</div>
                </template>

                <div class="badge-row">
                  <span class="badge" :class="video.is_hls ? 'ok' : 'idle'">{{ video.is_hls ? '可播放' : '转码中' }}</span>
                  <span class="badge" :class="video.is_published ? 'ok' : 'idle'">{{ video.is_published ? '已发布' : '未发布' }}</span>
                  <span class="badge" :class="video.is_recommend ? 'ok' : 'idle'">{{ video.is_recommend ? '已推荐' : '未推荐' }}</span>
                </div>

                <div class="meta-strip">
                  <span>ID {{ video.id }}</span>
                  <span>观看 {{ video.view_count || 0 }}</span>
                  <span>{{ formatUnixSeconds(video.created_at_unix) }}</span>
                </div>

                <div class="reaction-strip">
                  <button
                    type="button"
                    class="reaction-btn"
                    :class="{ active: activeReactionFor(video) === REACTION_TYPES.LIKE }"
                    :disabled="isReactionLoading(video)"
                    :aria-pressed="activeReactionFor(video) === REACTION_TYPES.LIKE"
                    @click="submitVideoReaction(video, REACTION_TYPES.LIKE)"
                  >
                    点赞 {{ reactionCountsFor(video).like_count }}
                  </button>
                  <button
                    type="button"
                    class="reaction-btn"
                    :class="{ active: activeReactionFor(video) === REACTION_TYPES.DOUBLE_LIKE }"
                    :disabled="isReactionLoading(video)"
                    :aria-pressed="activeReactionFor(video) === REACTION_TYPES.DOUBLE_LIKE"
                    @click="submitVideoReaction(video, REACTION_TYPES.DOUBLE_LIKE)"
                  >
                    点双赞 {{ reactionCountsFor(video).double_like_count }}
                  </button>
                  <button
                    type="button"
                    class="reaction-btn dislike"
                    :class="{ active: activeReactionFor(video) === REACTION_TYPES.DISLIKE }"
                    :disabled="isReactionLoading(video)"
                    :aria-pressed="activeReactionFor(video) === REACTION_TYPES.DISLIKE"
                    @click="submitVideoReaction(video, REACTION_TYPES.DISLIKE)"
                  >
                    倒赞
                  </button>
                </div>
                <p v-if="reactionErrorFor(video)" class="feedback error reaction-error">{{ reactionErrorFor(video) }}</p>

                <div class="button-row wrap">
                  <template v-if="isEditing(video)">
                    <button class="primary-btn small" :disabled="editorSaving || !editorTitle.trim()" @click="saveVideo(video)">
                      {{ editorSaving ? '保存中…' : '保存' }}
                    </button>
                    <button class="secondary-btn small" :disabled="editorSaving" @click="cancelEdit">取消</button>
                  </template>
                  <template v-else>
                    <button class="primary-btn small" @click="playVideo(video.id)">播放</button>
                    <button class="secondary-btn small" @click="startEdit(video)">编辑</button>
                    <button class="secondary-btn small" @click="togglePublished(video)">{{ video.is_published ? '取消发布' : '发布' }}</button>
                    <button class="secondary-btn small" @click="toggleRecommend(video)">{{ video.is_recommend ? '取消推荐' : '设为推荐' }}</button>
                    <button class="danger-btn small" @click="deleteVideo(video)">删除</button>
                  </template>
                </div>
              </div>
            </article>
          </div>
          <div v-else-if="!videosLoading" class="empty-state">暂无视频资源</div>
        </section>
      </main>
    </div>

    <div v-else class="tab-content">
      <section class="panel wide-panel">
        <div class="panel-head">
          <div>
            <p class="panel-tag">Questions</p>
            <h2>题库与按题推荐</h2>
          </div>
          <div class="panel-actions">
            <button class="secondary-btn" :disabled="questionListLoading" @click="fetchQuestions(questionPage)">刷新题库</button>
            <button class="secondary-btn" :disabled="questionListLoading || questionPage <= 1" @click="goQuestionPage(-1)">上一页</button>
            <button class="secondary-btn" :disabled="questionListLoading || questionPage >= questionTotalPages" @click="goQuestionPage(1)">下一页</button>
          </div>
        </div>
        <p class="muted-line section-intro">题目卡片做成更容易扫描的阅读结构，展开后直接触发按题推荐和片段试听。</p>

        <div class="meta-strip tight">
          <span>当前页 {{ questionPage }}</span>
          <span>总页数 {{ questionTotalPages }}</span>
          <span>总数 {{ questionTotal }}</span>
        </div>
        <p v-if="questionListError" class="feedback error">{{ questionListError }}</p>

        <div class="question-grid">
          <article v-for="question in questions" :key="question.id" class="question-card">
            <div class="question-header">
              <div>
                <div class="question-id">Q{{ question.id }}</div>
                <div class="question-meta">{{ question.subject || '-' }} · {{ question.type || '-' }}</div>
              </div>
              <button class="secondary-btn small" @click="openQuestion(question)">
                {{ expandedQuestionId === question.id ? '收起' : '查看' }}
              </button>
            </div>
            <p class="question-preview">{{ previewQuestion(question.content) || '-' }}</p>

            <div v-if="expandedQuestionId === question.id" class="question-detail">
              <div v-if="questionDetailLoading" class="feedback muted">题目详情加载中…</div>
              <template v-else-if="expandedQuestion">
                <div class="detail-block">
                  <div class="detail-label">题干</div>
                  <div class="detail-value" v-html="escapeHtml(expandedQuestion.content)"></div>
                </div>
                <div class="detail-block">
                  <div class="detail-label">答案</div>
                  <div class="detail-value" v-html="escapeHtml(expandedQuestion.answer || '-')"></div>
                </div>
                <div class="detail-block">
                  <div class="detail-label">解析</div>
                  <div class="detail-value" v-html="escapeHtml(expandedQuestion.analysis || '-')"></div>
                </div>

                <div class="button-row">
                  <button class="primary-btn small" :disabled="questionRecommendLoading" @click="fetchRecommendForQuestion">
                    {{ questionRecommendLoading ? '推荐中…' : '根据本题推荐视频' }}
                  </button>
                </div>
                <p v-if="questionRecommendError" class="feedback error">{{ questionRecommendError }}</p>

                <div v-if="questionPlayerQuestionId === question.id && questionPlayerSrc" class="question-player">
                  <HlsPlayer
                    :src="questionPlayerSrc"
                    :title="questionPlayerTitle"
                    :start-time-sec="questionPlayerStart"
                    :end-time-sec="questionPlayerEnd"
                    :watch-context="questionWatchContext"
                    @watch-progress="onQuestionWatchProgress"
                  />
                </div>

                <div class="recommend-card-list">
                  <article v-for="item in questionRecommendItems" :key="`question-${question.id}-${item.video_segment_id}`" class="recommend-card mini">
                    <div class="recommend-title">{{ item.title || '-' }}</div>
                    <div class="recommend-meta">{{ formatSegmentRange(item.start_time_sec, item.end_time_sec) }} · {{ Number(item.recommend_score || 0).toFixed(4) }}</div>
                    <div class="button-row wrap">
                      <button class="tiny-btn" :disabled="!item.play_url" @click="playQuestionRecommendationSegment(item)">片段播放</button>
                      <button class="tiny-btn ghost" :disabled="!item.play_url" @click="playQuestionRecommendationFull(item)">完整播放</button>
                      <button class="tiny-btn ghost" :disabled="!item.video_segment_id" @click="markLearned(item)">标记学会</button>
                    </div>
                    <div class="reaction-strip compact-reactions segment-reactions">
                      <button
                        type="button"
                        class="reaction-btn"
                        :class="{ active: activeSegmentReactionFor(item) === REACTION_TYPES.LIKE }"
                        :disabled="isSegmentReactionLoading(item)"
                        :aria-pressed="activeSegmentReactionFor(item) === REACTION_TYPES.LIKE"
                        @click="submitSegmentReaction(item, REACTION_TYPES.LIKE)"
                      >
                        赞 {{ segmentReactionCountsFor(item).like_count }}
                      </button>
                      <button
                        type="button"
                        class="reaction-btn"
                        :class="{ active: activeSegmentReactionFor(item) === REACTION_TYPES.DOUBLE_LIKE }"
                        :disabled="isSegmentReactionLoading(item)"
                        :aria-pressed="activeSegmentReactionFor(item) === REACTION_TYPES.DOUBLE_LIKE"
                        @click="submitSegmentReaction(item, REACTION_TYPES.DOUBLE_LIKE)"
                      >
                        双赞 {{ segmentReactionCountsFor(item).double_like_count }}
                      </button>
                      <button
                        type="button"
                        class="reaction-btn dislike"
                        :class="{ active: activeSegmentReactionFor(item) === REACTION_TYPES.DISLIKE }"
                        :disabled="isSegmentReactionLoading(item)"
                        :aria-pressed="activeSegmentReactionFor(item) === REACTION_TYPES.DISLIKE"
                        @click="submitSegmentReaction(item, REACTION_TYPES.DISLIKE)"
                      >
                        倒赞
                      </button>
                    </div>
                    <p v-if="segmentReactionErrorFor(item)" class="feedback error reaction-error">{{ segmentReactionErrorFor(item) }}</p>
                  </article>
                  <div v-if="!questionRecommendItems.length && !questionRecommendLoading" class="empty-state small">暂无推荐结果</div>
                </div>
              </template>
            </div>
          </article>
        </div>
      </section>

      <section class="panel wide-panel">
        <div class="panel-head">
          <div>
            <p class="panel-tag">Free Query</p>
          <h2>自由问题检索</h2>
          </div>
        </div>

        <div class="stack">
          <label class="field-stack">
            <span class="field-label">问题文本</span>
            <textarea
              class="field textarea"
              v-model="recommendDraft"
              rows="3"
              placeholder="输入任意问题文本，测试 /api/recommendations/by-question"
            ></textarea>
          </label>
          <div class="button-row">
            <button class="primary-btn" :disabled="recommendLoading || !recommendDraft.trim()" @click="fetchRecommendByDraft">
              {{ recommendLoading ? '检索中…' : '开始检索' }}
            </button>
          </div>
        </div>
        <p v-if="recommendError" class="feedback error">{{ recommendError }}</p>

        <div class="recommend-card-list">
          <article v-for="item in recommendItems" :key="`draft-${item.video_segment_id}-${item.video_id}`" class="recommend-card">
            <div class="recommend-cover">
              <img v-if="item.cover_url" :src="item.cover_url" alt="cover" />
              <span v-else>NO COVER</span>
            </div>
            <div class="recommend-body">
              <div class="recommend-title">{{ item.title || '-' }}</div>
              <div class="recommend-meta">
                片段 {{ formatSegmentRange(item.start_time_sec, item.end_time_sec) }} · 分数 {{ Number(item.recommend_score || 0).toFixed(4) }}
              </div>
              <div class="button-row wrap">
                <button class="tiny-btn" :disabled="!item.play_url" @click="playRecommendationSegment(item)">片段播放</button>
                <button class="tiny-btn ghost" :disabled="!item.video_id" @click="playRecommendationFull(item)">完整播放</button>
              </div>
              <div class="reaction-strip compact-reactions segment-reactions">
                <button
                  type="button"
                  class="reaction-btn"
                  :class="{ active: activeSegmentReactionFor(item) === REACTION_TYPES.LIKE }"
                  :disabled="isSegmentReactionLoading(item)"
                  :aria-pressed="activeSegmentReactionFor(item) === REACTION_TYPES.LIKE"
                  @click="submitSegmentReaction(item, REACTION_TYPES.LIKE)"
                >
                  赞 {{ segmentReactionCountsFor(item).like_count }}
                </button>
                <button
                  type="button"
                  class="reaction-btn"
                  :class="{ active: activeSegmentReactionFor(item) === REACTION_TYPES.DOUBLE_LIKE }"
                  :disabled="isSegmentReactionLoading(item)"
                  :aria-pressed="activeSegmentReactionFor(item) === REACTION_TYPES.DOUBLE_LIKE"
                  @click="submitSegmentReaction(item, REACTION_TYPES.DOUBLE_LIKE)"
                >
                  双赞 {{ segmentReactionCountsFor(item).double_like_count }}
                </button>
                <button
                  type="button"
                  class="reaction-btn dislike"
                  :class="{ active: activeSegmentReactionFor(item) === REACTION_TYPES.DISLIKE }"
                  :disabled="isSegmentReactionLoading(item)"
                  :aria-pressed="activeSegmentReactionFor(item) === REACTION_TYPES.DISLIKE"
                  @click="submitSegmentReaction(item, REACTION_TYPES.DISLIKE)"
                >
                  倒赞
                </button>
              </div>
              <p v-if="segmentReactionErrorFor(item)" class="feedback error reaction-error">{{ segmentReactionErrorFor(item) }}</p>
            </div>
          </article>
          <div v-if="!recommendItems.length && !recommendLoading" class="empty-state">暂无检索结果</div>
        </div>
      </section>
    </div>
  </div>
</template>
