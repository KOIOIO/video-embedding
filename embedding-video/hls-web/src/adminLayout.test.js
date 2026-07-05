import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, test } from 'vitest'

const here = dirname(fileURLToPath(import.meta.url))
const appVue = readFileSync(resolve(here, 'App.vue'), 'utf8')
const styles = readFileSync(resolve(here, 'style.css'), 'utf8')
const playerVue = readFileSync(resolve(here, 'components/HlsPlayer.vue'), 'utf8')

describe('admin layout guardrails', () => {
  test('management panels are placed on full-width dashboard rows', () => {
    expect(appVue).toContain('class="panel wide-panel video-management-panel"')
    expect(appVue).toContain('class="panel wide-panel question-panel"')
    expect(appVue).toContain('class="panel wide-panel free-query-panel"')
    expect(styles).toMatch(/\.dashboard-grid\s*{[\s\S]*grid-template-areas:\s*"upload playback \."[\s\S]*"videos videos videos"[\s\S]*"questions questions questions"[\s\S]*"free-query free-query free-query"/)
    expect(styles).not.toContain('.workflow-column .video-list')
  })

  test('similar video drawer does not resize the player grid', () => {
    expect(appVue).toContain('class="player-stage"')
    expect(styles).toMatch(/\.similar-drawer\s*{[\s\S]*position:\s*absolute/)
    expect(styles).not.toMatch(/\.player-panel\.drawer-open\s+\.player-workspace\s*{[\s\S]*grid-template-columns/)
  })

  test('player exposes a visible progress control outside native video chrome', () => {
    expect(playerVue).toContain('native-control-strip')
    expect(playerVue).toContain('seg-range native-range')
  })

  test('video resource library scrolls inside a fixed-height file rail', () => {
    expect(appVue).toContain('class="video-list-scroll"')
    expect(styles).toMatch(/\.video-list-scroll\s*{[\s\S]*max-height:\s*clamp\(/)
    expect(styles).toMatch(/\.video-list-scroll\s*{[\s\S]*overflow-y:\s*auto/)
    expect(styles).toMatch(/\.video-list-scroll\s*{[\s\S]*scrollbar-gutter:\s*stable/)
    expect(styles).toContain('.video-list-scroll::-webkit-scrollbar-thumb')
  })
})
