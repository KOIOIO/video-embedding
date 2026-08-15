import { describe, expect, it, vi } from 'vitest'
import { createWatchSession } from './watchSession.js'

describe('knowledge video watch session', () => {
  it('advances acknowledgement only after a successful report', async () => {
    const report = vi.fn().mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce({})
    const session = createWatchSession({ knowledgeVideoId: 88, userId: 7, sessionId: 'session-00000001', report })

    await expect(session.flush(15)).rejects.toThrow('offline')
    await session.flush(30)

    expect(report.mock.calls.map(([input]) => input.watchedSeconds)).toEqual([15, 30])
    expect(session.acknowledgedSeconds()).toBe(30)
  })

  it('skips duplicate and lower absolute values', async () => {
    const report = vi.fn().mockResolvedValue({})
    const session = createWatchSession({ knowledgeVideoId: 88, userId: 7, sessionId: 'session-00000001', report })
    await session.flush(20)
    await session.flush(20)
    await session.flush(10)
    expect(report).toHaveBeenCalledTimes(1)
  })
})
