import { describe, expect, it } from 'vitest'
import { clampIndex, pushDistinct, resolveSwipeIndex, visibleWindow } from './swipe.js'

describe('swipe helpers', () => {
  it('clamps indexes into range', () => {
    expect(clampIndex(-2, 4)).toBe(0)
    expect(clampIndex(2, 4)).toBe(2)
    expect(clampIndex(9, 4)).toBe(4)
    expect(clampIndex(0, 0)).toBe(0)
  })

  it('resolves swipe direction only above the threshold', () => {
    expect(resolveSwipeIndex(20, 1, 5)).toBe(1)
    expect(resolveSwipeIndex(-20, 1, 5)).toBe(1)
    expect(resolveSwipeIndex(120, 1, 5)).toBe(2)
    expect(resolveSwipeIndex(-120, 1, 5)).toBe(0)
    expect(resolveSwipeIndex(-120, 0, 5)).toBe(0)
  })

  it('returns a three-item visible window', () => {
    expect(visibleWindow(0, 5)).toEqual([0, 1])
    expect(visibleWindow(2, 5)).toEqual([1, 3])
    expect(visibleWindow(4, 5)).toEqual([3, 4])
    expect(visibleWindow(0, 0)).toEqual([0, -1])
  })

  it('caps the seen-key set size', () => {
    const seen = new Set(['k1'])
    for (let i = 2; i <= 40; i++) pushDistinct(seen, `k${i}`, 10)
    expect(seen.size).toBe(10)
    expect(seen.has('k31')).toBe(true)
    expect(seen.has('k1')).toBe(false)
  })
})
