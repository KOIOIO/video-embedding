<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  fetchMessages,
  formatMessageTime,
  getCachedUserProfile,
  getCurrentUserId,
  markAsRead,
  prefetchUserProfiles,
  sendMessage,
} from '../user/api.js'

const props = defineProps({
  otherUserId: { type: Number, required: true },
})

const emit = defineEmits(['back'])

const messages = ref([])
const inputText = ref('')
const loading = ref(true)
const loadError = ref('')
const sending = ref(false)
const page = ref(1)
const pageSize = 20
const hasMore = ref(true)
const loadingMore = ref(false)
const avatarFailed = ref(false)

const currentUserId = computed(() => getCurrentUserId())
const charCount = computed(() => inputText.value.length)
const canSend = computed(() => inputText.value.trim().length > 0 && !sending.value)

const otherProfile = ref(null)

function displayName() {
  return otherProfile.value?.nickname || `用户${props.otherUserId}`
}

function avatarUrl() {
  return otherProfile.value?.avatar_url || ''
}

function avatarText() {
  return displayName().slice(0, 1).toUpperCase()
}

function isOwnMessage(msg) {
  return msg.sender_id === currentUserId.value
}

async function loadMessages(reset = false) {
  if (reset) {
    page.value = 1
    hasMore.value = true
    messages.value = []
  }
  if (!hasMore.value) return
  if (reset) {
    loading.value = true
  } else {
    loadingMore.value = true
  }
  loadError.value = ''
  try {
    const result = await fetchMessages(props.otherUserId, page.value, pageSize)
    // 后端按 create_time 倒序返回，需要反转成正序（最新在底部）
    const reversed = [...result.list].reverse()
    if (reset) {
      messages.value = reversed
    } else {
      // 加载更多（更早的消息）：插到前面
      messages.value = [...reversed, ...messages.value]
    }
    if (result.list.length < pageSize) {
      hasMore.value = false
    } else {
      page.value++
    }
  } catch {
    loadError.value = '加载消息失败'
  } finally {
    loading.value = false
    loadingMore.value = false
    if (reset) {
      await nextTick()
      scrollToBottom()
    }
  }
}

async function onSend() {
  if (!canSend.value) return
  const content = inputText.value.trim()
  // 乐观更新：立即显示消息
  const optimisticMsg = {
    id: -Date.now(),
    sender_id: currentUserId.value,
    receiver_id: props.otherUserId,
    content,
    is_read: false,
    create_time: new Date().toISOString(),
    pending: true,
  }
  messages.value.push(optimisticMsg)
  inputText.value = ''
  await nextTick()
  scrollToBottom()

  sending.value = true
  try {
    const sent = await sendMessage(props.otherUserId, content)
    // 替换乐观消息
    const idx = messages.value.findIndex((m) => m.id === optimisticMsg.id)
    if (idx !== -1) {
      messages.value[idx] = { ...sent, pending: false }
    }
  } catch {
    // 发送失败，标记失败状态
    const idx = messages.value.findIndex((m) => m.id === optimisticMsg.id)
    if (idx !== -1) {
      messages.value[idx] = { ...optimisticMsg, pending: false, sendFailed: true }
    }
  } finally {
    sending.value = false
  }
}

function onResend(msg) {
  const idx = messages.value.findIndex((m) => m.id === msg.id)
  if (idx === -1) return
  messages.value.splice(idx, 1)
  inputText.value = msg.content
  onSend()
}

let pollTimer = null

async function pollNewMessages() {
  // 只拉第一页（最新的），合并去重
  try {
    const result = await fetchMessages(props.otherUserId, 1, pageSize)
    const reversed = [...result.list].reverse()
    const existingIds = new Set(messages.value.map((m) => m.id).filter((id) => id > 0))
    const newMsgs = reversed.filter((m) => !existingIds.has(m.id))
    if (newMsgs.length > 0) {
      messages.value = [...messages.value, ...newMsgs]
      await nextTick()
      scrollToBottom()
    }
  } catch {
    // 轮询失败静默处理
  }
}

function scrollToBottom() {
  const el = document.querySelector('.chat-messages')
  if (el) {
    el.scrollTop = el.scrollHeight
  }
}

function onScroll() {
  const el = document.querySelector('.chat-messages')
  if (!el) return
  if (el.scrollTop <= 10 && hasMore.value && !loadingMore.value && !loading.value) {
    loadMessages(false)
  }
}

function onBack() {
  emit('back')
}

function onAvatarError() {
  avatarFailed.value = true
}

async function init() {
  // 预取对方资料
  await prefetchUserProfiles([props.otherUserId])
  otherProfile.value = getCachedUserProfile(props.otherUserId)
  await loadMessages(true)
  // 进入聊天页标记已读
  try {
    await markAsRead(props.otherUserId)
  } catch {
    // 忽略
  }
  // 启动轮询
  pollTimer = setInterval(pollNewMessages, 3000)
}

onMounted(() => {
  init()
})

onBeforeUnmount(() => {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
})

watch(() => props.otherUserId, () => {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
  init()
})
</script>

<template>
  <div class="chat-view">
    <header class="topbar">
      <button class="back-btn" type="button" aria-label="返回" @click="onBack">←</button>
      <div class="chat-header-info">
        <div class="chat-avatar">
          <img
            v-if="avatarUrl() && !avatarFailed"
            :src="avatarUrl()"
            :alt="displayName()"
            @error="onAvatarError"
          />
          <div v-else class="avatar-fallback">{{ avatarText() }}</div>
        </div>
        <span class="chat-name">{{ displayName() }}</span>
      </div>
      <span class="topbar-spacer"></span>
    </header>

    <div v-if="loading" class="center">
      <div class="spinner"></div>
      <p>加载中…</p>
    </div>

    <div v-else class="chat-body">
      <div v-if="loadingMore" class="loading-more">加载更早消息…</div>
      <div v-if="loadError && messages.length === 0" class="center">
        <p class="error-text">{{ loadError }}</p>
        <button class="retry" type="button" @click="loadMessages(true)">重试</button>
      </div>
      <div v-else-if="messages.length === 0" class="center">
        <p class="empty-text">暂无消息，发送第一条吧</p>
      </div>
      <div v-else class="chat-messages" @scroll="onScroll">
        <div
          v-for="msg in messages"
          :key="msg.id"
          class="msg-row"
          :class="{ 'msg-own': isOwnMessage(msg), 'msg-other': !isOwnMessage(msg) }"
        >
          <div class="msg-bubble-wrap">
            <div
              class="msg-bubble"
              :class="{
                'bubble-own': isOwnMessage(msg),
                'bubble-other': !isOwnMessage(msg),
                'bubble-pending': msg.pending,
                'bubble-failed': msg.sendFailed,
              }"
            >
              {{ msg.content }}
            </div>
            <div v-if="msg.sendFailed" class="msg-failed-hint">
              <span>发送失败</span>
              <button class="resend-btn" type="button" @click="onResend(msg)">重试</button>
            </div>
          </div>
          <span class="msg-time">{{ formatMessageTime(msg.create_time) }}</span>
        </div>
      </div>
    </div>

    <footer class="chat-input-bar">
      <div class="input-wrap">
        <input
          v-model="inputText"
          class="msg-input"
          type="text"
          placeholder="说点什么…"
          maxlength="2000"
          @keyup.enter="onSend"
        />
        <span class="char-count">{{ charCount }}/2000</span>
      </div>
      <button
        class="send-btn"
        type="button"
        :disabled="!canSend"
        @click="onSend"
      >
        发送
      </button>
    </footer>
  </div>
</template>

<style scoped>
.chat-view {
  position: relative;
  height: 100%;
  width: 100%;
  background: #000;
  color: #fff;
  display: flex;
  flex-direction: column;
}

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: calc(12px + env(safe-area-inset-top)) 16px 12px;
  background: linear-gradient(180deg, rgba(0, 0, 0, 0.85), rgba(0, 0, 0, 0.6));
  backdrop-filter: blur(10px);
  flex-shrink: 0;
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

.chat-header-info {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  justify-content: center;
}

.chat-avatar img {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  object-fit: cover;
}

.avatar-fallback {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  background: linear-gradient(135deg, #333, #555);
  font-size: 14px;
  font-weight: 700;
  color: #ccc;
}

.chat-name {
  font-size: 15px;
  font-weight: 700;
}

.topbar-spacer {
  width: 36px;
}

.chat-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-height: 0;
}

.loading-more {
  text-align: center;
  padding: 8px;
  font-size: 12px;
  color: var(--text-dim, #999);
  flex-shrink: 0;
}

.chat-messages {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.msg-row {
  display: flex;
  flex-direction: column;
  max-width: 75%;
}

.msg-own {
  align-self: flex-end;
  align-items: flex-end;
}

.msg-other {
  align-self: flex-start;
  align-items: flex-start;
}

.msg-bubble-wrap {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-width: 100%;
}

.msg-bubble {
  padding: 10px 14px;
  border-radius: 16px;
  font-size: 14px;
  line-height: 1.5;
  word-break: break-word;
  white-space: pre-wrap;
}

.bubble-own {
  background: #fe2c55;
  color: #fff;
  border-bottom-right-radius: 4px;
}

.bubble-other {
  background: #2a2a2a;
  color: #fff;
  border-bottom-left-radius: 4px;
}

.bubble-pending {
  opacity: 0.6;
}

.bubble-failed {
  background: #3a1a1a;
  border: 1px solid #fe2c55;
}

.msg-failed-hint {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  color: #fe2c55;
}

.resend-btn {
  padding: 2px 8px;
  border-radius: 4px;
  background: rgba(254, 44, 85, 0.2);
  color: #fe2c55;
  font-size: 11px;
}

.msg-time {
  font-size: 10px;
  color: var(--text-dim, #666);
  margin-top: 2px;
}

.chat-input-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 16px calc(10px + env(safe-area-inset-bottom));
  background: #111;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  flex-shrink: 0;
}

.input-wrap {
  flex: 1;
  position: relative;
}

.msg-input {
  width: 100%;
  padding: 10px 50px 10px 14px;
  border-radius: 20px;
  background: #222;
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: #fff;
  font-size: 14px;
  outline: none;
  transition: border-color 0.15s;
}

.msg-input:focus {
  border-color: #fe2c55;
}

.char-count {
  position: absolute;
  right: 12px;
  top: 50%;
  transform: translateY(-50%);
  font-size: 10px;
  color: var(--text-dim, #666);
}

.send-btn {
  padding: 10px 20px;
  border-radius: 20px;
  background: linear-gradient(90deg, #fe2c55, #ff5470);
  color: #fff;
  font-size: 14px;
  font-weight: 700;
  transition: opacity 0.15s;
  flex-shrink: 0;
}

.send-btn:disabled {
  opacity: 0.4;
}

.center {
  flex: 1;
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
