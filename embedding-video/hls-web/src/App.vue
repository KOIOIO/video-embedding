<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { loadCurrentAdmin, loginAdmin } from './auth/api.js'
import { clearAuthSession, readAuthSession, writeAuthSession } from './auth/session.js'
import {
  clearLegacyAuthenticated,
  isKnownWorkspace,
  readActiveWorkspace,
  writeActiveWorkspace,
} from './config/consoleSession.js'
import RecommendationWorkspace from './workspaces/RecommendationWorkspace.vue'
import KnowledgeVideoWorkspace from './workspaces/KnowledgeVideoWorkspace.vue'
import VideoWorkspace from './workspaces/VideoWorkspace.vue'

clearLegacyAuthenticated()

const authSession = ref(readAuthSession())
const isUIUnlocked = computed(() => Boolean(authSession.value?.accessToken && authSession.value?.admin?.id))
const accountName = computed(() => authSession.value?.admin?.real_name || authSession.value?.admin?.username || '')
const activeWorkspace = ref(readActiveWorkspace())
const loginForm = reactive({
  username: '',
  password: '',
})
const loginError = ref('')
const loggingIn = ref(false)

async function submitLogin() {
	loginError.value = ''
	loggingIn.value = true
	try {
		const result = await loginAdmin(loginForm.username.trim(), loginForm.password)
		const session = { accessToken: result.access_token, admin: result.admin }
		writeAuthSession(session)
		authSession.value = session
		loginForm.password = ''
	} catch (error) {
		loginError.value = error?.status === 401 ? '账号或密码不正确' : String(error?.message || '登录失败')
	} finally {
		loggingIn.value = false
	}
}

function selectWorkspace(workspace) {
  if (!isKnownWorkspace(workspace)) return

  writeActiveWorkspace(workspace)
  activeWorkspace.value = workspace
}

function logout() {
	clearAuthSession()
	authSession.value = null
  loginForm.password = ''
  loginError.value = ''
}

function expireSession() {
	authSession.value = null
	loginError.value = '登录已失效，请重新登录'
}

onMounted(async () => {
	globalThis.addEventListener?.('admin-auth-expired', expireSession)
	if (!authSession.value?.accessToken) return
	try {
		const admin = await loadCurrentAdmin()
		authSession.value = { ...authSession.value, admin }
		writeAuthSession(authSession.value)
	} catch {
		clearAuthSession()
		authSession.value = null
	}
})

onBeforeUnmount(() => globalThis.removeEventListener?.('admin-auth-expired', expireSession))
</script>

<template>
  <main v-if="!isUIUnlocked" class="auth-shell">
    <section class="auth-panel" aria-labelledby="login-title">
      <div class="auth-brand">
        <span class="shell-brand-mark" aria-hidden="true">HS</span>
        <div>
          <p>视频视频平台</p>
          <h1 id="login-title">视频与推荐控制台</h1>
        </div>
      </div>

      <form class="auth-form" @submit.prevent="submitLogin">
        <label class="auth-field">
          <span>账号</span>
          <input
            v-model="loginForm.username"
            name="username"
            type="text"
            autocomplete="username"
            required
            autofocus
          />
        </label>

        <label class="auth-field">
          <span>密码</span>
          <input
            v-model="loginForm.password"
            name="password"
            type="password"
            autocomplete="current-password"
            required
          />
        </label>

        <p v-if="loginError" class="auth-error" role="alert">{{ loginError }}</p>
        <button class="login-button" type="submit" :disabled="loggingIn">{{ loggingIn ? '登录中...' : '登录' }}</button>
      </form>
    </section>
  </main>

  <div v-else class="application-shell">
    <header class="application-toolbar">
      <div class="toolbar-brand">
        <span class="toolbar-brand-mark" aria-hidden="true">HS</span>
        <div>
          <strong>视频视频平台</strong>
          <span>视频与推荐控制台</span>
        </div>
      </div>

      <nav class="workspace-switcher" aria-label="工作区">
        <button
          type="button"
          :class="{ active: activeWorkspace === 'video' }"
          :aria-pressed="activeWorkspace === 'video'"
          @click="selectWorkspace('video')"
        >
          视频调试台
        </button>
        <button
          type="button"
          :class="{ active: activeWorkspace === 'recommendation' }"
          :aria-pressed="activeWorkspace === 'recommendation'"
          @click="selectWorkspace('recommendation')"
        >
          推荐控制台
        </button>
        <button
          type="button"
          :class="{ active: activeWorkspace === 'knowledge-video' }"
          :aria-pressed="activeWorkspace === 'knowledge-video'"
          @click="selectWorkspace('knowledge-video')"
        >
          知识点视频
        </button>
      </nav>

      <div class="account-actions">
        <span class="account-name">{{ accountName }}</span>
        <button class="logout-button" type="button" @click="logout">退出</button>
      </div>
    </header>

    <main class="workspace-mount">
      <VideoWorkspace v-if="activeWorkspace === 'video'" />
      <RecommendationWorkspace v-else-if="activeWorkspace === 'recommendation'" />
      <KnowledgeVideoWorkspace v-else-if="activeWorkspace === 'knowledge-video'" />
    </main>
  </div>
</template>
