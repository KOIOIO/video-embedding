import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  fetchUserProfile,
  getCurrentUserId,
  normalizeUserProfile,
  setCurrentUserId,
  updateMyProfile,
  uploadAvatar,
} from './api.js'

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

describe('user api', () => {
  afterEach(() => {
    localStorageMock.clear()
    vi.restoreAllMocks()
  })

  it('getCurrentUserId returns default when unset', () => {
    expect(getCurrentUserId()).toBe(1001)
  })

  it('setCurrentUserId persists and getCurrentUserId reads it', () => {
    setCurrentUserId(4242)
    expect(getCurrentUserId()).toBe(4242)
  })

  it('getCurrentUserId falls back to default for invalid values', () => {
    localStorageMock.setItem('demo_user_id', 'abc')
    expect(getCurrentUserId()).toBe(1001)
    localStorageMock.setItem('demo_user_id', '0')
    expect(getCurrentUserId()).toBe(1001)
  })

  it('normalizes user profile with defaults', () => {
    const profile = normalizeUserProfile({
      id: '1',
      user_id: '1001',
      nickname: '  测试用户  ',
      avatar_url: '/avatars/1.jpg',
      bio: 'hello',
      gender: '1',
      location: '郑州',
      follow_count: '10',
      fans_count: '20',
    })
    expect(profile).toEqual({
      id: 1,
      user_id: 1001,
      nickname: '  测试用户  ',
      avatar_url: '/avatars/1.jpg',
      bio: 'hello',
      gender: 1,
      location: '郑州',
      follow_count: 10,
      fans_count: 20,
    })
    expect(normalizeUserProfile({}).nickname).toBe('')
    expect(normalizeUserProfile({}).user_id).toBe(0)
  })

  it('fetchUserProfile calls GET and normalizes result', async () => {
    const fetchImpl = vi.fn(async (url) => {
      expect(url).toBe('/api/users/1001/profile')
      return okResponse({ user_id: 1001, nickname: '用户1001', fans_count: 5 })
    })
    const profile = await fetchUserProfile(1001, fetchImpl)
    expect(profile.user_id).toBe(1001)
    expect(profile.nickname).toBe('用户1001')
    expect(profile.fans_count).toBe(5)
    expect(fetchImpl).toHaveBeenCalledTimes(1)
  })

  it('fetchUserProfile uses current user id when omitted', async () => {
    setCurrentUserId(777)
    const fetchImpl = vi.fn(async (url) => {
      expect(url).toBe('/api/users/777/profile')
      return okResponse({ user_id: 777, nickname: '用户777' })
    })
    await fetchUserProfile(undefined, fetchImpl)
    expect(fetchImpl).toHaveBeenCalledTimes(1)
  })

  it('updateMyProfile sends PUT with X-User-ID header and JSON body', async () => {
    setCurrentUserId(2002)
    const fetchImpl = vi.fn(async (url, init) => {
      expect(url).toBe('/api/me/profile')
      expect(init.method).toBe('PUT')
      expect(init.headers.get('X-User-ID')).toBe('2002')
      expect(init.headers.get('Content-Type')).toBe('application/json')
      const body = JSON.parse(init.body)
      expect(body.nickname).toBe('新昵称')
      expect(body.gender).toBe(1)
      return okResponse({ updated: true })
    })
    const result = await updateMyProfile({ nickname: '新昵称', bio: '', location: '', gender: 1 }, fetchImpl)
    expect(result.updated).toBe(true)
  })

  it('updateMyProfile throws on server error', async () => {
    const fetchImpl = vi.fn(async () => errResponse(400, 'nickname is required'))
    await expect(updateMyProfile({ nickname: '' }, fetchImpl)).rejects.toThrow('nickname is required')
  })

  it('uploadAvatar sends POST with FormData and X-User-ID header', async () => {
    setCurrentUserId(3003)
    const file = new File(['fake'], 'avatar.png', { type: 'image/png' })
    const fetchImpl = vi.fn(async (url, init) => {
      expect(url).toBe('/api/me/avatar')
      expect(init.method).toBe('POST')
      expect(init.headers.get('X-User-ID')).toBe('3003')
      expect(init.body).toBeInstanceOf(FormData)
      expect(init.body.get('avatar')).toBe(file)
      return okResponse({ avatar_url: '/videos/avatars/3003/123.png' })
    })
    const result = await uploadAvatar(file, fetchImpl)
    expect(result.avatar_url).toBe('/videos/avatars/3003/123.png')
  })

  it('uploadAvatar throws when no file provided', async () => {
    await expect(uploadAvatar(null)).rejects.toThrow('avatar file is required')
  })
})
