<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { clampIndex, resolveSwipeIndex, visibleWindow } from '../feed/swipe.js'
import { fetchLikedVideos, fetchUserVideos } from '../user/videoApi.js'
import VideoCard from './VideoCard.vue'

const props = defineProps({
  userId: { type: Number, required: true },
  tabType: { type: String, default: 'works' }, // 'works' | 'liked'
  videos: { type: Array, default: () => [] },
  startIndex: { type: Number, default: 0 },
  ownerName: { type: String, default: '' },
})
const emit = defineEmits(['back'])

const SWIPE_THRESHOLD = 60
const PAGE_SIZE = 12

const list = ref([])
const index = ref(0)
const dragDelta = ref(0)
const transitioning = ref(false)
const total = ref(0)
const loadingMore = ref(false)
const loadMoreFailed = ref(false)

const windowRange = computed(() => visibleWindow(index.value, list.value.length))
const windowItems = computed(() => list.value.slice(windowRange.value[0], windowRange.value[1] + 1))
const hasMore = computed(() => list.value.length < total.value)
const tabLabel = computed(() => (props.tabType === 'liked' ? '喜欢' : '作品'))

function slideOffset(item) {
  const itemIndex = list.value.indexOf(item)
  const containerHeight = typeof window !== 'undefined' ? window.innerHeight : 0
  return (itemIndex - index.value) * containerHeight + dragDelta.value
}

function nextPage() {
  return Math.floor(list.value.length / PAGE_SIZE) + 1
}

async function loadMore() {
  if (loadingMore.value || !hasMore.value) return
  loadingMore.value = true
  loadMoreFailed.value = false
  try {
    const fetcher = props.tabType === 'liked' ? fetchLikedVideos : fetchUserVideos
    const result = await fetcher(props.userId, { page: nextPage(), pageSize: PAGE_SIZE })
    list.value = list.value.concat(result.list)
    total.value = Number(result.total) || list.value.length
    if (result.list.length === 0) {
      total.value = list.value.length
    }
  } catch {
    loadMoreFailed.value = true
  } finally {
    loadingMore.value = false
  }
}

async function goNext() {
  if (index.value >= list.value.length - 1) {
    // 已到列表末尾：有更多则加载后继续，否则停在最后一屏
    if (hasMore.value && !loadingMore.value) {
      await loadMore()
      if (index.value < list.value.length - 1) {
        animateTo(index.value + 1)
      }
    }
    return
  }
  animateTo(index.value + 1)
}

function goPrev() {
  animateTo(index.value - 1)
}

function animateTo(targetIndex) {
  const target = clampIndex(targetIndex, list.value.length - 1)
  if (target === index.value) return
  transitioning.value = true
  index.value = target
  dragDelta.value = 0
  window.setTimeout(() => {
    transitioning.value = false
  }, 320)
  if (index.value >= list.value.length - 2) {
    void loadMore()
  }
}

function onWheel(event) {
  if (transitioning.value) return
  if (Math.abs(event.deltaY) < 4) return
  const delta = event.deltaY > 0 ? 1 : -1
  if (delta > 0) goNext()
  else goPrev()
}

function onTouchStart(event) {
  if (transitioning.value) return
  touchStartY = event.touches[0].clientY
  dragDelta.value = 0
}

let touchStartY = null

function onTouchMove(event) {
  if (touchStartY === null) return
  const delta = event.touches[0].clientY - touchStartY
  const maxIndex = list.value.length - 1
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
  const target = resolveSwipeIndex(dragDelta.value, index.value, list.value.length - 1, SWIPE_THRESHOLD)
  dragDelta.value = 0
  if (target !== index.value) animateTo(target)
}

function onCardEnded() {
  void goNext()
}

function onBack() {
  emit('back')
}

onMounted(() => {
  list.value = [...props.videos]
  index.value = clampIndex(props.startIndex, Math.max(0, list.value.length - 1))
  total.value = list.value.length
  if (props.videos.length >= PAGE_SIZE) {
    void loadMore()
  }
})

onBeforeUnmount(() => {
  touchStartY = null
})
</script>

<template>
  <div class="playlist" @wheel.prevent="onWheel">
    <div
      v-if="list.length > 0"
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
          :key="`${tabType}-${item.id}`"
          class="slide"
          :style="{
            transform: `translateY(${slideOffset(item)}px)`,
            transition: transitioning ? 'transform 0.3s cubic-bezier(0.25, 0.8, 0.25, 1)' : 'none',
          }"
        >
          <VideoCard
            :item="item"
            :active="list[index] === item"
            :user-id="userId"
            @ended="onCardEnded"
          />
        </div>
      </div>

      <header class="topbar">
        <button class="back-btn" type="button" aria-label="返回" @click="onBack">←</button>
        <span class="topbar-title">{{ ownerName }}的{{ tabLabel }}</span>
        <span class="topbar-spacer"></span>
      </header>

      <div v-if="loadingMore" class="more-tip">
        <span class="more-spinner"></span>
        <span>加载更多…</span>
      </div>
      <div v-else-if="loadMoreFailed" class="more-tip" role="button" tabindex="0" @click="loadMore">
        <span>加载失败，点此重试</span>
      </div>
      <div v-else-if="!hasMore" class="more-tip end-tip">
        <span>— 已播放完{{ tabLabel }}列表 —</span>
      </div>
    </div>

    <div v-else class="empty">
      <div class="empty-icon">🎬</div>
      <p>暂无{{ tabLabel }}视频</p>
      <button class="empty-back" type="button" @click="onBack">返回</button>
    </div>
  </div>
</template>

<style scoped>
.playlist {
  height: 100%;
  background: #000;
  overflow: hidden;
  color: #fff;
}

.stage {
  height: 100%;
  position: relative;
  overflow: hidden;
}

.track {
  position: absolute;
  inset: 0;
}

.slide {
  height: 100%;
  width: 100%;
  will-change: transform;
}

.topbar {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  z-index: 20;
  display: flex;
  align-items: center;
  padding: 10px 12px;
  padding-top: calc(10px + env(safe-area-inset-top));
  background: linear-gradient(180deg, rgba(0, 0, 0, 0.55), transparent);
  pointer-events: none;
}

.back-btn {
  pointer-events: auto;
  width: 34px;
  height: 34px;
  border: none;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.16);
  color: #fff;
  font-size: 18px;
  line-height: 1;
  cursor: pointer;
}

.topbar-title {
  flex: 1;
  text-align: center;
  font-size: 16px;
  font-weight: 600;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  padding: 0 8px;
}

.topbar-spacer {
  width: 34px;
}

.more-tip {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  z-index: 15;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 10px 0 14px;
  padding-bottom: calc(14px + env(safe-area-inset-bottom));
  font-size: 12px;
  color: rgba(255, 255, 255, 0.65);
  background: linear-gradient(0deg, rgba(0, 0, 0, 0.5), transparent);
}

.end-tip {
  color: rgba(255, 255, 255, 0.45);
}

.more-spinner {
  width: 12px;
  height: 12px;
  border: 2px solid rgba(255, 255, 255, 0.35);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.empty {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
}

.empty-icon {
  font-size: 48px;
  opacity: 0.6;
}

.empty p {
  margin: 0;
  font-size: 15px;
  color: rgba(255, 255, 255, 0.6);
}

.empty-back {
  padding: 8px 24px;
  border: none;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.16);
  color: #fff;
  font-size: 14px;
  cursor: pointer;
}
</style>
