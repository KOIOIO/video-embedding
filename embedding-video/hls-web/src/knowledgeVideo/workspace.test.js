import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const here = dirname(fileURLToPath(import.meta.url))
const app = readFileSync(resolve(here, '../App.vue'), 'utf8')
const workspacePath = resolve(here, '../workspaces/KnowledgeVideoWorkspace.vue')
const playerPath = resolve(here, '../components/HlsPlayer.vue')
const cssPath = resolve(here, 'knowledgeVideo.css')

describe('knowledge video workspace', () => {
  it('is available from the application workspace switcher', () => {
    expect(app).toContain("selectWorkspace('knowledge-video')")
    expect(app).toContain('KnowledgeVideoWorkspace')
  })

  it('contains upload, progress, tree filters, and playback controls', () => {
    const source = readFileSync(workspacePath, 'utf8')
    expect(source.match(/type="file"/g)).toHaveLength(2)
    expect(source).toContain('uploadPercent')
    expect(source).toContain('batchProgress')
    expect(source).toContain('statusFilter')
    expect(source).toContain('tree-node')
    expect(source).toContain('<HlsPlayer')
  })

  it('renders every ready video and records each player on its first play', () => {
    const source = readFileSync(workspacePath, 'utf8')
    const player = readFileSync(playerPath, 'utf8')
    expect(source).toContain('v-for="video in playback.videos"')
    expect(source).toContain(':title="video.display_name')
    expect(source).toContain('@play="recordPlayback(video.knowledge_video_id)"')
    expect(source).toContain('recordedVideoIds')
    expect(source).toContain('recordKnowledgePlayback')
    expect(player).toContain("defineEmits(['watch-progress', 'play'])")
    expect(player).toContain("emit('play')")
  })

  it('collapses the upload-first layout on narrow screens', () => {
    const css = readFileSync(cssPath, 'utf8')
    expect(css).toMatch(/@media\s*\(max-width:\s*900px\)/)
    expect(css).toMatch(/grid-template-columns:\s*1fr/)
  })
})
