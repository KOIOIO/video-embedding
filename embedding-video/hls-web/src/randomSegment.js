export function normalizeRandomPlayableSegment(data) {
  return {
    video_id: Number(data?.video_id || 0) || 0,
    video_segment_id: Number(data?.video_segment_id || 0) || 0,
    start_time_sec: Number(data?.start_time_sec || 0) || 0,
    end_time_sec: Number(data?.end_time_sec || 0) || 0,
    title: String(data?.title || '').trim(),
    cover_url: String(data?.cover_url || ''),
    play_url: String(data?.play_url || ''),
  }
}

export async function fetchRandomPlayableSegment({
  apiBase,
  requestJson,
}) {
  if (typeof requestJson !== 'function') throw new Error('requestJson is required')
  const data = await requestJson(`${apiBase}/video-segments/random-play`)
  return normalizeRandomPlayableSegment(data)
}
