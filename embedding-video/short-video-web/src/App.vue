<script setup>
import { onMounted, ref } from 'vue'
import { clearSession, readSession } from './auth/session.js'
import { loadCurrentAdmin } from './auth/api.js'
import { getCurrentUserId } from './user/api.js'
import LoginView from './components/LoginView.vue'
import RegisterView from './components/RegisterView.vue'
import FeedView from './components/FeedView.vue'
import ProfileView from './components/ProfileView.vue'
import ProfileEditView from './components/ProfileEditView.vue'
import PublishView from './components/PublishView.vue'
import FollowListView from './components/FollowListView.vue'
import MessageListView from './components/MessageListView.vue'
import ChatView from './components/ChatView.vue'

const status = ref('checking')
const session = ref(null)
const profileUserId = ref(getCurrentUserId())
const followListParams = ref({ userId: 0, type: 'following' })
const chatUserId = ref(0)

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

function onRegistered(nextSession) {
  session.value = nextSession
  status.value = 'feed'
}

function onGoRegister() {
  status.value = 'register'
}

function onGoLogin() {
  status.value = 'login'
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
  } else if (target?.view === 'publish') {
    status.value = 'publish'
  } else if (target?.view === 'messages') {
    status.value = 'messages'
  } else if (target?.view === 'chat') {
    chatUserId.value = Number(target.userId) || 0
    if (chatUserId.value > 0) {
      status.value = 'chat'
    }
  } else if (target?.view === 'friends') {
    status.value = 'friends'
  } else if (target?.view === 'feed') {
    status.value = 'feed'
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

function onProfilePublish() {
  status.value = 'publish'
}

function onProfileShowList(params) {
  followListParams.value = { userId: Number(params?.userId) || profileUserId.value, type: params?.type || 'following' }
  status.value = 'follow-list'
}

function onFollowListBack() {
  status.value = 'profile'
}

function onPublishBack() {
  // 从发布页返回时，回到之前的页面（feed 或 profile）
  status.value = 'feed'
}

function onPublishPublished() {
  // 发布成功后跳转到自己的主页
  profileUserId.value = getCurrentUserId()
  status.value = 'profile'
}

function onMessagesBack() {
  status.value = 'feed'
}

function onOpenChat(payload) {
  chatUserId.value = Number(payload?.userId) || 0
  if (chatUserId.value > 0) {
    status.value = 'chat'
  }
}

function onChatBack() {
  status.value = 'messages'
}

function onFriendsBack() {
  status.value = 'feed'
}
</script>

<template>
  <div v-if="status === 'checking'" class="boot">
    <div class="boot-logo">短</div>
    <p class="boot-text">加载中…</p>
  </div>
  <LoginView v-else-if="status === 'login'" @logged-in="onLoggedIn" @go-register="onGoRegister" />
  <RegisterView v-else-if="status === 'register'" @registered="onRegistered" @go-login="onGoLogin" />
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
    @publish="onProfilePublish"
    @show-list="onProfileShowList"
    @navigate="onNavigate"
  />
  <FollowListView
    v-else-if="status === 'follow-list'"
    :user-id="followListParams.userId"
    :type="followListParams.type"
    @back="onFollowListBack"
  />
  <ProfileEditView
    v-else-if="status === 'profile-edit'"
    @back="onProfileEditBack"
    @saved="onProfileEditSaved"
  />
  <PublishView
    v-else-if="status === 'publish'"
    @back="onPublishBack"
    @published="onPublishPublished"
  />
  <MessageListView
    v-else-if="status === 'messages'"
    @back="onMessagesBack"
    @open-chat="onOpenChat"
  />
  <ChatView
    v-else-if="status === 'chat'"
    :other-user-id="chatUserId"
    @back="onChatBack"
  />
  <div v-else-if="status === 'friends'" class="friends-placeholder">
    <div class="friends-icon">👥</div>
    <p class="friends-title">朋友</p>
    <p class="friends-desc">关注的人发布的视频将在这里显示</p>
    <button class="friends-back" type="button" @click="onFriendsBack">返回首页</button>
  </div>
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

.friends-placeholder {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  background: #000;
  color: #fff;
}

.friends-icon {
  font-size: 48px;
  opacity: 0.6;
}

.friends-title {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
}

.friends-desc {
  margin: 0;
  font-size: 14px;
  color: rgba(255, 255, 255, 0.5);
}

.friends-back {
  margin-top: 16px;
  padding: 10px 28px;
  border-radius: 999px;
  background: linear-gradient(90deg, #fe2c55, #ff5470);
  color: #fff;
  font-size: 14px;
  font-weight: 600;
}
</style>
