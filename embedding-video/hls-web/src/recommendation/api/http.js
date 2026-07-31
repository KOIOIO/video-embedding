import { apiFetch } from '../../auth/api.js'

export async function requestJson(url, options = {}) {
  const response = await apiFetch(url, {
    headers: {
      Accept: 'application/json',
      ...(options.body ? { 'Content-Type': 'application/json' } : {}),
      ...(options.headers || {}),
    },
    ...options,
  })

  if (!response.ok) {
    throw new Error(`request failed: ${response.status}`)
  }

  if (response.status === 204) {
    return null
  }

  return response.json()
}
