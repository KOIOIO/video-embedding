import { describe, expect, it, vi } from 'vitest'
import { buildWatchContext, reportWatchProgress } from './watchProgress.js'

describe('watch progress reporting', () => {
  it('builds a watch context for valid recommendation segments', () => {
    const context = buildWatchContext({ video_segment_id: 42 }, 9)

    expect(context).toEqual({
      questionId: 9,
      segmentId: 42,
      lastReportedSec: 0,
    })
  })

  it('skips invalid segments', () => {
    expect(buildWatchContext({ video_segment_id: 0 }, 9)).toBeNull()
  })

  it('reports only increasing watch durations', async () => {
    const requestJson = vi.fn().mockResolvedValue({})
    const context = buildWatchContext({ video_segment_id: 42 }, 9)

    await reportWatchProgress({
      apiBase: '/api',
      context,
      watchedSec: 3.8,
      isWatched: false,
      userId: 1,
      requestJson,
    })
    await reportWatchProgress({
      apiBase: '/api',
      context,
      watchedSec: 2,
      isWatched: true,
      userId: 1,
      requestJson,
    })

    expect(requestJson).toHaveBeenCalledTimes(1)
    expect(requestJson).toHaveBeenCalledWith('/api/watch-records', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        question_id: 9,
        user_id: 1,
        video_segment_id: 42,
        is_watched: false,
        watch_duration: 3,
      }),
    })
    expect(context.lastReportedSec).toBe(3)
  })

  it('surfaces request failures to the caller', async () => {
    const error = new Error('network failed')

    await expect(reportWatchProgress({
      apiBase: '/api',
      context: buildWatchContext({ video_segment_id: 42 }, 9),
      watchedSec: 5,
      isWatched: true,
      userId: 1,
      requestJson: vi.fn().mockRejectedValue(error),
    })).rejects.toThrow('network failed')
  })

  it('does not move local progress backward when reports finish out of order', async () => {
    const context = buildWatchContext({ video_segment_id: 42 }, 9)
    const requestJson = vi.fn().mockImplementation(async () => {
      context.lastReportedSec = 8
    })

    await reportWatchProgress({
      apiBase: '/api',
      context,
      watchedSec: 5,
      isWatched: true,
      userId: 1,
      requestJson,
    })

    expect(context.lastReportedSec).toBe(8)
  })
})
