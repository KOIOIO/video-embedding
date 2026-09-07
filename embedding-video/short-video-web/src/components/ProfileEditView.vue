<script setup>
import { computed, onMounted, ref } from 'vue'
import { fetchUserProfile, getCurrentUserId, refreshMyProfile, updateMyProfile, uploadAvatar } from '../user/api.js'

const emit = defineEmits(['back', 'saved'])

const nickname = ref('')
const bio = ref('')
const location = ref('')
const gender = ref(0)
const avatarUrl = ref('')
const avatarFailed = ref(false)

const loading = ref(true)
const submitting = ref(false)
const uploadingAvatar = ref(false)
const errorText = ref('')

const currentUserId = computed(() => getCurrentUserId())
const bioCount = computed(() => [...bio.value].length)
const avatarText = computed(() => (nickname.value || '用户').slice(0, 1).toUpperCase())

const fileInput = ref(null)

async function loadProfile() {
  loading.value = true
  errorText.value = ''
  try {
    const profile = await fetchUserProfile(currentUserId.value)
    nickname.value = profile.nickname
    bio.value = profile.bio
    location.value = profile.location
    gender.value = profile.gender
    avatarUrl.value = profile.avatar_url
  } catch {
    errorText.value = '加载资料失败'
  } finally {
    loading.value = false
  }
}

function triggerFileSelect() {
  fileInput.value?.click()
}

async function onFileChange(event) {
  const file = event.target.files?.[0]
  if (!file) return
  uploadingAvatar.value = true
  errorText.value = ''
  try {
    const result = await uploadAvatar(file)
    avatarUrl.value = result.avatar_url
    avatarFailed.value = false
    // 头像上传后刷新全局 profile，确保首页顶部等地方显示最新头像
    refreshMyProfile().catch(() => {})
  } catch (err) {
    errorText.value = err?.message || '头像上传失败'
  } finally {
    uploadingAvatar.value = false
    event.target.value = ''
  }
}

function onAvatarError() {
  avatarFailed.value = true
}

async function onSave() {
  const name = nickname.value.trim()
  if (!name) {
    errorText.value = '昵称不能为空'
    return
  }
  if ([...name].length > 100) {
    errorText.value = '昵称不能超过100个字符'
    return
  }
  if (bioCount.value > 500) {
    errorText.value = '签名不能超过500个字符'
    return
  }
  submitting.value = true
  errorText.value = ''
  try {
    await updateMyProfile({
      nickname: name,
      bio: bio.value,
      location: location.value,
      gender: gender.value,
    })
    emit('saved')
  } catch (err) {
    errorText.value = err?.message || '保存失败'
  } finally {
    submitting.value = false
  }
}

function onCancel() {
  emit('back')
}

onMounted(loadProfile)
</script>

<template>
  <div class="profile-edit">
    <header class="topbar">
      <button class="cancel-btn" type="button" @click="onCancel">取消</button>
      <span class="topbar-title">编辑资料</span>
      <button class="save-btn" type="button" :disabled="submitting || loading" @click="onSave">
        {{ submitting ? '保存中…' : '保存' }}
      </button>
    </header>

    <div v-if="loading" class="center">
      <div class="spinner"></div>
      <p>加载中…</p>
    </div>

    <form v-else class="form" @submit.prevent="onSave">
      <div class="avatar-section">
        <div class="avatar-wrap" @click="triggerFileSelect">
          <img
            v-if="avatarUrl && !avatarFailed"
            :src="avatarUrl"
            alt="头像"
            class="avatar-img"
            @error="onAvatarError"
          />
          <div v-else class="avatar-fallback">{{ avatarText }}</div>
          <div v-if="uploadingAvatar" class="avatar-overlay">
            <div class="mini-spinner"></div>
          </div>
        </div>
        <p class="avatar-hint">点击更换头像（jpg/png/gif/webp，≤2MB）</p>
        <input ref="fileInput" type="file" accept="image/jpeg,image/png,image/gif,image/webp" class="hidden-file" @change="onFileChange" />
      </div>

      <p v-if="errorText" class="error">{{ errorText }}</p>

      <label class="field">
        <span class="field-label">昵称</span>
        <input
          v-model="nickname"
          type="text"
          maxlength="100"
          placeholder="请输入昵称"
          class="field-input"
        />
      </label>

      <label class="field">
        <span class="field-label">性别</span>
        <select v-model.number="gender" class="field-input field-select">
          <option :value="0">未知</option>
          <option :value="1">男</option>
          <option :value="2">女</option>
        </select>
      </label>

      <label class="field">
        <span class="field-label">位置</span>
        <input
          v-model="location"
          type="text"
          maxlength="200"
          placeholder="请输入所在地"
          class="field-input"
        />
      </label>

      <label class="field">
        <span class="field-label">
          签名
          <span class="char-count">{{ bioCount }}/500</span>
        </span>
        <textarea
          v-model="bio"
          rows="4"
          maxlength="500"
          placeholder="介绍一下自己吧"
          class="field-input field-textarea"
        ></textarea>
      </label>
    </form>
  </div>
</template>

<style scoped>
.profile-edit {
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
  background: linear-gradient(180deg, rgba(0, 0, 0, 0.9), rgba(0, 0, 0, 0.7));
  backdrop-filter: blur(10px);
}

.cancel-btn {
  font-size: 15px;
  color: var(--text-dim, #999);
  padding: 6px 4px;
  transition: color 0.15s;
}

.cancel-btn:hover {
  color: #fff;
}

.topbar-title {
  font-size: 16px;
  font-weight: 700;
}

.save-btn {
  font-size: 15px;
  font-weight: 700;
  color: #fe2c55;
  padding: 6px 4px;
  transition: opacity 0.15s;
}

.save-btn:disabled {
  opacity: 0.4;
}

.form {
  padding: 20px 16px 40px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.avatar-section {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 12px 0 8px;
}

.avatar-wrap {
  position: relative;
  cursor: pointer;
}

.avatar-img {
  width: 88px;
  height: 88px;
  border-radius: 50%;
  object-fit: cover;
  border: 2px solid rgba(255, 255, 255, 0.15);
}

.avatar-fallback {
  width: 88px;
  height: 88px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  background: linear-gradient(135deg, #333, #555);
  font-size: 34px;
  font-weight: 700;
  color: #ccc;
  border: 2px solid rgba(255, 255, 255, 0.1);
}

.avatar-overlay {
  position: absolute;
  inset: 0;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.5);
  display: grid;
  place-items: center;
}

.mini-spinner {
  width: 24px;
  height: 24px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

.avatar-hint {
  margin: 0;
  font-size: 12px;
  color: var(--text-dim, #777);
}

.hidden-file {
  display: none;
}

.error {
  margin: 0;
  font-size: 13px;
  color: #ff6b81;
  text-align: center;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.field-label {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-dim, #aaa);
}

.char-count {
  font-size: 11px;
  font-weight: 400;
  color: var(--text-faint, #666);
}

.field-input {
  width: 100%;
  padding: 12px 14px;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.05);
  color: #fff;
  font-size: 15px;
  outline: none;
  transition: border-color 0.2s, background 0.2s;
  box-sizing: border-box;
}

.field-input::placeholder {
  color: var(--text-faint, #555);
}

.field-input:focus {
  border-color: rgba(254, 44, 85, 0.7);
  background: rgba(255, 255, 255, 0.08);
}

.field-select {
  appearance: none;
  -webkit-appearance: none;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 12 12'%3E%3Cpath fill='%23999' d='M6 8L1 3h10z'/%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right 14px center;
  padding-right: 36px;
}

.field-select option {
  background: #1a1a1a;
  color: #fff;
}

.field-textarea {
  resize: vertical;
  min-height: 90px;
  line-height: 1.5;
  font-family: inherit;
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
