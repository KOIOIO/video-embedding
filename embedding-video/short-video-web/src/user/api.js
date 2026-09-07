const DEMO_USER_ID_KEY = 'demo_user_id'
const DEFAULT_USER_ID = 1001

export function getCurrentUserId() {
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
