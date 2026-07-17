const validReactionTypes = new Set(['like', 'double_like', 'dislike'])

export function normalizeRandomPlayableSegment(data) {
  const rawReactionType = String(data?.user_reaction_type || '')
  const userReactionType = Boolean(data?.user_reacted) && validReactionTypes.has(rawReactionType) ? rawReactionType : ''

  return {
    video_id: Number(data?.video_id || 0) || 0,
    video_segment_id: Number(data?.video_segment_id || 0) || 0,
    start_time_sec: Number(data?.start_time_sec || 0) || 0,
    end_time_sec: Number(data?.end_time_sec || 0) || 0,
    title: String(data?.title || '').trim(),
    cover_url: String(data?.cover_url || ''),
    play_url: String(data?.play_url || ''),
    user_reacted: userReactionType !== '',
    user_reaction_type: userReactionType,
  }
}

export async function fetchRandomPlayableSegment({
  apiBase,
  requestJson,
  userId,
}) {
  if (typeof requestJson !== 'function') throw new Error('requestJson is required')
  const normalizedUserId = Number(userId || 0) || 0
  const query = normalizedUserId > 0 ? `?user_id=${encodeURIComponent(String(normalizedUserId))}` : ''
  const data = await requestJson(`${apiBase}/video-segments/random-play${query}`)
  return normalizeRandomPlayableSegment(data)
}
