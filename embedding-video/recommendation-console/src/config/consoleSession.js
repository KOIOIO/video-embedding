export const CONSOLE_ADMIN_USERNAME = 'aaddmmiinn'
export const CONSOLE_ADMIN_PASSWORD = 'admin123'
export const CONSOLE_SESSION_STORAGE_KEY = 'hstv-recommendation-console.authenticated'
export const CONSOLE_ACTIVE_SECTION_STORAGE_KEY = 'hstv-recommendation-console.active-section'

export function isValidConsoleLogin(username, password) {
  return String(username || '').trim() === CONSOLE_ADMIN_USERNAME && String(password || '') === CONSOLE_ADMIN_PASSWORD
}

export function readAuthenticated(storage) {
  return safeGetItem(storage, CONSOLE_SESSION_STORAGE_KEY) === 'true'
}

export function writeAuthenticated(storage, authenticated) {
  if (authenticated) {
    return safeSetItem(storage, CONSOLE_SESSION_STORAGE_KEY, 'true')
  }
  return safeRemoveItem(storage, CONSOLE_SESSION_STORAGE_KEY)
}

export function readActiveSection(storage, fallbackSection, navigationItems) {
  const storedSection = safeGetItem(storage, CONSOLE_ACTIVE_SECTION_STORAGE_KEY)
  return isKnownSection(storedSection, navigationItems) ? storedSection : fallbackSection
}

export function writeActiveSection(storage, section, navigationItems) {
  if (!isKnownSection(section, navigationItems)) {
    return false
  }
  return safeSetItem(storage, CONSOLE_ACTIVE_SECTION_STORAGE_KEY, section)
}

export function isKnownSection(section, navigationItems) {
  return Boolean(section && navigationItems.some((item) => item.key === section))
}

function safeGetItem(storage, key) {
  try {
    return storage?.getItem(key) || null
  } catch {
    return null
  }
}

function safeSetItem(storage, key, value) {
  try {
    storage?.setItem(key, value)
    return true
  } catch {
    return false
  }
}

function safeRemoveItem(storage, key) {
  try {
    storage?.removeItem(key)
    return true
  } catch {
    return false
  }
}
