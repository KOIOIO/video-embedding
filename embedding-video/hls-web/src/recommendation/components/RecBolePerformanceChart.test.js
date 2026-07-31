import { existsSync, readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const here = dirname(fileURLToPath(import.meta.url))
const componentPath = resolve(here, 'RecBolePerformanceChart.vue')
const workspacePath = resolve(here, '../../workspaces/RecommendationWorkspace.vue')

describe('RecBolePerformanceChart component contract', () => {
  it('provides RecBole metrics and stable request states', () => {
    expect(existsSync(componentPath)).toBe(true)
    const source = readFileSync(componentPath, 'utf8')
    expect(source).toContain('aria-label="RecBole 推荐性能趋势"')
    expect(source).toContain("const metric = ref('NDCG@20')")
    expect(source).toContain("'召回率（Recall@20）'")
    expect(source).toContain("'排序质量（NDCG@20）'")
    expect(source).toContain("'命中率（Hit@20）'")
    expect(source).toContain("'准确率（Precision@20）'")
    expect(source).toContain('衡量前 20 条推荐覆盖用户感兴趣内容的程度，越高越好。')
    expect(source).toContain('衡量前 20 条推荐的相关性和排序位置，相关内容越靠前得分越高。')
    expect(source).toContain('衡量前 20 条推荐中至少命中一条用户感兴趣内容的比例，越高越好。')
    expect(source).toContain('衡量前 20 条推荐中用户感兴趣内容所占的比例，越高越好。')
    expect(source).toContain('class="recbole-metric-description"')
    expect(source).toContain('tooltip.point.modelVersion')
    expect(source).toContain('role="status"')
    expect(source).toContain('role="alert"')
    expect(source).toContain('viewBox="0 0 720 280"')
  })

  it('renders before existing business metrics', () => {
    const source = readFileSync(workspacePath, 'utf8')
    expect(source).toContain("import RecBolePerformanceChart from '../recommendation/components/RecBolePerformanceChart.vue'")
    const effectsSection = source.indexOf("activeSection === 'effects'")
    const chart = source.indexOf('<RecBolePerformanceChart', effectsSection)
    const businessMetrics = source.indexOf('aria-label="命中效果状态"', effectsSection)
    expect(chart).toBeGreaterThan(effectsSection)
    expect(businessMetrics).toBeGreaterThan(chart)
  })
})
