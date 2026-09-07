import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { loadCurrentAdmin, loginAdmin, registerUser } from './api.js'

function okResponse(data) {
  return { ok: true, status: 200, json: async () => ({ success: true, data }) }
}

function failResponse(status, message) {
  return {
    ok: false,
    status,
    json: async () => ({ success: false, error: { message } }),
  }
}

function memoryStorage() {
  const map = new Map()
  return {
    getItem: (k) => (map.has(k) ? map.get(k) : null),
    setItem: (k, v) => map.set(k, String(v)),
    removeItem: (k) => map.delete(k),
  }
}

describe('auth api', () => {
  beforeEach(() => {
    vi.stubGlobal('localStorage', memoryStorage())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('logs in and persists a normalized session', async () => {
    const fetchImpl = vi.fn(async (url, init) => {
      expect(url).toBe('/api/auth/login')
      expect(JSON.parse(init.body)).toEqual({ username: 'admin', password: 'secret' })
      return okResponse({
        access_token: 'jwt-token',
        expires_at: '2026-01-01T00:00:00Z',
        admin: { id: 7, username: 'admin', real_name: '管理员', user_type: 3 },
      })
    })
    const session = await loginAdmin('admin', 'secret', fetchImpl)
    expect(session).toEqual({
      accessToken: 'jwt-token',
      admin: { id: 7, username: 'admin', realName: '管理员', userType: 3 },
    })
    expect(JSON.parse(localStorage.getItem('short-video.session'))).toEqual(session)
  })

  it('rejects wrong credentials with 401', async () => {
    const fetchImpl = vi.fn(async () => failResponse(401, 'invalid credentials'))
    await expect(loginAdmin('admin', 'wrong', fetchImpl)).rejects.toThrow('invalid credentials')
  })

  it('loads current admin with bearer token', async () => {
    localStorage.setItem(
      'short-video.session',
      JSON.stringify({ accessToken: 'jwt-token', admin: { id: 7, username: 'admin' } }),
    )
    const fetchImpl = vi.fn(async (url, init) => {
      expect(url).toBe('/api/auth/me')
      expect(init.headers.get('Authorization')).toBe('Bearer jwt-token')
      return okResponse({ id: 7, username: 'admin', real_name: '管理员', user_type: 3 })
    })
    const admin = await loadCurrentAdmin(fetchImpl)
    expect(admin).toEqual({ id: 7, username: 'admin', realName: '管理员', userType: 3 })
  })

  it('registers a new user and persists a normalized session', async () => {
    const fetchImpl = vi.fn(async (url, init) => {
      expect(url).toBe('/api/auth/register')
      expect(JSON.parse(init.body)).toEqual({ username: 'newuser', password: 'secret123', nickname: 'New User' })
      return okResponse({
        access_token: 'new-jwt-token',
        expires_at: '2026-01-01T00:00:00Z',
        admin: { id: 42, username: 'newuser', real_name: 'New User', user_type: 2 },
      })
    })
    const session = await registerUser('newuser', 'secret123', 'New User', fetchImpl)
    expect(session).toEqual({
      accessToken: 'new-jwt-token',
      admin: { id: 42, username: 'newuser', realName: 'New User', userType: 2 },
    })
    expect(JSON.parse(localStorage.getItem('short-video.session'))).toEqual(session)
  })

  it('rejects registration with duplicate username (409)', async () => {
    const fetchImpl = vi.fn(async () => failResponse(409, 'username already exists'))
    await expect(registerUser('existing', 'secret123', '', fetchImpl)).rejects.toThrow('username already exists')
  })
})
