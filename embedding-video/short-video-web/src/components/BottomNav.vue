<script setup>
const props = defineProps({
  active: { type: String, default: 'feed' },
  unreadCount: { type: Number, default: 0 },
})

const emit = defineEmits(['navigate'])

function go(view) {
  if (props.active === view) return
  emit('navigate', { view })
}

function goProfile() {
  emit('navigate', { view: 'profile', userId: 0, isMe: true })
}

function goPublish() {
  emit('navigate', { view: 'publish' })
}

function displayUnread() {
  const n = Number(props.unreadCount) || 0
  if (n <= 0) return ''
  return n > 99 ? '99+' : String(n)
}
</script>

<template>
  <nav class="bottom-nav">
    <button
      class="nav-item"
      :class="{ active: active === 'feed' }"
      type="button"
      @click="go('feed')"
    >
      <svg class="nav-icon" viewBox="0 0 24 24" fill="currentColor">
        <path d="M12 3 2 12h3v8h5v-6h4v6h5v-8h3L12 3Z"/>
      </svg>
      <span class="nav-label">首页</span>
    </button>

    <button
      class="nav-item"
      :class="{ active: active === 'friends' }"
      type="button"
      @click="go('friends')"
    >
      <svg class="nav-icon" viewBox="0 0 24 24" fill="currentColor">
        <path d="M16 11c1.66 0 2.99-1.34 2.99-3S17.66 5 16 5c-1.66 0-3 1.34-3 3s1.34 3 3 3Zm-8 0c1.66 0 2.99-1.34 2.99-3S9.66 5 8 5C6.34 5 5 6.34 5 8s1.34 3 3 3Zm0 2c-2.33 0-7 1.17-7 3.5V19h14v-2.5c0-2.33-4.67-3.5-7-3.5Zm8 0c-.29 0-.62.02-.97.05 1.16.84 1.97 1.97 1.97 3.45V19h6v-2.5c0-2.33-4.67-3.5-7-3.5Z"/>
      </svg>
      <span class="nav-label">朋友</span>
    </button>

    <button class="nav-item publish-center" type="button" @click="goPublish">
      <span class="publish-box">
        <span class="publish-plus">＋</span>
      </span>
    </button>

    <button
      class="nav-item"
      :class="{ active: active === 'messages' }"
      type="button"
      @click="go('messages')"
    >
      <div class="nav-icon-wrap">
        <svg class="nav-icon" viewBox="0 0 24 24" fill="currentColor">
          <path d="M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2Z"/>
        </svg>
        <span v-if="displayUnread()" class="badge">{{ displayUnread() }}</span>
      </div>
      <span class="nav-label">消息</span>
    </button>

    <button
      class="nav-item"
      :class="{ active: active === 'profile' }"
      type="button"
      @click="goProfile"
    >
      <svg class="nav-icon" viewBox="0 0 24 24" fill="currentColor">
        <path d="M12 12a5 5 0 1 0-5-5 5 5 0 0 0 5 5Zm0 2c-4.42 0-8 2.24-8 5v3h16v-3c0-2.76-3.58-5-8-5Z"/>
      </svg>
      <span class="nav-label">我</span>
    </button>
  </nav>
</template>

<style scoped>
.bottom-nav {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: calc(50px + env(safe-area-inset-bottom));
  padding-bottom: env(safe-area-inset-bottom);
  display: flex;
  align-items: center;
  justify-content: space-around;
  background: #000;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  z-index: 30;
}

.nav-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 2px;
  flex: 1;
  height: 100%;
  color: rgba(255, 255, 255, 0.5);
  transition: color 0.15s;
}

.nav-item.active {
  color: #fff;
}

.nav-item:active {
  opacity: 0.6;
}

.nav-icon {
  width: 22px;
  height: 22px;
}

.nav-icon-wrap {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.badge {
  position: absolute;
  top: -6px;
  right: -8px;
  min-width: 16px;
  height: 16px;
  padding: 0 4px;
  border-radius: 8px;
  background: #fe2c55;
  color: #fff;
  font-size: 10px;
  font-weight: 600;
  line-height: 16px;
  text-align: center;
  box-sizing: border-box;
}

.nav-icon-wrap {
  position: relative;
  display: grid;
  place-items: center;
}

.badge {
  position: absolute;
  top: -6px;
  right: -10px;
  min-width: 16px;
  height: 16px;
  padding: 0 4px;
  border-radius: 8px;
  background: #fe2c55;
  color: #fff;
  font-size: 10px;
  font-weight: 700;
  line-height: 16px;
  text-align: center;
  box-shadow: 0 0 0 2px #000;
}

.nav-label {
  font-size: 10px;
  font-weight: 500;
}

.publish-center {
  flex: 0 0 60px;
}

.publish-box {
  width: 44px;
  height: 30px;
  border-radius: 8px;
  background: linear-gradient(90deg, #25f4ee 0%, #fe2c55 100%);
  display: grid;
  place-items: center;
  transition: transform 0.15s;
}

.publish-center:active .publish-box {
  transform: scale(0.92);
}

.publish-plus {
  font-size: 22px;
  font-weight: 300;
  color: #fff;
  line-height: 1;
  margin-top: -2px;
}
</style>
