import { describe, expect, it, vi } from 'vitest'
import { apiFetch, loginAdmin } from './api.js'
import { AUTH_SESSION_KEY } from './session.js'

describe('administrator auth API', () => {
  it('logs in with username and password', async () => {
    const fetchImpl = vi.fn(async () => ({ ok: true, status: 200, json: async () => ({ success: true, data: { access_token: 'token-1', admin: { id: 7 } } }) }))
    await expect(loginAdmin('admin', 'secret', fetchImpl)).resolves.toMatchObject({ access_token: 'token-1' })
    expect(fetchImpl).toHaveBeenCalledWith('/api/auth/login', expect.objectContaining({ method: 'POST', body: JSON.stringify({ username: 'admin', password: 'secret' }) }))
  })

  it('adds bearer token without changing existing headers', async () => {
    const fetchImpl = vi.fn(async () => ({ ok: true, status: 200 }))
    const storage = { getItem: vi.fn(() => JSON.stringify({ accessToken: 'token-1', admin: { id: 7 } })) }
    await apiFetch('/api/system/metrics', { headers: { Accept: 'application/json' } }, { fetchImpl, storage })
    expect(fetchImpl).toHaveBeenCalledWith('/api/system/metrics', expect.objectContaining({ headers: expect.objectContaining({ accept: 'application/json', authorization: 'Bearer token-1' }) }))
  })

  it('clears session and emits expiration on unauthorized response', async () => {
    const fetchImpl = vi.fn(async () => ({ ok: false, status: 401 }))
    const storage = { getItem: vi.fn(() => JSON.stringify({ accessToken: 'token-1', admin: { id: 7 } })), removeItem: vi.fn() }
    const dispatch = vi.fn()
    await apiFetch('/api/system/metrics', {}, { fetchImpl, storage, dispatch })
    expect(storage.removeItem).toHaveBeenCalledWith(AUTH_SESSION_KEY)
    expect(dispatch).toHaveBeenCalledWith('admin-auth-expired')
  })
})
