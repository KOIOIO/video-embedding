import { describe, expect, it, vi } from 'vitest'
import { fetchRandomPlayableSegment, normalizeRandomPlayableSegment } from './randomSegment.js'

describe('random playable segment', () => {
  it('normalizes random segment response fields', () => {
    expect(normalizeRandomPlayableSegment({
      video_id: '12',
      video_segment_id: '34',
      start_time_sec: '10',
      end_time_sec: '40',
      title: '  segment title  ',
      cover_url: '/covers/12.jpg',
      play_url: '/videos/hls/2026/06/09/demo/master.m3u8',
      user_reacted: true,
      user_reaction_type: 'like',
    })).toEqual({
      video_id: 12,
      video_segment_id: 34,
      start_time_sec: 10,
      end_time_sec: 40,
      title: 'segment title',
      cover_url: '/covers/12.jpg',
      play_url: '/videos/hls/2026/06/09/demo/master.m3u8',
      user_reacted: true,
      user_reaction_type: 'like',
    })
  })

  it('requests the backend random-play endpoint with user id', async () => {
    const requestJson = vi.fn().mockResolvedValue({
      video_id: 12,
      video_segment_id: 34,
      start_time_sec: 10,
      end_time_sec: 40,
      title: 'segment title',
      cover_url: '/covers/12.jpg',
      play_url: '/videos/hls/2026/06/09/demo/master.m3u8',
    })

    const item = await fetchRandomPlayableSegment({
      apiBase: '/api',
      requestJson,
      userId: 6,
    })

    expect(requestJson).toHaveBeenCalledWith('/api/video-segments/random-play?user_id=6')
    expect(item.video_segment_id).toBe(34)
    expect(item.play_url).toBe('/videos/hls/2026/06/09/demo/master.m3u8')
  })

  it('requires a requestJson function', async () => {
    await expect(fetchRandomPlayableSegment({
      apiBase: '/api',
    })).rejects.toThrow('requestJson is required')
  })
})
