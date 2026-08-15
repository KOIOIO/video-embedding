import { describe, expect, it, vi } from 'vitest'
import { fetchKnowledgeTree, pollKnowledgeVideoBatch, recordKnowledgePlayback, reportKnowledgeWatchSession, resolveKnowledgePlayback, uploadKnowledgeVideoBatch } from './api.js'

const ok = (data, status = 200) => Promise.resolve({ ok: true, status, json: async () => ({ success: true, data }) })

it('normalizes nested knowledge tree data', async () => {
	const fetchImpl = vi.fn(() => ok({ nodes: [{ id: 1, name: '函数', children: [{ id: 9, parent_id: 1, name: '一次函数', children: [], videos: [{ id: 8, status: 'pending' }, { id: 9, status: 'ready' }], video: { id: 8, status: 'pending' } }] }] }))
	const tree = await fetchKnowledgeTree({ fetchImpl })
	expect(tree[0].children[0]).toMatchObject({ id: 9, parentId: 1, status: 'ready', videos: [{ id: 8 }, { id: 9 }] })
})

it('uploads multipart files and reports byte progress', async () => {
  const listeners = {}
  const xhr = {
    upload: { addEventListener: (name, fn) => { listeners[name] = fn } },
    addEventListener: (name, fn) => { listeners[name] = fn },
    open: vi.fn(),
    setRequestHeader: vi.fn(),
    send: vi.fn(function () {
      listeners.progress({ lengthComputable: true, loaded: 40, total: 100 })
      this.status = 202
      this.responseText = JSON.stringify({ success: true, data: { batch_id: 12 } })
      listeners.load()
    }),
  }
  const onProgress = vi.fn()
  const result = await uploadKnowledgeVideoBatch({ archive: new Blob(['zip']), mapping: new Blob(['xlsx']), accessToken: 'token-1', onProgress, xhrFactory: () => xhr })
  expect(onProgress).toHaveBeenCalledWith({ loaded: 40, total: 100, percent: 40 })
  expect(xhr.setRequestHeader).toHaveBeenCalledWith('Authorization', 'Bearer token-1')
  const form = xhr.send.mock.calls[0][0]
  expect(form.has('upload_user_id')).toBe(false)
  expect(result.batch_id).toBe(12)
})

describe('batch polling', () => {
  it('stops on partial_failed', async () => {
    const request = vi.fn().mockResolvedValue({ status: 'partial_failed' })
    const result = await pollKnowledgeVideoBatch(101, { request, delay: async () => {} })
    expect(result.status).toBe('partial_failed')
    expect(request).toHaveBeenCalledTimes(1)
  })

  it('continues until terminal and can be cancelled', async () => {
    const request = vi.fn().mockResolvedValueOnce({ status: 'processing' }).mockResolvedValueOnce({ status: 'completed' })
    expect((await pollKnowledgeVideoBatch(2, { request, delay: async () => {} })).status).toBe('completed')
    expect(request).toHaveBeenCalledTimes(2)
    await expect(pollKnowledgeVideoBatch(2, { request, cancelled: () => true })).resolves.toBeNull()
  })
})

it('lists all ready playback videos without recording', async () => {
	const fetchImpl = vi.fn(() => ok({ videos: [{ knowledge_video_id: 8, playback_url: '/knowledge-video-media/hls/8/master.m3u8' }] }))
	const result = await resolveKnowledgePlayback(9, { fetchImpl })
	expect(fetchImpl.mock.calls[0][0]).toBe('/api/knowledge-points/9/videos')
	expect(result.videos[0].playback_url).toContain('master.m3u8')
})

it('records playback for the concrete knowledge video', async () => {
	const fetchImpl = vi.fn(() => ok({}))
	await recordKnowledgePlayback(88, 7, { fetchImpl })
	expect(fetchImpl).toHaveBeenCalledWith('/api/knowledge-videos/88/playbacks', expect.objectContaining({
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ user_id: 7 }),
	}))
})

it('puts absolute watch seconds with optional keepalive', async () => {
	const fetchImpl = vi.fn(() => ok({ total_watched_seconds: 60 }))
	await reportKnowledgeWatchSession(88, 'session-00000001', 7, 40, { fetchImpl, keepalive: true })
	expect(fetchImpl).toHaveBeenCalledWith('/api/knowledge-videos/88/watch-sessions/session-00000001', {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ user_id: 7, watched_seconds: 40 }),
		keepalive: true,
	})
})
