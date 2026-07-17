import { describe, expect, it } from 'vitest'
import {
  buildTrendGeometry,
  formatPerformanceValue,
  normalizePerformancePoints,
} from './gorsePerformance.js'

describe('Gorse performance chart helpers', () => {
  it('normalizes valid points and sorts them chronologically', () => {
    expect(normalizePerformancePoints([
      { timestamp: '2026-07-14T00:00:00Z', value: 0.375 },
      { timestamp: 'invalid', value: 0.9 },
      { timestamp: '2026-07-10T00:00:00Z', value: '0.142857' },
      { timestamp: '2026-07-13T00:00:00Z', value: 'not-a-number' },
    ])).toEqual([
      { timestamp: '2026-07-10T00:00:00.000Z', time: Date.parse('2026-07-10T00:00:00Z'), value: 0.142857 },
      { timestamp: '2026-07-14T00:00:00.000Z', time: Date.parse('2026-07-14T00:00:00Z'), value: 0.375 },
    ])
  })

  it('places a single point inside stable chart bounds', () => {
    const geometry = buildTrendGeometry([
      { timestamp: '2026-07-14T00:00:00Z', value: 0.375 },
    ], { width: 720, height: 280, padding: 36 })

    expect(geometry.points).toHaveLength(1)
    expect(geometry.points[0].x).toBe(360)
    expect(geometry.points[0].y).toBeGreaterThanOrEqual(36)
    expect(geometry.points[0].y).toBeLessThanOrEqual(244)
    expect(geometry.linePath).toContain('M 360')
    expect(geometry.areaPath).toContain('244')
  })

  it('keeps equal-value series finite and within the plot', () => {
    const geometry = buildTrendGeometry([
      { timestamp: '2026-07-10T00:00:00Z', value: 0.5 },
      { timestamp: '2026-07-14T00:00:00Z', value: 0.5 },
    ], { width: 720, height: 280, padding: 36 })

    expect(geometry.max).toBeGreaterThan(geometry.min)
    for (const point of geometry.points) {
      expect(Number.isFinite(point.x)).toBe(true)
      expect(Number.isFinite(point.y)).toBe(true)
      expect(point.x).toBeGreaterThanOrEqual(36)
      expect(point.x).toBeLessThanOrEqual(684)
      expect(point.y).toBeGreaterThanOrEqual(36)
      expect(point.y).toBeLessThanOrEqual(244)
    }
  })

  it('formats performance values without noisy precision', () => {
    expect(formatPerformanceValue(0.142857142857)).toBe('0.14286')
    expect(formatPerformanceValue(12.5)).toBe('12.5')
    expect(formatPerformanceValue(undefined)).toBe('-')
  })
})
