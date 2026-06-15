import { describe, expect, it, vi } from 'vitest'
import { normalizeSegmentReactionCounts, segmentIdOf, submitSegmentReaction } from './segmentReaction.js'

describe('segment reactions', () => {
  it('extracts segment ids from recommendation items', () => {
    expect(segmentIdOf({ video_segment_id: 42 })).toBe(42)
    expect(segmentIdOf({ segment_id: 43 })).toBe(43)
    expect(segmentIdOf(44)).toBe(44)
    expect(segmentIdOf({ video_segment_id: 0 })).toBe(0)
  })

  it('normalizes missing reaction counts to zero', () => {
    expect(normalizeSegmentReactionCounts({ like_count: 3, double_like_count: 2 })).toEqual({
      like_count: 3,
      double_like_count: 2,
    })
    expect(normalizeSegmentReactionCounts(null)).toEqual({
      like_count: 0,
      double_like_count: 0,
    })
  })

  it('submits a segment reaction to the segment endpoint', async () => {
    const requestJson = vi.fn().mockResolvedValue({
      segment_id: 42,
      reaction_type: 'double_like',
      active: true,
      like_count: 1,
      double_like_count: 4,
    })

    const result = await submitSegmentReaction({
      apiBase: '/api',
      item: { video_segment_id: 42 },
      reactionType: 'double_like',
      userId: 7,
      requestJson,
    })

    expect(requestJson).toHaveBeenCalledWith('/api/video-segments/42/reactions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        user_id: 7,
        reaction_type: 'double_like',
      }),
    })
    expect(result).toEqual({
      segmentId: 42,
      active: true,
      reactionType: 'double_like',
      counts: {
        like_count: 1,
        double_like_count: 4,
      },
    })
  })

  it('requires a valid segment id', async () => {
    await expect(submitSegmentReaction({
      apiBase: '/api',
      item: { video_segment_id: 0 },
      reactionType: 'like',
      userId: 7,
      requestJson: vi.fn(),
    })).rejects.toThrow('video_segment_id is required')
  })
})
