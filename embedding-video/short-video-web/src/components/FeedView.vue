<script setup>
import { computed, onBeforeUnmount, ref } from 'vue'
import { fetchRandomVideoDistinct, uniqueKeyOf } from '../feed/api.js'
import { clampIndex, pushDistinct, resolveSwipeIndex, visibleWindow } from '../feed/swipe.js'
import { getCurrentUserId } from '../user/api.js'
import VideoCard from './VideoCard.vue'

const props = defineProps({
  session: { type: Object, required: true },
})
const emit = defineEmits(['logout', 'navigate'])

const SWIPE_THRESHOLD = 60
const MIN_FETCH_AHEAD = 2

const items = ref([])
const index = ref(0)
const dragDelta = ref(0)
const transitioning = ref(false)
const loading = ref(true)
const loadError = ref('')
const notice = ref('')

const seenKeys = new Set()
let noticeTimer = null
let wheelAccumulator = 0
let wheelLocked = false
let touchStartY = null

const userId = computed(() => props.session.admin.id)
const username = computed(() => props.session.admin.username || '用户')
const avatarText = computed(() => username.value.slice(0, 1).toUpperCase())

const windowRange = computed(() => visibleWindow(index.value, items.value.length))
const windowItems = computed(() => items.value.slice(windowRange.value[0], windowRange.value[1] + 1))
const hasPrev = computed(() => index.value > 0)

function slideOffset(item) {
  const itemIndex = items.value.indexOf(item)
  const containerHeight = typeof window !== 'undefined' ? window.innerHeight : 0
  return (itemIndex - index.value) * containerHeight + dragDelta.value
}

function showNotice(text) {
  notice.value = text
  clearTimeout(noticeTimer)
  noticeTimer = setTimeout(() => {
    notice.value = ''
  }, 2200)
}

async function pushNextItem() {
  try {
    const item = await fetchRandomVideoDistinct(userId.value, seenKeys)
    pushDistinct(seenKeys, uniqueKeyOf(item))
    items.value.push(item)
    return true
  } catch {
    return false
  }
}

async function ensurePrefetched() {
  const needed = index.value + MIN_FETCH_AHEAD + 1
  while (items.value.length < needed) {
    const ok = await pushNextItem()
    if (!ok) {
      showNotice('加载视频失败，请稍后重试')
      return false
    }
  }
  return true
}

async function bootstrap() {
  loading.value = true
  loadError.value = ''
  try {
    for (let i = 0; i < MIN_FETCH_AHEAD; i++) {
      const ok = await pushNextItem()
      if (!ok) throw new Error('fetch failed')
    }
  } catch {
    loadError.value = '暂时没有可播放的视频，请稍后重试'
  } finally {
    loading.value = false
  }
}

function animateTo(targetIndex) {
  const target = clampIndex(targetIndex, items.value.length - 1)
  if (target === index.value) return
  transitioning.value = true
  index.value = target
  dragDelta.value = 0
  window.setTimeout(() => {
    transitioning.value = false
  }, 320)
  void ensurePrefetched()
}

function goNext() {
  animateTo(index.value + 1)
}

function goPrev() {
  animateTo(index.value - 1)
}

function onWheel(event) {
  if (transitioning.value || loading.value) return
  if (wheelLocked) return
  if (Math.abs(event.deltaY) < 4) return
  wheelAccumulator += event.deltaY
  if (Math.abs(wheelAccumulator) < SWIPE_THRESHOLD) return
  wheelAccumulator = 0
  wheelLocked = true
  window.setTimeout(() => {
    wheelLocked = false
  }, 420)
  if (event.deltaY > 0) goNext()
  else goPrev()
}

function onTouchStart(event) {
  if (transitioning.value || loading.value) return
  touchStartY = event.touches[0].clientY
  dragDelta.value = 0
}

function onTouchMove(event) {
  if (touchStartY === null) return
  const delta = event.touches[0].clientY - touchStartY
  const maxIndex = items.value.length - 1
  const blockedUp = index.value <= 0 && delta > 0
  const blockedDown = index.value >= maxIndex && delta < 0
  if (blockedUp || blockedDown) {
    dragDelta.value = delta / 3
  } else {
    dragDelta.value = delta
  }
}

function onTouchEnd() {
  if (touchStartY === null) return
  touchStartY = null
  const target = resolveSwipeIndex(dragDelta.value, index.value, items.value.length - 1, SWIPE_THRESHOLD)
  dragDelta.value = 0
  if (target !== index.value) animateTo(target)
}

function onRetry() {
  loadError.value = ''
  bootstrap()
}

function onCardEnded() {
  goNext()
}

function onLogout() {
  emit('logout')
}

function onMyProfile() {
  emit('navigate', { view: 'profile', userId: getCurrentUserId() })
}

bootstrap()
onBeforeUnmount(() => {
  clearTimeout(noticeTimer)
})
</script>

<template>
  <div class="feed" @wheel.prevent="onWheel">
    <div
      v-if="!loading && !loadError"
      class="stage"
      :class="{ dragging: Math.abs(dragDelta) > 0 }"
      @touchstart.passive="onTouchStart"
      @touchmove.passive="onTouchMove"
      @touchend="onTouchEnd"
      @touchcancel="onTouchEnd"
    >
      <div class="track">
        <div
          v-for="item in windowItems"
          :key="uniqueKeyOf(item)"
          class="slide"
          :style="{
            transform: `translateY(${slideOffset(item)}px)`,
            transition: transitioning ? 'transform 0.3s cubic-bezier(0.25, 0.8, 0.25, 1)' : 'none',
          }"
        >
          <VideoCard
            :item="item"
            :active="items[index] === item"
            :user-id="userId"
            @ended="onCardEnded"
          />
        </div>
      </div>

      <header class="topbar">
        <div class="brand">
          <span class="brand-logo">短</span>
          <span class="brand-name">短视频</span>
        </div>
        <div class="tabs">
          <span class="tab active">推荐</span>
        </div>
        <div class="userbox">
          <button class="my-profile" type="button" aria-label="我的主页" @click="onMyProfile">
            <div class="avatar">{{ avatarText }}</div>
          </button>
          <span class="user-name">{{ username }}</span>
          <button class="logout" type="button" @click="onLogout">退出</button>
        </div>
      </header>

      <button
        v-if="hasPrev"
        class="up-hint"
        type="button"
        aria-label="上一个"
        @click="goPrev"
      >
        ↑
      </button>
    </div>

    <div v-else-if="loading" class="center">
      <div class="spinner"></div>
      <p>正在为你挑选视频…</p>
    </div>

    <div v-else class="center">
      <p class="empty-title">{{ loadError }}</p>
      <button class="retry" type="button" @click="onRetry">重新加载</button>
    </div>

    <transition name="notice">
      <div v-if="notice" class="notice">{{ notice }}</div>
    </transition>
  </div>
</template>

<style scoped>
.feed {
  position: relative;
  height: 100%;
  width: 100%;
  background: #000;
  overflow: hidden;
}

.stage {
  position: absolute;
  inset: 0;
  touch-action: pan-y;
  cursor: grab;
}

.stage.dragging {
  cursor: grabbing;
}

.track {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
}

.slide {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  will-change: transform;
}

.topbar {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: calc(14px + env(safe-area-inset-top)) 18px 14px;
  background: linear-gradient(180deg, rgba(0, 0, 0, 0.55), rgba(0, 0, 0, 0));
  pointer-events: none;
}

.topbar > * {
  pointer-events: auto;
}

.brand {
  display: flex;
  align-items: center;
  gap: 8px;
}

.brand-logo {
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  border-radius: 9px;
  font-size: 15px;
  font-weight: 800;
  color: #fff;
  background: linear-gradient(135deg, #fe2c55, #ff7a59);
  box-shadow: 0 0 18px rgba(254, 44, 85, 0.5);
}

.brand-name {
  font-size: 17px;
  font-weight: 800;
  letter-spacing: 1px;
}

.tabs {
  position: absolute;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  gap: 22px;
}

.tab {
  font-size: 17px;
  font-weight: 600;
  color: var(--text-dim);
  padding-bottom: 6px;
}

.tab.active {
  color: #fff;
  font-weight: 700;
  border-bottom: 3px solid #fff;
}

.userbox {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 6px 6px 6px 12px;
  border-radius: 999px;
  background: rgba(0, 0, 0, 0.45);
  border: 1px solid rgba(255, 255, 255, 0.14);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
}

.avatar {
  width: 28px;
  height: 28px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: linear-gradient(135deg, #25f4ee, #3a86ff);
  font-size: 14px;
  font-weight: 700;
  color: #04252b;
}

.my-profile {
  display: grid;
  place-items: center;
  padding: 0;
  border-radius: 50%;
  transition: transform 0.15s, box-shadow 0.15s;
}

.my-profile:hover {
  transform: scale(1.08);
  box-shadow: 0 0 12px rgba(37, 244, 238, 0.5);
}

.user-name {
  font-size: 13px;
  color: var(--text-dim);
  max-width: 110px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.logout {
  padding: 7px 14px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.1);
  font-size: 12px;
  color: var(--text);
  transition: background 0.15s;
}

.logout:hover {
  background: rgba(254, 44, 85, 0.75);
}

.up-hint {
  position: absolute;
  top: calc(64px + env(safe-area-inset-top));
  left: 50%;
  transform: translateX(-50%);
  width: 34px;
  height: 34px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.14);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  color: #fff;
  font-size: 16px;
  animation: fade-in 0.3s ease both;
}

.center {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
  color: var(--text-dim);
}

.empty-title {
  margin: 0;
  font-size: 15px;
}

.retry {
  padding: 10px 26px;
  border-radius: 999px;
  background: linear-gradient(90deg, #fe2c55, #ff5470);
  color: #fff;
  font-size: 14px;
  font-weight: 700;
}

.spinner {
  width: 34px;
  height: 34px;
  border: 3px solid rgba(255, 255, 255, 0.2);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

.notice {
  position: absolute;
  left: 50%;
  bottom: 14%;
  transform: translateX(-50%);
  padding: 9px 18px;
  border-radius: 999px;
  background: rgba(20, 20, 20, 0.85);
  border: 1px solid rgba(255, 255, 255, 0.12);
  font-size: 13px;
  color: var(--text);
  white-space: nowrap;
}

.notice-enter-active,
.notice-leave-active {
  transition: opacity 0.25s;
}

.notice-enter-from,
.notice-leave-to {
  opacity: 0;
}
</style>
