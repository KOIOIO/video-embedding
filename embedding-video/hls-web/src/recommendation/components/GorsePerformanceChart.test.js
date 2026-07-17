import { existsSync, readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const here = dirname(fileURLToPath(import.meta.url))
const componentPath = resolve(here, 'GorsePerformanceChart.vue')
const workspacePath = resolve(here, '../../workspaces/RecommendationWorkspace.vue')

describe('GorsePerformanceChart component contract', () => {
  it('provides accessible filters and stable request states', () => {
    expect(existsSync(componentPath)).toBe(true)
    const source = readFileSync(componentPath, 'utf8')

    expect(source).toContain('aria-label="Gorse 推荐性能趋势"')
    expect(source).toContain('type="date"')
    expect(source).toContain('<select')
    expect(source).toContain('role="status"')
    expect(source).toContain('role="alert"')
    expect(source).toContain('@click="loadPerformance"')
    expect(source).toContain('viewBox="0 0 720 280"')
  })

  it('renders before the existing business metrics in the effects section', () => {
    const source = readFileSync(workspacePath, 'utf8')
    expect(source).toContain("import GorsePerformanceChart from '../recommendation/components/GorsePerformanceChart.vue'")
    const effectsSection = source.indexOf("activeSection === 'effects'")
    const chart = source.indexOf('<GorsePerformanceChart', effectsSection)
    const businessMetrics = source.indexOf('aria-label="命中效果状态"', effectsSection)

    expect(effectsSection).toBeGreaterThanOrEqual(0)
    expect(chart).toBeGreaterThan(effectsSection)
    expect(businessMetrics).toBeGreaterThan(chart)
  })
})
