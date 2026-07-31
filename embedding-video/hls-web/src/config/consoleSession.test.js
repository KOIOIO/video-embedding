import { describe, expect, it, vi } from 'vitest'
import {
  CONSOLE_ACTIVE_WORKSPACE_STORAGE_KEY,
  DEFAULT_WORKSPACE,
  LEGACY_CONSOLE_AUTH_STORAGE_KEY,
  clearLegacyAuthenticated,
  isKnownWorkspace,
  readActiveWorkspace,
  writeActiveWorkspace,
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
  it('recognizes the video, recommendation, and knowledge video workspaces', () => {
    expect(DEFAULT_WORKSPACE).toBe('video')
    expect(isKnownWorkspace('video')).toBe(true)
    expect(isKnownWorkspace('recommendation')).toBe(true)
    expect(isKnownWorkspace('knowledge-video')).toBe(true)
    expect(isKnownWorkspace('unknown')).toBe(false)
    expect(isKnownWorkspace()).toBe(false)
  })

  it('reads a known workspace and falls back to video otherwise', () => {
    expect(CONSOLE_ACTIVE_WORKSPACE_STORAGE_KEY).toBe('video_app-console.active-workspace')
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

    expect(LEGACY_CONSOLE_AUTH_STORAGE_KEY).toBe('video_app-recommendation-console.authenticated')
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

    expect(() => readActiveWorkspace(readFailure)).not.toThrow()
    expect(readActiveWorkspace(readFailure)).toBe(DEFAULT_WORKSPACE)
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

      expect(() => { result = readActiveWorkspace() }).not.toThrow()
      expect(result).toBe(DEFAULT_WORKSPACE)
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
