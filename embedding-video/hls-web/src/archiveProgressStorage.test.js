import { describe, expect, it, vi } from 'vitest'
import {
  ARCHIVE_PROGRESS_STORAGE_KEY,
  clearSavedArchiveUpload,
  loadSavedArchiveUpload,
  saveArchiveUpload,
} from './archiveProgressStorage.js'

function makeStorage(initial = {}) {
  const values = new Map(Object.entries(initial))
  return {
    getItem: vi.fn((key) => values.get(key) || null),
    setItem: vi.fn((key, value) => values.set(key, value)),
    removeItem: vi.fn((key) => values.delete(key)),
  }
}

describe('archive progress storage', () => {
  it('saves and loads the latest archive batch summary', () => {
    const storage = makeStorage()
    const upload = {
      batch_id: 'batch-1',
      total: 237,
      uploaded: 236,
      failed: 1,
      skipped: 0,
      videos: [{ video_id: 9, task_id: '9', file_name: 'lesson.mp4' }],
    }

    saveArchiveUpload(upload, storage)

    expect(storage.setItem).toHaveBeenCalledWith(ARCHIVE_PROGRESS_STORAGE_KEY, expect.any(String))
    expect(loadSavedArchiveUpload(storage)).toEqual(expect.objectContaining({
      batch_id: 'batch-1',
      total: 237,
      uploaded: 236,
      failed: 1,
      skipped: 0,
      videos: [{ video_id: 9, task_id: '9', file_name: 'lesson.mp4' }],
    }))
  })

  it('ignores uploads without a batch id', () => {
    const storage = makeStorage()

    saveArchiveUpload({ total: 1 }, storage)

    expect(storage.setItem).not.toHaveBeenCalled()
  })

  it('clears invalid saved data', () => {
    const storage = makeStorage({ [ARCHIVE_PROGRESS_STORAGE_KEY]: '{bad json' })

    expect(loadSavedArchiveUpload(storage)).toBeNull()
    expect(storage.removeItem).toHaveBeenCalledWith(ARCHIVE_PROGRESS_STORAGE_KEY)
  })

  it('removes the saved archive batch', () => {
    const storage = makeStorage()

    clearSavedArchiveUpload(storage)

    expect(storage.removeItem).toHaveBeenCalledWith(ARCHIVE_PROGRESS_STORAGE_KEY)
  })
})
