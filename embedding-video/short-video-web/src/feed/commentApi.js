import { readSession } from '../auth/session.js'

export const COMMENT_REACTION_LIKE = 'like'
export const COMMENT_REACTION_DOUBLE_LIKE = 'double_like'
export const COMMENT_REACTION_DISLIKE = 'dislike'

export function normalizeComment(data) {
  return {
    id: Number(data?.id || 0) || 0,
    user_id: Number(data?.user_id || 0) || 0,
    username: String(data?.username || ''),
    reply_to_username: String(data?.reply_to_username || ''),
    content: String(data?.content || ''),
    like_count: Number(data?.like_count || 0) || 0,
    double_like_count: Number(data?.double_like_count || 0) || 0,
    user_reaction_type: String(data?.user_reaction_type || ''),
    created_at_unix: Number(data?.created_at_unix || 0) || 0,
    reply_count: Number(data?.reply_count || 0) || 0,
    has_more_replies: Boolean(data?.has_more_replies),
    replies: Array.isArray(data?.replies) ? data.replies.map(normalizeComment) : [],
  }
}

export function normalizeCommentList(data) {
  return {
    total: Number(data?.total || 0) || 0,
    page: Number(data?.page || 0) || 0,
    page_size: Number(data?.page_size || 0) || 0,
    comments: Array.isArray(data?.comments) ? data.comments.map(normalizeComment) : [],
  }
}

function responseError(payload, response) {
  const error = new Error(payload?.error?.message || payload?.message || `HTTP ${response.status}`)
  error.status = response.status
  return error
}

async function requestJson(url, init = {}, fetchImpl = fetch) {
  const session = readSession()
  const headers = new Headers(init.headers || {})
  if (init.body) headers.set('Content-Type', 'application/json')
  if (session?.accessToken) headers.set('Authorization', `Bearer ${session.accessToken}`)
  const response = await fetchImpl(url, { ...init, headers })
  const payload = await response.json().catch(() => null)
  if (!response.ok || payload?.success === false) throw responseError(payload, response)
  return payload?.data ?? payload
}

export async function fetchCommentCounts(segmentId, fetchImpl = fetch) {
  const data = await requestJson(`/api/video-segments/${encodeURIComponent(String(segmentId))}/comment-counts`, {}, fetchImpl)
  return Number(data?.total || 0) || 0
}

export async function fetchComments({ segmentId, userId = 0, page = 1, pageSize = 10, fetchImpl = fetch } = {}) {
  const params = new URLSearchParams()
  if (Number(userId) > 0) params.set('user_id', String(Number(userId)))
  params.set('page', String(page))
  params.set('page_size', String(pageSize))
  const data = await requestJson(`/api/video-segments/${encodeURIComponent(String(segmentId))}/comments?${params}`, {}, fetchImpl)
  return normalizeCommentList(data)
}

export async function fetchReplies({ commentId, userId = 0, page = 1, pageSize = 20, fetchImpl = fetch } = {}) {
  const params = new URLSearchParams()
  if (Number(userId) > 0) params.set('user_id', String(Number(userId)))
  params.set('page', String(page))
  params.set('page_size', String(pageSize))
  const data = await requestJson(`/api/comments/${encodeURIComponent(String(commentId))}/replies?${params}`, {}, fetchImpl)
  return normalizeCommentList(data)
}

export async function createComment({ segmentId, content, fetchImpl = fetch }) {
  const data = await requestJson(`/api/video-segments/${encodeURIComponent(String(segmentId))}/comments`, {
    method: 'POST',
    body: JSON.stringify({ content }),
  }, fetchImpl)
  return normalizeComment(data)
}

export async function createReply({ commentId, content, fetchImpl = fetch }) {
  const data = await requestJson(`/api/comments/${encodeURIComponent(String(commentId))}/replies`, {
    method: 'POST',
    body: JSON.stringify({ content }),
  }, fetchImpl)
  return normalizeComment(data)
}

export async function toggleCommentReaction({ commentId, reactionType, fetchImpl = fetch }) {
  const data = await requestJson(`/api/comments/${encodeURIComponent(String(commentId))}/reactions`, {
    method: 'POST',
    body: JSON.stringify({ reaction_type: reactionType }),
  }, fetchImpl)
  return {
    active: Boolean(data?.active),
    reaction_type: String(data?.reaction_type || reactionType || ''),
    like_count: Number(data?.like_count || 0) || 0,
    double_like_count: Number(data?.double_like_count || 0) || 0,
  }
}
