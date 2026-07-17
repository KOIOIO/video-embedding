import { describe, expect, it } from 'vitest'
import {
  RECOMMENDATION_SECTION_STORAGE_KEY,
  isKnownSection,
  readActiveSection,
  writeActiveSection,
} from './sectionSession.js'

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
  }
}

describe('recommendation section session helpers', () => {
  it('uses the recommendation console active-section storage key', () => {
    expect(RECOMMENDATION_SECTION_STORAGE_KEY).toBe('hstv-recommendation-console.active-section')
  })

  it('recognizes only configured navigation sections', () => {
    expect(isKnownSection('effects', sections)).toBe(true)
    expect(isKnownSection('removed-section', sections)).toBe(false)
    expect(isKnownSection('', sections)).toBe(false)
  })

  it('restores a stored active section when it is known', () => {
    const storage = createStorage({ [RECOMMENDATION_SECTION_STORAGE_KEY]: 'effects' })

    expect(readActiveSection('diagnostics', sections, storage)).toBe('effects')
  })

  it('falls back when the stored active section is unknown or missing', () => {
    const unknownStorage = createStorage({ [RECOMMENDATION_SECTION_STORAGE_KEY]: 'removed-section' })
    const emptyStorage = createStorage()

    expect(readActiveSection('diagnostics', sections, unknownStorage)).toBe('diagnostics')
    expect(readActiveSection('diagnostics', sections, emptyStorage)).toBe('diagnostics')
  })

  it('falls back when storage cannot be read', () => {
    const storage = {
      getItem() {
        throw new Error('storage unavailable')
      },
    }

    expect(readActiveSection('diagnostics', sections, storage)).toBe('diagnostics')
  })

  it('writes only known active sections', () => {
    const storage = createStorage()

    expect(writeActiveSection('redis', sections, storage)).toBe(true)
    expect(storage.getItem(RECOMMENDATION_SECTION_STORAGE_KEY)).toBe('redis')

    expect(writeActiveSection('missing', sections, storage)).toBe(false)
    expect(storage.getItem(RECOMMENDATION_SECTION_STORAGE_KEY)).toBe('redis')
  })

  it('returns false when storage cannot be written', () => {
    const storage = {
      setItem() {
        throw new Error('storage unavailable')
      },
    }

    expect(writeActiveSection('redis', sections, storage)).toBe(false)
  })

  it('falls back when the global localStorage property is inaccessible', () => {
    const originalDescriptor = Object.getOwnPropertyDescriptor(globalThis, 'localStorage')
    let result

    try {
      Object.defineProperty(globalThis, 'localStorage', {
        configurable: true,
        get() {
          throw new DOMException('blocked', 'SecurityError')
        },
      })

      expect(() => { result = readActiveSection('diagnostics', sections) }).not.toThrow()
      expect(result).toBe('diagnostics')
      expect(() => { result = writeActiveSection('redis', sections) }).not.toThrow()
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
