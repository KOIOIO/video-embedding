import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  fetchConversations,
  fetchFollowers,
  fetchFollowing,
  fetchMessages,
  fetchMyVisits,
  fetchRelation,
  fetchUserProfile,
  followUser,
  formatMessageTime,
  getCachedUserProfile,
  getCurrentUserId,
  markAsRead,
  normalizeUserProfile,
  prefetchUserProfiles,
  recordVisit,
  searchUsers,
  sendMessage,
  setCurrentUserId,
  unfollowUser,
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

  it('followUser sends POST with X-User-ID header', async () => {
    setCurrentUserId(1001)
    const fetchImpl = vi.fn(async (url, init) => {
      expect(url).toBe('/api/users/2002/follow')
      expect(init.method).toBe('POST')
      expect(init.headers.get('X-User-ID')).toBe('1001')
      return okResponse({ status: 'following' })
    })
    const result = await followUser(2002, fetchImpl)
    expect(result).toBe('following')
    expect(fetchImpl).toHaveBeenCalledTimes(1)
  })

  it('unfollowUser sends DELETE with X-User-ID header', async () => {
    setCurrentUserId(1001)
    const fetchImpl = vi.fn(async (url, init) => {
      expect(url).toBe('/api/users/2002/follow')
      expect(init.method).toBe('DELETE')
      expect(init.headers.get('X-User-ID')).toBe('1001')
      return okResponse({ status: 'none' })
    })
    const result = await unfollowUser(2002, fetchImpl)
    expect(result).toBe('none')
  })

  it('fetchFollowing sends GET and normalizes list result', async () => {
    const fetchImpl = vi.fn(async (url) => {
      expect(url).toContain('/api/users/1001/following')
      expect(url).toContain('page=1')
      expect(url).toContain('page_size=20')
      return okResponse({
        list: [
          { id: '1', follower_id: '1001', following_id: '2002', nickname: '用户2002', avatar_url: '/a.jpg', bio: 'hi' },
        ],
        total: 1,
        page: 1,
        page_size: 20,
      })
    })
    const result = await fetchFollowing(1001, 1, 20, fetchImpl)
    expect(result.total).toBe(1)
    expect(result.list).toHaveLength(1)
    expect(result.list[0].following_id).toBe(2002)
    expect(result.list[0].nickname).toBe('用户2002')
  })

  it('fetchFollowers sends GET and normalizes list result', async () => {
    const fetchImpl = vi.fn(async (url) => {
      expect(url).toContain('/api/users/1001/followers')
      return okResponse({
        list: [
          { id: '1', follower_id: '2002', following_id: '1001', nickname: '用户2002', avatar_url: '', bio: '' },
        ],
        total: 1,
        page: 1,
        page_size: 20,
      })
    })
    const result = await fetchFollowers(1001, 1, 20, fetchImpl)
    expect(result.list[0].follower_id).toBe(2002)
    expect(result.total).toBe(1)
  })

  it('fetchRelation sends GET with X-User-ID and returns relation', async () => {
    setCurrentUserId(1001)
    const fetchImpl = vi.fn(async (url, init) => {
      expect(url).toBe('/api/users/2002/relation')
      expect(init.headers.get('X-User-ID')).toBe('1001')
      return okResponse({ relation: 'mutual' })
    })
    const result = await fetchRelation(2002, fetchImpl)
    expect(result).toBe('mutual')
  })

  it('fetchRelation defaults to none when response missing', async () => {
    const fetchImpl = vi.fn(async () => okResponse({}))
    const result = await fetchRelation(2002, fetchImpl)
    expect(result).toBe('none')
  })

  // --- 私信 API 测试 ---

  it('sendMessage sends POST with X-User-ID and content body', async () => {
    setCurrentUserId(1001)
    const fetchImpl = vi.fn(async (url, init) => {
      expect(url).toBe('/api/messages/2002')
      expect(init.method).toBe('POST')
      expect(init.headers.get('X-User-ID')).toBe('1001')
      expect(init.headers.get('Content-Type')).toBe('application/json')
      const body = JSON.parse(init.body)
      expect(body.content).toBe('hello')
      return okResponse({ message: { id: 1, sender_id: 1001, receiver_id: 2002, content: 'hello', is_read: false, create_time: '2026-01-01T10:00:00Z' } })
    })
    const result = await sendMessage(2002, 'hello', fetchImpl)
    expect(result.id).toBe(1)
    expect(result.sender_id).toBe(1001)
    expect(result.content).toBe('hello')
    expect(fetchImpl).toHaveBeenCalledTimes(1)
  })

  it('fetchConversations sends GET with X-User-ID and normalizes list', async () => {
    setCurrentUserId(1001)
    const fetchImpl = vi.fn(async (url, init) => {
      expect(url).toBe('/api/messages/conversations')
      expect(init.headers.get('X-User-ID')).toBe('1001')
      return okResponse({
        list: [
          { conversation_id: '1001_2002', other_user_id: 2002, last_message: 'hi', last_message_time: '2026-01-01T10:00:00Z', unread_count: 3 },
        ],
      })
    })
    const result = await fetchConversations(fetchImpl)
    expect(result).toHaveLength(1)
    expect(result[0].other_user_id).toBe(2002)
    expect(result[0].last_message).toBe('hi')
    expect(result[0].unread_count).toBe(3)
  })

  it('fetchMessages sends GET with pagination and normalizes list', async () => {
    setCurrentUserId(1001)
    const fetchImpl = vi.fn(async (url) => {
      expect(url).toContain('/api/messages/2002')
      expect(url).toContain('page=1')
      expect(url).toContain('page_size=20')
      return okResponse({
        list: [
          { id: 1, sender_id: 2002, receiver_id: 1001, content: 'hello', is_read: false, create_time: '2026-01-01T10:00:00Z' },
        ],
        page: 1,
        page_size: 20,
      })
    })
    const result = await fetchMessages(2002, 1, 20, fetchImpl)
    expect(result.list).toHaveLength(1)
    expect(result.list[0].content).toBe('hello')
    expect(result.page).toBe(1)
    expect(result.page_size).toBe(20)
  })

  it('markAsRead sends POST to read endpoint', async () => {
    setCurrentUserId(1001)
    const fetchImpl = vi.fn(async (url, init) => {
      expect(url).toBe('/api/messages/2002/read')
      expect(init.method).toBe('POST')
      expect(init.headers.get('X-User-ID')).toBe('1001')
      return okResponse({ success: true })
    })
    const result = await markAsRead(2002, fetchImpl)
    expect(result).toBe(true)
  })

  it('formatMessageTime formats today as HH:MM', () => {
    const now = new Date()
    const pad = (n) => String(n).padStart(2, '0')
    const todayStr = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}T14:30:00`
    const result = formatMessageTime(todayStr)
    expect(result).toBe('14:30')
  })

  it('formatMessageTime returns empty for invalid input', () => {
    expect(formatMessageTime('')).toBe('')
    expect(formatMessageTime(null)).toBe('')
    expect(formatMessageTime('invalid-date')).toBe('')
  })

  it('getCachedUserProfile returns null for uncached user', () => {
    expect(getCachedUserProfile(99999)).toBeNull()
    expect(getCachedUserProfile(0)).toBeNull()
  })

  it('prefetchUserProfiles fetches and caches profiles', async () => {
    const fetchImpl = vi.fn(async (url) => {
      const userId = url.match(/\/api\/users\/(\d+)\/profile/)[1]
      return okResponse({ user_id: Number(userId), nickname: `用户${userId}` })
    })
    await prefetchUserProfiles([5001, 5002], fetchImpl)
    expect(fetchImpl).toHaveBeenCalledTimes(2)
    const p1 = getCachedUserProfile(5001)
    expect(p1).not.toBeNull()
    expect(p1.nickname).toBe('用户5001')
    const p2 = getCachedUserProfile(5002)
    expect(p2).not.toBeNull()
    expect(p2.nickname).toBe('用户5002')
  })

  it('prefetchUserProfiles does not refetch already cached profiles', async () => {
    const fetchImpl = vi.fn(async (url) => {
      const userId = url.match(/\/api\/users\/(\d+)\/profile/)[1]
      return okResponse({ user_id: Number(userId), nickname: `用户${userId}` })
    })
    // 5001 已在上一个测试中缓存
    await prefetchUserProfiles([5001, 6001], fetchImpl)
    expect(fetchImpl).toHaveBeenCalledTimes(1)
    expect(getCachedUserProfile(6001)).not.toBeNull()
  })

  it('recordVisit sends POST to visit endpoint and does not throw on error', async () => {
    setCurrentUserId(1001)
    const fetchImpl = vi.fn(async (url, init) => {
      expect(url).toBe('/api/users/2002/visit')
      expect(init.method).toBe('POST')
      expect(init.headers.get('X-User-ID')).toBe('1001')
      return errResponse(500, 'server error')
    })
    // fire and forget：即使失败也不抛异常
    await expect(recordVisit(2002, fetchImpl)).resolves.toBeUndefined()
    expect(fetchImpl).toHaveBeenCalledTimes(1)
  })

  it('recordVisit does nothing for invalid userId', async () => {
    const fetchImpl = vi.fn()
    await recordVisit(0, fetchImpl)
    await recordVisit(null, fetchImpl)
    expect(fetchImpl).not.toHaveBeenCalled()
  })

  it('fetchMyVisits sends GET to /api/me/visits without date by default', async () => {
    setCurrentUserId(1001)
    const fetchImpl = vi.fn(async (url, init) => {
      expect(url).toBe('/api/me/visits')
      expect(init.method).toBeUndefined()
      expect(init.headers.get('X-User-ID')).toBe('1001')
      return okResponse({ date: '2026-09-07', unique_visitors: 5, total_visits: 12 })
    })
    const result = await fetchMyVisits(undefined, fetchImpl)
    expect(result).toEqual({ date: '2026-09-07', unique_visitors: 5, total_visits: 12 })
  })

  it('fetchMyVisits includes date query when provided', async () => {
    setCurrentUserId(1001)
    const fetchImpl = vi.fn(async (url) => {
      expect(url).toBe('/api/me/visits?date=2026-09-01')
      return okResponse({ date: '2026-09-01', unique_visitors: 3, total_visits: 7 })
    })
    const result = await fetchMyVisits('2026-09-01', fetchImpl)
    expect(result.unique_visitors).toBe(3)
    expect(result.total_visits).toBe(7)
  })

  // --- 用户搜索 API 测试 ---

  it('searchUsers sends GET with q, page, page_size and Authorization header', async () => {
    setCurrentUserId(1001)
    const fetchImpl = vi.fn(async (url, init) => {
      expect(url).toContain('/api/users/search')
      expect(url).toContain('q=alice')
      expect(url).toContain('page=1')
      expect(url).toContain('page_size=20')
      expect(init.headers.get('X-User-ID')).toBe('1001')
      return okResponse({
        list: [
          { id: 2, username: 'alice', nickname: '爱丽丝', avatar_url: '/a.png', is_following: true },
          { id: 3, username: 'alice2', nickname: '爱丽丝2', avatar_url: '', is_following: false },
        ],
        total: 2,
        page: 1,
        page_size: 20,
      })
    })
    const result = await searchUsers('alice', 1, 20, fetchImpl)
    expect(result.total).toBe(2)
    expect(result.list).toHaveLength(2)
    expect(result.list[0].id).toBe(2)
    expect(result.list[0].nickname).toBe('爱丽丝')
    expect(result.list[0].is_following).toBe(true)
    expect(result.list[1].avatar_url).toBe('')
    expect(result.list[1].is_following).toBe(false)
  })

  it('searchUsers returns empty for blank keyword without fetching', async () => {
    const fetchImpl = vi.fn()
    const result = await searchUsers('   ', 1, 20, fetchImpl)
    expect(result.list).toEqual([])
    expect(result.total).toBe(0)
    expect(fetchImpl).not.toHaveBeenCalled()
  })

  it('searchUsers trims keyword and uses defaults', async () => {
    setCurrentUserId(1001)
    const fetchImpl = vi.fn(async (url) => {
      expect(url).toContain('q=bob')
      expect(url).toContain('page=1')
      expect(url).toContain('page_size=20')
      return okResponse({ list: [], total: 0, page: 1, page_size: 20 })
    })
    await searchUsers('  bob  ', undefined, undefined, fetchImpl)
    expect(fetchImpl).toHaveBeenCalledTimes(1)
  })
})
