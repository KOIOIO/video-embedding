<script setup>
import { computed, onMounted, ref } from 'vue'
import VideoCard from './VideoCard.vue'
import { authHeaders, requestJson } from '../user/api.js'

const props = defineProps({
  videoId: { type: Number, default: 0 },
  segmentId: { type: Number, default: 0 },
  highlightCommentId: { type: Number, default: 0 },
})

const emit = defineEmits(['back', 'navigate'])

const loading = ref(true)
const loadError = ref('')
const item = ref(null)

async function loadVideo() {
  if (!props.videoId) {
    loadError.value = '无效的视频ID'
    loading.value = false
    return
  }
  loading.value = true
  loadError.value = ''
  try {
    const data = await requestJson(`/api/videos/${encodeURIComponent(String(props.videoId))}/play`, {
      headers: authHeaders(),
    })
    const video = data?.video || {}
    const playURL = data?.play_url || video?.hls_url || video?.raw_url || ''
    item.value = {
      video_id: Number(video?.video_id || props.videoId) || props.videoId,
      video_segment_id: props.segmentId,
      play_url: playURL,
      cover_url: String(video?.cover_url || ''),
      title: String(video?.title || ''),
      description: String(video?.description || ''),
      author_id: 0,
      user_reacted: false,
      user_reaction_type: '',
      like_count: 0,
      double_like_count: 0,
      comment_count: 0,
    }
  } catch (err) {
    loadError.value = '视频加载失败，请稍后重试'
  } finally {
    loading.value = false
  }
}

function onBack() {
  emit('back')
}

function onNavigate(target) {
  emit('navigate', target)
}

onMounted(() => {
  loadVideo()
})
</script>

<template>
  <div class="video-view">
    <button class="back-btn" type="button" aria-label="返回" @click="onBack">←</button>

    <div v-if="loading" class="center">
      <div class="spinner"></div>
      <p>加载中…</p>
    </div>

    <div v-else-if="loadError || !item" class="center">
      <p class="error-text">{{ loadError || '视频不存在' }}</p>
      <button class="retry" type="button" @click="onBack">返回</button>
    </div>

    <VideoCard
      v-else
      :item="item"
      :highlight-comment-id="highlightCommentId"
      :autoplay="true"
      @navigate="onNavigate"
    />
  </div>
</template>

<style scoped>
.video-view {
  position: relative;
  width: 100%;
  height: 100%;
  background: #000;
  color: #fff;
  overflow: hidden;
}

.back-btn {
  position: absolute;
  top: calc(12px + env(safe-area-inset-top));
  left: 16px;
  z-index: 20;
  width: 36px;
  height: 36px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.5);
  font-size: 18px;
  color: #fff;
  backdrop-filter: blur(8px);
  transition: background 0.15s;
}

.back-btn:hover {
  background: rgba(254, 44, 85, 0.6);
}

.center {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 14px;
  color: #999;
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
