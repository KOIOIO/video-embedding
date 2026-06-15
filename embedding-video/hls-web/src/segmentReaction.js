export function segmentIdOf(item) {
  return Number(item?.video_segment_id || item?.segment_id || item || 0) || 0
}

export function normalizeSegmentReactionCounts(data) {
  return {
    like_count: Number(data?.like_count || 0) || 0,
    double_like_count: Number(data?.double_like_count || 0) || 0,
  }
}

export async function submitSegmentReaction({
  apiBase,
  item,
  reactionType,
  userId,
  requestJson,
}) {
  const segmentId = segmentIdOf(item)
  if (!segmentId) throw new Error('video_segment_id is required')
  if (typeof requestJson !== 'function') throw new Error('requestJson is required')

  const data = await requestJson(`${apiBase}/video-segments/${encodeURIComponent(String(segmentId))}/reactions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      user_id: userId,
      reaction_type: reactionType,
    }),
  })

  return {
    segmentId,
    active: Boolean(data?.active),
    reactionType: String(data?.reaction_type || reactionType || ''),
    counts: normalizeSegmentReactionCounts(data),
  }
}
