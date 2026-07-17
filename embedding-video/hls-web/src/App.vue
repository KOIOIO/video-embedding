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

clearLegacyAuthenticated()

const isUIUnlocked = ref(readUIUnlocked())
const activeWorkspace = ref(readActiveWorkspace())
const loginForm = reactive({
  username: '',
  password: '',
})
const loginError = ref('')

function submitLogin() {
  loginError.value = ''
  if (!isValidConsoleLogin(loginForm.username, loginForm.password)) {
    loginError.value = '账号或密码不正确'
    return
  }

  writeUIUnlocked(true)
  isUIUnlocked.value = true
  loginForm.password = ''
}

function selectWorkspace(workspace) {
  if (!isKnownWorkspace(workspace)) return

  writeActiveWorkspace(workspace)
  activeWorkspace.value = workspace
}

function logout() {
  writeUIUnlocked(false)
  isUIUnlocked.value = false
  loginForm.password = ''
  loginError.value = ''
}
</script>

<template>
  <main v-if="!isUIUnlocked" class="auth-shell">
    <section class="auth-panel" aria-labelledby="login-title">
      <div class="auth-brand">
        <span class="shell-brand-mark" aria-hidden="true">HS</span>
        <div>
          <p>衡水视频平台</p>
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
        <button class="login-button" type="submit">登录</button>
      </form>
    </section>
  </main>

  <div v-else class="application-shell">
    <header class="application-toolbar">
      <div class="toolbar-brand">
        <span class="toolbar-brand-mark" aria-hidden="true">HS</span>
        <div>
          <strong>衡水视频平台</strong>
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
      </nav>

      <div class="account-actions">
        <span class="account-name">{{ CONSOLE_ADMIN_USERNAME }}</span>
        <button class="logout-button" type="button" @click="logout">退出</button>
      </div>
    </header>

    <main class="workspace-mount">
      <VideoWorkspace v-if="activeWorkspace === 'video'" />
      <RecommendationWorkspace v-else-if="activeWorkspace === 'recommendation'" />
    </main>
  </div>
</template>
