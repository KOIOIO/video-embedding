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
    })).toEqual({
      video_id: 12,
      video_segment_id: 34,
      start_time_sec: 10,
      end_time_sec: 40,
      title: 'segment title',
      cover_url: '/covers/12.jpg',
      play_url: '/videos/hls/2026/06/09/demo/master.m3u8',
    })
  })

  it('requests the backend random-play endpoint', async () => {
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
    })

    expect(requestJson).toHaveBeenCalledWith('/api/video-segments/random-play')
    expect(item.video_segment_id).toBe(34)
    expect(item.play_url).toBe('/videos/hls/2026/06/09/demo/master.m3u8')
  })

  it('requires a requestJson function', async () => {
    await expect(fetchRandomPlayableSegment({
      apiBase: '/api',
    })).rejects.toThrow('requestJson is required')
  })
})
