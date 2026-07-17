const DEFAULT_CHUNK_SIZE = 8 * 1024 * 1024
const STORAGE_KEY_PREFIX = 'hengshui-video-upload'

export function buildUploadSessionKey(file) {
  return [
    STORAGE_KEY_PREFIX,
    encodeURIComponent(String(file?.name || '')),
    Number(file?.size || 0),
    Number(file?.lastModified || 0),
  ].join(':')
}

export async function uploadVideoInChunks({
  apiBase,
  kind = 'video',
  file,
  userId = 1,
  title = '',
  description = '',
  chunkSize = DEFAULT_CHUNK_SIZE,
  requestJson,
  uploadChunk = uploadChunkWithXHR,
  storage = globalThis.localStorage,
  onProgress = () => {},
}) {
  if (!file) throw new Error('file is required')
  if (typeof requestJson !== 'function') throw new Error('requestJson is required')

  let normalizedChunkSize = Math.max(1, Number(chunkSize || DEFAULT_CHUNK_SIZE))
  let totalChunks = Math.max(1, Math.ceil(Number(file.size || 0) / normalizedChunkSize))
  const normalizedUserId = Math.max(1, Math.floor(Number(userId || 1)))
  const key = buildUploadSessionKey(file)
  const isArchive = kind === 'archive'
  let uploadID = storage?.getItem(key) || ''
  let status = null

  if (uploadID) {
    try {
      status = await requestJson(`${apiBase}/videos/uploads/${encodeURIComponent(uploadID)}`)
    } catch {
      storage?.removeItem(key)
      uploadID = ''
    }
  }

  if (!uploadID) {
    status = await requestJson(`${apiBase}${isArchive ? '/videos/archive/uploads' : '/videos/uploads'}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        file_name: file.name,
        content_type: file.type || '',
        title: String(title || '').trim(),
        description: String(description || ''),
        user_id: normalizedUserId,
        file_size: Number(file.size || 0),
        chunk_size: normalizedChunkSize,
        total_chunks: totalChunks,
      }),
    })
    uploadID = String(status?.upload_id || '')
    if (!uploadID) throw new Error('upload_id is required')
    storage?.setItem(key, uploadID)
  } else {
    const serverChunkSize = Number(status?.chunk_size || 0)
    const serverTotalChunks = Number(status?.total_chunks || 0)
    if (serverChunkSize > 0) normalizedChunkSize = serverChunkSize
    if (serverTotalChunks > 0) totalChunks = serverTotalChunks
  }

  const uploaded = new Set((status?.uploaded_chunks || []).map((item) => Number(item)).filter((item) => item >= 0))
  reportChunkProgress(uploaded.size, totalChunks, 0, onProgress)

  for (let index = 0; index < totalChunks; index += 1) {
    if (uploaded.has(index)) continue
    const start = index * normalizedChunkSize
    const end = Math.min(Number(file.size || 0), start + normalizedChunkSize)
    const chunk = file.slice(start, end)
    await uploadChunk({
      url: `${apiBase}/videos/uploads/${encodeURIComponent(uploadID)}/chunks/${index}`,
      chunk,
      onProgress: (ratio) => {
        reportChunkProgress(uploaded.size, totalChunks, ratio, onProgress)
      },
    })
    uploaded.add(index)
    reportChunkProgress(uploaded.size, totalChunks, 0, onProgress)
  }

  const completePath = isArchive
    ? `/videos/archive/uploads/${encodeURIComponent(uploadID)}/complete`
    : `/videos/uploads/${encodeURIComponent(uploadID)}/complete`
  const result = await requestJson(`${apiBase}${completePath}`, {
    method: 'POST',
  })
  storage?.removeItem(key)
  onProgress(100)
  return result
}

export function uploadChunkWithXHR({ url, chunk, onProgress = () => {} }) {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('PUT', url)
    xhr.upload.onprogress = (event) => {
      if (!event.lengthComputable || !event.total) return
      onProgress(Math.max(0, Math.min(1, event.loaded / event.total)))
    }
    xhr.onload = () => {
      let payload = null
      try {
        payload = xhr.responseText ? JSON.parse(xhr.responseText) : null
      } catch {
        reject(new Error('上传失败：响应解析失败'))
        return
      }
      if (xhr.status < 200 || xhr.status >= 300 || payload?.success === false) {
        reject(new Error(payload?.error?.message || payload?.message || `HTTP ${xhr.status}`))
        return
      }
      resolve(payload?.data ?? payload)
    }
    xhr.onerror = () => reject(new Error('上传失败：网络错误'))
    xhr.send(chunk)
  })
}

function reportChunkProgress(uploadedChunks, totalChunks, currentChunkRatio, onProgress) {
  const total = Math.max(1, Number(totalChunks || 0))
  const uploaded = Math.max(0, Number(uploadedChunks || 0))
  const current = Math.max(0, Math.min(1, Number(currentChunkRatio || 0)))
  const percent = Math.floor(((uploaded + current) / total) * 100)
  onProgress(Math.max(0, Math.min(99, percent)))
}
