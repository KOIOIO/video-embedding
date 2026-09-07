<script setup>
import Hls from 'hls.js'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  fetchReactionCounts,
  REACTION_DISLIKE,
  REACTION_DOUBLE_LIKE,
  REACTION_LIKE,
  submitReaction,
} from '../feed/api.js'
import { fetchCommentCounts } from '../feed/commentApi.js'
import { fetchRelation, followUser, getCurrentUserId, unfollowUser } from '../user/api.js'
import CommentsPanel from './CommentsPanel.vue'

const props = defineProps({
  item: { type: Object, required: true },
  active: { type: Boolean, default: false },
  userId: { type: Number, default: 0 },
  highlightCommentId: { type: Number, default: 0 },
})
const emit = defineEmits(['ended', 'navigate'])

const videoEl = ref(null)
const coverVisible = ref(true)
const playing = ref(false)
const muted = ref(false)
const progress = ref(0)
const errorText = ref('')
const activeReaction = ref(String(props.item.user_reaction_type || ''))
const likeCount = ref(0)
const doubleLikeCount = ref(0)
const dislikeCount = ref(0)
const reactionBusy = ref(false)
const burstVisible = ref(false)
const floatHearts = ref([])
const commentCount = ref(0)
const showComments = ref(false)
const authorRelation = ref('none')
const authorBusy = ref(false)
const shareVisible = ref(false)

const authorId = computed(() => Number(props.item?.author_id || 0) || 0)
const isOwnVideo = computed(() => authorId.value > 0 && authorId.value === getCurrentUserId())
const authorAvatarText = computed(() => {
  // 用 author_id 生成一个字母作为默认头像
  const id = authorId.value || 1
  return String.fromCharCode(65 + (id % 26))
})

const isHlsSrc = computed(() => String(props.item.play_url || '').toLowerCase().includes('.m3u8'))
const segmentMode = computed(() => {
  const start = Number(props.item.start_time_sec || 0)
  const end = Number(props.item.end_time_sec || 0)
  return end > start
})

let hls = null
let burstTimer = null
let heartId = 0
let lastTapAt = 0
let singleTapTimer = null

function emitEnded() {
  if (!props.active) return
  emit('ended')
}

function onTimeUpdate() {
  const video = videoEl.value
  if (!video || !video.duration) return
  if (segmentMode.value) {
    const start = Number(props.item.start_time_sec || 0)
    const end = Number(props.item.end_time_sec || 0)
    progress.value = Math.min(1, Math.max(0, (video.currentTime - start) / (end - start)))
    if (video.currentTime >= end) {
      video.pause()
      emitEnded()
    }
    return
  }
  progress.value = Math.min(1, Math.max(0, video.currentTime / video.duration))
}

function destroyPlayer() {
  clearTimeout(singleTapTimer)
  singleTapTimer = null
  const video = videoEl.value
  if (video) {
    video.pause()
    video.removeAttribute('src')
    try {
      video.load()
    } catch {
    }
  }
  if (hls) {
    hls.destroy()
    hls = null
  }
  coverVisible.value = true
  playing.value = false
  progress.value = 0
  errorText.value = ''
  muted.value = false
}

function setupPlayer() {
  destroyPlayer()
  if (!props.active || !props.item.play_url) return
  const video = videoEl.value
  if (!video) return
  const src = props.item.play_url

  video.onloadedmetadata = () => {
    if (segmentMode.value) {
      try {
        video.currentTime = Number(props.item.start_time_sec || 0)
      } catch {
      }
    }
    void tryPlay()
  }
  video.ontimeupdate = onTimeUpdate
  video.onplaying = () => {
    coverVisible.value = false
    playing.value = true
  }
  video.onpause = () => {
    playing.value = false
  }
  video.onended = () => {
    if (!segmentMode.value) emitEnded()
  }
  video.onerror = () => {
    errorText.value = '视频加载失败'
  }

  const tryHls = () => {
    hls = new Hls({ enableWorker: true })
    hls.on(Hls.Events.ERROR, (_, data) => {
      if (data?.fatal) {
        errorText.value = '视频加载失败'
        destroyPlayer()
      }
    })
    hls.loadSource(src)
    hls.attachMedia(video)
  }

  if (isHlsSrc.value) {
    if (video.canPlayType('application/vnd.apple.mpegurl')) {
      video.src = src
    } else if (Hls.isSupported()) {
      tryHls()
    } else {
      errorText.value = '当前浏览器不支持 HLS 播放'
    }
  } else {
    video.src = src
  }
}

async function tryPlay() {
  const video = videoEl.value
  if (!video) return
  try {
    await video.play()
  } catch {
    muted.value = true
    video.muted = true
    try {
      await video.play()
    } catch {
    }
  }
}

function togglePlay() {
  const video = videoEl.value
  if (!video) return
  if (video.paused) void tryPlay()
  else video.pause()
}

function toggleMute() {
  const video = videoEl.value
  if (!video) return
  muted.value = !muted.value
  video.muted = muted.value
}

function popCenterHeart() {
  burstVisible.value = true
  clearTimeout(burstTimer)
  burstTimer = setTimeout(() => {
    burstVisible.value = false
  }, 900)
}

function popFloatHeart() {
  const id = ++heartId
  floatHearts.value.push(id)
  setTimeout(() => {
    floatHearts.value = floatHearts.value.filter((x) => x !== id)
  }, 900)
}

function onTap() {
  const now = Date.now()
  if (now - lastTapAt < 320) {
    clearTimeout(singleTapTimer)
    singleTapTimer = null
    lastTapAt = 0
    if (activeReaction.value === REACTION_LIKE) popCenterHeart()
    else void react(REACTION_LIKE)
    return
  }
  lastTapAt = now
  singleTapTimer = setTimeout(() => {
    singleTapTimer = null
    togglePlay()
  }, 320)
}

function applyReactionChange(type, wasActive) {
  if (wasActive) {
    activeReaction.value = ''
    if (type === REACTION_LIKE) likeCount.value = Math.max(0, likeCount.value - 1)
    if (type === REACTION_DOUBLE_LIKE) doubleLikeCount.value = Math.max(0, doubleLikeCount.value - 1)
    if (type === REACTION_DISLIKE) dislikeCount.value = Math.max(0, dislikeCount.value - 1)
    return
  }
  const previous = activeReaction.value
  if (previous === REACTION_LIKE) likeCount.value = Math.max(0, likeCount.value - 1)
  if (previous === REACTION_DOUBLE_LIKE) doubleLikeCount.value = Math.max(0, doubleLikeCount.value - 1)
  if (previous === REACTION_DISLIKE) dislikeCount.value = Math.max(0, dislikeCount.value - 1)
  activeReaction.value = type
  if (type === REACTION_LIKE) {
    likeCount.value += 1
    popCenterHeart()
    popFloatHeart()
  }
  if (type === REACTION_DOUBLE_LIKE) doubleLikeCount.value += 1
  if (type === REACTION_DISLIKE) dislikeCount.value += 1
}

async function react(reactionType) {
  if (reactionBusy.value) return
  const previous = {
    type: activeReaction.value,
    like: likeCount.value,
    doubleLike: doubleLikeCount.value,
    dislike: dislikeCount.value,
  }
  applyReactionChange(reactionType, activeReaction.value === reactionType)
  reactionBusy.value = true
  try {
    const result = await submitReaction({ userId: props.userId, item: props.item, reactionType })
    activeReaction.value = result.active ? result.reaction_type : ''
    likeCount.value = result.like_count
    doubleLikeCount.value = result.double_like_count
    dislikeCount.value = result.dislike_count || 0
  } catch {
    activeReaction.value = previous.type
    likeCount.value = previous.like
    doubleLikeCount.value = previous.doubleLike
    dislikeCount.value = previous.dislike
  } finally {
    reactionBusy.value = false
  }
}

async function loadReactionCounts() {
  try {
    const counts = await fetchReactionCounts(props.item)
    likeCount.value = counts.like_count
    doubleLikeCount.value = counts.double_like_count
    dislikeCount.value = counts.dislike_count || 0
  } catch {
    likeCount.value = 0
    doubleLikeCount.value = 0
    dislikeCount.value = 0
  }
}

async function loadCommentCount() {
  try {
    commentCount.value = await fetchCommentCounts(props.item.video_segment_id)
  } catch {
    commentCount.value = 0
  }
}

function openComments() {
  showComments.value = true
  const video = videoEl.value
  if (video) video.pause()
}

async function loadAuthorRelation() {
  if (!authorId.value || isOwnVideo.value) return
  try {
    authorRelation.value = await fetchRelation(authorId.value)
  } catch {
    authorRelation.value = 'none'
  }
}

async function toggleFollow() {
  if (authorBusy.value || !authorId.value || isOwnVideo.value) return
  authorBusy.value = true
  try {
    if (authorRelation.value === 'none') {
      await followUser(authorId.value)
      authorRelation.value = 'following'
    } else {
      await unfollowUser(authorId.value)
      authorRelation.value = 'none'
    }
  } catch {
    // 静默失败
  } finally {
    authorBusy.value = false
  }
}

function onAuthorClick() {
  if (!authorId.value) return
  emit('navigate', { view: 'profile', userId: authorId.value })
}

function onShare() {
  shareVisible.value = true
  setTimeout(() => {
    shareVisible.value = false
  }, 1800)
}

watch(() => props.active, (active) => {
  if (active) {
    setupPlayer()
    return
  }
  destroyPlayer()
})

watch(() => props.item.play_url, () => {
  if (props.active) setupPlayer()
})

onMounted(() => {
  loadReactionCounts()
  loadCommentCount()
  loadAuthorRelation()
  if (props.active) setupPlayer()
  if (props.highlightCommentId > 0) {
    setTimeout(() => openComments(), 300)
  }
})
onBeforeUnmount(() => {
  destroyPlayer()
  clearTimeout(burstTimer)
})
</script>

<template>
  <div class="card" @click="onTap">
    <video
      ref="videoEl"
      class="video"
      playsinline
      webkit-playsinline
      preload="auto"
      :poster="item.cover_url"
    ></video>

    <img
      v-if="coverVisible && item.cover_url"
      class="cover"
      :src="item.cover_url"
      alt=""
      draggable="false"
    />

    <div v-if="errorText" class="error">{{ errorText }}</div>

    <div v-if="!playing && !coverVisible && !errorText" class="play-hint">
      <svg viewBox="0 0 24 24" fill="currentColor"><path d="M8 5.14v13.72c0 .8.87 1.3 1.56.88l10.54-6.86a1.05 1.05 0 0 0 0-1.76L9.56 4.26A1.04 1.04 0 0 0 8 5.14Z"/></svg>
    </div>

    <button
      v-if="muted && playing"
      class="mute-chip"
      type="button"
      @click.stop="toggleMute"
    >
      <svg viewBox="0 0 24 24" fill="currentColor"><path d="M3 10v4a1 1 0 0 0 1 1h3l4.29 3.43A1 1 0 0 0 13 17.66V6.34a1 1 0 0 0-1.71-.7L7 9H4a1 1 0 0 0-1 1Z"/><path d="M16.24 8.34a.75.75 0 0 1 1.06-.03 5.25 5.25 0 0 1 0 7.38.75.75 0 0 1-1.09-1.03 3.75 3.75 0 0 0 0-5.23.75.75 0 0 1 .03-1.09Z"/><path d="M19 5.62a.75.75 0 0 1 1.06.04 9.5 9.5 0 0 1 0 12.68.75.75 0 0 1-1.09-1.03 8 8 0 0 0 0-10.62.75.75 0 0 1 .03-1.07Z"/></svg>
      点击开启声音
    </button>

    <div class="info">
      <p class="caption">{{ item.title || '随机推荐视频' }}</p>
      <div class="music">
        <span class="music-track">
          <svg class="music-note" viewBox="0 0 24 24" fill="currentColor"><path d="M9 18.5a3.5 3.5 0 1 1-2-3.16V5.5a1 1 0 0 1 1.13-.99l9 1.29A1 1 0 0 1 18 6.78v8.72a3.5 3.5 0 1 1-2-3.16V8.35l-7-1v11.15Z"/></svg>
          <span class="music-text">♪ 随机推荐 · 上下滑动浏览更多 ♪ 随机推荐 · 上下滑动浏览更多 ♪</span>
        </span>
      </div>
    </div>

    <aside class="rail">
      <!-- 作者头像 -->
      <div v-if="authorId" class="rail-author">
        <button class="author-avatar-btn" type="button" @click.stop="onAuthorClick">
          <div class="author-avatar">{{ authorAvatarText }}</div>
        </button>
        <button
          v-if="!isOwnVideo"
          class="author-follow-btn"
          :class="{ followed: authorRelation !== 'none', busy: authorBusy }"
          type="button"
          @click.stop="toggleFollow"
        >
          <span v-if="authorRelation === 'none'">＋</span>
          <span v-else>✓</span>
        </button>
      </div>

      <button class="rail-btn" type="button" @click.stop="react(REACTION_LIKE)">
        <span class="rail-icon" :class="{ active: activeReaction === REACTION_LIKE }">
          <svg viewBox="0 0 24 24" fill="currentColor">
            <path d="M12 21s-7.5-4.7-10-9.3C.6 8.5 2.5 4.6 6.1 4.6c2 0 3.4 1 4.2 2.2.3.5 1.1.5 1.4 0 .8-1.2 2.2-2.2 4.2-2.2 3.6 0 5.5 3.9 4.1 7.1C19.5 16.3 12 21 12 21Z"/>
          </svg>
        </span>
        <span class="rail-count" :class="{ active: activeReaction === REACTION_LIKE }">{{ likeCount }}</span>
        <span
          v-for="id in floatHearts"
          :key="id"
          class="float-heart"
        >
          <svg viewBox="0 0 24 24" fill="#fe2c55"><path d="M12 21s-7.5-4.7-10-9.3C.6 8.5 2.5 4.6 6.1 4.6c2 0 3.4 1 4.2 2.2.3.5 1.1.5 1.4 0 .8-1.2 2.2-2.2 4.2-2.2 3.6 0 5.5 3.9 4.1 7.1C19.5 16.3 12 21 12 21Z"/></svg>
        </span>
      </button>

      <button class="rail-btn" type="button" @click.stop="react(REACTION_DOUBLE_LIKE)">
        <span class="rail-icon" :class="{ active: activeReaction === REACTION_DOUBLE_LIKE }">
          <svg viewBox="0 0 24 24" fill="currentColor">
            <path transform="translate(1.8 1.2) scale(0.7)" d="M12 21s-7.5-4.7-10-9.3C.6 8.5 2.5 4.6 6.1 4.6c2 0 3.4 1 4.2 2.2.3.5 1.1.5 1.4 0 .8-1.2 2.2-2.2 4.2-2.2 3.6 0 5.5 3.9 4.1 7.1C19.5 16.3 12 21 12 21Z"/>
            <path transform="translate(-1.8 0.4) scale(0.7)" d="M12 21s-7.5-4.7-10-9.3C.6 8.5 2.5 4.6 6.1 4.6c2 0 3.4 1 4.2 2.2.3.5 1.1.5 1.4 0 .8-1.2 2.2-2.2 4.2-2.2 3.6 0 5.5 3.9 4.1 7.1C19.5 16.3 12 21 12 21Z"/>
          </svg>
        </span>
        <span class="rail-count" :class="{ active: activeReaction === REACTION_DOUBLE_LIKE }">{{ doubleLikeCount }}</span>
      </button>

      <button class="rail-btn" type="button" @click.stop="react(REACTION_DISLIKE)">
        <span class="rail-icon" :class="{ active: activeReaction === REACTION_DISLIKE }">
          <svg viewBox="0 0 24 24" fill="currentColor">
            <path transform="rotate(180 12 12)" d="M7 10v11a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V10a1 1 0 0 1 1-1h1a1 1 0 0 1 1 1Zm13.8.8c-.3-1.2-1.4-2-2.7-2h-3.7a.3.3 0 0 1-.3-.4l.7-3.6a3 3 0 0 0-2.9-3.4 2 2 0 0 0-1.8 1.2l-2.5 5.2a3 3 0 0 0-.3 1.2v6.5a3 3 0 0 0 3 3h6.4a3 3 0 0 0 2.9-2.4l1.2-4.3v-1Z"/>
          </svg>
        </span>
        <span class="rail-count" :class="{ active: activeReaction === REACTION_DISLIKE }">{{ dislikeCount || '倒赞' }}</span>
      </button>

      <button class="rail-btn" type="button" @click.stop="openComments">
        <span class="rail-icon">
          <svg viewBox="0 0 24 24" fill="currentColor"><path d="M12 2.6A9.4 9.4 0 0 0 2.6 12c0 3.8 2.3 7.2 5.9 8.7.4.1.6-.2.6-.4v-1.5c-2.4.5-2.9-1.2-2.9-1.2-.4-1-1-1.3-1-1.3-.8-.5.1-.5.1-.5.9.1 1.3.9 1.3.9.8 1.3 2 1 2.5.7.1-.5.3-.9.6-1.1-2-.2-4-.9-4-4.2 0-.9.3-1.7.9-2.3-.1-.2-.4-1.1.1-2.3 0 0 .7-.2 2.4.9a8.2 8.2 0 0 1 4.3 0c1.7-1.1 2.4-.9 2.4-.9.5 1.2.2 2.1.1 2.3.6.6.9 1.4.9 2.3 0 3.3-2 4-4 4.2.3.3.5.7.5 1.2v1.7c0 .2.2.5.6.4a9.4 9.4 0 0 0 5.9-8.7A9.4 9.4 0 0 0 12 2.6Z"/></svg>
        </span>
        <span class="rail-count">{{ commentCount }}</span>
      </button>

      <button class="rail-btn" type="button" @click.stop="onShare">
        <span class="rail-icon">
          <svg viewBox="0 0 24 24" fill="currentColor"><path d="M14 9V5l7 7-7 7v-4.1c-5 0-8.5 1.6-11 5.1 1-5 4-10 11-11Z"/></svg>
        </span>
        <span class="rail-count">分享</span>
      </button>
    </aside>

    <transition name="share-toast">
      <div v-if="shareVisible" class="share-toast">链接已复制</div>
    </transition>

    <div v-if="burstVisible" class="burst">
      <svg viewBox="0 0 24 24" fill="#fe2c55">
        <path d="M12 21s-7.5-4.7-10-9.3C.6 8.5 2.5 4.6 6.1 4.6c2 0 3.4 1 4.2 2.2.3.5 1.1.5 1.4 0 .8-1.2 2.2-2.2 4.2-2.2 3.6 0 5.5 3.9 4.1 7.1C19.5 16.3 12 21 12 21Z"/>
      </svg>
    </div>

    <div class="progress-track">
      <div class="progress-fill" :style="{ width: `${progress * 100}%` }"></div>
    </div>

    <CommentsPanel
      v-if="showComments"
      :segment-id="item.video_segment_id"
      :user-id="userId"
      :highlight-comment-id="highlightCommentId"
      @close="showComments = false"
      @total-change="commentCount = $event"
      @navigate="(target) => emit('navigate', target)"
    />
  </div>
</template>

<style scoped>
.card {
  position: relative;
  height: 100%;
  width: 100%;
  overflow: hidden;
  background: #000;
  user-select: none;
  -webkit-user-select: none;
  -webkit-tap-highlight-color: transparent;
}

.video,
.cover {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}

.video {
  object-fit: contain;
  background: #000;
}

.cover {
  object-fit: cover;
  filter: blur(6px) brightness(0.55);
  transform: scale(1.12);
}

.error {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  color: var(--text-dim);
  font-size: 14px;
}

.play-hint {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  pointer-events: none;
}

.play-hint svg {
  width: 72px;
  height: 72px;
  color: rgba(255, 255, 255, 0.75);
  filter: drop-shadow(0 4px 18px rgba(0, 0, 0, 0.6));
}

.mute-chip {
  position: absolute;
  top: 16%;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: 999px;
  background: rgba(0, 0, 0, 0.5);
  border: 1px solid rgba(255, 255, 255, 0.2);
  color: #fff;
  font-size: 13px;
  animation: fade-in 0.3s ease both;
}

.mute-chip svg {
  width: 16px;
  height: 16px;
}

.info {
  position: absolute;
  left: 16px;
  right: 92px;
  bottom: calc(76px + env(safe-area-inset-bottom));
  display: flex;
  flex-direction: column;
  gap: 7px;
  pointer-events: none;
}

.caption {
  margin: 0;
  font-size: 14px;
  line-height: 1.5;
  color: rgba(255, 255, 255, 0.92);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  text-shadow: 0 1px 8px rgba(0, 0, 0, 0.7);
}

.music {
  overflow: hidden;
  max-width: 100%;
}

.music-track {
  display: flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
  width: max-content;
  animation: marquee 8s linear infinite;
}

.music-note {
  width: 14px;
  height: 14px;
  color: #fff;
}

.music-text {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.85);
}

.rail {
  position: absolute;
  right: 10px;
  bottom: calc(72px + env(safe-area-inset-bottom));
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 18px;
}

.rail-btn {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 5px;
  min-width: 52px;
}

.rail-icon {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  color: #fff;
  filter: drop-shadow(0 2px 6px rgba(0, 0, 0, 0.55));
  transition: transform 0.15s, color 0.15s;
}

.rail-icon svg {
  width: 34px;
  height: 34px;
}

.rail-btn:active .rail-icon {
  transform: scale(0.88);
}

.rail-icon.active {
  color: var(--accent);
  animation: icon-pop 0.35s ease both;
}

.rail-count {
  font-size: 12px;
  color: #fff;
  text-shadow: 0 1px 6px rgba(0, 0, 0, 0.7);
}

.rail-count.active {
  color: var(--accent);
}

.float-heart {
  position: absolute;
  top: 4px;
  left: 50%;
  transform: translateX(-50%);
  animation: heart-float 0.9s ease-out both;
  pointer-events: none;
}

.float-heart svg {
  width: 22px;
  height: 22px;
}

.burst {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  pointer-events: none;
}

.burst svg {
  width: 128px;
  height: 128px;
  filter: drop-shadow(0 8px 30px rgba(254, 44, 85, 0.6));
  animation: heart-pop 0.9s ease both;
}

.progress-track {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 3px;
  background: rgba(255, 255, 255, 0.22);
}

.progress-fill {
  height: 100%;
  background: linear-gradient(90deg, #25f4ee, #fe2c55);
  transition: width 0.2s linear;
}

/* 作者头像 */
.rail-author {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  margin-bottom: 8px;
}

.author-avatar-btn {
  padding: 0;
  border-radius: 50%;
  transition: transform 0.15s;
}

.author-avatar-btn:active {
  transform: scale(0.92);
}

.author-avatar {
  width: 44px;
  height: 44px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: linear-gradient(135deg, #fe2c55, #ff7a59);
  border: 2px solid #fff;
  font-size: 18px;
  font-weight: 700;
  color: #fff;
}

.author-follow-btn {
  width: 22px;
  height: 22px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: #fe2c55;
  color: #fff;
  font-size: 16px;
  font-weight: 700;
  line-height: 1;
  margin-top: -14px;
  border: 2px solid #000;
  transition: background 0.15s, transform 0.15s;
}

.author-follow-btn.followed {
  background: rgba(255, 255, 255, 0.25);
  font-size: 12px;
}

.author-follow-btn:active {
  transform: scale(0.88);
}

.author-follow-btn.busy {
  opacity: 0.5;
  pointer-events: none;
}

/* 分享 toast */
.share-toast {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  padding: 10px 24px;
  border-radius: 8px;
  background: rgba(0, 0, 0, 0.8);
  color: #fff;
  font-size: 14px;
  z-index: 50;
}

.share-toast-enter-active,
.share-toast-leave-active {
  transition: opacity 0.2s;
}

.share-toast-enter-from,
.share-toast-leave-to {
  opacity: 0;
}
</style>
