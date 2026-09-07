import { readSession, writeSession } from '../auth/session.js'

const DEMO_USER_ID_KEY = 'demo_user_id'
const DEFAULT_USER_ID = 1001

export function getCurrentUserId() {
  // 优先从登录 session（JWT）取用户ID，确保登录后身份正确切换
  const session = readSession()
  const sessionId = Number(session?.admin?.id || 0)
  if (sessionId > 0) return sessionId

  // fallback：开发调试用的 demo_user_id（X-User-ID 模拟模式）
  try {
    const raw = localStorage.getItem(DEMO_USER_ID_KEY)
    const value = Number(raw)
    if (Number.isFinite(value) && value > 0) return value
  } catch {
    // localStorage unavailable
  }
  return DEFAULT_USER_ID
}

export function setCurrentUserId(userId) {
  try {
    localStorage.setItem(DEMO_USER_ID_KEY, String(Number(userId) || DEFAULT_USER_ID))
  } catch {
    // ignore
  }
}

export async function requestJson(url, init = {}, fetchImpl = fetch) {
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

export function authHeaders(extra = {}) {
  const headers = {
    'X-User-ID': String(getCurrentUserId()),
    ...extra,
  }
  const session = readSession()
  if (session?.accessToken) {
    headers['Authorization'] = `Bearer ${session.accessToken}`
  }
  return headers
}

export function normalizeUserProfile(data) {
  return {
    id: Number(data?.id || 0) || 0,
    user_id: Number(data?.user_id || 0) || 0,
    nickname: String(data?.nickname || ''),
    avatar_url: String(data?.avatar_url || ''),
    bio: String(data?.bio || ''),
    gender: Number(data?.gender || 0) || 0,
    location: String(data?.location || ''),
    follow_count: Number(data?.follow_count || 0) || 0,
    fans_count: Number(data?.fans_count || 0) || 0,
  }
}

export async function fetchUserProfile(userId, fetchImpl = fetch) {
  const id = Number(userId) || getCurrentUserId()
  const data = await requestJson(`/api/users/${encodeURIComponent(String(id))}/profile`, {}, fetchImpl)
  return normalizeUserProfile(data)
}

export async function fetchMe(fetchImpl = fetch) {
  const data = await requestJson('/api/me', {
    headers: authHeaders(),
  }, fetchImpl)
  return normalizeUserProfile(data)
}

/**
 * 刷新当前用户的 profile 到全局 session（localStorage + 内存）
 * 登录后、编辑资料后、上传头像后调用，确保首页顶部等全局显示同步更新
 */
export async function refreshMyProfile(fetchImpl = fetch) {
  const profile = await fetchMe(fetchImpl)
  const session = readSession()
  if (session) {
    session.profile = {
      nickname: profile.nickname,
      avatar_url: profile.avatar_url,
      bio: profile.bio,
      gender: profile.gender,
      location: profile.location,
    }
    writeSession(session)
  }
  return profile
}

export async function updateMyProfile({ nickname, bio, location, gender } = {}, fetchImpl = fetch) {
  const data = await requestJson('/api/me/profile', {
    method: 'PUT',
    headers: authHeaders(),
    body: JSON.stringify({
      nickname: String(nickname || ''),
      bio: String(bio || ''),
      location: String(location || ''),
      gender: Number(gender) || 0,
    }),
  }, fetchImpl)
  return data
}

export async function uploadAvatar(file, fetchImpl = fetch) {
  if (!file) throw new Error('avatar file is required')
  const formData = new FormData()
  formData.append('avatar', file)
  const data = await requestJson('/api/me/avatar', {
    method: 'POST',
    headers: authHeaders(),
    body: formData,
  }, fetchImpl)
  return {
    avatar_url: String(data?.avatar_url || ''),
  }
}

export async function followUser(userId, fetchImpl = fetch) {
  const id = Number(userId) || 0
  const data = await requestJson(`/api/users/${encodeURIComponent(String(id))}/follow`, {
    method: 'POST',
    headers: authHeaders(),
  }, fetchImpl)
  return String(data?.status || '')
}

export async function unfollowUser(userId, fetchImpl = fetch) {
  const id = Number(userId) || 0
  const data = await requestJson(`/api/users/${encodeURIComponent(String(id))}/follow`, {
    method: 'DELETE',
    headers: authHeaders(),
  }, fetchImpl)
  return String(data?.status || '')
}

export async function fetchFollowing(userId, page = 1, pageSize = 20, fetchImpl = fetch) {
  const id = Number(userId) || 0
  const params = new URLSearchParams({ page: String(page), page_size: String(pageSize) })
  const data = await requestJson(`/api/users/${encodeURIComponent(String(id))}/following?${params.toString()}`, {}, fetchImpl)
  return {
    list: Array.isArray(data?.list) ? data.list.map(normalizeFollowItem) : [],
    total: Number(data?.total || 0) || 0,
    page: Number(data?.page || page) || page,
    page_size: Number(data?.page_size || pageSize) || pageSize,
  }
}

export async function fetchFollowers(userId, page = 1, pageSize = 20, fetchImpl = fetch) {
  const id = Number(userId) || 0
  const params = new URLSearchParams({ page: String(page), page_size: String(pageSize) })
  const data = await requestJson(`/api/users/${encodeURIComponent(String(id))}/followers?${params.toString()}`, {}, fetchImpl)
  return {
    list: Array.isArray(data?.list) ? data.list.map(normalizeFollowItem) : [],
    total: Number(data?.total || 0) || 0,
    page: Number(data?.page || page) || page,
    page_size: Number(data?.page_size || pageSize) || pageSize,
  }
}

export async function fetchRelation(userId, fetchImpl = fetch) {
  const id = Number(userId) || 0
  const data = await requestJson(`/api/users/${encodeURIComponent(String(id))}/relation`, {
    headers: authHeaders(),
  }, fetchImpl)
  return String(data?.relation || 'none')
}

function normalizeFollowItem(item) {
  return {
    id: Number(item?.id || 0) || 0,
    follower_id: Number(item?.follower_id || 0) || 0,
    following_id: Number(item?.following_id || 0) || 0,
    create_time: item?.create_time || '',
    nickname: String(item?.nickname || ''),
    avatar_url: String(item?.avatar_url || ''),
    bio: String(item?.bio || ''),
  }
}

// --- 用户资料缓存 ---

const profileCache = new Map()

export function getCachedUserProfile(userId) {
  const id = Number(userId) || 0
  if (!id) return null
  return profileCache.get(id) || null
}

export async function prefetchUserProfiles(userIds, fetchImpl = fetch) {
  const ids = (Array.isArray(userIds) ? userIds : [userIds])
    .map((id) => Number(id) || 0)
    .filter((id) => id > 0 && !profileCache.has(id))
  const uniqueIds = [...new Set(ids)]
  await Promise.all(uniqueIds.map(async (id) => {
    try {
      const profile = await fetchUserProfile(id, fetchImpl)
      profileCache.set(id, profile)
    } catch {
      // 单个失败不影响其他
    }
  }))
}

// --- 私信 API ---

export async function sendMessage(userId, content, fetchImpl = fetch) {
  const id = Number(userId) || 0
  const data = await requestJson(`/api/messages/${encodeURIComponent(String(id))}`, {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify({ content: String(content || '') }),
  }, fetchImpl)
  return normalizeMessage(data?.message || data)
}

export async function fetchUnreadCount(fetchImpl = fetch) {
  const data = await requestJson('/api/messages/unread-count', {
    headers: authHeaders(),
  }, fetchImpl)
  return Number(data?.unread_count || 0) || 0
}

export async function fetchConversations(fetchImpl = fetch) {
  const data = await requestJson('/api/messages/conversations', {
    headers: authHeaders(),
  }, fetchImpl)
  const list = Array.isArray(data?.list) ? data.list : []
  return list.map(normalizeConversation)
}

export async function fetchMessages(userId, page = 1, pageSize = 20, fetchImpl = fetch) {
  const id = Number(userId) || 0
  const params = new URLSearchParams({ page: String(page), page_size: String(pageSize) })
  const data = await requestJson(`/api/messages/${encodeURIComponent(String(id))}?${params.toString()}`, {
    headers: authHeaders(),
  }, fetchImpl)
  return {
    list: Array.isArray(data?.list) ? data.list.map(normalizeMessage) : [],
    page: Number(data?.page || page) || page,
    page_size: Number(data?.page_size || pageSize) || pageSize,
  }
}

export async function markAsRead(userId, fetchImpl = fetch) {
  const id = Number(userId) || 0
  const data = await requestJson(`/api/messages/${encodeURIComponent(String(id))}/read`, {
    method: 'POST',
    headers: authHeaders(),
  }, fetchImpl)
  return Boolean(data?.success)
}

// --- 私信数据规范化 ---

function normalizeMessage(item) {
  return {
    id: Number(item?.id || 0) || 0,
    sender_id: Number(item?.sender_id || 0) || 0,
    receiver_id: Number(item?.receiver_id || 0) || 0,
    conversation_id: String(item?.conversation_id || ''),
    content: String(item?.content || ''),
    is_read: Boolean(item?.is_read),
    create_time: item?.create_time || '',
  }
}

function normalizeConversation(item) {
  return {
    conversation_id: String(item?.conversation_id || ''),
    other_user_id: Number(item?.other_user_id || 0) || 0,
    last_message: String(item?.last_message || ''),
    last_message_time: item?.last_message_time || '',
    unread_count: Number(item?.unread_count || 0) || 0,
  }
}

// --- 主页访问统计 API ---

export async function recordVisit(userId, fetchImpl = fetch) {
  const id = Number(userId) || 0
  if (!id) return
  try {
    await requestJson(`/api/users/${encodeURIComponent(String(id))}/visit`, {
      method: 'POST',
      headers: authHeaders(),
    }, fetchImpl)
  } catch {
    // fire and forget：静默失败，不影响页面
  }
}

export async function fetchMyVisits(date, fetchImpl = fetch) {
  const params = new URLSearchParams()
  if (date) params.set('date', String(date))
  const query = params.toString()
  const url = query ? `/api/me/visits?${query}` : '/api/me/visits'
  const data = await requestJson(url, {
    headers: authHeaders(),
  }, fetchImpl)
  return {
    date: String(data?.date || ''),
    unique_visitors: Number(data?.unique_visitors || 0) || 0,
    total_visits: Number(data?.total_visits || 0) || 0,
  }
}

// --- 用户搜索（评论 @提及） ---

export async function searchUsers(keyword, page = 1, pageSize = 20, fetchImpl = fetch) {
  const q = String(keyword || '').trim()
  if (!q) return { list: [], total: 0, page, page_size: pageSize }
  const params = new URLSearchParams({ q, page: String(page), page_size: String(pageSize) })
  const data = await requestJson(`/api/users/search?${params.toString()}`, {
    headers: authHeaders(),
  }, fetchImpl)
  return {
    list: Array.isArray(data?.list) ? data.list.map(normalizeSearchUser) : [],
    total: Number(data?.total || 0) || 0,
    page: Number(data?.page || page) || page,
    page_size: Number(data?.page_size || pageSize) || pageSize,
  }
}

function normalizeSearchUser(item) {
  return {
    id: Number(item?.id || 0) || 0,
    username: String(item?.username || ''),
    nickname: String(item?.nickname || ''),
    avatar_url: String(item?.avatar_url || ''),
    is_following: Boolean(item?.is_following),
  }
}

// --- 用户通知 API ---

export async function fetchNotifications({ page = 1, pageSize = 20 } = {}, fetchImpl = fetch) {
  const params = new URLSearchParams({ page: String(page), page_size: String(pageSize) })
  const data = await requestJson(`/api/notifications?${params.toString()}`, {
    headers: authHeaders(),
  }, fetchImpl)
  return {
    total: Number(data?.total || 0) || 0,
    page: Number(data?.page || page) || page,
    page_size: Number(data?.page_size || pageSize) || pageSize,
    notifications: Array.isArray(data?.notifications) ? data.notifications.map(normalizeNotification) : [],
  }
}

export async function fetchNotificationUnreadCount(fetchImpl = fetch) {
  const data = await requestJson('/api/notifications/unread-count', {
    headers: authHeaders(),
  }, fetchImpl)
  return Number(data?.unread_count || 0) || 0
}

export async function markNotificationRead(notificationId, fetchImpl = fetch) {
  const id = Number(notificationId) || 0
  if (!id) return false
  const data = await requestJson(`/api/notifications/${encodeURIComponent(String(id))}/read`, {
    method: 'POST',
    headers: authHeaders(),
  }, fetchImpl)
  return Boolean(data?.success)
}

export async function markAllNotificationsRead(fetchImpl = fetch) {
  const data = await requestJson('/api/notifications/read-all', {
    method: 'POST',
    headers: authHeaders(),
  }, fetchImpl)
  return Boolean(data?.success)
}

function normalizeNotification(item) {
  return {
    id: Number(item?.id || 0) || 0,
    type: String(item?.type || ''),
    from_user_id: Number(item?.from_user_id || 0) || 0,
    from_user_nickname: String(item?.from_user_nickname || ''),
    from_user_avatar: String(item?.from_user_avatar || ''),
    video_id: Number(item?.video_id || 0) || 0,
    video_segment_id: Number(item?.video_segment_id || 0) || 0,
    comment_id: Number(item?.comment_id || 0) || 0,
    content: String(item?.content || ''),
    is_read: Number(item?.is_read || 0) || 0,
    created_at_unix: Number(item?.created_at_unix || 0) || 0,
  }
}

// --- 时间格式化 ---

export function formatMessageTime(timestamp) {
  if (!timestamp) return ''
  const date = new Date(timestamp)
  if (Number.isNaN(date.getTime())) return ''
  const now = new Date()
  const pad = (n) => String(n).padStart(2, '0')
  const timeStr = `${pad(date.getHours())}:${pad(date.getMinutes())}`
  const isToday = date.toDateString() === now.toDateString()
  if (isToday) return timeStr
  const yesterday = new Date(now)
  yesterday.setDate(now.getDate() - 1)
  if (date.toDateString() === yesterday.toDateString()) return `昨天 ${timeStr}`
  return `${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${timeStr}`
}
