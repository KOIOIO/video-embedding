<script setup>
import { computed, onMounted, ref } from 'vue'
import { fetchUserProfile, getCurrentUserId } from '../user/api.js'

const props = defineProps({
  userId: { type: Number, required: true },
})

const emit = defineEmits(['back', 'edit'])

const profile = ref(null)
const loading = ref(true)
const loadError = ref('')
const avatarFailed = ref(false)

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

function onAvatarError() {
  avatarFailed.value = true
}

function onBack() {
  emit('back')
}

function onEdit() {
  emit('edit')
}

onMounted(loadProfile)
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
        <div class="stat">
          <span class="stat-num">{{ profile.follow_count }}</span>
          <span class="stat-label">关注</span>
        </div>
        <div class="stat">
          <span class="stat-num">{{ profile.fans_count }}</span>
          <span class="stat-label">粉丝</span>
        </div>
      </div>

      <div v-if="isOwnProfile" class="actions">
        <button class="edit-btn" type="button" @click="onEdit">编辑资料</button>
      </div>

      <div class="works-section">
        <div class="works-tabs">
          <span class="works-tab active">作品</span>
        </div>
        <div class="works-empty">
          <div class="works-empty-icon">🎬</div>
          <p>暂无作品</p>
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
