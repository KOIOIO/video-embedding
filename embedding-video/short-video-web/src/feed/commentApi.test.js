import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  createComment,
  createReply,
  fetchCommentCounts,
  fetchComments,
  fetchReplies,
  normalizeComment,
  normalizeCommentList,
  toggleCommentReaction,
} from './commentApi.js'

function okResponse(data) {
  return { ok: true, status: 200, json: async () => ({ success: true, data }) }
}

function memoryStorage() {
  const map = new Map()
  return {
    getItem: (k) => (map.has(k) ? map.get(k) : null),
    setItem: (k, v) => map.set(k, String(v)),
    removeItem: (k) => map.delete(k),
  }
}

describe('comment api', () => {
  beforeEach(() => {
    vi.stubGlobal('localStorage', memoryStorage())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('normalizes comment payloads with defaults', () => {
    const comment = normalizeComment({
      id: '1',
      user_id: '7',
      username: '管理员',
      reply_to_username: '学生',
      content: '不错',
      like_count: '3',
      double_like_count: '2',
      user_reaction_type: 'like',
      created_at_unix: '1787800000',
      reply_count: '2',
      has_more_replies: true,
      replies: [{ id: '2', user_id: '2', username: '学生', content: '同感', created_at_unix: '1787800010' }],
    })
    expect(comment.id).toBe(1)
    expect(comment.like_count).toBe(3)
    expect(comment.double_like_count).toBe(2)
    expect(comment.user_reaction_type).toBe('like')
    expect(comment.reply_to_username).toBe('学生')
    expect(comment.replies).toHaveLength(1)
    expect(normalizeComment({}).username).toBe('')
    expect(normalizeComment({}).user_reaction_type).toBe('')
  })

  it('normalizes comment lists', () => {
    const list = normalizeCommentList({
      total: 42,
      page: 2,
      page_size: 10,
      comments: [{ id: 1, content: 'hi' }],
    })
    expect(list.total).toBe(42)
    expect(list.page).toBe(2)
    expect(list.comments).toHaveLength(1)
  })

  it('fetches comment counts and lists with viewer id', async () => {
    const fetchImpl = vi.fn(async (url) => {
      if (url.includes('comment-counts')) return okResponse({ video_segment_id: 21, total: 5 })
      expect(url).toBe('/api/video-segments/21/comments?user_id=7&page=2&page_size=10')
      return okResponse({ total: 5, page: 2, page_size: 10, comments: [] })
    })
    expect(await fetchCommentCounts(21, fetchImpl)).toBe(5)
    const list = await fetchComments({ segmentId: 21, userId: 7, page: 2, fetchImpl })
    expect(list.total).toBe(5)
  })

  it('sends bearer token on comment writes', async () => {
    localStorage.setItem(
      'short-video.session',
      JSON.stringify({ accessToken: 'jwt-token', admin: { id: 7, username: 'admin' } }),
    )
    const fetchImpl = vi.fn(async (url, init) => {
      expect(init.headers.get('Authorization')).toBe('Bearer jwt-token')
      return okResponse({ id: 9, content: 'hello' })
    })

    await createComment({ segmentId: 21, content: 'hello', fetchImpl })
    expect(fetchImpl).toHaveBeenLastCalledWith(
      '/api/video-segments/21/comments',
      expect.objectContaining({ method: 'POST' }),
    )

    await createReply({ commentId: 9, content: 'hi', fetchImpl })
    expect(fetchImpl).toHaveBeenLastCalledWith(
      '/api/comments/9/replies',
      expect.objectContaining({ method: 'POST' }),
    )

    await toggleCommentReaction({ commentId: 9, reactionType: 'double_like', fetchImpl })
    expect(fetchImpl).toHaveBeenLastCalledWith(
      '/api/comments/9/reactions',
      expect.objectContaining({ method: 'POST' }),
    )
  })

  it('fetches replies with pagination', async () => {
    const fetchImpl = vi.fn(async (url) => {
      expect(url).toBe('/api/comments/9/replies?user_id=7&page=1&page_size=20')
      return okResponse({ total: 2, comments: [{ id: 2, content: 'r1' }, { id: 3, content: 'r2' }] })
    })
    const list = await fetchReplies({ commentId: 9, userId: 7, fetchImpl })
    expect(list.comments).toHaveLength(2)
  })

  it('toggles comment reaction and reads counts', async () => {
    localStorage.setItem(
      'short-video.session',
      JSON.stringify({ accessToken: 'jwt', admin: { id: 7, username: 'admin' } }),
    )
    const fetchImpl = vi.fn(async (url, init) => {
      expect(url).toBe('/api/comments/9/reactions')
      expect(JSON.parse(init.body)).toEqual({ reaction_type: 'like' })
      return okResponse({ comment_id: 9, active: true, reaction_type: 'like', like_count: 4, double_like_count: 2 })
    })
    const result = await toggleCommentReaction({ commentId: 9, reactionType: 'like', fetchImpl })
    expect(result).toEqual({ active: true, reaction_type: 'like', like_count: 4, double_like_count: 2 })
  })
})
