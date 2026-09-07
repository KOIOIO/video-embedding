<script setup>
import { computed, onMounted, ref } from 'vue'
import {
  fetchConversations,
  formatMessageTime,
  getCachedUserProfile,
  getCurrentUserId,
  prefetchUserProfiles,
} from '../user/api.js'

const emit = defineEmits(['open-chat', 'back'])

const conversations = ref([])
const loading = ref(true)
const loadError = ref('')
const avatarFailed = ref({})

const currentUserId = computed(() => getCurrentUserId())

async function loadConversations() {
  loading.value = true
  loadError.value = ''
  try {
    const list = await fetchConversations()
    conversations.value = list
    // 批量预取对方用户资料
    const otherIds = list.map((c) => c.other_user_id).filter((id) => id > 0)
    if (otherIds.length > 0) {
      await prefetchUserProfiles(otherIds)
    }
  } catch {
    loadError.value = '加载消息失败，请稍后重试'
  } finally {
    loading.value = false
  }
}

function getProfile(userId) {
  return getCachedUserProfile(userId)
}

function displayName(conv) {
  const profile = getProfile(conv.other_user_id)
  return profile?.nickname || `用户${conv.other_user_id}`
}

function avatarUrl(conv) {
  const profile = getProfile(conv.other_user_id)
  return profile?.avatar_url || ''
}

function avatarText(conv) {
  return displayName(conv).slice(0, 1).toUpperCase()
}

function truncatedContent(content) {
  if (!content) return ''
  return content.length > 30 ? content.slice(0, 30) + '…' : content
}

function onOpenChat(conv) {
  emit('open-chat', { userId: conv.other_user_id })
}

function onBack() {
  emit('back')
}

function onAvatarError(index) {
  avatarFailed.value[index] = true
}

onMounted(() => {
  loadConversations()
})
</script>

<template>
  <div class="message-list">
    <header class="topbar">
      <button class="back-btn" type="button" aria-label="返回" @click="onBack">←</button>
      <span class="topbar-title">消息</span>
      <button class="refresh-btn" type="button" aria-label="刷新" :disabled="loading" @click="loadConversations">
        {{ loading ? '…' : '↻' }}
      </button>
    </header>

    <div v-if="loading" class="center">
      <div class="spinner"></div>
      <p>加载中…</p>
    </div>

    <div v-else-if="loadError && conversations.length === 0" class="center">
      <p class="error-text">{{ loadError }}</p>
      <button class="retry" type="button" @click="loadConversations">重试</button>
    </div>

    <div v-else-if="conversations.length === 0" class="center">
      <p class="empty-text">暂无消息</p>
    </div>

    <div v-else class="list-content">
      <div
        v-for="(conv, index) in conversations"
        :key="conv.conversation_id"
        class="conversation-item"
        @click="onOpenChat(conv)"
      >
        <div class="item-avatar-wrap">
          <div class="item-avatar">
            <img
              v-if="avatarUrl(conv) && !avatarFailed[index]"
              :src="avatarUrl(conv)"
              :alt="displayName(conv)"
              @error="onAvatarError(index)"
            />
            <div v-else class="avatar-fallback">{{ avatarText(conv) }}</div>
          </div>
          <span v-if="conv.unread_count > 0" class="unread-badge">
            {{ conv.unread_count > 99 ? '99+' : conv.unread_count }}
          </span>
        </div>
        <div class="item-info">
          <div class="item-top-row">
            <span class="item-nickname">{{ displayName(conv) }}</span>
            <span class="item-time">{{ formatMessageTime(conv.last_message_time) }}</span>
          </div>
          <span class="item-last-msg">{{ truncatedContent(conv.last_message) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.message-list {
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

.back-btn,
.refresh-btn {
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

.back-btn:hover,
.refresh-btn:hover {
  background: rgba(254, 44, 85, 0.6);
}

.refresh-btn:disabled {
  opacity: 0.5;
}

.topbar-title {
  font-size: 16px;
  font-weight: 700;
}

.list-content {
  padding: 4px 0 40px;
}

.conversation-item {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  cursor: pointer;
  transition: background 0.15s;
}

.conversation-item:active {
  background: rgba(255, 255, 255, 0.05);
}

.item-avatar-wrap {
  position: relative;
  flex-shrink: 0;
}

.item-avatar img {
  width: 52px;
  height: 52px;
  border-radius: 50%;
  object-fit: cover;
  border: 2px solid rgba(255, 255, 255, 0.1);
}

.avatar-fallback {
  width: 52px;
  height: 52px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  background: linear-gradient(135deg, #333, #555);
  font-size: 22px;
  font-weight: 700;
  color: #ccc;
}

.unread-badge {
  position: absolute;
  top: -2px;
  right: -2px;
  min-width: 20px;
  height: 20px;
  padding: 0 5px;
  border-radius: 10px;
  background: #fe2c55;
  color: #fff;
  font-size: 11px;
  font-weight: 700;
  display: grid;
  place-items: center;
  border: 2px solid #000;
}

.item-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.item-top-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.item-nickname {
  font-size: 15px;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.item-time {
  font-size: 11px;
  color: var(--text-dim, #999);
  flex-shrink: 0;
}

.item-last-msg {
  font-size: 13px;
  color: var(--text-dim, #999);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
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

.error-text,
.empty-text {
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
