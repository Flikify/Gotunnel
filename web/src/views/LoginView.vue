<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { login, setToken } from '../api'

const router = useRouter()
const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

const handleLogin = async () => {
  if (!username.value || !password.value) {
    error.value = '请输入用户名和密码'
    return
  }

  loading.value = true
  error.value = ''

  try {
    const { data } = await login(username.value, password.value)
    setToken(data.token)
    router.push('/')
  } catch (e: any) {
    error.value = e.response?.data?.error || '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <!-- Animated particles -->
    <div class="particles">
      <div class="particle particle-1"></div>
      <div class="particle particle-2"></div>
      <div class="particle particle-3"></div>
      <div class="particle particle-4"></div>
    </div>

    <!-- Login card -->
    <div class="login-card">
      <div class="login-header">
        <div class="logo-icon">
          <svg xmlns="http://www.w3.org/2000/svg" class="w-8 h-8" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
          </svg>
        </div>
        <h1 class="logo-text">GoTunnel</h1>
        <p class="subtitle">安全的内网穿透工具</p>
      </div>

      <form @submit.prevent="handleLogin" class="login-form">
        <div class="form-group">
          <label class="form-label">用户名</label>
          <input
            v-model="username"
            type="text"
            class="glass-input"
            placeholder="请输入用户名"
            :disabled="loading"
          />
        </div>

        <div class="form-group">
          <label class="form-label">密码</label>
          <input
            v-model="password"
            type="password"
            class="glass-input"
            placeholder="请输入密码"
            :disabled="loading"
          />
        </div>

        <div v-if="error" class="error-alert">
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <span>{{ error }}</span>
        </div>

        <button type="submit" class="glass-button" :disabled="loading">
          <span v-if="loading" class="loading-spinner"></span>
          {{ loading ? '登录中...' : '登录' }}
        </button>
      </form>

      <div class="login-footer">
        <span>欢迎使用 GoTunnel</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-bg-primary);
  padding: 16px;
  position: relative;
  overflow: hidden;
}

/* 移除浮动粒子动画 */
.particles {
  display: none;
}

/* Login card */
.login-card {
  width: 100%;
  max-width: 380px;
  background: var(--color-bg-tertiary);
  border-radius: 16px;
  border: 1px solid var(--color-border);
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.4);
  padding: 40px 32px;
  position: relative;
  z-index: 10;
}

/* Header */
.login-header {
  text-align: center;
  margin-bottom: 32px;
}

.logo-icon {
  width: 56px;
  height: 56px;
  margin: 0 auto 16px;
  background: var(--color-accent);
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.logo-icon svg {
  color: white;
  width: 28px;
  height: 28px;
}

.logo-text {
  font-size: 24px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 8px 0;
}

.subtitle {
  color: var(--color-text-secondary);
  margin: 0;
  font-size: 14px;
}

/* Form */
.login-form {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.form-label {
  font-size: 14px;
  font-weight: 500;
  color: var(--color-text-primary);
}

.glass-input {
  background: var(--color-bg-elevated);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 12px 14px;
  color: var(--color-text-primary);
  font-size: 15px;
  width: 100%;
  transition: all 0.15s;
  outline: none;
}

.glass-input:focus {
  border-color: var(--color-accent);
}

.glass-input::placeholder {
  color: var(--color-text-muted);
}

.glass-input:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

/* Error alert */
.error-alert {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 14px;
  background: rgba(244, 33, 46, 0.1);
  border: 1px solid rgba(244, 33, 46, 0.3);
  border-radius: 8px;
  color: var(--color-error);
  font-size: 14px;
}

.error-alert svg {
  flex-shrink: 0;
}

/* Button */
.glass-button {
  background: var(--color-accent);
  border: none;
  border-radius: 8px;
  padding: 12px 24px;
  color: white;
  font-size: 15px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.glass-button:hover:not(:disabled) {
  background: var(--color-accent-hover);
}

.glass-button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

/* Loading spinner */
.loading-spinner {
  width: 16px;
  height: 16px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top-color: white;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

/* Footer */
.login-footer {
  text-align: center;
  margin-top: 24px;
  padding-top: 24px;
  border-top: 1px solid var(--color-border);
  color: var(--color-text-muted);
  font-size: 13px;
}
</style>
