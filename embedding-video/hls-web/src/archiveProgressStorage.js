export const ARCHIVE_PROGRESS_STORAGE_KEY = 'video-archive-progress'

export function saveArchiveUpload(upload, storage = globalThis.localStorage) {
  const batchId = String(upload?.batch_id || '').trim()
  if (!batchId || !storage) return
  const summary = {
    batch_id: batchId,
    total: Number(upload?.total || 0) || 0,
    uploaded: Number(upload?.uploaded || 0) || 0,
    failed: Number(upload?.failed || 0) || 0,
    skipped: Number(upload?.skipped || 0) || 0,
    skipped_files: Array.isArray(upload?.skipped_files) ? upload.skipped_files : [],
    errors: Array.isArray(upload?.errors) ? upload.errors : [],
    videos: Array.isArray(upload?.videos) ? upload.videos : [],
    saved_at: Date.now(),
  }
  storage.setItem(ARCHIVE_PROGRESS_STORAGE_KEY, JSON.stringify(summary))
}

export function loadSavedArchiveUpload(storage = globalThis.localStorage) {
  if (!storage) return null
  const raw = storage.getItem(ARCHIVE_PROGRESS_STORAGE_KEY)
  if (!raw) return null
  try {
    const data = JSON.parse(raw)
    const batchId = String(data?.batch_id || '').trim()
    if (!batchId) {
      storage.removeItem(ARCHIVE_PROGRESS_STORAGE_KEY)
      return null
    }
    return {
      batch_id: batchId,
      total: Number(data?.total || 0) || 0,
      uploaded: Number(data?.uploaded || 0) || 0,
      failed: Number(data?.failed || 0) || 0,
      skipped: Number(data?.skipped || 0) || 0,
      skipped_files: Array.isArray(data?.skipped_files) ? data.skipped_files : [],
      errors: Array.isArray(data?.errors) ? data.errors : [],
      videos: Array.isArray(data?.videos) ? data.videos : [],
      saved_at: Number(data?.saved_at || 0) || 0,
    }
  } catch {
    storage.removeItem(ARCHIVE_PROGRESS_STORAGE_KEY)
    return null
  }
}

export function clearSavedArchiveUpload(storage = globalThis.localStorage) {
  storage?.removeItem(ARCHIVE_PROGRESS_STORAGE_KEY)
}
