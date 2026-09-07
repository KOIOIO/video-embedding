const SESSION_KEY = 'short-video.session'

export function readSession(storage = localStorage) {
  try {
    const raw = storage.getItem(SESSION_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw)
    if (!parsed?.accessToken || !parsed?.admin?.id) return null
    return parsed
  } catch {
    return null
  }
}

export function writeSession(session, storage = localStorage) {
  storage.setItem(SESSION_KEY, JSON.stringify(session))
}

export function clearSession(storage = localStorage) {
  storage.removeItem(SESSION_KEY)
}
