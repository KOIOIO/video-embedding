<script setup>
import { computed, ref } from 'vue'
import { followUser, unfollowUser, getCurrentUserId } from '../user/api.js'

const props = defineProps({
  userId: { type: Number, required: true },
  relation: { type: String, default: 'none' },
})

const emit = defineEmits(['change'])

const loading = ref(false)
const errorMsg = ref('')

const isOwn = computed(() => Number(props.userId) === getCurrentUserId())

const buttonLabel = computed(() => {
  switch (props.relation) {
    case 'following': return '已关注'
    case 'mutual': return '互相关注'
    default: return '关注'
  }
})

const buttonClass = computed(() => {
  if (props.relation === 'none') return 'follow-btn follow-btn--active'
  return 'follow-btn follow-btn--idle'
})

async function onClick() {
  if (loading.value || isOwn.value) return
  loading.value = true
  errorMsg.value = ''
  try {
    if (props.relation === 'none') {
      await followUser(props.userId)
      emit('change', 'following')
    } else {
      await unfollowUser(props.userId)
      emit('change', 'none')
    }
  } catch (err) {
    errorMsg.value = err?.message || '操作失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="follow-btn-wrap">
    <button
      v-if="!isOwn"
      :class="buttonClass"
      type="button"
      :disabled="loading"
      @click="onClick"
    >
      <span v-if="loading" class="btn-spinner"></span>
      <span v-else>{{ buttonLabel }}</span>
    </button>
    <p v-if="errorMsg" class="follow-error">{{ errorMsg }}</p>
  </div>
</template>

<style scoped>
.follow-btn-wrap {
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

.follow-btn {
  padding: 8px 24px;
  border-radius: 999px;
  font-size: 14px;
  font-weight: 700;
  transition: opacity 0.15s, background 0.15s;
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.follow-btn--active {
  background: linear-gradient(90deg, #fe2c55, #ff5470);
  color: #fff;
}

.follow-btn--active:hover {
  opacity: 0.85;
}

.follow-btn--idle {
  background: rgba(255, 255, 255, 0.12);
  color: #ccc;
}

.follow-btn--idle:hover {
  background: rgba(255, 255, 255, 0.2);
}

.follow-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.follow-error {
  margin: 0;
  font-size: 11px;
  color: #fe2c55;
}
</style>
