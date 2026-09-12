import { getCurrentUserId } from './api.js'

// 视频处理状态（与后端 domain/video 常量对应）
export const STATUS_UPLOADED = 1
export const STATUS_PROCESSING = 2
export const STATUS_PUBLISHED = 3
export const STATUS_FAILED = 4

export const MAX_VIDEO_SIZE_BYTES = 500 * 1024 * 1024 // 500MB
export const MAX_DURATION_SECONDS = Infinity // 上传时长已解禁，不限时长

const ALLOWED_EXTENSIONS = ['mp4', 'mov', 'avi', 'webm']

async function requestJson(url, init = {}, fetchImpl = fetch) {
  const headers = new Headers(init.headers || {})
  if (init.body && !(init.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json')
  }
  const response = await fetchImpl(url, { ...init, headers })
  const payload = await response.json().catch(() => null)
  if (!response.ok || payload?.success === false) {
    const error = new Error(payload?.error?.message || payload?.message || `HTTP ${response.status}`)
    error.status = response.status
    throw error
  }
  return payload?.data ?? payload
}

function authHeaders(extra = {}) {
  return {
    'X-User-ID': String(getCurrentUserId()),
    ...extra,
  }
}

export function normalizeUserVideo(data) {
  return {
    id: Number(data?.id || 0) || 0,
    author_id: Number(data?.author_id || data?.user_id || 0) || 0,
    title: String(data?.title || ''),
    description: String(data?.description || ''),
    cover_url: String(data?.cover_url || ''),
    duration: Number(data?.duration || 0) || 0,
    status: Number(data?.status || 0) || 0,
    view_count: Number(data?.view_count || 0) || 0,
    play_url: String(data?.play_url || ''),
    create_time: String(data?.create_time || ''),
  }
}

export function formatDuration(seconds) {
  const s = Math.max(0, Number(seconds) || 0)
  const m = Math.floor(s / 60)
  const r = s % 60
  return `${String(m).padStart(2, '0')}:${String(r).padStart(2, '0')}`
}

export function validateVideoFile(file) {
  if (!file) return { valid: false, reason: '请选择视频文件' }
  if (file.size > MAX_VIDEO_SIZE_BYTES) {
    return { valid: false, reason: '视频大小不能超过500MB' }
  }
  const ext = String(file.name || '').split('.').pop()?.toLowerCase() || ''
  if (!ALLOWED_EXTENSIONS.includes(ext)) {
    return { valid: false, reason: '仅支持 mp4、mov、avi、webm 格式' }
  }
  return { valid: true }
}

export async function publishVideo({ file, title, description }, fetchImpl = fetch) {
  if (!file) throw new Error('video file is required')
  const formData = new FormData()
  formData.append('file', file)
  formData.append('title', String(title || ''))
  if (description !== undefined && description !== null) {
    formData.append('description', String(description))
  }
  const data = await requestJson('/api/me/videos', {
    method: 'POST',
    headers: authHeaders(),
    body: formData,
  }, fetchImpl)
  return {
    video_id: Number(data?.video_id || 0) || 0,
    status: Number(data?.status || 0) || 0,
  }
}

export async function getVideoStatus(videoId, fetchImpl = fetch) {
  const id = Number(videoId) || 0
  if (!id) throw new Error('video_id is required')
  const data = await requestJson(`/api/me/videos/${encodeURIComponent(String(id))}/status`, {
    headers: authHeaders(),
  }, fetchImpl)
  return {
    id: Number(data?.id || 0) || 0,
    status: Number(data?.status || 0) || 0,
    error_msg: String(data?.error_msg || ''),
  }
}

export async function fetchUserVideos(userId, { page = 1, pageSize = 12 } = {}, fetchImpl = fetch) {
  const id = Number(userId) || getCurrentUserId()
  const params = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  })
  const data = await requestJson(`/api/users/${encodeURIComponent(String(id))}/videos?${params.toString()}`, {}, fetchImpl)
  const list = Array.isArray(data?.list) ? data.list.map(normalizeUserVideo) : []
  return {
    list,
    total: Number(data?.total || 0) || 0,
    page: Number(data?.page || page) || page,
    page_size: Number(data?.page_size || pageSize) || pageSize,
  }
}

export async function fetchLikedVideos(userId, { page = 1, pageSize = 12 } = {}, fetchImpl = fetch) {
  const id = Number(userId) || getCurrentUserId()
  const params = new URLSearchParams({
    page: String(page),
    page_size: String(pageSize),
  })
  const data = await requestJson(`/api/users/${encodeURIComponent(String(id))}/liked-videos?${params.toString()}`, {}, fetchImpl)
  const list = Array.isArray(data?.list) ? data.list.map(normalizeUserVideo) : []
  return {
    list,
    total: Number(data?.total || 0) || 0,
    page: Number(data?.page || page) || page,
    page_size: Number(data?.page_size || pageSize) || pageSize,
  }
}
