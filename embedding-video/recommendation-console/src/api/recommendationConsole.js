import { requestJson } from './http.js'

const ADMIN_RECOMMENDATION_BASE = '/api/admin/recommendation'

export const recommendationConsoleEndpoints = {
  overview: `${ADMIN_RECOMMENDATION_BASE}/overview`,
  diagnostics: `${ADMIN_RECOMMENDATION_BASE}/diagnostics`,
  datasources: `${ADMIN_RECOMMENDATION_BASE}/datasources`,
  effects: `${ADMIN_RECOMMENDATION_BASE}/effects`,
  traceRandomPlay: `${ADMIN_RECOMMENDATION_BASE}/trace/random-play`,
  traceQuestion: `${ADMIN_RECOMMENDATION_BASE}/trace/by-question`,
  redisState: `${ADMIN_RECOMMENDATION_BASE}/redis-state`,
  previewRandomPlay: `${ADMIN_RECOMMENDATION_BASE}/preview/random-play`,
  previewQuestion: `${ADMIN_RECOMMENDATION_BASE}/preview/by-question`,
  userState: `${ADMIN_RECOMMENDATION_BASE}/users`,
  gorseSync: `${ADMIN_RECOMMENDATION_BASE}/gorse/sync`,
  recboleModels: `${ADMIN_RECOMMENDATION_BASE}/recbole/models`,
}

export function buildPreviewRandomPlayURL({ userID, limit } = {}) {
  return buildUserLimitURL(recommendationConsoleEndpoints.previewRandomPlay, { userID, limit })
}

export function buildTraceRandomPlayURL({ userID, limit } = {}) {
  return buildUserLimitURL(recommendationConsoleEndpoints.traceRandomPlay, { userID, limit })
}

function buildUserLimitURL(endpoint, { userID, limit } = {}) {
  const url = new URL(endpoint, 'http://console.local')
  if (userID) {
    url.searchParams.set('user_id', String(userID))
  }
  if (limit) {
    url.searchParams.set('limit', String(limit))
  }
  return `${url.pathname}${url.search}`
}

export function fetchRecommendationOverview(request = requestJson) {
  return request(recommendationConsoleEndpoints.overview)
}

export function fetchRecommendationDatasources(request = requestJson) {
  return request(recommendationConsoleEndpoints.datasources)
}

export function fetchRecommendationDiagnostics({ days, limit } = {}, request = requestJson) {
  const url = new URL(recommendationConsoleEndpoints.diagnostics, 'http://console.local')
  const normalizedDays = toPositiveInteger(days)
  if (normalizedDays) {
    url.searchParams.set('days', String(normalizedDays))
  }
  const normalizedLimit = toPositiveInteger(limit)
  if (normalizedLimit) {
    url.searchParams.set('limit', String(normalizedLimit))
  }
  return request(`${url.pathname}${url.search}`)
}

export function fetchRecommendationEffects({ days } = {}, request = requestJson) {
  const url = new URL(recommendationConsoleEndpoints.effects, 'http://console.local')
  const normalizedDays = toPositiveInteger(days)
  if (normalizedDays) {
    url.searchParams.set('days', String(normalizedDays))
  }
  return request(`${url.pathname}${url.search}`)
}

export function fetchRecommendationRedisState({ userID } = {}, request = requestJson) {
  const url = new URL(recommendationConsoleEndpoints.redisState, 'http://console.local')
  const normalizedUserID = toPositiveInteger(userID)
  if (normalizedUserID) {
    url.searchParams.set('user_id', String(normalizedUserID))
  }
  return request(`${url.pathname}${url.search}`)
}

export function previewRandomPlay(params, request = requestJson) {
  return request(buildPreviewRandomPlayURL(params))
}

export function traceRandomPlay(params, request = requestJson) {
  return request(buildTraceRandomPlayURL(params))
}

export function previewByQuestion(params = {}, request = requestJson) {
  return request(recommendationConsoleEndpoints.previewQuestion, {
    method: 'POST',
    body: JSON.stringify(buildPreviewByQuestionPayload(params)),
  })
}

export function traceByQuestion(params = {}, request = requestJson) {
  return request(recommendationConsoleEndpoints.traceQuestion, {
    method: 'POST',
    body: JSON.stringify(buildPreviewByQuestionPayload(params)),
  })
}

function buildPreviewByQuestionPayload({ questionID, questionText, userID, limit } = {}) {
  const payload = {}
  const normalizedQuestionID = toPositiveInteger(questionID)
  if (normalizedQuestionID) {
    payload.question_id = normalizedQuestionID
  }
  const normalizedText = String(questionText || '').trim()
  if (normalizedText) {
    payload.question_text = normalizedText
  }
  const normalizedUserID = toPositiveInteger(userID)
  if (normalizedUserID) {
    payload.user_id = normalizedUserID
  }
  const normalizedLimit = toPositiveInteger(limit)
  if (normalizedLimit) {
    payload.limit = normalizedLimit
  }
  return payload
}

function toPositiveInteger(value) {
  const number = Number(value)
  if (!Number.isFinite(number) || number <= 0) {
    return 0
  }
  return Math.trunc(number)
}
