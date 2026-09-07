import { describe, expect, it } from 'vitest'
import { formatRelativeTime } from './relativeTime.js'

const NOW = 1787800000000

describe('formatRelativeTime', () => {
  it('formats recent times', () => {
    expect(formatRelativeTime(NOW / 1000 - 10, NOW)).toBe('刚刚')
    expect(formatRelativeTime(NOW / 1000 - 5 * 60, NOW)).toBe('5分钟前')
    expect(formatRelativeTime(NOW / 1000 - 3 * 3600, NOW)).toBe('3小时前')
    expect(formatRelativeTime(NOW / 1000 - 2 * 86400, NOW)).toBe('2天前')
  })

  it('formats older times as dates', () => {
    const older = new Date('2026-01-02T00:00:00Z').getTime() / 1000
    expect(formatRelativeTime(older, NOW)).toBe('2026-01-02')
  })

  it('handles invalid input', () => {
    expect(formatRelativeTime(0, NOW)).toBe('')
    expect(formatRelativeTime(undefined, NOW)).toBe('')
  })
})
