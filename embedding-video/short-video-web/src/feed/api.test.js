import { describe, expect, it, vi } from 'vitest'
import {
  fetchRandomVideo,
  fetchRandomVideoDistinct,
  fetchReactionCounts,
  normalizeFeedItem,
  submitReaction,
  uniqueKeyOf,
} from './api.js'

function okResponse(data) {
  return { ok: true, status: 200, json: async () => ({ success: true, data }) }
}

describe('feed api', () => {
  it('normalizes a random-play item with defaults', () => {
    const item = normalizeFeedItem({
      video_id: '3',
      video_segment_id: '12',
      start_time_sec: '5',
      end_time_sec: '25',
      title: '  导数入门  ',
      cover_url: '/covers/3.jpg',
      play_url: '/videos/hls/3/master.m3u8',
      user_reacted: true,
      user_reaction_type: 'like',
      author_id: '42',
    })
    expect(item).toEqual({
      video_id: 3,
      video_segment_id: 12,
      start_time_sec: 5,
      end_time_sec: 25,
      title: '导数入门',
      cover_url: '/covers/3.jpg',
      play_url: '/videos/hls/3/master.m3u8',
      user_reaction_type: 'like',
      author_id: 42,
    })
    expect(normalizeFeedItem({}).play_url).toBe('')
  })

  it('keeps double_like and dislike reactions, drops invalid ones', () => {
    expect(normalizeFeedItem({ user_reacted: true, user_reaction_type: 'double_like' }).user_reaction_type).toBe('double_like')
    expect(normalizeFeedItem({ user_reacted: true, user_reaction_type: 'dislike' }).user_reaction_type).toBe('dislike')
    expect(normalizeFeedItem({ user_reacted: true, user_reaction_type: 'bad' }).user_reaction_type).toBe('')
    expect(normalizeFeedItem({ user_reacted: false, user_reaction_type: 'like' }).user_reaction_type).toBe('')
  })

  it('builds a unique key from video and segment ids', () => {
    expect(uniqueKeyOf({ video_id: 3, video_segment_id: 12 })).toBe('3-12')
  })

  it('fetches random-play with user_id and unwraps data', async () => {
    const fetchImpl = vi.fn(async (url) => {
      expect(url).toBe('/api/video-segments/random-play?user_id=7')
      return okResponse({ video_id: 3, video_segment_id: 12, play_url: 'm3u8' })
    })
    const item = await fetchRandomVideo(7, fetchImpl)
    expect(item.video_segment_id).toBe(12)
  })

  it('skips user_id when not provided', async () => {
    const fetchImpl = vi.fn(async (url) => {
      expect(url).toBe('/api/video-segments/random-play')
      return okResponse({ video_id: 1, video_segment_id: 2 })
    })
    await fetchRandomVideo(0, fetchImpl)
  })

  it('retries until a distinct item appears', async () => {
    const seen = new Set(['1-1', '1-2'])
    const fetchImpl = vi
      .fn()
      .mockResolvedValueOnce(okResponse({ video_id: 1, video_segment_id: 1 }))
      .mockResolvedValueOnce(okResponse({ video_id: 1, video_segment_id: 2 }))
      .mockResolvedValueOnce(okResponse({ video_id: 2, video_segment_id: 5 }))
    const item = await fetchRandomVideoDistinct(7, seen, { fetchImpl })
    expect(uniqueKeyOf(item)).toBe('2-5')
    expect(fetchImpl).toHaveBeenCalledTimes(3)
  })

  it('submits a like reaction and reads counts from the response', async () => {
    const fetchImpl = vi.fn(async (url, init) => {
      expect(url).toBe('/api/video-segments/12/reactions')
      expect(JSON.parse(init.body)).toEqual({ user_id: 7, reaction_type: 'like' })
      return okResponse({ active: true, reaction_type: 'like', like_count: 42, double_like_count: 3 })
    })
    const result = await submitReaction({ userId: 7, item: { video_segment_id: 12 }, reactionType: 'like', fetchImpl })
    expect(result).toEqual({ active: true, reaction_type: 'like', like_count: 42, double_like_count: 3 })
  })

  it('submits double_like and dislike reactions', async () => {
    const fetchImpl = vi.fn(async (url, init) => {
      expect(JSON.parse(init.body).reaction_type).toBe('double_like')
      return okResponse({ active: true, reaction_type: 'double_like', like_count: 0, double_like_count: 5 })
    })
    const result = await submitReaction({ userId: 7, item: { video_segment_id: 12 }, reactionType: 'double_like', fetchImpl })
    expect(result).toEqual({ active: true, reaction_type: 'double_like', like_count: 0, double_like_count: 5 })

    const dislikeFetch = vi.fn(async (url, init) => {
      expect(JSON.parse(init.body).reaction_type).toBe('dislike')
      return okResponse({ active: true, reaction_type: 'dislike', like_count: 0, double_like_count: 5 })
    })
    const dislike = await submitReaction({ userId: 7, item: { video_segment_id: 12 }, reactionType: 'dislike', fetchImpl: dislikeFetch })
    expect(dislike.active).toBe(true)
  })

  it('rejects invalid reaction types and missing segment ids', async () => {
    await expect(
      submitReaction({ userId: 7, item: { video_segment_id: 12 }, reactionType: 'bad', fetchImpl: vi.fn() }),
    ).rejects.toThrow('reaction_type must be one of like, double_like, dislike')
    await expect(
      submitReaction({ userId: 7, item: {}, reactionType: 'like', fetchImpl: vi.fn() }),
    ).rejects.toThrow('video_segment_id is required')
  })

  it('fetches reaction counts', async () => {
    const fetchImpl = vi.fn(async (url) => {
      expect(url).toBe('/api/video-segments/12/reaction-counts')
      return okResponse({ segment_id: 12, like_count: 42, double_like_count: 3 })
    })
    const counts = await fetchReactionCounts({ video_segment_id: 12 }, fetchImpl)
    expect(counts).toEqual({ like_count: 42, double_like_count: 3 })
  })
})
