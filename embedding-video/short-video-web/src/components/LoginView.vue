<script setup>
import { ref } from 'vue'
import { loginAdmin } from '../auth/api.js'

const emit = defineEmits(['logged-in'])

const username = ref('')
const password = ref('')
const submitting = ref(false)
const errorText = ref('')

async function submit() {
  const name = username.value.trim()
  const pass = password.value
  if (!name || !pass) {
    errorText.value = '请输入账号和密码'
    return
  }
  submitting.value = true
  errorText.value = ''
  try {
    const session = await loginAdmin(name, pass)
    emit('logged-in', session)
  } catch (error) {
    errorText.value = error?.status === 401 ? '账号或密码错误' : (error?.message || '登录失败，请稍后重试')
  } finally {
    submitting.value = false
  }
}

function onSubmit(event) {
  event.preventDefault()
  submit()
}
</script>

<template>
  <div class="login">
    <div class="halo halo-1"></div>
    <div class="halo halo-2"></div>

    <form class="card" @submit="onSubmit">
      <div class="logo">短</div>
      <h1 class="title">短视频</h1>
      <p class="subtitle">上下滑动 · 随机好片刷不停</p>

      <label class="field">
        <input
          v-model="username"
          type="text"
          autocomplete="username"
          placeholder="账号"
          maxlength="64"
        />
      </label>
      <label class="field">
        <input
          v-model="password"
          type="password"
          autocomplete="current-password"
          placeholder="密码"
          maxlength="128"
        />
      </label>

      <p v-if="errorText" class="error">{{ errorText }}</p>

      <button class="submit" type="submit" :disabled="submitting">
        <span v-if="submitting" class="spinner"></span>
        {{ submitting ? '登录中…' : '登录' }}
      </button>
    </form>
  </div>
</template>

<style scoped>
.login {
  position: relative;
  height: 100%;
  display: grid;
  place-items: center;
  background: #000;
  overflow: hidden;
}

.halo {
  position: absolute;
  border-radius: 50%;
  filter: blur(90px);
  opacity: 0.35;
  pointer-events: none;
}

.halo-1 {
  width: 420px;
  height: 420px;
  background: #fe2c55;
  top: -120px;
  left: -120px;
}

.halo-2 {
  width: 380px;
  height: 380px;
  background: #25f4ee;
  bottom: -120px;
  right: -100px;
}

.card {
  position: relative;
  width: min(92vw, 380px);
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 40px 32px 36px;
  border-radius: 24px;
  background: rgba(22, 22, 22, 0.72);
  border: 1px solid rgba(255, 255, 255, 0.09);
  backdrop-filter: blur(22px);
  -webkit-backdrop-filter: blur(22px);
  box-shadow: 0 24px 80px rgba(0, 0, 0, 0.6);
  animation: fade-in 0.4s ease both;
}

.logo {
  width: 58px;
  height: 58px;
  margin: 0 auto;
  display: grid;
  place-items: center;
  border-radius: 16px;
  font-size: 28px;
  font-weight: 800;
  color: #fff;
  background: linear-gradient(135deg, #fe2c55 0%, #ff7a59 100%);
  box-shadow: 0 0 34px rgba(254, 44, 85, 0.55);
}

.title {
  margin: 0;
  text-align: center;
  font-size: 26px;
  font-weight: 800;
  letter-spacing: 1px;
}

.subtitle {
  margin: 0 0 8px;
  text-align: center;
  font-size: 13px;
  color: var(--text-dim);
}

.field input {
  width: 100%;
  padding: 13px 16px;
  border-radius: 12px;
  border: 1px solid rgba(255, 255, 255, 0.14);
  background: rgba(255, 255, 255, 0.06);
  color: var(--text);
  font-size: 15px;
  outline: none;
  transition: border-color 0.2s, background 0.2s;
}

.field input::placeholder {
  color: var(--text-faint);
}

.field input:focus {
  border-color: rgba(254, 44, 85, 0.8);
  background: rgba(255, 255, 255, 0.09);
}

.error {
  margin: 0;
  font-size: 13px;
  color: #ff6b81;
}

.submit {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  width: 100%;
  padding: 13px;
  margin-top: 6px;
  border-radius: 12px;
  background: linear-gradient(90deg, #fe2c55, #ff5470);
  color: #fff;
  font-size: 16px;
  font-weight: 700;
  letter-spacing: 6px;
  text-indent: 6px;
  transition: transform 0.15s, filter 0.15s;
}

.submit:hover:not(:disabled) {
  filter: brightness(1.08);
  transform: translateY(-1px);
}

.submit:active:not(:disabled) {
  transform: scale(0.98);
}

.submit:disabled {
  opacity: 0.65;
  cursor: default;
}

.spinner {
  width: 16px;
  height: 16px;
  border: 2px solid rgba(255, 255, 255, 0.4);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}
</style>
