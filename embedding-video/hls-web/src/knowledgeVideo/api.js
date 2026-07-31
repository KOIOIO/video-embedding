const API_BASE = '/api'
export const TERMINAL_BATCH_STATUSES = new Set(['completed', 'partial_failed', 'failed'])

async function requestJson(url, { fetchImpl, ...init } = {}) {
  const response = fetchImpl ? await fetchImpl(url, init) : await apiFetch(url, init)
  const payload = await response.json().catch(() => null)
  if (!response.ok || payload?.success === false) {
    const error = new Error(payload?.error?.message || payload?.message || `HTTP ${response.status}`)
    error.status = response.status
    error.issues = payload?.error?.issues || []
    throw error
  }
  return payload?.data ?? payload
}

function normalizeNode(node) {
  const video = node?.video || null
  const videos = Array.isArray(node?.videos) ? node.videos : (video ? [video] : [])
  const statuses = videos.map((item) => item?.status)
  const status = statuses.includes('ready')
    ? 'ready'
    : statuses.some((value) => value === 'pending' || value === 'transcoding')
      ? 'processing'
      : statuses.includes('failed') ? 'failed' : 'unassigned'
  return {
    id: Number(node?.id || 0),
    parentId: Number(node?.parent_id || 0),
    name: String(node?.name || ''),
    children: Array.isArray(node?.children) ? node.children.map(normalizeNode) : [],
    videos,
    video,
    status,
  }
}

export async function fetchKnowledgeTree(options = {}) {
  const data = await requestJson(`${API_BASE}/knowledge-videos/tree`, options)
  return Array.isArray(data?.nodes) ? data.nodes.map(normalizeNode) : []
}

export function uploadKnowledgeVideoBatch({ archive, mapping, accessToken = readAuthSession()?.accessToken, onProgress, xhrFactory = () => new XMLHttpRequest() }) {
  return new Promise((resolve, reject) => {
    const form = new FormData()
    form.append('archive', archive, archive.name || 'videos.zip')
    form.append('mapping', mapping, mapping.name || 'mapping.xlsx')
    const xhr = xhrFactory()
    xhr.open('POST', `${API_BASE}/admin/knowledge-videos/batches`)
    if (accessToken) xhr.setRequestHeader('Authorization', `Bearer ${accessToken}`)
    xhr.upload.addEventListener('progress', (event) => {
      if (!event.lengthComputable) return
      onProgress?.({ loaded: event.loaded, total: event.total, percent: Math.round((event.loaded / event.total) * 100) })
    })
    xhr.addEventListener('load', () => {
      let payload = null
      try { payload = JSON.parse(xhr.responseText || 'null') } catch { /* handled below */ }
      if (xhr.status < 200 || xhr.status >= 300 || payload?.success === false) {
        const error = new Error(payload?.error?.message || `HTTP ${xhr.status}`)
        error.status = xhr.status
        error.issues = payload?.error?.issues || []
        reject(error)
        return
      }
      resolve(payload?.data ?? payload)
    })
    xhr.addEventListener('error', () => reject(new Error('上传连接中断')))
    xhr.addEventListener('abort', () => reject(new DOMException('上传已取消', 'AbortError')))
    xhr.send(form)
  })
}

export function fetchKnowledgeVideoBatch(batchId, options = {}) {
  return requestJson(`${API_BASE}/admin/knowledge-videos/batches/${encodeURIComponent(String(batchId))}`, options)
}

export async function pollKnowledgeVideoBatch(batchId, { request = fetchKnowledgeVideoBatch, delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms)), interval = 2000, cancelled = () => false } = {}) {
  while (!cancelled()) {
    const batch = await request(batchId)
    if (TERMINAL_BATCH_STATUSES.has(batch?.status)) return batch
    await delay(interval)
  }
  return null
}

export function resolveKnowledgePlayback(knowledgePointId, options = {}) {
  return requestJson(`${API_BASE}/knowledge-points/${encodeURIComponent(String(knowledgePointId))}/videos`, options)
}

export function recordKnowledgePlayback(knowledgeVideoId, userId, options = {}) {
  return requestJson(`${API_BASE}/knowledge-videos/${encodeURIComponent(String(knowledgeVideoId))}/playbacks`, {
    ...options,
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
    body: JSON.stringify({ user_id: userId }),
  })
}
import { apiFetch } from '../auth/api.js'
import { readAuthSession } from '../auth/session.js'
