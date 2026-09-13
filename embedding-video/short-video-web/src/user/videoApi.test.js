import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  publishVideo,
  getVideoStatus,
  fetchUserVideos,
  normalizeUserVideo,
  formatDuration,
  validateVideoFile,
  STATUS_UPLOADED,
  STATUS_PROCESSING,
  STATUS_PUBLISHED,
  STATUS_FAILED,
  MAX_VIDEO_SIZE_BYTES,
} from './videoApi.js'
import { setCurrentUserId } from './api.js'

const localStorageMock = (() => {
  let store = {}
  return {
    getItem: (key) => (key in store ? store[key] : null),
    setItem: (key, value) => { store[key] = String(value) },
    removeItem: (key) => { delete store[key] },
    clear: () => { store = {} },
  }
})()

beforeEach(() => {
  vi.stubGlobal('localStorage', localStorageMock)
  localStorageMock.clear()
  setCurrentUserId(1001)
})

function okResponse(data) {
  return { ok: true, status: 200, json: async () => ({ success: true, data }) }
}

function errResponse(status, message) {
  return {
    ok: false,
    status,
    json: async () => ({ success: false, error: { code: 'test', message } }),
  }
}

describe('videoApi status constants', () => {
  it('matches backend domain values', () => {
    expect(STATUS_UPLOADED).toBe(1)
    expect(STATUS_PROCESSING).toBe(2)
    expect(STATUS_PUBLISHED).toBe(3)
    expect(STATUS_FAILED).toBe(4)
  })
})

describe('normalizeUserVideo', () => {
  it('normalizes with defaults', () => {
    const v = normalizeUserVideo({
      id: '5',
      title: '  测试  ',
      description: 'desc',
      cover_url: '/cover.jpg',
      duration: '120',
      status: '3',
      view_count: '10',
      create_time: '2026-01-01T00:00:00Z',
    })
    expect(v).toEqual({
      id: 5,
      author_id: 0,
      author_avatar_url: '',
      title: '  测试  ',
      description: 'desc',
      cover_url: '/cover.jpg',
      duration: 120,
      status: 3,
      view_count: 10,
      play_url: '',
      create_time: '2026-01-01T00:00:00Z',
    })
  })

  it('handles empty input', () => {
    const v = normalizeUserVideo({})
    expect(v.id).toBe(0)
    expect(v.title).toBe('')
    expect(v.duration).toBe(0)
  })
})

describe('formatDuration', () => {
  it('formats seconds as mm:ss', () => {
    expect(formatDuration(0)).toBe('00:00')
    expect(formatDuration(5)).toBe('00:05')
    expect(formatDuration(65)).toBe('01:05')
    expect(formatDuration(3661)).toBe('61:01')
  })

  it('handles invalid input', () => {
    expect(formatDuration(null)).toBe('00:00')
    expect(formatDuration(undefined)).toBe('00:00')
    expect(formatDuration(-10)).toBe('00:00')
  })
})

describe('validateVideoFile', () => {
  it('accepts valid mp4 file', () => {
    const file = new File(['x'], 'test.mp4', { type: 'video/mp4' })
    expect(validateVideoFile(file).valid).toBe(true)
  })

  it('accepts mov, avi, webm', () => {
    for (const name of ['a.mov', 'b.avi', 'c.webm']) {
      const file = new File(['x'], name, { type: 'video' })
      expect(validateVideoFile(file).valid).toBe(true)
    }
  })

  it('rejects unsupported extension', () => {
    const file = new File(['x'], 'test.exe', { type: 'application/octet-stream' })
    const result = validateVideoFile(file)
    expect(result.valid).toBe(false)
    expect(result.reason).toContain('mp4')
  })

  it('rejects oversized file', () => {
    const file = new File([new Uint8Array(MAX_VIDEO_SIZE_BYTES + 1)], 'big.mp4', { type: 'video/mp4' })
    const result = validateVideoFile(file)
    expect(result.valid).toBe(false)
    expect(result.reason).toContain('500MB')
  })

  it('rejects null file', () => {
    expect(validateVideoFile(null).valid).toBe(false)
  })
})

describe('publishVideo', () => {
  afterEach(() => { vi.restoreAllMocks() })

  it('sends POST with FormData and X-User-ID header', async () => {
    const file = new File(['fake'], 'video.mp4', { type: 'video/mp4' })
    const fetchImpl = vi.fn(async (url, init) => {
      expect(url).toBe('/api/me/videos')
      expect(init.method).toBe('POST')
      expect(init.headers.get('X-User-ID')).toBe('1001')
      expect(init.body).toBeInstanceOf(FormData)
      expect(init.body.get('file')).toBe(file)
      expect(init.body.get('title')).toBe('我的视频')
      expect(init.body.get('description')).toBe('描述内容')
      return okResponse({ video_id: 42, status: 1 })
    })
    const result = await publishVideo({ file, title: '我的视频', description: '描述内容' }, fetchImpl)
    expect(result.video_id).toBe(42)
    expect(result.status).toBe(1)
  })

  it('omits description when not provided', async () => {
    const file = new File(['fake'], 'video.mp4', { type: 'video/mp4' })
    const fetchImpl = vi.fn(async (_url, init) => {
      expect(init.body.has('description')).toBe(false)
      return okResponse({ video_id: 1, status: 1 })
    })
    await publishVideo({ file, title: '标题' }, fetchImpl)
  })

  it('throws when no file provided', async () => {
    await expect(publishVideo({ file: null, title: 'x' })).rejects.toThrow('video file is required')
  })

  it('throws on server error', async () => {
    const file = new File(['fake'], 'video.mp4', { type: 'video/mp4' })
    const fetchImpl = vi.fn(async () => errResponse(400, 'title is required'))
    await expect(publishVideo({ file, title: '' }, fetchImpl)).rejects.toThrow('title is required')
  })
})

describe('getVideoStatus', () => {
  afterEach(() => { vi.restoreAllMocks() })

  it('sends GET with X-User-ID header', async () => {
    const fetchImpl = vi.fn(async (url, init) => {
      expect(url).toBe('/api/me/videos/42/status')
      expect(init.headers.get('X-User-ID')).toBe('1001')
      return okResponse({ id: 42, status: 3, error_msg: '' })
    })
    const result = await getVideoStatus(42, fetchImpl)
    expect(result.id).toBe(42)
    expect(result.status).toBe(3)
    expect(result.error_msg).toBe('')
  })

  it('returns error_msg on failed status', async () => {
    const fetchImpl = vi.fn(async () => okResponse({ id: 1, status: 4, error_msg: 'duration exceeded' }))
    const result = await getVideoStatus(1, fetchImpl)
    expect(result.status).toBe(4)
    expect(result.error_msg).toBe('duration exceeded')
  })

  it('throws for invalid video id', async () => {
    await expect(getVideoStatus(0)).rejects.toThrow('video_id is required')
  })
})

describe('fetchUserVideos', () => {
  afterEach(() => { vi.restoreAllMocks() })

  it('sends GET with pagination query params', async () => {
    const fetchImpl = vi.fn(async (url) => {
      expect(url).toContain('/api/users/1001/videos')
      expect(url).toContain('page=1')
      expect(url).toContain('page_size=12')
      return okResponse({
        list: [{ id: 1, title: 'v1', duration: 60 }],
        total: 1,
        page: 1,
        page_size: 12,
      })
    })
    const result = await fetchUserVideos(1001, { page: 1, pageSize: 12 }, fetchImpl)
    expect(result.list).toHaveLength(1)
    expect(result.list[0].id).toBe(1)
    expect(result.list[0].duration).toBe(60)
    expect(result.total).toBe(1)
  })

  it('uses default pagination when omitted', async () => {
    const fetchImpl = vi.fn(async (url) => {
      expect(url).toContain('page=1')
      expect(url).toContain('page_size=12')
      return okResponse({ list: [], total: 0, page: 1, page_size: 12 })
    })
    await fetchUserVideos(1001, {}, fetchImpl)
  })

  it('uses current user id when omitted', async () => {
    setCurrentUserId(555)
    const fetchImpl = vi.fn(async (url) => {
      expect(url).toContain('/api/users/555/videos')
      return okResponse({ list: [], total: 0, page: 1, page_size: 12 })
    })
    await fetchUserVideos(undefined, {}, fetchImpl)
  })

  it('normalizes list items', async () => {
    const fetchImpl = vi.fn(async () => okResponse({
      list: [{ id: '7', title: 't', duration: '90', status: '2' }],
      total: 1,
      page: 1,
      page_size: 12,
    }))
    const result = await fetchUserVideos(1, {}, fetchImpl)
    expect(result.list[0].id).toBe(7)
    expect(result.list[0].duration).toBe(90)
    expect(result.list[0].status).toBe(2)
  })
})
