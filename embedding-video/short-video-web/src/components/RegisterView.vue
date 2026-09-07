<script setup>
import { computed, ref } from 'vue'
import { registerUser } from '../auth/api.js'

const emit = defineEmits(['registered', 'go-login'])

const username = ref('')
const password = ref('')
const confirmPassword = ref('')
const nickname = ref('')
const submitting = ref(false)
const errorText = ref('')

const usernamePattern = /^[a-zA-Z0-9_]{4,20}$/
const usernameValid = computed(() => usernamePattern.test(username.value.trim()))
const passwordValid = computed(() => password.value.length >= 6 && password.value.length <= 32)
const confirmValid = computed(() => confirmPassword.value.length > 0 && confirmPassword.value === password.value)
const formValid = computed(() => usernameValid.value && passwordValid.value && confirmValid.value)

const usernameHint = computed(() => {
  const v = username.value.trim()
  if (!v) return '4-20位，字母、数字或下划线'
  if (!usernamePattern.test(v)) return '用户名格式不正确：4-20位字母、数字或下划线'
  return '用户名可用'
})

async function submit() {
  if (!formValid.value) return
  submitting.value = true
  errorText.value = ''
  try {
    const session = await registerUser(username.value.trim(), password.value, nickname.value.trim())
    emit('registered', session)
  } catch (error) {
    if (error?.status === 409) {
      errorText.value = '用户名已被注册'
    } else if (error?.status === 400) {
      errorText.value = error?.message || '注册信息不合法'
    } else {
      errorText.value = error?.message || '注册失败，请稍后重试'
    }
  } finally {
    submitting.value = false
  }
}

function onSubmit(event) {
  event.preventDefault()
  submit()
}

function goLogin() {
  emit('go-login')
}
</script>

<template>
  <div class="register">
    <div class="halo halo-1"></div>
    <div class="halo halo-2"></div>

    <form class="card" @submit="onSubmit">
      <div class="logo">短</div>
      <h1 class="title">创建账号</h1>
      <p class="subtitle">加入短视频，刷到停不下来</p>

      <label class="field">
        <input
          v-model="username"
          type="text"
          autocomplete="username"
          placeholder="用户名"
          maxlength="20"
        />
        <span class="hint" :class="{ ok: usernameValid && username.trim() }">{{ usernameHint }}</span>
      </label>

      <label class="field">
        <input
          v-model="password"
          type="password"
          autocomplete="new-password"
          placeholder="密码（6-32位）"
          maxlength="32"
        />
      </label>

      <label class="field">
        <input
          v-model="confirmPassword"
          type="password"
          autocomplete="new-password"
          placeholder="确认密码"
          maxlength="32"
        />
        <span v-if="confirmPassword && !confirmValid" class="hint error">两次输入的密码不一致</span>
      </label>

      <label class="field">
        <input
          v-model="nickname"
          type="text"
          autocomplete="nickname"
          placeholder="昵称（选填，默认用用户名）"
          maxlength="32"
        />
      </label>

      <p v-if="errorText" class="error">{{ errorText }}</p>

      <button class="submit" type="submit" :disabled="!formValid || submitting">
        <span v-if="submitting" class="spinner"></span>
        {{ submitting ? '注册中…' : '注册并登录' }}
      </button>

      <p class="footer">
        已有账号？
        <button type="button" class="link" @click="goLogin">去登录</button>
      </p>
    </form>
  </div>
</template>

<style scoped>
.register {
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
  gap: 12px;
  padding: 36px 32px 32px;
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
  font-size: 24px;
  font-weight: 800;
  letter-spacing: 1px;
}

.subtitle {
  margin: 0 0 4px;
  text-align: center;
  font-size: 13px;
  color: var(--text-dim);
}

.field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.field input {
  width: 100%;
  padding: 12px 16px;
  border-radius: 12px;
  border: 1px solid rgba(255, 255, 255, 0.14);
  background: rgba(255, 255, 255, 0.06);
  color: var(--text);
  font-size: 15px;
  outline: none;
  transition: border-color 0.2s, background 0.2s;
  box-sizing: border-box;
}

.field input::placeholder {
  color: var(--text-faint);
}

.field input:focus {
  border-color: rgba(254, 44, 85, 0.8);
  background: rgba(255, 255, 255, 0.09);
}

.hint {
  font-size: 12px;
  color: var(--text-faint);
  padding-left: 4px;
}

.hint.ok {
  color: #25f4ee;
}

.hint.error {
  color: #ff6b81;
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
  margin-top: 4px;
  border-radius: 12px;
  background: linear-gradient(90deg, #fe2c55, #ff5470);
  color: #fff;
  font-size: 16px;
  font-weight: 700;
  letter-spacing: 4px;
  text-indent: 4px;
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
  opacity: 0.5;
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

.footer {
  margin: 4px 0 0;
  text-align: center;
  font-size: 13px;
  color: var(--text-dim);
}

.link {
  background: none;
  border: none;
  padding: 0;
  color: #25f4ee;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}

.link:hover {
  text-decoration: underline;
}
</style>
