export function buildWatchContext(item, questionId) {
  const segmentId = Number(item?.video_segment_id || 0)
  const qid = Number(questionId || 0)
  if (!segmentId) return null
  return {
    questionId: qid,
    segmentId,
    lastReportedSec: 0,
  }
}

export async function reportWatchProgress({
  apiBase,
  context,
  watchedSec,
  isWatched = true,
  userId,
  requestJson,
}) {
  const duration = Math.max(0, Math.floor(Number(watchedSec || 0)))
  const lastReportedSec = Number(context?.lastReportedSec || 0)
  if (!context?.segmentId) return false
  if (duration <= lastReportedSec) return false

  await requestJson(`${apiBase}/watch-records`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      question_id: context.questionId,
      user_id: userId,
      video_segment_id: context.segmentId,
      is_watched: isWatched,
      watch_duration: duration,
    }),
  })
  context.lastReportedSec = Math.max(Number(context.lastReportedSec || 0), duration)
  return true
}
