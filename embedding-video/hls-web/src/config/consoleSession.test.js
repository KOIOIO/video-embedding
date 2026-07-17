import { describe, expect, it, vi } from 'vitest'
import {
  CONSOLE_ACTIVE_WORKSPACE_STORAGE_KEY,
  CONSOLE_ADMIN_PASSWORD,
  CONSOLE_ADMIN_USERNAME,
  CONSOLE_AUTH_STORAGE_KEY,
  DEFAULT_WORKSPACE,
  LEGACY_CONSOLE_AUTH_STORAGE_KEY,
  clearLegacyAuthenticated,
  isKnownWorkspace,
  isValidConsoleLogin,
  readActiveWorkspace,
  readUIUnlocked,
  writeActiveWorkspace,
  writeUIUnlocked,
} from './consoleSession.js'

function makeStorage(initial = {}) {
  const values = new Map(Object.entries(initial))
  return {
    getItem: vi.fn((key) => values.get(key) ?? null),
    setItem: vi.fn((key, value) => values.set(key, value)),
    removeItem: vi.fn((key) => values.delete(key)),
  }
}

describe('console session', () => {
  it('accepts only the configured administrator credentials', () => {
    expect(CONSOLE_ADMIN_USERNAME).toBe('aaddmmiinn')
    expect(CONSOLE_ADMIN_PASSWORD).toBe('admin123')
    expect(isValidConsoleLogin('aaddmmiinn', 'admin123')).toBe(true)
    expect(isValidConsoleLogin('aaddmmiinn', 'wrong')).toBe(false)
    expect(isValidConsoleLogin('wrong', 'admin123')).toBe(false)
    expect(isValidConsoleLogin('AADDMMIINN', 'admin123')).toBe(false)
  })

  it('unlocks the UI only for the exact stored string true', () => {
    expect(CONSOLE_AUTH_STORAGE_KEY).toBe('hstv-console.authenticated')
    expect(readUIUnlocked(makeStorage({ [CONSOLE_AUTH_STORAGE_KEY]: 'true' }))).toBe(true)
    expect(readUIUnlocked(makeStorage({ [CONSOLE_AUTH_STORAGE_KEY]: 'TRUE' }))).toBe(false)
    expect(readUIUnlocked(makeStorage({ [CONSOLE_AUTH_STORAGE_KEY]: '1' }))).toBe(false)
    expect(readUIUnlocked(makeStorage())).toBe(false)
  })

  it('writes and clears the UI unlock marker', () => {
    const storage = makeStorage()

    expect(writeUIUnlocked(true, storage)).toBe(true)
    expect(storage.setItem).toHaveBeenCalledWith(CONSOLE_AUTH_STORAGE_KEY, 'true')
    expect(writeUIUnlocked(false, storage)).toBe(true)
    expect(storage.removeItem).toHaveBeenCalledWith(CONSOLE_AUTH_STORAGE_KEY)
  })

  it('recognizes only the video and recommendation workspaces', () => {
    expect(DEFAULT_WORKSPACE).toBe('video')
    expect(isKnownWorkspace('video')).toBe(true)
    expect(isKnownWorkspace('recommendation')).toBe(true)
    expect(isKnownWorkspace('unknown')).toBe(false)
    expect(isKnownWorkspace()).toBe(false)
  })

  it('reads a known workspace and falls back to video otherwise', () => {
    expect(CONSOLE_ACTIVE_WORKSPACE_STORAGE_KEY).toBe('hstv-console.active-workspace')
    expect(readActiveWorkspace(makeStorage({
      [CONSOLE_ACTIVE_WORKSPACE_STORAGE_KEY]: 'recommendation',
    }))).toBe('recommendation')
    expect(readActiveWorkspace(makeStorage({
      [CONSOLE_ACTIVE_WORKSPACE_STORAGE_KEY]: 'unknown',
    }))).toBe('video')
    expect(readActiveWorkspace(makeStorage())).toBe('video')
  })

  it('persists only known workspaces', () => {
    const storage = makeStorage()

    expect(writeActiveWorkspace('recommendation', storage)).toBe(true)
    expect(storage.setItem).toHaveBeenCalledWith(
      CONSOLE_ACTIVE_WORKSPACE_STORAGE_KEY,
      'recommendation',
    )
    storage.setItem.mockClear()

    expect(writeActiveWorkspace('unknown', storage)).toBe(false)
    expect(storage.setItem).not.toHaveBeenCalled()
  })

  it('clears the legacy recommendation console marker', () => {
    const storage = makeStorage()

    expect(LEGACY_CONSOLE_AUTH_STORAGE_KEY).toBe('hstv-recommendation-console.authenticated')
    expect(clearLegacyAuthenticated(storage)).toBe(true)
    expect(storage.removeItem).toHaveBeenCalledWith(LEGACY_CONSOLE_AUTH_STORAGE_KEY)
  })

  it('fails closed when storage operations throw', () => {
    const readFailure = {
      getItem: vi.fn(() => { throw new Error('blocked') }),
    }
    const writeFailure = {
      setItem: vi.fn(() => { throw new Error('blocked') }),
      removeItem: vi.fn(() => { throw new Error('blocked') }),
    }

    expect(() => readUIUnlocked(readFailure)).not.toThrow()
    expect(readUIUnlocked(readFailure)).toBe(false)
    expect(() => readActiveWorkspace(readFailure)).not.toThrow()
    expect(readActiveWorkspace(readFailure)).toBe(DEFAULT_WORKSPACE)
    expect(() => writeUIUnlocked(true, writeFailure)).not.toThrow()
    expect(writeUIUnlocked(true, writeFailure)).toBe(false)
    expect(() => writeUIUnlocked(false, writeFailure)).not.toThrow()
    expect(writeUIUnlocked(false, writeFailure)).toBe(false)
    expect(() => writeActiveWorkspace('video', writeFailure)).not.toThrow()
    expect(writeActiveWorkspace('video', writeFailure)).toBe(false)
    expect(() => clearLegacyAuthenticated(writeFailure)).not.toThrow()
    expect(clearLegacyAuthenticated(writeFailure)).toBe(false)
  })

  it('fails closed when the global localStorage property is inaccessible', () => {
    const originalDescriptor = Object.getOwnPropertyDescriptor(globalThis, 'localStorage')
    let result

    try {
      Object.defineProperty(globalThis, 'localStorage', {
        configurable: true,
        get() {
          throw new DOMException('blocked', 'SecurityError')
        },
      })

      expect(() => { result = readUIUnlocked() }).not.toThrow()
      expect(result).toBe(false)
      expect(() => { result = readActiveWorkspace() }).not.toThrow()
      expect(result).toBe(DEFAULT_WORKSPACE)
      expect(() => { result = writeUIUnlocked(true) }).not.toThrow()
      expect(result).toBe(false)
      expect(() => { result = writeActiveWorkspace('video') }).not.toThrow()
      expect(result).toBe(false)
      expect(() => { result = clearLegacyAuthenticated() }).not.toThrow()
      expect(result).toBe(false)
    } finally {
      if (originalDescriptor) {
        Object.defineProperty(globalThis, 'localStorage', originalDescriptor)
      } else {
        Reflect.deleteProperty(globalThis, 'localStorage')
      }
    }
  })
})
