<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { fetchFollowers, fetchFollowing, fetchRelation, getCurrentUserId } from '../user/api.js'
import FollowButton from './FollowButton.vue'

const props = defineProps({
  userId: { type: Number, required: true },
  type: { type: String, default: 'following' }, // 'following' | 'followers'
})

const emit = defineEmits(['back'])

const list = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const loading = ref(false)
const loadingMore = ref(false)
const loadError = ref('')
const avatarFailed = ref({})

const title = computed(() => (props.type === 'followers' ? '粉丝' : '关注'))
const hasMore = computed(() => list.value.length < total.value)

async function loadList(reset = false) {
  if (loading.value || loadingMore.value) return
  if (reset) {
    list.value = []
    page.value = 1
    total.value = 0
  }
  if (reset) {
    loading.value = true
  } else {
    loadingMore.value = true
  }
  loadError.value = ''
  try {
    const fetcher = props.type === 'followers' ? fetchFollowers : fetchFollowing
    const result = await fetcher(props.userId, page.value, pageSize)
    // 为每个条目查询当前登录用户与其的关系
    const currentUserId = getCurrentUserId()
    const items = await Promise.all(result.list.map(async (item) => {
      const targetId = props.type === 'followers' ? item.follower_id : item.following_id
      let rel = 'none'
      if (targetId && targetId !== currentUserId) {
        try {
          rel = await fetchRelation(targetId)
        } catch {
          rel = 'none'
        }
      }
      return { ...item, relation: rel, targetId }
    }))
    list.value = list.value.concat(items)
    total.value = result.total
    if (items.length < pageSize || list.value.length >= total.value) {
      // no more
    } else {
      page.value++
    }
  } catch {
    loadError.value = '加载失败，请稍后重试'
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

function onBack() {
  emit('back')
}

function onRelationChange(index, newRelation) {
  if (list.value[index]) {
    list.value[index].relation = newRelation
  }
}

function onAvatarError(index) {
  avatarFailed.value[index] = true
}

onMounted(() => {
  loadList(true)
})

watch(() => [props.userId, props.type], () => {
  loadList(true)
})
</script>

<template>
  <div class="follow-list">
    <header class="topbar">
      <button class="back-btn" type="button" aria-label="返回" @click="onBack">←</button>
      <span class="topbar-title">{{ title }}</span>
      <span class="topbar-spacer"></span>
    </header>

    <div v-if="loading" class="center">
      <div class="spinner"></div>
      <p>加载中…</p>
    </div>

    <div v-else-if="loadError && list.length === 0" class="center">
      <p class="error-text">{{ loadError }}</p>
      <button class="retry" type="button" @click="loadList(true)">重试</button>
    </div>

    <div v-else-if="list.length === 0" class="center">
      <p class="empty-text">{{ type === 'followers' ? '暂无粉丝' : '暂无关注' }}</p>
    </div>

    <div v-else class="list-content">
      <div
        v-for="(item, index) in list"
        :key="item.id || index"
        class="list-item"
      >
        <div class="item-avatar">
          <img
            v-if="item.avatar_url && !avatarFailed[index]"
            :src="item.avatar_url"
            :alt="item.nickname"
            @error="onAvatarError(index)"
          />
          <div v-else class="avatar-fallback">{{ (item.nickname || '?').slice(0, 1).toUpperCase() }}</div>
        </div>
        <div class="item-info">
          <span class="item-nickname">{{ item.nickname || `用户${item.targetId}` }}</span>
          <span v-if="item.bio" class="item-bio">{{ item.bio }}</span>
        </div>
        <FollowButton
          v-if="item.targetId && item.targetId !== getCurrentUserId()"
          :user-id="item.targetId"
          :relation="item.relation"
          @change="(r) => onRelationChange(index, r)"
        />
      </div>

      <div v-if="hasMore" class="load-more-wrap">
        <button
          class="load-more-btn"
          type="button"
          :disabled="loadingMore"
          @click="loadList(false)"
        >
          {{ loadingMore ? '加载中…' : '加载更多' }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.follow-list {
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
}

.topbar-spacer {
  width: 36px;
}

.list-content {
  padding: 8px 0 40px;
}

.list-item {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 12px 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}

.item-avatar img {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  object-fit: cover;
  border: 2px solid rgba(255, 255, 255, 0.1);
}

.avatar-fallback {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  background: linear-gradient(135deg, #333, #555);
  font-size: 20px;
  font-weight: 700;
  color: #ccc;
}

.item-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.item-nickname {
  font-size: 15px;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.item-bio {
  font-size: 12px;
  color: var(--text-dim, #999);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
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

.load-more-btn:disabled {
  opacity: 0.5;
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
