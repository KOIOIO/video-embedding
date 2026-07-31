import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AUTH_SESSION_KEY, clearAuthSession, readAuthSession, writeAuthSession } from './session.js'

function storage(initial = {}) {
  const values = new Map(Object.entries(initial))
  return {
    getItem: vi.fn((key) => values.get(key) ?? null),
    setItem: vi.fn((key, value) => values.set(key, value)),
    removeItem: vi.fn((key) => values.delete(key)),
  }
}

beforeEach(() => vi.restoreAllMocks())

describe('administrator auth session', () => {
  it('persists only token and administrator profile', () => {
    const target = storage()
    writeAuthSession({ accessToken: 'token-1', admin: { id: 7, username: 'admin' } }, target)
    expect(target.setItem).toHaveBeenCalledWith(AUTH_SESSION_KEY, JSON.stringify({ accessToken: 'token-1', admin: { id: 7, username: 'admin' } }))
    expect(readAuthSession(target)).toEqual({ accessToken: 'token-1', admin: { id: 7, username: 'admin' } })
  })

  it('fails closed for malformed or incomplete storage', () => {
    expect(readAuthSession(storage({ [AUTH_SESSION_KEY]: '{bad' }))).toBeNull()
    expect(readAuthSession(storage({ [AUTH_SESSION_KEY]: JSON.stringify({ accessToken: '' }) }))).toBeNull()
  })

  it('clears persisted authentication', () => {
    const target = storage()
    clearAuthSession(target)
    expect(target.removeItem).toHaveBeenCalledWith(AUTH_SESSION_KEY)
  })
})
