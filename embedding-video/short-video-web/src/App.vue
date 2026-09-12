<script setup>
import { onMounted, ref } from 'vue'
import { clearSession, readSession } from './auth/session.js'
import { loadCurrentAdmin } from './auth/api.js'
import { getCurrentUserId, refreshMyProfile } from './user/api.js'
import LoginView from './components/LoginView.vue'
import RegisterView from './components/RegisterView.vue'
import FeedView from './components/FeedView.vue'
import ProfileView from './components/ProfileView.vue'
import ProfileEditView from './components/ProfileEditView.vue'
import PublishView from './components/PublishView.vue'
import FollowListView from './components/FollowListView.vue'
import MessageListView from './components/MessageListView.vue'
import ChatView from './components/ChatView.vue'
import VideoView from './components/VideoView.vue'
import ProfilePlaylistView from './components/ProfilePlaylistView.vue'

const status = ref('checking')
const session = ref(null)
const profileUserId = ref(getCurrentUserId())
const profileTab = ref('works')
const followListParams = ref({ userId: 0, type: 'following' })
const chatUserId = ref(0)
const videoParams = ref({ videoId: 0, segmentId: 0, commentId: 0 })
const playlistParams = ref({ userId: 0, tabType: 'works', videos: [], startIndex: 0, ownerName: '' })

onMounted(async () => {
  const saved = readSession()
  if (!saved) {
    status.value = 'login'
    return
  }
  try {
    await loadCurrentAdmin()
    session.value = saved
    // 后台刷新 profile 到全局 session（确保首页顶部显示最新昵称/头像）
    refreshMyProfile().then((profile) => {
      if (session.value) {
        session.value = { ...session.value, profile: {
          nickname: profile.nickname,
          avatar_url: profile.avatar_url,
          bio: profile.bio,
          gender: profile.gender,
          location: profile.location,
        } }
      }
    }).catch(() => {})
    status.value = 'feed'
  } catch {
    clearSession()
    status.value = 'login'
  }
})

async function onLoggedIn(nextSession) {
  session.value = nextSession
  status.value = 'feed'
  // 登录后刷新 profile 到全局 session
  try {
    const profile = await refreshMyProfile()
    session.value = { ...session.value, profile: {
      nickname: profile.nickname,
      avatar_url: profile.avatar_url,
      bio: profile.bio,
      gender: profile.gender,
      location: profile.location,
    } }
  } catch {
    // profile 刷新失败不影响登录
  }
}

async function onRegistered(nextSession) {
  session.value = nextSession
  status.value = 'feed'
  // 注册后刷新 profile 到全局 session
  try {
    const profile = await refreshMyProfile()
    session.value = { ...session.value, profile: {
      nickname: profile.nickname,
      avatar_url: profile.avatar_url,
      bio: profile.bio,
      gender: profile.gender,
      location: profile.location,
    } }
  } catch {
    // profile 刷新失败不影响注册
  }
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
  } else if (target?.view === 'video') {
    videoParams.value = {
      videoId: Number(target.videoId) || 0,
      segmentId: Number(target.segmentId) || 0,
      commentId: Number(target.commentId) || 0,
    }
    if (videoParams.value.segmentId > 0) {
      status.value = 'video'
    }
  } else if (target?.view === 'profile-play') {
    playlistParams.value = {
      userId: Number(target.userId) || profileUserId.value,
      tabType: target.tabType === 'liked' ? 'liked' : 'works',
      videos: Array.isArray(target.videos) ? target.videos : [],
      startIndex: Math.max(0, Number(target.startIndex) || 0),
      ownerName: String(target.ownerName || ''),
    }
    status.value = 'profile-play'
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

async function onProfileEditSaved() {
  // 编辑资料后刷新全局 profile，确保首页顶部等地方显示最新信息
  try {
    const profile = await refreshMyProfile()
    session.value = { ...session.value, profile: {
      nickname: profile.nickname,
      avatar_url: profile.avatar_url,
      bio: profile.bio,
      gender: profile.gender,
      location: profile.location,
    } }
  } catch {
    // profile 刷新失败不影响返回
  }
  status.value = 'profile'
}

function onProfilePublish() {
  status.value = 'publish'
}

function onProfileShowList(params) {
  followListParams.value = { userId: Number(params?.userId) || profileUserId.value, type: params?.type || 'following' }
  status.value = 'follow-list'
}

function onProfileTabChange(tab) {
  profileTab.value = tab === 'liked' ? 'liked' : 'works'
}

function onPlaylistBack() {
  status.value = 'profile'
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

function onVideoBack() {
  status.value = 'feed'
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
    :initial-tab="profileTab"
    @tab-change="onProfileTabChange"
    @back="onProfileBack"
    @edit="onProfileEdit"
    @publish="onProfilePublish"
    @show-list="onProfileShowList"
    @navigate="onNavigate"
  />
  <ProfilePlaylistView
    v-else-if="status === 'profile-play'"
    :user-id="playlistParams.userId"
    :tab-type="playlistParams.tabType"
    :videos="playlistParams.videos"
    :start-index="playlistParams.startIndex"
    :owner-name="playlistParams.ownerName"
    @back="onPlaylistBack"
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
    @navigate="onNavigate"
  />
  <VideoView
    v-else-if="status === 'video'"
    :video-id="videoParams.videoId"
    :segment-id="videoParams.segmentId"
    :highlight-comment-id="videoParams.commentId"
    @back="onVideoBack"
    @navigate="onNavigate"
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
