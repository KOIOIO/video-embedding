export const CONSOLE_ACTIVE_WORKSPACE_STORAGE_KEY = 'video_app-console.active-workspace'
export const LEGACY_CONSOLE_AUTH_STORAGE_KEY = 'video_app-recommendation-console.authenticated'
export const DEFAULT_WORKSPACE = 'video'

const KNOWN_WORKSPACES = new Set(['video', 'recommendation', 'knowledge-video'])

function resolveStorage(storage) {
  return storage === undefined ? globalThis.localStorage : storage
}

function safeGetItem(storage, key) {
  try {
    return resolveStorage(storage)?.getItem(key) ?? null
  } catch {
    return null
  }
}

function safeSetItem(storage, key, value) {
  try {
    const resolvedStorage = resolveStorage(storage)
    if (!resolvedStorage) return false
    resolvedStorage.setItem(key, value)
    return true
  } catch {
    return false
  }
}

function safeRemoveItem(storage, key) {
  try {
    const resolvedStorage = resolveStorage(storage)
    if (!resolvedStorage) return false
    resolvedStorage.removeItem(key)
    return true
  } catch {
    return false
  }
}

export function isKnownWorkspace(workspace) {
  return KNOWN_WORKSPACES.has(workspace)
}

export function readActiveWorkspace(storage) {
  const workspace = safeGetItem(storage, CONSOLE_ACTIVE_WORKSPACE_STORAGE_KEY)
  return isKnownWorkspace(workspace) ? workspace : DEFAULT_WORKSPACE
}

export function writeActiveWorkspace(workspace, storage) {
  if (!isKnownWorkspace(workspace)) return false
  return safeSetItem(storage, CONSOLE_ACTIVE_WORKSPACE_STORAGE_KEY, workspace)
}

export function clearLegacyAuthenticated(storage) {
  return safeRemoveItem(storage, LEGACY_CONSOLE_AUTH_STORAGE_KEY)
}
