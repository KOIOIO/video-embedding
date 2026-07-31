import { describe, expect, it } from 'vitest'
import {
  buildTrendGeometry,
  formatPerformanceValue,
  normalizePerformancePoints,
} from './recbolePerformance.js'

describe('RecBole performance chart helpers', () => {
  it('normalizes valid points, preserves model versions, and sorts chronologically', () => {
    expect(normalizePerformancePoints([
      { timestamp: '2026-07-14T00:00:00Z', value: 0.375, model_version: ' recbole_v2 ' },
      { timestamp: 'invalid', value: 0.9, model_version: 'bad' },
      { timestamp: '2026-07-10T00:00:00Z', value: '0.142857', model_version: 'recbole_v1' },
    ])).toEqual([
      { timestamp: '2026-07-10T00:00:00.000Z', time: Date.parse('2026-07-10T00:00:00Z'), value: 0.142857, modelVersion: 'recbole_v1' },
      { timestamp: '2026-07-14T00:00:00.000Z', time: Date.parse('2026-07-14T00:00:00Z'), value: 0.375, modelVersion: 'recbole_v2' },
    ])
  })

  it('keeps equal-value series finite and formats values', () => {
    const geometry = buildTrendGeometry([
      { timestamp: '2026-07-10T00:00:00Z', value: 0.5 },
      { timestamp: '2026-07-14T00:00:00Z', value: 0.5 },
    ], { width: 720, height: 280, padding: 36 })
    expect(geometry.points.every((point) => Number.isFinite(point.x) && Number.isFinite(point.y))).toBe(true)
    expect(formatPerformanceValue(0.142857142857)).toBe('0.14286')
    expect(formatPerformanceValue(undefined)).toBe('-')
  })
})
