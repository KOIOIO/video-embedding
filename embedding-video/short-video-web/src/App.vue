<script setup>
import { onMounted, ref } from 'vue'
import { clearSession, readSession } from './auth/session.js'
import { loadCurrentAdmin } from './auth/api.js'
import { getCurrentUserId } from './user/api.js'
import LoginView from './components/LoginView.vue'
import FeedView from './components/FeedView.vue'
import ProfileView from './components/ProfileView.vue'
import ProfileEditView from './components/ProfileEditView.vue'

const status = ref('checking')
const session = ref(null)
const profileUserId = ref(getCurrentUserId())

onMounted(async () => {
  const saved = readSession()
  if (!saved) {
    status.value = 'login'
    return
  }
  try {
    await loadCurrentAdmin()
    session.value = saved
    status.value = 'feed'
  } catch {
    clearSession()
    status.value = 'login'
  }
})

function onLoggedIn(nextSession) {
  session.value = nextSession
  status.value = 'feed'
}

function onLogout() {
  clearSession()
  session.value = null
  status.value = 'login'
}

function onNavigate(target) {
  if (target?.view === 'profile') {
    profileUserId.value = Number(target.userId) || getCurrentUserId()
    status.value = 'profile'
  } else if (target?.view === 'profile-edit') {
    status.value = 'profile-edit'
  }
}

function onProfileBack() {
  status.value = 'feed'
}

function onProfileEdit() {
  status.value = 'profile-edit'
}

function onProfileEditBack() {
  status.value = 'profile'
}

function onProfileEditSaved() {
  status.value = 'profile'
}
</script>

<template>
  <div v-if="status === 'checking'" class="boot">
    <div class="boot-logo">短</div>
    <p class="boot-text">加载中…</p>
  </div>
  <LoginView v-else-if="status === 'login'" @logged-in="onLoggedIn" />
  <FeedView
    v-else-if="status === 'feed'"
    :session="session"
    @logout="onLogout"
    @navigate="onNavigate"
  />
  <ProfileView
    v-else-if="status === 'profile'"
    :user-id="profileUserId"
    @back="onProfileBack"
    @edit="onProfileEdit"
  />
  <ProfileEditView
    v-else-if="status === 'profile-edit'"
    @back="onProfileEditBack"
    @saved="onProfileEditSaved"
  />
</template>

<style scoped>
.boot {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 14px;
  background: #000;
}

.boot-logo {
  width: 64px;
  height: 64px;
  display: grid;
  place-items: center;
  border-radius: 18px;
  font-size: 30px;
  font-weight: 800;
  color: #fff;
  background: linear-gradient(135deg, #fe2c55 0%, #ff7a59 100%);
  box-shadow: 0 0 40px rgba(254, 44, 85, 0.5);
}

.boot-text {
  margin: 0;
  color: var(--text-dim);
  font-size: 14px;
}
</style>
