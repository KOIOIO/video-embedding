# HLS Web 与推荐控制台合并 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `recommendation-console/` 完整迁入 `hls-web/`，提供统一固定账号登录和顶部双工作区切换，并在验证后删除旧前端目录。

**Architecture:** `hls-web/src/App.vue` 变为只负责 UI 解锁状态和工作区切换的应用壳；原视频页面与推荐页面分别迁入互斥挂载的 `VideoWorkspace.vue` 和 `RecommendationWorkspace.vue`。现有业务 API 和大组件内部逻辑保持不变，推荐样式使用 Vue scoped 外部样式隔离，纯函数 helper 负责可测试的 `localStorage` 状态。

**Tech Stack:** Vue 3 Composition API、Vite 8、Vitest 4、hls.js、原生 Fetch/XHR、CSS

---

## 文件结构

- Create: `hls-web/src/config/consoleSession.js`：固定凭据、UI 解锁状态、当前工作区和旧键清理。
- Create: `hls-web/src/config/consoleSession.test.js`：统一会话 helper 的 TDD 覆盖。
- Create: `hls-web/src/workspaces/VideoWorkspace.vue`：从现有 `hls-web/src/App.vue` 移出的完整视频工作台。
- Create: `hls-web/src/workspaces/RecommendationWorkspace.vue`：从推荐控制台移出的业务页面，不含登录。
- Create: `hls-web/src/recommendation/config/sectionSession.js`：推荐子栏目恢复和校验。
- Create: `hls-web/src/recommendation/config/sectionSession.test.js`：推荐子栏目状态测试。
- Create: `hls-web/src/recommendation/api/http.js`：迁移推荐请求 helper。
- Create: `hls-web/src/recommendation/api/recommendationConsole.js`：迁移推荐 API helper。
- Create: `hls-web/src/recommendation/api/recommendationConsole.test.js`：迁移推荐 API 契约测试。
- Create: `hls-web/src/recommendation/components/PreviewTable.vue`：迁移推荐预览表。
- Create: `hls-web/src/recommendation/config/navigation.js`：迁移推荐内部导航。
- Create: `hls-web/src/recommendation/recommendation.css`：推荐工作区 scoped 外部样式。
- Create: `hls-web/src/appShell.css`：统一登录与顶部工作区切换样式。
- Create: `hls-web/src/appShell.test.js`：应用壳和两个工作区的静态结构契约。
- Modify: `hls-web/src/App.vue`：替换为统一应用壳。
- Modify: `hls-web/src/main.js`：加载应用壳样式。
- Modify: `hls-web/src/adminLayout.test.js`：读取迁移后的 `VideoWorkspace.vue`。
- Modify: `hls-web/src/style.css`：仅在必要处收紧视频工作区选择器。
- Modify: `hls-web/README.md`、`README.md`、`docs/README.md`：统一项目与安全边界说明。
- Delete: `recommendation-console/`：全部迁移和验证完成后删除。

任何提交都不得暂存以下用户已有修改：

```text
video-service/internal/infrastructure/persistence/gorm_video_repository.go
video-service/internal/infrastructure/persistence/gorm_video_repository_test.go
```

### Task 1: 记录两个旧前端的可执行基线

**Files:**
- Verify only: `hls-web/`
- Verify only: `recommendation-console/`

- [ ] **Step 1: 运行视频前端测试**

Run:

```bash
cd hls-web && npm test
```

Expected: `6 passed` test files and `25 passed` tests.

- [ ] **Step 2: 运行推荐前端测试**

Run:

```bash
cd recommendation-console && npm test
```

Expected: `2 passed` test files and `17 passed` tests.

- [ ] **Step 3: 运行两个生产构建**

Run:

```bash
cd hls-web && npm run build
cd recommendation-console && npm run build
```

Expected: both commands exit 0 and write Vite output under each ignored `dist/` directory. The existing `hls-web` bundle may emit its known chunk-size warning; no new compile or CSS errors are allowed.

- [ ] **Step 4: 确认工作区中只有既有 Go 修改**

Run:

```bash
git status --short
```

Expected: only the two known Go files are modified before implementation begins.

### Task 2: 用 TDD 建立统一 UI 会话 helper

**Files:**
- Create: `hls-web/src/config/consoleSession.test.js`
- Create: `hls-web/src/config/consoleSession.js`

- [ ] **Step 1: 写失败测试**

Create `hls-web/src/config/consoleSession.test.js`:

```js
import { describe, expect, it } from 'vitest'
import {
  CONSOLE_ACTIVE_WORKSPACE_STORAGE_KEY,
  CONSOLE_ADMIN_PASSWORD,
  CONSOLE_ADMIN_USERNAME,
  CONSOLE_AUTH_STORAGE_KEY,
  DEFAULT_WORKSPACE,
  LEGACY_CONSOLE_AUTH_STORAGE_KEY,
  clearLegacyAuthenticated,
  isKnownWorkspace,
  isValidConsoleLogin,
  readActiveWorkspace,
  readUIUnlocked,
  writeActiveWorkspace,
  writeUIUnlocked,
} from './consoleSession.js'

function createStorage(initial = {}) {
  const data = new Map(Object.entries(initial))
  return {
    getItem(key) {
      return data.has(key) ? data.get(key) : null
    },
    setItem(key, value) {
      data.set(key, String(value))
    },
    removeItem(key) {
      data.delete(key)
    },
  }
}

const throwingStorage = {
  getItem() { throw new Error('blocked') },
  setItem() { throw new Error('blocked') },
  removeItem() { throw new Error('blocked') },
}

describe('console session', () => {
  it('accepts only the configured fixed account', () => {
    expect(isValidConsoleLogin(CONSOLE_ADMIN_USERNAME, CONSOLE_ADMIN_PASSWORD)).toBe(true)
    expect(isValidConsoleLogin('admin', CONSOLE_ADMIN_PASSWORD)).toBe(false)
    expect(isValidConsoleLogin(CONSOLE_ADMIN_USERNAME, 'wrong')).toBe(false)
  })

  it('reads only the exact UI unlock marker', () => {
    expect(readUIUnlocked(createStorage({ [CONSOLE_AUTH_STORAGE_KEY]: 'true' }))).toBe(true)
    expect(readUIUnlocked(createStorage({ [CONSOLE_AUTH_STORAGE_KEY]: 'false' }))).toBe(false)
    expect(readUIUnlocked(createStorage())).toBe(false)
  })

  it('writes and clears the UI unlock marker', () => {
    const storage = createStorage()
    expect(writeUIUnlocked(storage, true)).toBe(true)
    expect(storage.getItem(CONSOLE_AUTH_STORAGE_KEY)).toBe('true')
    expect(writeUIUnlocked(storage, false)).toBe(true)
    expect(storage.getItem(CONSOLE_AUTH_STORAGE_KEY)).toBe(null)
  })

  it('restores only known workspaces and falls back to video', () => {
    expect(isKnownWorkspace('video')).toBe(true)
    expect(isKnownWorkspace('recommendation')).toBe(true)
    expect(isKnownWorkspace('unknown')).toBe(false)
    expect(readActiveWorkspace(createStorage({ [CONSOLE_ACTIVE_WORKSPACE_STORAGE_KEY]: 'recommendation' }))).toBe('recommendation')
    expect(readActiveWorkspace(createStorage({ [CONSOLE_ACTIVE_WORKSPACE_STORAGE_KEY]: 'unknown' }))).toBe(DEFAULT_WORKSPACE)
    expect(readActiveWorkspace(createStorage())).toBe('video')
  })

  it('persists only known workspaces', () => {
    const storage = createStorage()
    expect(writeActiveWorkspace(storage, 'recommendation')).toBe(true)
    expect(writeActiveWorkspace(storage, 'unknown')).toBe(false)
    expect(storage.getItem(CONSOLE_ACTIVE_WORKSPACE_STORAGE_KEY)).toBe('recommendation')
  })

  it('removes the legacy recommendation unlock marker', () => {
    const storage = createStorage({ [LEGACY_CONSOLE_AUTH_STORAGE_KEY]: 'true' })
    expect(clearLegacyAuthenticated(storage)).toBe(true)
    expect(storage.getItem(LEGACY_CONSOLE_AUTH_STORAGE_KEY)).toBe(null)
  })

  it('fails closed when browser storage is unavailable', () => {
    expect(readUIUnlocked(throwingStorage)).toBe(false)
    expect(readActiveWorkspace(throwingStorage)).toBe(DEFAULT_WORKSPACE)
    expect(writeUIUnlocked(throwingStorage, true)).toBe(false)
    expect(writeActiveWorkspace(throwingStorage, 'video')).toBe(false)
    expect(clearLegacyAuthenticated(throwingStorage)).toBe(false)
  })
})
```

- [ ] **Step 2: 验证测试因实现缺失而失败**

Run:

```bash
cd hls-web && npm test -- src/config/consoleSession.test.js
```

Expected: FAIL because `./consoleSession.js` does not exist.

- [ ] **Step 3: 写最小实现**

Create `hls-web/src/config/consoleSession.js`:

```js
export const CONSOLE_ADMIN_USERNAME = 'aaddmmiinn'
export const CONSOLE_ADMIN_PASSWORD = 'admin123'
export const CONSOLE_AUTH_STORAGE_KEY = 'video_app-console.authenticated'
export const CONSOLE_ACTIVE_WORKSPACE_STORAGE_KEY = 'video_app-console.active-workspace'
export const LEGACY_CONSOLE_AUTH_STORAGE_KEY = 'video_app-recommendation-console.authenticated'
export const DEFAULT_WORKSPACE = 'video'

const KNOWN_WORKSPACES = new Set(['video', 'recommendation'])

export function isKnownWorkspace(workspace) {
  return KNOWN_WORKSPACES.has(workspace)
}

export function isValidConsoleLogin(username, password) {
  return String(username || '').trim() === CONSOLE_ADMIN_USERNAME
    && String(password || '') === CONSOLE_ADMIN_PASSWORD
}

export function readUIUnlocked(storage) {
  return safeGetItem(storage, CONSOLE_AUTH_STORAGE_KEY) === 'true'
}

export function writeUIUnlocked(storage, unlocked) {
  return unlocked
    ? safeSetItem(storage, CONSOLE_AUTH_STORAGE_KEY, 'true')
    : safeRemoveItem(storage, CONSOLE_AUTH_STORAGE_KEY)
}

export function readActiveWorkspace(storage) {
  const stored = safeGetItem(storage, CONSOLE_ACTIVE_WORKSPACE_STORAGE_KEY)
  return isKnownWorkspace(stored) ? stored : DEFAULT_WORKSPACE
}

export function writeActiveWorkspace(storage, workspace) {
  if (!isKnownWorkspace(workspace)) return false
  return safeSetItem(storage, CONSOLE_ACTIVE_WORKSPACE_STORAGE_KEY, workspace)
}

export function clearLegacyAuthenticated(storage) {
  return safeRemoveItem(storage, LEGACY_CONSOLE_AUTH_STORAGE_KEY)
}

function safeGetItem(storage, key) {
  try {
    return storage?.getItem(key) || null
  } catch {
    return null
  }
}

function safeSetItem(storage, key, value) {
  try {
    storage?.setItem(key, value)
    return true
  } catch {
    return false
  }
}

function safeRemoveItem(storage, key) {
  try {
    storage?.removeItem(key)
    return true
  } catch {
    return false
  }
}
```

- [ ] **Step 4: 验证 helper 测试通过**

Run:

```bash
cd hls-web && npm test -- src/config/consoleSession.test.js
```

Expected: 7 tests pass.

- [ ] **Step 5: 提交统一会话 helper**

```bash
git add hls-web/src/config/consoleSession.js hls-web/src/config/consoleSession.test.js
git commit -m "feat(frontend): add shared console session state"
```

### Task 3: 迁移推荐 API 与子栏目状态

**Files:**
- Create: `hls-web/src/recommendation/api/http.js`
- Create: `hls-web/src/recommendation/api/recommendationConsole.js`
- Create: `hls-web/src/recommendation/api/recommendationConsole.test.js`
- Create: `hls-web/src/recommendation/components/PreviewTable.vue`
- Create: `hls-web/src/recommendation/config/navigation.js`
- Create: `hls-web/src/recommendation/config/sectionSession.js`
- Create: `hls-web/src/recommendation/config/sectionSession.test.js`

- [ ] **Step 1: 先迁移 API 测试并验证缺少实现**

Use file moves so Git preserves history:

```bash
mkdir -p hls-web/src/recommendation/api
git mv recommendation-console/src/api/recommendationConsole.test.js hls-web/src/recommendation/api/recommendationConsole.test.js
```

Run:

```bash
cd hls-web && npm test -- src/recommendation/api/recommendationConsole.test.js
```

Expected: FAIL because `./recommendationConsole.js` is absent.

- [ ] **Step 2: 迁移 API 实现与预览组件**

```bash
mkdir -p hls-web/src/recommendation/components hls-web/src/recommendation/config
git mv recommendation-console/src/api/http.js hls-web/src/recommendation/api/http.js
git mv recommendation-console/src/api/recommendationConsole.js hls-web/src/recommendation/api/recommendationConsole.js
git mv recommendation-console/src/components/PreviewTable.vue hls-web/src/recommendation/components/PreviewTable.vue
git mv recommendation-console/src/config/navigation.js hls-web/src/recommendation/config/navigation.js
```

No code changes are required in the moved API files because their relative imports remain within `api/`.

- [ ] **Step 3: 写推荐子栏目失败测试**

Create `hls-web/src/recommendation/config/sectionSession.test.js`:

```js
import { describe, expect, it } from 'vitest'
import {
  RECOMMENDATION_SECTION_STORAGE_KEY,
  isKnownSection,
  readActiveSection,
  writeActiveSection,
} from './sectionSession.js'

const sections = [{ key: 'diagnostics' }, { key: 'effects' }, { key: 'redis' }]

function createStorage(initial = {}) {
  const data = new Map(Object.entries(initial))
  return {
    getItem: (key) => data.has(key) ? data.get(key) : null,
    setItem: (key, value) => data.set(key, String(value)),
  }
}

describe('recommendation section session', () => {
  it('recognizes only configured sections', () => {
    expect(isKnownSection('effects', sections)).toBe(true)
    expect(isKnownSection('removed', sections)).toBe(false)
  })

  it('restores a known section and falls back for unknown values', () => {
    expect(readActiveSection(createStorage({ [RECOMMENDATION_SECTION_STORAGE_KEY]: 'effects' }), 'diagnostics', sections)).toBe('effects')
    expect(readActiveSection(createStorage({ [RECOMMENDATION_SECTION_STORAGE_KEY]: 'removed' }), 'diagnostics', sections)).toBe('diagnostics')
  })

  it('persists only a known section', () => {
    const storage = createStorage()
    expect(writeActiveSection(storage, 'redis', sections)).toBe(true)
    expect(writeActiveSection(storage, 'removed', sections)).toBe(false)
    expect(storage.getItem(RECOMMENDATION_SECTION_STORAGE_KEY)).toBe('redis')
  })
})
```

- [ ] **Step 4: 验证推荐子栏目测试失败**

Run:

```bash
cd hls-web && npm test -- src/recommendation/config/sectionSession.test.js
```

Expected: FAIL because `./sectionSession.js` does not exist.

- [ ] **Step 5: 实现推荐子栏目存储**

Create `hls-web/src/recommendation/config/sectionSession.js`:

```js
export const RECOMMENDATION_SECTION_STORAGE_KEY = 'video_app-recommendation-console.active-section'

export function isKnownSection(section, navigationItems) {
  return Boolean(section && navigationItems.some((item) => item.key === section))
}

export function readActiveSection(storage, fallbackSection, navigationItems) {
  const stored = safeGetItem(storage, RECOMMENDATION_SECTION_STORAGE_KEY)
  return isKnownSection(stored, navigationItems) ? stored : fallbackSection
}

export function writeActiveSection(storage, section, navigationItems) {
  if (!isKnownSection(section, navigationItems)) return false
  return safeSetItem(storage, RECOMMENDATION_SECTION_STORAGE_KEY, section)
}

function safeGetItem(storage, key) {
  try {
    return storage?.getItem(key) || null
  } catch {
    return null
  }
}

function safeSetItem(storage, key, value) {
  try {
    storage?.setItem(key, value)
    return true
  } catch {
    return false
  }
}
```

- [ ] **Step 6: 运行迁移后的推荐测试**

Run:

```bash
cd hls-web && npm test -- src/recommendation/api/recommendationConsole.test.js src/recommendation/config/sectionSession.test.js
```

Expected: 15 tests pass: 12 API tests and 3 section tests.

- [ ] **Step 7: 提交推荐基础模块**

```bash
git add hls-web/src/recommendation recommendation-console/src/api recommendation-console/src/components recommendation-console/src/config/navigation.js
git commit -m "refactor(frontend): move recommendation helpers into hls web"
```

### Task 4: 提取视频工作区并保持构建可用

**Files:**
- Create: `hls-web/src/workspaces/VideoWorkspace.vue`
- Create: `hls-web/src/App.vue`
- Create: `hls-web/src/workspaceBoundary.test.js`
- Modify: `hls-web/src/adminLayout.test.js`

- [ ] **Step 1: 移动原视频 App 并修正相对导入**

```bash
mkdir -p hls-web/src/workspaces
git mv hls-web/src/App.vue hls-web/src/workspaces/VideoWorkspace.vue
```

In `VideoWorkspace.vue`, replace the six source-relative imports with these parent-relative imports:

```js
import { clearSavedArchiveUpload, loadSavedArchiveUpload, saveArchiveUpload } from '../archiveProgressStorage.js'
import HlsPlayer from '../components/HlsPlayer.vue'
import { uploadVideoInChunks } from '../chunkedUpload.js'
import { fetchRandomPlayableSegment } from '../randomSegment.js'
import { normalizeSegmentReactionCounts, segmentIdOf, submitSegmentReaction as submitSegmentReactionRequest } from '../segmentReaction.js'
import { buildWatchContext, reportWatchProgress as submitWatchProgress } from '../watchProgress.js'
```

- [ ] **Step 2: 写工作区边界测试并更新布局测试路径**

Create `hls-web/src/workspaceBoundary.test.js`:

```js
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, test } from 'vitest'

const here = dirname(fileURLToPath(import.meta.url))

describe('workspace boundaries', () => {
  test('video workspace owns the original admin shell', () => {
    const source = readFileSync(resolve(here, 'workspaces/VideoWorkspace.vue'), 'utf8')
    expect(source).toContain('class="admin-shell video-workspace"')
    expect(source).toContain('onBeforeUnmount(() =>')
    expect(source).toContain('stopPolling()')
    expect(source).toContain('stopArchiveProgressPolling()')
    expect(source).toContain('stopSystemMetricsPolling()')
  })
})
```

In `hls-web/src/adminLayout.test.js`, replace:

```js
const appVue = readFileSync(resolve(here, 'App.vue'), 'utf8')
```

with:

```js
const appVue = readFileSync(resolve(here, 'workspaces/VideoWorkspace.vue'), 'utf8')
```

- [ ] **Step 3: 验证视频工作区边界测试失败**

Run:

```bash
cd hls-web && npm test -- src/workspaceBoundary.test.js
```

Expected: FAIL because the moved root still has only `class="admin-shell"`.

- [ ] **Step 4: 添加工作区根类并创建临时最小应用入口**

In `VideoWorkspace.vue`, add `video-workspace` to the existing root element:

```vue
<div class="admin-shell video-workspace">
```

Create `hls-web/src/App.vue`:

```vue
<script setup>
import VideoWorkspace from './workspaces/VideoWorkspace.vue'
</script>

<template>
  <VideoWorkspace />
</template>
```

- [ ] **Step 5: 验证视频测试和构建**

Run:

```bash
cd hls-web && npm test
cd hls-web && npm run build
```

Expected: all current `hls-web` tests pass and Vite build exits 0.

- [ ] **Step 6: 提交视频工作区提取**

```bash
git add hls-web/src/App.vue hls-web/src/workspaces/VideoWorkspace.vue hls-web/src/adminLayout.test.js hls-web/src/workspaceBoundary.test.js
git commit -m "refactor(frontend): extract video workspace"
```

### Task 5: 迁移不含登录的推荐工作区

**Files:**
- Create: `hls-web/src/workspaces/RecommendationWorkspace.vue`
- Modify: `hls-web/src/workspaceBoundary.test.js`
- Delete later: `recommendation-console/src/App.vue`

- [ ] **Step 1: 先扩展失败的工作区边界测试**

Add this test before the final closing `})` of the existing `describe('workspace boundaries')` block:

```js
test('recommendation workspace excludes its old login shell', () => {
  const source = readFileSync(resolve(here, 'workspaces/RecommendationWorkspace.vue'), 'utf8')
  expect(source).toContain('class="console-shell recommendation-workspace"')
  expect(source).not.toContain('isAuthenticated')
  expect(source).not.toContain('submitLogin')
  expect(source).not.toContain('class="login-shell"')
  expect(source).not.toContain('@click="logout"')
})
```

- [ ] **Step 2: 验证测试因推荐工作区缺失而失败**

Run:

```bash
cd hls-web && npm test -- src/workspaceBoundary.test.js
```

Expected: FAIL with `ENOENT` for `workspaces/RecommendationWorkspace.vue`.

- [ ] **Step 3: 移动推荐页面并更新导入**

```bash
git mv recommendation-console/src/App.vue hls-web/src/workspaces/RecommendationWorkspace.vue
```

Replace the recommendation imports at the top with:

```js
import { computed, onMounted, reactive, ref } from 'vue'
import {
  fetchRecommendationDiagnostics,
  fetchRecommendationDatasources,
  fetchRecommendationEffects,
  fetchRecommendationOverview,
  fetchRecommendationRedisState,
  previewByQuestion,
  previewRandomPlay,
  traceByQuestion,
  traceRandomPlay,
} from '../recommendation/api/recommendationConsole.js'
import PreviewTable from '../recommendation/components/PreviewTable.vue'
import {
  isKnownSection,
  readActiveSection,
  writeActiveSection,
} from '../recommendation/config/sectionSession.js'
import { navigationItems, panelRows } from '../recommendation/config/navigation.js'
```

Delete the old login-only state:

```js
const isAuthenticated = ref(readAuthenticated(browserStorage))
const loginError = ref('')
const loginForm = reactive({
  username: '',
  password: '',
})
```

Replace the mounted hook with unconditional workspace loading:

```js
onMounted(() => {
  loadAll()
})
```

Delete the complete `submitLogin()` and `logout()` functions. Keep `selectSection()` and all recommendation data functions unchanged.

- [ ] **Step 4: 删除旧登录模板并保留业务壳**

Delete the complete old login `<main>` block that starts with `<main v-if="!isAuthenticated" class="login-shell">` and ends immediately before the business console `<main>`. Change the business console main element to:

```vue
<main class="console-shell recommendation-workspace">
```

Delete only the old logout button from `.header-actions`:

```vue
<button class="control-button secondary-button" type="button" @click="logout">
  退出
</button>
```

Keep the refresh button and all seven `activeSection` panels unchanged.

- [ ] **Step 5: 验证推荐工作区边界和构建**

Run:

```bash
cd hls-web && npm test -- src/workspaceBoundary.test.js src/recommendation/api/recommendationConsole.test.js src/recommendation/config/sectionSession.test.js
cd hls-web && npm run build
```

Expected: workspace tests and 15 migrated recommendation tests pass; build exits 0.

- [ ] **Step 6: 提交推荐工作区**

```bash
git add hls-web/src/workspaces/RecommendationWorkspace.vue hls-web/src/workspaceBoundary.test.js recommendation-console/src/App.vue
git commit -m "refactor(frontend): move recommendation workspace into hls web"
```

### Task 6: 用 TDD 实现统一应用壳

**Files:**
- Create: `hls-web/src/appShell.test.js`
- Modify: `hls-web/src/App.vue`

- [ ] **Step 1: 写应用壳失败测试**

Create `hls-web/src/appShell.test.js`:

```js
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, test } from 'vitest'

const here = dirname(fileURLToPath(import.meta.url))
const app = readFileSync(resolve(here, 'App.vue'), 'utf8')

describe('shared application shell', () => {
  test('gates both mutually exclusive workspaces behind one login', () => {
    expect(app).toContain('v-if="!isUIUnlocked"')
    expect(app).toContain('@submit.prevent="submitLogin"')
    expect(app).toContain('账号或密码不正确')
    expect(app).toContain('<VideoWorkspace v-if="activeWorkspace === \'video\'" />')
    expect(app).toContain('<RecommendationWorkspace v-else-if="activeWorkspace === \'recommendation\'" />')
  })

  test('offers shared workspace and account actions', () => {
    expect(app).toContain('视频调试台')
    expect(app).toContain('推荐控制台')
    expect(app).toContain('@click="selectWorkspace(\'video\')"')
    expect(app).toContain('@click="selectWorkspace(\'recommendation\')"')
    expect(app).toContain('@click="logout"')
    expect(app).toContain('CONSOLE_ADMIN_USERNAME')
  })
})
```

- [ ] **Step 2: 验证最小 App 不满足应用壳契约**

Run:

```bash
cd hls-web && npm test -- src/appShell.test.js
```

Expected: both tests FAIL because the temporary App only renders `VideoWorkspace`.

- [ ] **Step 3: 实现统一应用壳**

Replace `hls-web/src/App.vue` with:

```vue
<script setup>
import { reactive, ref } from 'vue'
import {
  CONSOLE_ADMIN_USERNAME,
  clearLegacyAuthenticated,
  isKnownWorkspace,
  isValidConsoleLogin,
  readActiveWorkspace,
  readUIUnlocked,
  writeActiveWorkspace,
  writeUIUnlocked,
} from './config/consoleSession.js'
import RecommendationWorkspace from './workspaces/RecommendationWorkspace.vue'
import VideoWorkspace from './workspaces/VideoWorkspace.vue'

const browserStorage = typeof window === 'undefined' ? null : window.localStorage
clearLegacyAuthenticated(browserStorage)

const isUIUnlocked = ref(readUIUnlocked(browserStorage))
const activeWorkspace = ref(readActiveWorkspace(browserStorage))
const loginError = ref('')
const loginForm = reactive({ username: '', password: '' })

function submitLogin() {
  loginError.value = ''
  if (!isValidConsoleLogin(loginForm.username, loginForm.password)) {
    loginError.value = '账号或密码不正确'
    return
  }
  writeUIUnlocked(browserStorage, true)
  isUIUnlocked.value = true
  loginForm.password = ''
}

function selectWorkspace(workspace) {
  if (!isKnownWorkspace(workspace)) return
  writeActiveWorkspace(browserStorage, workspace)
  activeWorkspace.value = workspace
}

function logout() {
  writeUIUnlocked(browserStorage, false)
  isUIUnlocked.value = false
  loginForm.password = ''
  loginError.value = ''
}
</script>

<template>
  <main v-if="!isUIUnlocked" class="auth-shell">
    <section class="auth-panel" aria-label="视频视频平台登录">
      <div class="auth-brand">
        <span class="auth-brand__mark">HS</span>
        <div>
          <strong>视频视频平台</strong>
          <small>视频与推荐控制台</small>
        </div>
      </div>
      <form class="auth-form" @submit.prevent="submitLogin">
        <label>
          <span>账号</span>
          <input v-model="loginForm.username" autocomplete="username" autofocus />
        </label>
        <label>
          <span>密码</span>
          <input v-model="loginForm.password" type="password" autocomplete="current-password" />
        </label>
        <p v-if="loginError" class="auth-error" role="alert">{{ loginError }}</p>
        <button class="auth-submit" type="submit">登录</button>
      </form>
    </section>
  </main>

  <div v-else class="application-shell">
    <header class="application-toolbar">
      <div class="application-brand">
        <span class="application-brand__mark">HS</span>
        <div>
          <strong>视频视频平台</strong>
          <small>统一控制台</small>
        </div>
      </div>
      <nav class="workspace-switcher" aria-label="工作区切换">
        <button
          type="button"
          :aria-pressed="activeWorkspace === 'video'"
          @click="selectWorkspace('video')"
        >
          视频调试台
        </button>
        <button
          type="button"
          :aria-pressed="activeWorkspace === 'recommendation'"
          @click="selectWorkspace('recommendation')"
        >
          推荐控制台
        </button>
      </nav>
      <div class="account-actions">
        <span>{{ CONSOLE_ADMIN_USERNAME }}</span>
        <button type="button" aria-label="退出登录" @click="logout">退出</button>
      </div>
    </header>
    <VideoWorkspace v-if="activeWorkspace === 'video'" />
    <RecommendationWorkspace v-else-if="activeWorkspace === 'recommendation'" />
  </div>
</template>
```

- [ ] **Step 4: 验证应用壳测试、全量测试和构建**

Run:

```bash
cd hls-web && npm test -- src/appShell.test.js src/config/consoleSession.test.js
cd hls-web && npm test
cd hls-web && npm run build
```

Expected: app shell tests and all accumulated tests pass; build exits 0.

- [ ] **Step 5: 提交统一应用壳**

```bash
git add hls-web/src/App.vue hls-web/src/appShell.test.js
git commit -m "feat(frontend): add shared login and workspace switcher"
```

### Task 7: 隔离样式并完成响应式外壳

**Files:**
- Create: `hls-web/src/appShell.css`
- Create: `hls-web/src/recommendation/recommendation.css`
- Modify: `hls-web/src/main.js`
- Modify: `hls-web/src/workspaces/RecommendationWorkspace.vue`
- Modify: `hls-web/src/style.css`
- Modify: `hls-web/src/appShell.test.js`

- [ ] **Step 1: 扩展样式边界失败测试**

Append to `hls-web/src/appShell.test.js`:

```js
test('loads isolated shell and recommendation styles', () => {
  const main = readFileSync(resolve(here, 'main.js'), 'utf8')
  const recommendation = readFileSync(resolve(here, 'workspaces/RecommendationWorkspace.vue'), 'utf8')
  expect(main).toContain("import './appShell.css'")
  expect(recommendation).toContain('<style scoped src="../recommendation/recommendation.css"></style>')
})
```

- [ ] **Step 2: 验证测试因样式入口缺失而失败**

Run:

```bash
cd hls-web && npm test -- src/appShell.test.js
```

Expected: the new style-boundary test FAILS.

- [ ] **Step 3: 创建应用壳样式**

Create `hls-web/src/appShell.css`:

```css
.auth-shell {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 24px;
  background: #f5f7f8;
  color: #1d2430;
}

.auth-panel {
  width: min(100%, 420px);
  padding: 28px;
  border: 1px solid #dbe3e1;
  border-radius: 8px;
  background: #ffffff;
  box-shadow: 0 18px 48px rgba(29, 36, 48, 0.1);
}

.auth-brand,
.application-brand {
  display: flex;
  align-items: center;
  gap: 12px;
}

.auth-brand__mark,
.application-brand__mark {
  width: 42px;
  height: 42px;
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  border-radius: 6px;
  background: #0f766e;
  color: #ffffff;
  font-weight: 800;
}

.auth-brand strong,
.auth-brand small,
.application-brand strong,
.application-brand small {
  display: block;
}

.auth-brand small,
.application-brand small {
  margin-top: 2px;
  color: #62706f;
}

.auth-form {
  display: grid;
  gap: 16px;
  margin-top: 28px;
}

.auth-form label {
  display: grid;
  gap: 7px;
  font-weight: 700;
}

.auth-form input {
  min-width: 0;
  min-height: 44px;
  padding: 0 12px;
  border: 1px solid #b7c6c2;
  border-radius: 6px;
  background: #ffffff;
  color: #1d2430;
  font: inherit;
}

.auth-form input:focus-visible {
  outline: 3px solid rgba(15, 118, 110, 0.2);
  border-color: #0f766e;
}

.auth-error {
  margin: 0;
  color: #be123c;
  font-weight: 700;
}

.auth-submit {
  min-height: 44px;
  border: 0;
  border-radius: 6px;
  background: #0f766e;
  color: #ffffff;
  font: inherit;
  font-weight: 800;
  cursor: pointer;
}

.application-shell {
  min-height: 100vh;
  background: #f5f7f8;
}

.application-toolbar {
  min-height: 72px;
  display: grid;
  grid-template-columns: minmax(210px, 1fr) auto minmax(210px, 1fr);
  align-items: center;
  gap: 20px;
  padding: 12px 20px;
  border-bottom: 1px solid #dbe3e1;
  background: #ffffff;
}

.workspace-switcher {
  display: grid;
  grid-template-columns: repeat(2, minmax(124px, 1fr));
  padding: 4px;
  border: 1px solid #dbe3e1;
  border-radius: 7px;
  background: #eef3f2;
}

.workspace-switcher button,
.account-actions button {
  min-height: 36px;
  border: 0;
  border-radius: 5px;
  background: transparent;
  color: #43514f;
  font: inherit;
  font-weight: 750;
  cursor: pointer;
}

.workspace-switcher button[aria-pressed='true'] {
  background: #ffffff;
  color: #0b5f59;
  box-shadow: 0 1px 4px rgba(29, 36, 48, 0.12);
}

.account-actions {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 10px;
  color: #62706f;
  font-weight: 700;
}

.account-actions button {
  padding: 0 12px;
  border: 1px solid #dbe3e1;
  background: #ffffff;
}

@media (max-width: 820px) {
  .application-toolbar {
    grid-template-columns: 1fr auto;
  }

  .workspace-switcher {
    grid-column: 1 / -1;
    grid-row: 2;
  }
}

@media (max-width: 480px) {
  .auth-shell {
    padding: 16px;
  }

  .auth-panel {
    padding: 22px;
  }

  .application-toolbar {
    grid-template-columns: 1fr;
    gap: 12px;
    padding: 12px;
  }

  .account-actions {
    justify-content: space-between;
  }

  .workspace-switcher {
    grid-column: 1;
    grid-row: auto;
    width: 100%;
  }
}
```

- [ ] **Step 4: 迁移并作用域化推荐样式**

```bash
git mv recommendation-console/src/style.css hls-web/src/recommendation/recommendation.css
```

In `recommendation.css`, replace the top-level `:root` selector with `.recommendation-workspace`, and replace the top-level `body` block with `.recommendation-workspace`. Keep the remaining selectors unchanged because Vue will scope the external stylesheet.

Append to `RecommendationWorkspace.vue` after `</template>`:

```vue
<style scoped src="../recommendation/recommendation.css"></style>
```

In `hls-web/src/main.js`, keep the video stylesheet and add the shell stylesheet:

```js
import { createApp } from 'vue'
import './style.css'
import './appShell.css'
import App from './App.vue'

createApp(App).mount('#app')
```

- [ ] **Step 5: 收紧与推荐页面冲突的视频类名**

The two stylesheets share only `brand-block`, `brand-mark`, `empty-state`, `eyebrow`, and `metric-card`. In `hls-web/src/style.css`, replace their video selectors with these scoped forms while leaving shared `:root`, reset, `body`, `a`, and form font rules unchanged:

```css
.video-workspace .brand-block {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 48px;
}

.video-workspace .brand-mark {
  width: 44px;
  height: 44px;
  border-radius: 8px;
  display: inline-grid;
  place-items: center;
  background: #dff4f1;
  color: #0b5f59;
  font-weight: 800;
  font-size: 15px;
}

.video-workspace .brand-block strong,
.video-workspace .brand-block span {
  display: block;
}

.video-workspace .brand-block strong {
  font-size: 17px;
  line-height: 1.25;
}

.video-workspace .brand-block span {
  margin-top: 2px;
  color: var(--sidebar-muted);
  font-size: 13px;
}

.video-workspace .eyebrow,
.video-workspace .panel-tag,
.video-workspace .detail-label {
  margin: 0;
  color: var(--primary);
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0;
  text-transform: uppercase;
}

.video-workspace .metric-card {
  min-width: 0;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--surface-soft);
  padding: 14px;
  display: grid;
  gap: 10px;
}

.video-workspace .empty-state {
  border: 1px dashed var(--border-strong);
  border-radius: 8px;
  padding: 22px;
  text-align: center;
  color: var(--text-muted);
  background: var(--surface-soft);
}

.video-workspace .empty-state.small {
  padding: 12px;
}
```

This is a selector-only change: preserve every declaration body byte-for-byte and do not reformat adjacent CSS.

- [ ] **Step 6: 验证样式契约、全量测试和构建**

Run:

```bash
cd hls-web && npm test
cd hls-web && npm run build
```

Expected: all tests pass and Vite build exits 0 without CSS parsing warnings.

- [ ] **Step 7: 提交样式隔离**

```bash
git add hls-web/src/appShell.css hls-web/src/main.js hls-web/src/style.css hls-web/src/appShell.test.js hls-web/src/workspaces/RecommendationWorkspace.vue hls-web/src/recommendation/recommendation.css recommendation-console/src/style.css
git commit -m "style(frontend): unify shell and isolate workspaces"
```

### Task 8: 更新文档、浏览器验收并删除旧项目

**Files:**
- Modify: `hls-web/README.md`
- Modify: `README.md`
- Modify: `docs/README.md`
- Delete: `recommendation-console/`

- [ ] **Step 1: 更新合并后前端 README**

Rewrite the introduction and feature sections of `hls-web/README.md` so they explicitly include:

```markdown
# hls-web - 视频与推荐统一控制台

该 Vue 3 + Vite 工程是 `video-service/` 的统一联调前端。登录后可在“视频调试台”和“推荐控制台”两个工作区之间切换。

> 登录账号 `aaddmmiinn`、密码 `admin123` 仅在浏览器内校验，用于避免误操作，不是安全鉴权。后端管理 API 在受可信网关或真实鉴权保护前，不得暴露到不受信任网络。

## 推荐控制台模块

- 诊断中心、推荐概览、数据源健康和命中效果
- random-play 与按题推荐链路 Trace
- Redis 播放桶状态
- random-play 与按题推荐预览

推荐管理接口统一位于 `/api/admin/recommendation/*`。
```

Keep the existing install, dev server, video feature, test, proxy, and Docker instructions. Add the migrated recommendation test paths to the testing section.

- [ ] **Step 2: 更新仓库索引**

In root `README.md`, change the `hls-web/` bullet to:

```markdown
- `hls-web/`：Vue 3 + Vite 视频与推荐统一控制台，用于本地联调上传、播放、反馈、推荐管理和链路诊断。
```

Keep the repository tree with only `hls-web/`; do not add `recommendation-console/`.

In `docs/README.md`, add these entries under current content:

```markdown
- `superpowers/specs/2026-07-13-frontend-console-merge-design.md`：视频与推荐前端合并设计。
- `superpowers/plans/2026-07-13-frontend-console-merge-plan.md`：视频与推荐前端合并实施计划。
```

- [ ] **Step 3: 运行删除前的全量测试和构建**

Run:

```bash
cd hls-web && npm test
cd hls-web && npm run build
```

Expected: at least 42 migrated/existing tests plus the newly added session and shell tests pass; build exits 0.

- [ ] **Step 4: 启动合并后的开发服务器**

Run:

```bash
cd hls-web && npm run dev -- --host 127.0.0.1
```

Expected: Vite prints a local URL on port 5173. Keep this process running during browser checks.

- [ ] **Step 5: 在桌面视口完成浏览器验收**

At `1440x900`, verify in order:

```text
1. 未登录时只显示统一登录页。
2. 错误密码显示“账号或密码不正确”。
3. aaddmmiinn / admin123 登录后进入视频调试台。
4. 视频工作台保留系统指标、上传、播放、视频管理和题库锚点。
5. 切换到推荐控制台后出现七个推荐栏目。
6. 点击七个栏目时只显示对应内容区。
7. 刷新页面仍停留在推荐控制台和最后栏目。
8. 退出后两个工作区均卸载并返回登录页。
```

Capture screenshots for login, video workspace, and recommendation workspace. Inspect browser console and accept backend connection failures only when the local backend is intentionally absent; no Vue runtime, import, or CSS parsing errors are allowed.

- [ ] **Step 6: 在移动视口完成浏览器验收**

At `390x844`, repeat login and both workspace switches. Verify the application toolbar wraps without overlap, both workspace buttons remain fully readable, account actions do not cover the brand, video navigation can scroll, and recommendation tables remain horizontally scrollable. Capture login and both workspace screenshots.

- [ ] **Step 7: 校验 Docker Compose 仍只部署 hls-web**

Run:

```bash
docker compose config --services
```

Expected: output includes `frontend_web` and no recommendation frontend service.

Run:

```bash
rg -n 'frontend_web:|/hls-web:/app|--port 1325' docker-compose.yml docker-compose.cloud.yml
```

Expected: both compose files still mount `hls-web` and expose the existing Vite service.

- [ ] **Step 8: 删除旧推荐项目**

Only after Steps 3-7 pass:

```bash
rm -rf recommendation-console
```

The user explicitly requested deletion. Do not delete `hls-web/`, backend files, or root deployment files.

- [ ] **Step 9: 运行删除后的最终验证**

Run:

```bash
test ! -d recommendation-console
cd hls-web && npm test
cd hls-web && npm run build
git diff --check
git status --short
```

Expected: old directory is absent; all tests and build pass; diff check is clean; status contains only intended frontend/docs changes plus the two pre-existing Go modifications.

- [ ] **Step 10: 提交文档与旧项目删除**

```bash
git add README.md docs/README.md hls-web/README.md
git add -A recommendation-console
git commit -m "docs(frontend): document merged console and remove legacy app"
```

- [ ] **Step 11: 确认最终提交范围**

Run:

```bash
git show --stat --oneline HEAD
git status --short
```

Expected: the final commit includes only documentation and `recommendation-console/` deletion; the two unrelated Go files remain modified and unstaged.

## 最终验证清单

- [ ] `hls-web` 原 25 个视频测试全部保留。
- [ ] 推荐 API 与栏目状态原有行为迁移到 `hls-web` 测试中。
- [ ] 新统一会话、旧键清理和应用壳契约测试通过。
- [ ] `npm run build` 通过。
- [ ] `1440x900` 与 `390x844` 浏览器流程和截图完成。
- [ ] 两个工作区互斥挂载，切换或退出后旧工作区停止轮询。
- [ ] README 明确 UI 门禁不等于安全鉴权。
- [ ] Docker Compose 仍只部署 `hls-web`。
- [ ] `recommendation-console/` 已删除。
- [ ] 两处用户已有 Go 修改未暂存、未提交、未改写。
