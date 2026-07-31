export const AUTH_SESSION_KEY = 'video_app-console.admin-session'

function resolveStorage(storage) {
  if (storage !== undefined) return storage
  try { return globalThis.localStorage } catch { return null }
}

export function readAuthSession(storage) {
  try {
    const raw = resolveStorage(storage)?.getItem(AUTH_SESSION_KEY)
    if (!raw) return null
    const session = JSON.parse(raw)
    if (!session?.accessToken || !session?.admin?.id) return null
    return { accessToken: String(session.accessToken), admin: session.admin }
  } catch {
    return null
  }
}

export function writeAuthSession(session, storage) {
  if (!session?.accessToken || !session?.admin?.id) return false
  try {
    resolveStorage(storage)?.setItem(AUTH_SESSION_KEY, JSON.stringify({ accessToken: String(session.accessToken), admin: session.admin }))
    return true
  } catch {
    return false
  }
}

export function clearAuthSession(storage) {
  try {
    resolveStorage(storage)?.removeItem(AUTH_SESSION_KEY)
    return true
  } catch {
    return false
  }
}
