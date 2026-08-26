export const REACTION_LIKE = 'like'
export const REACTION_DOUBLE_LIKE = 'double_like'
export const REACTION_DISLIKE = 'dislike'

const VALID_REACTION_TYPES = new Set([REACTION_LIKE, REACTION_DOUBLE_LIKE, REACTION_DISLIKE])

export function normalizeFeedItem(data) {
  const rawReactionType = String(data?.user_reaction_type || '')
  const userReactionType = Boolean(data?.user_reacted) && VALID_REACTION_TYPES.has(rawReactionType)
    ? rawReactionType
    : ''

  return {
    video_id: Number(data?.video_id || 0) || 0,
    video_segment_id: Number(data?.video_segment_id || 0) || 0,
    start_time_sec: Number(data?.start_time_sec || 0) || 0,
    end_time_sec: Number(data?.end_time_sec || 0) || 0,
    title: String(data?.title || '').trim(),
    cover_url: String(data?.cover_url || ''),
    play_url: String(data?.play_url || ''),
    user_reaction_type: userReactionType,
  }
}

export function uniqueKeyOf(item) {
  return `${item.video_id}-${item.video_segment_id}`
}

async function requestJson(url, init = {}, fetchImpl = fetch) {
  const headers = new Headers(init.headers || {})
  if (init.body) headers.set('Content-Type', 'application/json')
  const response = await fetchImpl(url, { ...init, headers })
  const payload = await response.json().catch(() => null)
  if (!response.ok || payload?.success === false) {
    const error = new Error(payload?.error?.message || payload?.message || `HTTP ${response.status}`)
    error.status = response.status
    throw error
  }
  return payload?.data ?? payload
}

export async function fetchRandomVideo(userId, fetchImpl = fetch) {
  const query = Number(userId || 0) > 0 ? `?user_id=${encodeURIComponent(String(Number(userId)))}` : ''
  const data = await requestJson(`/api/video-segments/random-play${query}`, {}, fetchImpl)
  return normalizeFeedItem(data)
}

export async function fetchRandomVideoDistinct(userId, seenKeys, { maxAttempts = 6, fetchImpl = fetch } = {}) {
  for (let i = 0; i < maxAttempts; i++) {
    const item = await fetchRandomVideo(userId, fetchImpl)
    if (!seenKeys.has(uniqueKeyOf(item))) return item
  }
  return fetchRandomVideo(userId, fetchImpl)
}

export async function submitReaction({ userId, item, reactionType, fetchImpl = fetch }) {
  const segmentId = Number(item?.video_segment_id || 0)
  if (!segmentId) throw new Error('video_segment_id is required')
  if (!VALID_REACTION_TYPES.has(reactionType)) throw new Error('reaction_type must be one of like, double_like, dislike')
  const data = await requestJson(`/api/video-segments/${encodeURIComponent(String(segmentId))}/reactions`, {
    method: 'POST',
    body: JSON.stringify({
      user_id: Number(userId),
      reaction_type: reactionType,
    }),
  }, fetchImpl)
  return {
    active: Boolean(data?.active),
    reaction_type: String(data?.reaction_type || reactionType || ''),
    like_count: Number(data?.like_count || 0) || 0,
    double_like_count: Number(data?.double_like_count || 0) || 0,
  }
}

export async function fetchReactionCounts(item, fetchImpl = fetch) {
  const segmentId = Number(item?.video_segment_id || 0)
  if (!segmentId) throw new Error('video_segment_id is required')
  const data = await requestJson(`/api/video-segments/${encodeURIComponent(String(segmentId))}/reaction-counts`, {}, fetchImpl)
  return {
    like_count: Number(data?.like_count || 0) || 0,
    double_like_count: Number(data?.double_like_count || 0) || 0,
  }
}
