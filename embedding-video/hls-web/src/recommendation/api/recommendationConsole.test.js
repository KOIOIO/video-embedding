import { describe, expect, it } from 'vitest'
import {
  buildPreviewRandomPlayURL,
  buildTraceRandomPlayURL,
  fetchRecommendationDiagnostics,
  fetchRecommendationDatasources,
  fetchRecommendationEffects,
  fetchGorsePerformance,
  fetchRecommendationOverview,
  fetchRecommendationRedisState,
  previewByQuestion,
  previewRandomPlay,
  recommendationConsoleEndpoints,
  traceByQuestion,
  traceRandomPlay,
} from './recommendationConsole.js'

describe('recommendation console API helpers', () => {
  it('builds a random-play preview URL with user and limit params', () => {
    expect(buildPreviewRandomPlayURL({ userID: 7, limit: 5 })).toBe(
      '/api/admin/recommendation/preview/random-play?user_id=7&limit=5',
    )
  })

  it('builds a random-play trace URL with user and limit params', () => {
    expect(buildTraceRandomPlayURL({ userID: 7, limit: 5 })).toBe(
      '/api/admin/recommendation/trace/random-play?user_id=7&limit=5',
    )
  })

  it('keeps endpoint paths under the admin recommendation namespace', () => {
    for (const endpoint of Object.values(recommendationConsoleEndpoints)) {
      expect(endpoint.startsWith('/api/admin/recommendation')).toBe(true)
    }
  })

  it('requests the overview endpoint', async () => {
    const calls = []
    const response = await fetchRecommendationOverview((url, options) => {
      calls.push({ url, options })
      return Promise.resolve({ success: true })
    })

    expect(response).toEqual({ success: true })
    expect(calls).toEqual([{ url: '/api/admin/recommendation/overview', options: undefined }])
  })

  it('requests datasource stats', async () => {
    const calls = []
    await fetchRecommendationDatasources((url, options) => {
      calls.push({ url, options })
      return Promise.resolve({ success: true })
    })

    expect(calls).toEqual([{ url: '/api/admin/recommendation/datasources', options: undefined }])
  })

  it('requests diagnostics with days and request limit', async () => {
    const calls = []
    await fetchRecommendationDiagnostics(
      { days: 14, limit: 20 },
      (url, options) => {
        calls.push({ url, options })
        return Promise.resolve({ success: true })
      },
    )

    expect(calls).toEqual([{ url: '/api/admin/recommendation/diagnostics?days=14&limit=20', options: undefined }])
  })

  it('requests effect metrics with a days window', async () => {
    const calls = []
    await fetchRecommendationEffects(
      { days: 14 },
      (url, options) => {
        calls.push({ url, options })
        return Promise.resolve({ success: true })
      },
    )

    expect(calls).toEqual([{ url: '/api/admin/recommendation/effects?days=14', options: undefined }])
  })

  it('requests Gorse performance with encoded metric and date range', async () => {
    const calls = []
    await fetchGorsePerformance(
      {
        metric: 'cf_ndcg',
        begin: '2026-07-09T00:00:00.000Z',
        end: '2026-07-16T23:59:59.999Z',
      },
      (url, options) => {
        calls.push({ url, options })
        return Promise.resolve({ success: true })
      },
    )

    expect(calls).toEqual([{
      url: '/api/admin/recommendation/gorse/performance?metric=cf_ndcg&begin=2026-07-09T00%3A00%3A00.000Z&end=2026-07-16T23%3A59%3A59.999Z',
      options: undefined,
    }])
  })

  it('requests redis state with a user id', async () => {
    const calls = []
    await fetchRecommendationRedisState(
      { userID: 12 },
      (url, options) => {
        calls.push({ url, options })
        return Promise.resolve({ success: true })
      },
    )

    expect(calls).toEqual([{ url: '/api/admin/recommendation/redis-state?user_id=12', options: undefined }])
  })

  it('requests a random-play preview with query params', async () => {
    const calls = []
    await previewRandomPlay(
      { userID: 12, limit: 4 },
      (url, options) => {
        calls.push({ url, options })
        return Promise.resolve({ success: true })
      },
    )

    expect(calls).toEqual([
      { url: '/api/admin/recommendation/preview/random-play?user_id=12&limit=4', options: undefined },
    ])
  })

  it('requests a random-play trace with query params', async () => {
    const calls = []
    await traceRandomPlay(
      { userID: 12, limit: 4 },
      (url, options) => {
        calls.push({ url, options })
        return Promise.resolve({ success: true })
      },
    )

    expect(calls).toEqual([
      { url: '/api/admin/recommendation/trace/random-play?user_id=12&limit=4', options: undefined },
    ])
  })

  it('posts a trimmed by-question preview payload', async () => {
    const calls = []
    await previewByQuestion(
      { questionText: '  how to factor  ', userID: '9', limit: '2' },
      (url, options) => {
        calls.push({ url, options })
        return Promise.resolve({ success: true })
      },
    )

    expect(calls).toHaveLength(1)
    expect(calls[0].url).toBe('/api/admin/recommendation/preview/by-question')
    expect(calls[0].options.method).toBe('POST')
    expect(JSON.parse(calls[0].options.body)).toEqual({
      question_text: 'how to factor',
      user_id: 9,
      limit: 2,
    })
  })

  it('posts a trimmed by-question trace payload', async () => {
    const calls = []
    await traceByQuestion(
      { questionID: '8', questionText: '  quadratic  ', userID: '9', limit: '2' },
      (url, options) => {
        calls.push({ url, options })
        return Promise.resolve({ success: true })
      },
    )

    expect(calls).toHaveLength(1)
    expect(calls[0].url).toBe('/api/admin/recommendation/trace/by-question')
    expect(calls[0].options.method).toBe('POST')
    expect(JSON.parse(calls[0].options.body)).toEqual({
      question_id: 8,
      question_text: 'quadratic',
      user_id: 9,
      limit: 2,
    })
  })
})
