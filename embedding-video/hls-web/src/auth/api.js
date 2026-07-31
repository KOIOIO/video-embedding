import { clearAuthSession, readAuthSession } from './session.js'

function responseError(payload, response) {
  const error = new Error(payload?.error?.message || payload?.message || `HTTP ${response.status}`)
  error.status = response.status
  error.issues = payload?.error?.issues || []
  return error
}

export async function loginAdmin(username, password, fetchImpl = fetch) {
  const response = await fetchImpl('/api/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })
  const payload = await response.json().catch(() => null)
  if (!response.ok || payload?.success === false) throw responseError(payload, response)
  return payload?.data ?? payload
}

export async function apiFetch(url, init = {}, options = {}) {
  const fetchImpl = options.fetchImpl || fetch
  const session = readAuthSession(options.storage)
  const headers = new Headers(init.headers || {})
  if (session?.accessToken) headers.set('Authorization', `Bearer ${session.accessToken}`)
  const response = await fetchImpl(url, { ...init, headers: Object.fromEntries(headers.entries()) })
  if (response.status === 401 || response.status === 403) {
    clearAuthSession(options.storage)
    if (options.dispatch) options.dispatch('admin-auth-expired')
    else globalThis.dispatchEvent?.(new CustomEvent('admin-auth-expired'))
  }
  return response
}

export async function loadCurrentAdmin(options = {}) {
  const response = await apiFetch('/api/auth/me', {}, options)
  const payload = await response.json().catch(() => null)
  if (!response.ok || payload?.success === false) throw responseError(payload, response)
  return payload?.data ?? payload
}
