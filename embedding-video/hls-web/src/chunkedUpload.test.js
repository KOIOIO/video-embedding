import { describe, expect, it, vi } from 'vitest'
import { buildUploadSessionKey, uploadVideoInChunks } from './chunkedUpload.js'

function makeFile({ name = 'lesson.mp4', size = 10, lastModified = 123, type = 'video/mp4' } = {}) {
  return {
    name,
    size,
    lastModified,
    type,
    slice(start, end) {
      return { start, end, size: end - start }
    },
  }
}

function makeStorage(initial = {}) {
  const values = new Map(Object.entries(initial))
  return {
    getItem: vi.fn((key) => values.get(key) || null),
    setItem: vi.fn((key, value) => values.set(key, value)),
    removeItem: vi.fn((key) => values.delete(key)),
  }
}

describe('chunked video upload', () => {
  it('starts a new upload, uploads every chunk, then clears the saved session', async () => {
    const file = makeFile()
    const storage = makeStorage()
    const requestJson = vi.fn()
      .mockResolvedValueOnce({
        upload_id: 'upload-1',
        file_name: 'lesson.mp4',
        file_size: 10,
        chunk_size: 5,
        total_chunks: 2,
        uploaded_chunks: [],
        completed: false,
      })
      .mockResolvedValueOnce({
        video_id: 9,
        task_id: '9',
        raw_url: '/videos/raw/lesson.mp4',
        hls_url: '/videos/hls/lesson/master.m3u8',
      })
    const uploadChunk = vi.fn().mockResolvedValue({})
    const onProgress = vi.fn()

    const result = await uploadVideoInChunks({
      apiBase: '/api',
      file,
      title: 'Lesson',
      description: 'Desc',
      chunkSize: 5,
      requestJson,
      uploadChunk,
      storage,
      onProgress,
    })

    expect(requestJson).toHaveBeenNthCalledWith(1, '/api/videos/uploads', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        file_name: 'lesson.mp4',
        content_type: 'video/mp4',
        title: 'Lesson',
        description: 'Desc',
        file_size: 10,
        chunk_size: 5,
        total_chunks: 2,
      }),
    })
    expect(uploadChunk).toHaveBeenCalledTimes(2)
    expect(uploadChunk).toHaveBeenNthCalledWith(1, expect.objectContaining({
      url: '/api/videos/uploads/upload-1/chunks/0',
      chunk: { start: 0, end: 5, size: 5 },
    }))
    expect(uploadChunk).toHaveBeenNthCalledWith(2, expect.objectContaining({
      url: '/api/videos/uploads/upload-1/chunks/1',
      chunk: { start: 5, end: 10, size: 5 },
    }))
    expect(requestJson).toHaveBeenNthCalledWith(2, '/api/videos/uploads/upload-1/complete', {
      method: 'POST',
    })
    expect(result).toEqual(expect.objectContaining({ video_id: 9, task_id: '9' }))
    expect(storage.removeItem).toHaveBeenCalledWith(buildUploadSessionKey(file))
    expect(onProgress).toHaveBeenCalledWith(100)
  })

  it('uses archive upload endpoints for zip files', async () => {
    const file = makeFile({ name: 'lessons.zip', size: 10, type: 'application/zip' })
    const storage = makeStorage()
    const requestJson = vi.fn()
      .mockResolvedValueOnce({
        upload_id: 'archive-1',
        file_name: 'lessons.zip',
        file_size: 10,
        chunk_size: 5,
        total_chunks: 2,
        uploaded_chunks: [],
        completed: false,
      })
      .mockResolvedValueOnce({
        total: 2,
        uploaded: 1,
        skipped: 1,
        videos: [{ video_id: 9, task_id: '9' }],
      })
    const uploadChunk = vi.fn().mockResolvedValue({})

    const result = await uploadVideoInChunks({
      apiBase: '/api',
      kind: 'archive',
      file,
      description: 'Batch',
      chunkSize: 5,
      requestJson,
      uploadChunk,
      storage,
      onProgress: vi.fn(),
    })

    expect(requestJson).toHaveBeenNthCalledWith(1, '/api/videos/archive/uploads', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        file_name: 'lessons.zip',
        content_type: 'application/zip',
        title: '',
        description: 'Batch',
        file_size: 10,
        chunk_size: 5,
        total_chunks: 2,
      }),
    })
    expect(uploadChunk).toHaveBeenCalledWith(expect.objectContaining({
      url: '/api/videos/uploads/archive-1/chunks/0',
    }))
    expect(requestJson).toHaveBeenNthCalledWith(2, '/api/videos/archive/uploads/archive-1/complete', {
      method: 'POST',
    })
    expect(result).toEqual(expect.objectContaining({ total: 2, uploaded: 1 }))
  })

  it('resumes a saved upload and skips chunks already reported by the server', async () => {
    const file = makeFile()
    const key = buildUploadSessionKey(file)
    const storage = makeStorage({ [key]: 'upload-1' })
    const requestJson = vi.fn()
      .mockResolvedValueOnce({
        upload_id: 'upload-1',
        file_name: 'lesson.mp4',
        file_size: 10,
        chunk_size: 5,
        total_chunks: 2,
        uploaded_chunks: [0],
        completed: false,
      })
      .mockResolvedValueOnce({ video_id: 9, task_id: '9' })
    const uploadChunk = vi.fn().mockResolvedValue({})

    await uploadVideoInChunks({
      apiBase: '/api',
      file,
      chunkSize: 5,
      requestJson,
      uploadChunk,
      storage,
      onProgress: vi.fn(),
    })

    expect(requestJson).toHaveBeenNthCalledWith(1, '/api/videos/uploads/upload-1')
    expect(uploadChunk).toHaveBeenCalledTimes(1)
    expect(uploadChunk).toHaveBeenCalledWith(expect.objectContaining({
      url: '/api/videos/uploads/upload-1/chunks/1',
    }))
    expect(requestJson).not.toHaveBeenCalledWith('/api/videos/uploads', expect.anything())
  })

  it('uses the server chunk size when resuming an existing upload', async () => {
    const file = makeFile({ size: 12 })
    const key = buildUploadSessionKey(file)
    const storage = makeStorage({ [key]: 'upload-1' })
    const requestJson = vi.fn()
      .mockResolvedValueOnce({
        upload_id: 'upload-1',
        file_name: 'lesson.mp4',
        file_size: 12,
        chunk_size: 4,
        total_chunks: 3,
        uploaded_chunks: [0],
        completed: false,
      })
      .mockResolvedValueOnce({ video_id: 9, task_id: '9' })
    const uploadChunk = vi.fn().mockResolvedValue({})

    await uploadVideoInChunks({
      apiBase: '/api',
      file,
      chunkSize: 6,
      requestJson,
      uploadChunk,
      storage,
      onProgress: vi.fn(),
    })

    expect(uploadChunk).toHaveBeenCalledTimes(2)
    expect(uploadChunk).toHaveBeenNthCalledWith(1, expect.objectContaining({
      url: '/api/videos/uploads/upload-1/chunks/1',
      chunk: { start: 4, end: 8, size: 4 },
    }))
    expect(uploadChunk).toHaveBeenNthCalledWith(2, expect.objectContaining({
      url: '/api/videos/uploads/upload-1/chunks/2',
      chunk: { start: 8, end: 12, size: 4 },
    }))
  })

  it('starts over when the saved upload id is no longer valid', async () => {
    const file = makeFile()
    const key = buildUploadSessionKey(file)
    const storage = makeStorage({ [key]: 'missing-upload' })
    const requestJson = vi.fn()
      .mockRejectedValueOnce(new Error('upload not found'))
      .mockResolvedValueOnce({
        upload_id: 'upload-2',
        file_name: 'lesson.mp4',
        file_size: 10,
        chunk_size: 5,
        total_chunks: 2,
        uploaded_chunks: [],
        completed: false,
      })
      .mockResolvedValueOnce({ video_id: 10, task_id: '10' })
    const uploadChunk = vi.fn().mockResolvedValue({})

    await uploadVideoInChunks({
      apiBase: '/api',
      file,
      chunkSize: 5,
      requestJson,
      uploadChunk,
      storage,
      onProgress: vi.fn(),
    })

    expect(storage.removeItem).toHaveBeenCalledWith(key)
    expect(storage.setItem).toHaveBeenCalledWith(key, 'upload-2')
    expect(uploadChunk).toHaveBeenCalledTimes(2)
  })
})
