import { existsSync, readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, test } from 'vitest'

const here = dirname(fileURLToPath(import.meta.url))
const appVue = readFileSync(resolve(here, 'App.vue'), 'utf8')
const mainJs = readFileSync(resolve(here, 'main.js'), 'utf8')
const videoCss = readFileSync(resolve(here, 'style.css'), 'utf8')
const recommendationWorkspaceVue = readFileSync(
  resolve(here, 'workspaces/RecommendationWorkspace.vue'),
  'utf8',
)
const appShellCssPath = resolve(here, 'appShell.css')
const recommendationCssPath = resolve(here, 'recommendation/recommendation.css')

describe('shared application shell', () => {
  test('uses one login gate for the whole console', () => {
    expect(appVue).toContain('v-if="!isUIUnlocked"')
    expect(appVue).toContain('@submit.prevent="submitLogin"')
    expect(appVue.match(/@submit\.prevent="submitLogin"/g)).toHaveLength(1)
    expect(appVue).toContain('账号或密码不正确')
  })

  test('switches between mutually exclusive workspaces from the shared toolbar', () => {
    expect(appVue).toContain("import VideoWorkspace from './workspaces/VideoWorkspace.vue'")
    expect(appVue).toContain("import RecommendationWorkspace from './workspaces/RecommendationWorkspace.vue'")
    expect(appVue).toContain('<VideoWorkspace v-if="activeWorkspace === \'video\'" />')
    expect(appVue).toContain('<RecommendationWorkspace v-else-if="activeWorkspace === \'recommendation\'" />')
    expect(appVue).toContain('视频调试台')
    expect(appVue).toContain('推荐控制台')
    expect(appVue).toContain('@click="selectWorkspace(\'video\')"')
    expect(appVue).toContain('@click="selectWorkspace(\'recommendation\')"')
    expect(appVue).toContain('@click="logout"')
    expect(appVue).toContain('CONSOLE_ADMIN_USERNAME')
  })

  test('keeps browser storage access behind safe session helpers', () => {
    expect(appVue).toContain('clearLegacyAuthenticated()')
    expect(appVue).toContain('readUIUnlocked()')
    expect(appVue).toContain('readActiveWorkspace()')
    expect(appVue).not.toContain('window.localStorage')
    expect(appVue).not.toContain('globalThis.localStorage')

    const selectWorkspace = appVue.match(
      /function selectWorkspace\(workspace\) \{([\s\S]*?)\n\}/,
    )?.[1] ?? ''
    expect(selectWorkspace).toContain('if (!isKnownWorkspace(workspace)) return')
    expect(selectWorkspace).not.toContain('if (!writeActiveWorkspace')

    const persistIndex = selectWorkspace.indexOf('writeActiveWorkspace(workspace)')
    const updateIndex = selectWorkspace.indexOf('activeWorkspace.value = workspace')
    expect(persistIndex).toBeGreaterThanOrEqual(0)
    expect(updateIndex).toBeGreaterThan(persistIndex)
  })

  test('loads shared shell styles from the application entrypoint', () => {
    expect(mainJs).toContain("import './appShell.css'")
    expect(appVue).not.toMatch(/<style\b/)
    expect(existsSync(appShellCssPath)).toBe(true)
  })

  test('scopes recommendation styles to the recommendation workspace', () => {
    expect(recommendationWorkspaceVue).toContain(
      '<style scoped src="../recommendation/recommendation.css"></style>',
    )
    expect(existsSync(recommendationCssPath)).toBe(true)

    const recommendationCss = readFileSync(recommendationCssPath, 'utf8')
    expect(recommendationCss).toMatch(/^\.recommendation-workspace\s*\{/m)
    expect(recommendationCss).not.toMatch(/^\s*(?::root|body)\s*\{/m)
    expect(recommendationCss).not.toMatch(/\.login-(?:shell|panel|brand|form)\b/)
    expect(recommendationCss).toContain('.preview-table-wrap :deep(.preview-table)')
    expect(recommendationCss).toContain(':deep(.strategy-chip)')
    expect(recommendationCss).toContain(':deep(.empty-state)')
    expect(recommendationCss).toContain(':deep(a)')
  })

  test('namespaces shared class selectors to the video workspace', () => {
    const conflictClasses = [
      'brand-block',
      'brand-mark',
      'eyebrow',
      'metric-card',
      'empty-state',
    ]
    const conflictSelectorGroups = [...videoCss.matchAll(/([^{}]+)\{/g)]
      .map((match) => match[1].trim())
      .filter((selectors) =>
        conflictClasses.some((className) => selectors.includes(`.${className}`)),
      )

    expect(conflictSelectorGroups.length).toBeGreaterThan(0)
    for (const selectorGroup of conflictSelectorGroups) {
      for (const selector of selectorGroup.split(',')) {
        expect(selector.trim()).toMatch(/^\.video-workspace\s/)
      }
    }
  })
})
