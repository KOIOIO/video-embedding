import { describe, expect, it } from 'vitest'
import {
  CONSOLE_ACTIVE_SECTION_STORAGE_KEY,
  CONSOLE_SESSION_STORAGE_KEY,
  isValidConsoleLogin,
  readActiveSection,
  readAuthenticated,
  writeActiveSection,
  writeAuthenticated,
} from './consoleSession.js'

const sections = [
  { key: 'diagnostics' },
  { key: 'effects' },
  { key: 'redis' },
]

function createStorage(initial = {}) {
  const data = new Map(Object.entries(initial))
  return {
    getItem(key) {
      return data.has(key) ? data.get(key) : null
    },
    setItem(key, value) {
      data.set(key, String(value))
    },
    removeItem(key) {
      data.delete(key)
    },
  }
}

describe('recommendation console session helpers', () => {
  it('accepts only the configured control panel account', () => {
    expect(isValidConsoleLogin('aaddmmiinn', 'admin123')).toBe(true)
    expect(isValidConsoleLogin('admin', 'admin123')).toBe(false)
    expect(isValidConsoleLogin('aaddmmiinn', 'admin23')).toBe(false)
  })

  it('restores a stored active section when it is still known', () => {
    const storage = createStorage({
      [CONSOLE_ACTIVE_SECTION_STORAGE_KEY]: 'effects',
    })

    expect(readActiveSection(storage, 'diagnostics', sections)).toBe('effects')
  })

  it('falls back when the stored active section is unknown', () => {
    const storage = createStorage({
      [CONSOLE_ACTIVE_SECTION_STORAGE_KEY]: 'removed-section',
    })

    expect(readActiveSection(storage, 'diagnostics', sections)).toBe('diagnostics')
  })

  it('persists only known active sections', () => {
    const storage = createStorage()

    expect(writeActiveSection(storage, 'redis', sections)).toBe(true)
    expect(storage.getItem(CONSOLE_ACTIVE_SECTION_STORAGE_KEY)).toBe('redis')

    expect(writeActiveSection(storage, 'missing', sections)).toBe(false)
    expect(storage.getItem(CONSOLE_ACTIVE_SECTION_STORAGE_KEY)).toBe('redis')
  })

  it('stores and clears authentication state', () => {
    const storage = createStorage()

    expect(readAuthenticated(storage)).toBe(false)
    writeAuthenticated(storage, true)
    expect(storage.getItem(CONSOLE_SESSION_STORAGE_KEY)).toBe('true')
    expect(readAuthenticated(storage)).toBe(true)

    writeAuthenticated(storage, false)
    expect(storage.getItem(CONSOLE_SESSION_STORAGE_KEY)).toBe(null)
    expect(readAuthenticated(storage)).toBe(false)
  })
})
