import { describe, expect, it } from 'vitest'
import { clearSession, readSession, writeSession } from './session.js'

function memoryStorage() {
  const map = new Map()
  return {
    getItem: (k) => (map.has(k) ? map.get(k) : null),
    setItem: (k, v) => map.set(k, String(v)),
    removeItem: (k) => map.delete(k),
  }
}

describe('session', () => {
  it('writes and reads a valid session', () => {
    const storage = memoryStorage()
    const session = { accessToken: 'tok', admin: { id: 7, username: 'admin' } }
    writeSession(session, storage)
    expect(readSession(storage)).toEqual(session)
  })

  it('rejects malformed or incomplete stored data', () => {
    const storage = memoryStorage()
    storage.setItem('short-video.session', '{bad json')
    expect(readSession(storage)).toBeNull()
    storage.setItem('short-video.session', JSON.stringify({ accessToken: 'tok' }))
    expect(readSession(storage)).toBeNull()
  })

  it('clears the session', () => {
    const storage = memoryStorage()
    writeSession({ accessToken: 'tok', admin: { id: 1 } }, storage)
    clearSession(storage)
    expect(readSession(storage)).toBeNull()
  })
})
