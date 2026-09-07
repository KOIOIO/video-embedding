import { clearSession, readSession, writeSession } from './session.js'

function responseError(payload, response) {
  const error = new Error(payload?.error?.message || payload?.message || `HTTP ${response.status}`)
  error.status = response.status
  return error
}

async function requestJson(url, init = {}, fetchImpl = fetch) {
  const session = readSession()
  const headers = new Headers(init.headers || {})
  if (init.body) headers.set('Content-Type', 'application/json')
  if (session?.accessToken) headers.set('Authorization', `Bearer ${session.accessToken}`)
  const response = await fetchImpl(url, { ...init, headers })
  const payload = await response.json().catch(() => null)
  if (!response.ok || payload?.success === false) throw responseError(payload, response)
  return payload?.data ?? payload
}

export async function loginAdmin(username, password, fetchImpl = fetch) {
  const data = await requestJson('/api/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  }, fetchImpl)
  const session = {
    accessToken: data?.access_token || '',
    admin: {
      id: Number(data?.admin?.id || 0),
      username: String(data?.admin?.username || username || ''),
      realName: String(data?.admin?.real_name || ''),
      userType: Number(data?.admin?.user_type || 0),
    },
  }
  if (!session.accessToken || !session.admin.id) throw new Error('登录响应缺少 access_token 或管理员信息')
  writeSession(session)
  return session
}

export async function registerUser(username, password, nickname, fetchImpl = fetch) {
  const data = await requestJson('/api/auth/register', {
    method: 'POST',
    body: JSON.stringify({ username, password, nickname }),
  }, fetchImpl)
  const session = {
    accessToken: data?.access_token || '',
    admin: {
      id: Number(data?.admin?.id || 0),
      username: String(data?.admin?.username || username || ''),
      realName: String(data?.admin?.real_name || nickname || username || ''),
      userType: Number(data?.admin?.user_type || 0),
    },
  }
  if (!session.accessToken || !session.admin.id) throw new Error('注册响应缺少 access_token 或用户信息')
  writeSession(session)
  return session
}

export async function loadCurrentAdmin(fetchImpl = fetch) {
  const data = await requestJson('/api/auth/me', {}, fetchImpl)
  return {
    id: Number(data?.admin?.id || data?.id || 0),
    username: String(data?.admin?.username || data?.username || ''),
    realName: String(data?.admin?.real_name || data?.real_name || ''),
    userType: Number(data?.admin?.user_type || data?.user_type || 0),
  }
}

export async function logout() {
  clearSession()
}
