<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { fetchMyVisits, fetchRelation, fetchUserProfile, getCurrentUserId, recordVisit } from '../user/api.js'
import { fetchUserVideos, formatDuration, STATUS_PUBLISHED } from '../user/videoApi.js'
import FollowButton from './FollowButton.vue'

const props = defineProps({
  userId: { type: Number, required: true },
})

const emit = defineEmits(['back', 'edit', 'publish', 'show-list', 'navigate'])

const profile = ref(null)
const loading = ref(true)
const loadError = ref('')
const avatarFailed = ref(false)
const relation = ref('none')
const visitReported = ref(false)
const todayVisits = ref({ unique_visitors: 0, total_visits: 0 })

const works = ref([])
const worksTotal = ref(0)
const worksPage = ref(1)
const worksLoading = ref(false)
const worksLoaded = ref(false)
const worksPageSize = 12

const isOwnProfile = computed(() => Number(props.userId) === getCurrentUserId())
const displayName = computed(() => profile.value?.nickname || `用户${props.userId}`)
const avatarText = computed(() => displayName.value.slice(0, 1).toUpperCase())
const genderLabel = computed(() => {
  switch (profile.value?.gender) {
    case 1: return '男'
    case 2: return '女'
    default: return ''
  }
})
const hasMoreWorks = computed(() => works.value.length < worksTotal.value)

async function loadProfile() {
  loading.value = true
  loadError.value = ''
  try {
    profile.value = await fetchUserProfile(props.userId)
  } catch {
    loadError.value = '加载用户资料失败'
  } finally {
    loading.value = false
  }
}

async function loadRelation() {
  if (isOwnProfile.value) {
    relation.value = 'none'
    return
  }
  try {
    relation.value = await fetchRelation(props.userId)
  } catch {
    relation.value = 'none'
  }
}

function reportVisit() {
  if (visitReported.value) return
  if (isOwnProfile.value) return
  visitReported.value = true
  recordVisit(props.userId)
}

async function loadMyVisits() {
  if (!isOwnProfile.value) return
  try {
    todayVisits.value = await fetchMyVisits()
  } catch {
    // 静默失败
  }
}

function onRelationChange(newRelation) {
  relation.value = newRelation
  if (!profile.value) return
  if (newRelation === 'following') {
    profile.value.follow_count = (profile.value.follow_count || 0) + 1
  } else if (newRelation === 'none') {
    profile.value.follow_count = Math.max(0, (profile.value.follow_count || 0) - 1)
  }
}

function onMessage() {
  emit('navigate', { view: 'chat', userId: props.userId })
}

function onShowFollowing() {
  emit('show-list', { type: 'following', userId: props.userId })
}

function onShowFollowers() {
  emit('show-list', { type: 'followers', userId: props.userId })
}

async function loadProfile() {
  loading.value = true
  loadError.value = ''
  try {
    profile.value = await fetchUserProfile(props.userId)
  } catch {
    loadError.value = '加载用户资料失败'
  } finally {
    loading.value = false
  }
}

async function loadWorks(reset = false) {
  if (worksLoading.value) return
  if (reset) {
    works.value = []
    worksPage.value = 1
    worksTotal.value = 0
    worksLoaded.value = false
  }
  if (worksLoaded.value && !reset) return
  worksLoading.value = true
  try {
    const result = await fetchUserVideos(props.userId, { page: worksPage.value, pageSize: worksPageSize })
    works.value = works.value.concat(result.list)
    worksTotal.value = result.total
    if (result.list.length < worksPageSize || works.value.length >= worksTotal.value) {
      worksLoaded.value = true
    } else {
      worksPage.value++
    }
  } catch {
    // 静默失败，显示空状态
  } finally {
    worksLoading.value = false
  }
}

function onLoadMore() {
  if (hasMoreWorks.value && !worksLoading.value) {
    loadWorks(false)
  }
}

function onAvatarError() {
  avatarFailed.value = true
}

function onBack() {
  emit('back')
}

function onEdit() {
  emit('edit')
}

function onPublish() {
  emit('publish')
}

function onWorkClick(video) {
  // 第一版简单提示，后续阶段完善播放
  alert(`播放视频：${video.title}`)
}

onMounted(() => {
  loadProfile()
  loadRelation()
  loadWorks(true)
  reportVisit()
  loadMyVisits()
})

watch(() => props.userId, () => {
  visitReported.value = false
  reportVisit()
  loadMyVisits()
})
</script>

<template>
  <div class="profile">
    <header class="topbar">
      <button class="back-btn" type="button" aria-label="返回" @click="onBack">←</button>
      <span class="topbar-title">{{ displayName }}</span>
      <span class="topbar-spacer"></span>
    </header>

    <div v-if="loading" class="center">
      <div class="spinner"></div>
      <p>加载中…</p>
    </div>

    <div v-else-if="loadError" class="center">
      <p class="error-text">{{ loadError }}</p>
      <button class="retry" type="button" @click="loadProfile">重试</button>
    </div>

    <div v-else class="content">
      <div class="profile-header">
        <div class="avatar-wrap">
          <img
            v-if="profile.avatar_url && !avatarFailed"
            :src="profile.avatar_url"
            :alt="displayName"
            class="avatar-img"
            @error="onAvatarError"
          />
          <div v-else class="avatar-fallback">{{ avatarText }}</div>
        </div>
        <div class="profile-info">
          <h1 class="nickname">{{ displayName }}</h1>
          <p v-if="profile.bio" class="bio">{{ profile.bio }}</p>
          <div class="meta">
            <span v-if="genderLabel" class="meta-item">{{ genderLabel }}</span>
            <span v-if="profile.location" class="meta-item">{{ profile.location }}</span>
          </div>
        </div>
      </div>

      <div class="stats">
        <div class="stat" role="button" tabindex="0" @click="onShowFollowing" @keydown.enter="onShowFollowing">
          <span class="stat-num">{{ profile.follow_count }}</span>
          <span class="stat-label">关注</span>
        </div>
        <div class="stat" role="button" tabindex="0" @click="onShowFollowers" @keydown.enter="onShowFollowers">
          <span class="stat-num">{{ profile.fans_count }}</span>
          <span class="stat-label">粉丝</span>
        </div>
        <div v-if="isOwnProfile" class="stat stat-visitor" :title="`独立访客 ${todayVisits.unique_visitors} · 总访问 ${todayVisits.total_visits}`">
          <span class="stat-num">{{ todayVisits.unique_visitors }}</span>
          <span class="stat-label">今日访客</span>
        </div>
      </div>

      <div class="actions">
        <FollowButton
          v-if="!isOwnProfile"
          :user-id="userId"
          :relation="relation"
          @change="onRelationChange"
        />
        <button v-if="!isOwnProfile" class="message-btn" type="button" @click="onMessage">发消息</button>
        <button v-else class="edit-btn" type="button" @click="onEdit">编辑资料</button>
      </div>

      <div class="works-section">
        <div class="works-tabs">
          <span class="works-tab active">作品</span>
        </div>
        <div v-if="worksLoading && works.length === 0" class="works-skeleton">
          <div v-for="i in 6" :key="i" class="skeleton-item"></div>
        </div>
        <div v-else-if="works.length > 0" class="works-grid">
          <div
            v-for="video in works"
            :key="video.id"
            class="work-card"
            @click="onWorkClick(video)"
          >
            <div class="work-cover">
              <img v-if="video.cover_url" :src="video.cover_url" :alt="video.title" loading="lazy" />
              <div v-else class="work-cover-placeholder">
                <span>{{ displayName.slice(0, 1) }}</span>
              </div>
              <span class="work-duration">{{ formatDuration(video.duration) }}</span>
            </div>
            <p class="work-title">{{ video.title }}</p>
          </div>
        </div>
        <div v-else class="works-empty">
          <div class="works-empty-icon">🎬</div>
          <p>暂无作品</p>
          <button v-if="isOwnProfile" class="publish-first-btn" type="button" @click="onPublish">发布第一个视频</button>
        </div>
        <div v-if="hasMoreWorks" class="load-more-wrap">
          <button class="load-more-btn" type="button" :disabled="worksLoading" @click="onLoadMore">
            {{ worksLoading ? '加载中…' : '加载更多' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.profile {
  position: relative;
  height: 100%;
  width: 100%;
  background: #000;
  color: #fff;
  overflow-y: auto;
}

.topbar {
  position: sticky;
  top: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: calc(12px + env(safe-area-inset-top)) 16px 12px;
  background: linear-gradient(180deg, rgba(0, 0, 0, 0.85), rgba(0, 0, 0, 0.6));
  backdrop-filter: blur(10px);
}

.back-btn {
  width: 36px;
  height: 36px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.1);
  font-size: 18px;
  color: #fff;
  transition: background 0.15s;
}

.back-btn:hover {
  background: rgba(254, 44, 85, 0.6);
}

.topbar-title {
  font-size: 16px;
  font-weight: 700;
  max-width: 60%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.topbar-spacer {
  width: 36px;
}

.content {
  padding: 0 16px 40px;
}

.profile-header {
  display: flex;
  align-items: flex-start;
  gap: 18px;
  padding: 24px 0 20px;
}

.avatar-wrap {
  flex-shrink: 0;
}

.avatar-img {
  width: 80px;
  height: 80px;
  border-radius: 50%;
  object-fit: cover;
  border: 2px solid rgba(255, 255, 255, 0.15);
}

.avatar-fallback {
  width: 80px;
  height: 80px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  background: linear-gradient(135deg, #333, #555);
  font-size: 32px;
  font-weight: 700;
  color: #ccc;
  border: 2px solid rgba(255, 255, 255, 0.1);
}

.profile-info {
  flex: 1;
  min-width: 0;
  padding-top: 4px;
}

.nickname {
  margin: 0 0 6px;
  font-size: 20px;
  font-weight: 800;
}

.bio {
  margin: 0 0 8px;
  font-size: 13px;
  color: var(--text-dim, #999);
  line-height: 1.5;
  word-break: break-word;
}

.meta {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.meta-item {
  font-size: 12px;
  color: var(--text-dim, #999);
}

.stats {
  display: flex;
  gap: 32px;
  padding: 16px 0;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.stat {
  display: flex;
  flex-direction: column;
  gap: 2px;
  cursor: pointer;
  transition: opacity 0.15s;
}

.stat:hover {
  opacity: 0.7;
}

.stat-visitor {
  cursor: default;
}

.stat-visitor:hover {
  opacity: 1;
}

.stat-num {
  font-size: 18px;
  font-weight: 800;
}

.stat-label {
  font-size: 12px;
  color: var(--text-dim, #999);
}

.actions {
  padding: 16px 0;
  display: flex;
  gap: 10px;
}

.actions > * {
  flex: 1;
}

.message-btn {
  padding: 11px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.1);
  color: #fff;
  font-size: 14px;
  font-weight: 600;
  transition: background 0.15s;
}

.message-btn:hover {
  background: rgba(254, 44, 85, 0.7);
}

.edit-btn {
  width: 100%;
  padding: 11px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.1);
  color: #fff;
  font-size: 14px;
  font-weight: 600;
  transition: background 0.15s;
}

.edit-btn:hover {
  background: rgba(254, 44, 85, 0.7);
}

.works-section {
  padding-top: 8px;
}

.works-tabs {
  display: flex;
  justify-content: center;
  gap: 32px;
  padding: 12px 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.works-tab {
  font-size: 14px;
  color: var(--text-dim, #999);
  padding-bottom: 6px;
}

.works-tab.active {
  color: #fff;
  font-weight: 700;
  border-bottom: 2px solid #fe2c55;
}

.works-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 60px 0;
  color: var(--text-dim, #666);
}

.works-empty-icon {
  font-size: 40px;
  opacity: 0.5;
}

.works-empty p {
  margin: 0;
  font-size: 14px;
}

.publish-first-btn {
  margin-top: 8px;
  padding: 8px 20px;
  border-radius: 999px;
  background: linear-gradient(90deg, #fe2c55, #ff5470);
  color: #fff;
  font-size: 13px;
  font-weight: 600;
}

.works-skeleton {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 2px;
}
.skeleton-item {
  aspect-ratio: 3 / 4;
  background: linear-gradient(90deg, #1a1a1a 25%, #2a2a2a 50%, #1a1a1a 75%);
  background-size: 200% 100%;
  animation: shimmer 1.2s infinite;
  border-radius: 4px;
}
@keyframes shimmer {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}

.works-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 2px;
}
.work-card {
  cursor: pointer;
  transition: opacity 0.15s;
}
.work-card:active { opacity: 0.7; }
.work-cover {
  position: relative;
  aspect-ratio: 3 / 4;
  background: #111;
  border-radius: 4px;
  overflow: hidden;
}
.work-cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.work-cover-placeholder {
  width: 100%;
  height: 100%;
  display: grid;
  place-items: center;
  background: linear-gradient(135deg, #2a2a2a, #1a1a1a);
  color: #555;
  font-size: 28px;
  font-weight: 700;
}
.work-duration {
  position: absolute;
  right: 6px;
  bottom: 6px;
  padding: 2px 6px;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.7);
  color: #fff;
  font-size: 11px;
  font-weight: 600;
}
.work-title {
  margin: 6px 4px 8px;
  font-size: 12px;
  line-height: 1.3;
  color: #ddd;
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.load-more-wrap {
  display: flex;
  justify-content: center;
  padding: 16px 0 8px;
}
.load-more-btn {
  padding: 8px 24px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.1);
  color: #ccc;
  font-size: 13px;
}
.load-more-btn:disabled { opacity: 0.5; }

.center {
  height: 60vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 14px;
  color: var(--text-dim, #999);
}

.error-text {
  margin: 0;
  font-size: 14px;
}

.retry {
  padding: 9px 24px;
  border-radius: 999px;
  background: linear-gradient(90deg, #fe2c55, #ff5470);
  color: #fff;
  font-size: 13px;
  font-weight: 700;
}

.spinner {
  width: 30px;
  height: 30px;
  border: 3px solid rgba(255, 255, 255, 0.2);
  border-top-color: #fe2c55;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>
