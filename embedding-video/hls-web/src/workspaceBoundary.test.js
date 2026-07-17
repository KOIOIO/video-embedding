import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, test } from 'vitest'

const here = dirname(fileURLToPath(import.meta.url))
const workspaceVue = readFileSync(resolve(here, 'workspaces/VideoWorkspace.vue'), 'utf8')

describe('video workspace boundary', () => {
  test('keeps the workspace shell and polling cleanup in the extracted component', () => {
    expect(workspaceVue).toContain('class="admin-shell video-workspace"')
    expect(workspaceVue).toContain('onBeforeUnmount')
    expect(workspaceVue).toContain('stopPolling')
    expect(workspaceVue).toContain('stopArchiveProgressPolling')
    expect(workspaceVue).toContain('stopSystemMetricsPolling')
  })

  test('does not restart upload polling after the workspace is unmounted', () => {
    const uploadSuccess = workspaceVue.match(
      /function handleUploadSuccess\(data\) \{([\s\S]*?)\n\}/,
    )?.[1] ?? ''
    const mountedHook = workspaceVue.match(/onMounted\(\(\) => \{([\s\S]*?)\n\}\)/)?.[1] ?? ''
    const unmountedHook = workspaceVue.match(
      /onBeforeUnmount\(\(\) => \{([\s\S]*?)\n\}\)/,
    )?.[1] ?? ''

    expect(mountedHook).toContain('isWorkspaceMounted = true')
    expect(unmountedHook).toContain('isWorkspaceMounted = false')
    expect(unmountedHook.indexOf('isWorkspaceMounted = false')).toBeLessThan(
      unmountedHook.indexOf('stopPolling()'),
    )
    expect(uploadSuccess.trimStart()).toMatch(/^if \(!isWorkspaceMounted\) return/)
  })

  test('keeps the recommendation workspace free of login state', () => {
    const recommendationWorkspaceVue = readFileSync(
      resolve(here, 'workspaces/RecommendationWorkspace.vue'),
      'utf8',
    )

    expect(recommendationWorkspaceVue).toContain('class="console-shell recommendation-workspace"')
    expect(recommendationWorkspaceVue).not.toContain('isAuthenticated')
    expect(recommendationWorkspaceVue).not.toContain('submitLogin')
    expect(recommendationWorkspaceVue).not.toContain('class="login-shell"')
    expect(recommendationWorkspaceVue).not.toContain('@click="logout"')
    expect(recommendationWorkspaceVue).not.toContain('window.localStorage')
    expect(recommendationWorkspaceVue).not.toContain('globalThis.localStorage')
  })
})
