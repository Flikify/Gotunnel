<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { login, setToken } from '../api'

const router = useRouter()
const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

const features = [
  '统一管理隧道、客户端与规则状态',
  '自动下发配置，客户端零配置接入',
  '内置更新与运行状态查看，便于运维排障',
]

const canSubmit = computed(() => Boolean(username.value && password.value) && !loading.value)

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
    <div class="login-frame glass-card">
      <section class="login-hero">
        <span class="login-badge">GoTunnel Console</span>
        <h1>更统一、更轻量的管理界面。</h1>
        <p>聚焦连接状态、更新能力与节点管理，让日常操作更直观、页面更简洁。</p>
        <ul>
          <li v-for="item in features" :key="item">{{ item }}</li>
        </ul>
      </section>

      <section class="login-panel">
        <div class="login-panel__header">
          <h2>登录控制台</h2>
          <p>使用服务端配置的 Web 账号进入管理界面。</p>
        </div>

        <form class="login-form" @submit.prevent="handleLogin">
          <label class="form-group">
            <span>用户名</span>
            <input v-model="username" class="glass-input" type="text" autocomplete="username" placeholder="请输入用户名" />
          </label>
          <label class="form-group">
            <span>密码</span>
            <input v-model="password" class="glass-input" type="password" autocomplete="current-password" placeholder="请输入密码" />
          </label>

          <div v-if="error" class="error-alert">{{ error }}</div>

          <button class="glass-btn primary submit-btn" type="submit" :disabled="!canSubmit">
            {{ loading ? '登录中...' : '进入控制台' }}
          </button>
        </form>
      </section>
    </div>
  </div>
</template>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}

.login-frame {
  width: min(1080px, 100%);
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(320px, 420px);
  overflow: hidden;
}

.login-hero,
.login-panel {
  padding: 40px;
}

.login-hero {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 18px;
  background:
    radial-gradient(circle at top right, color-mix(in srgb, var(--color-warning) 16%, transparent), transparent 48%),
    linear-gradient(135deg, color-mix(in srgb, var(--color-accent) 14%, transparent), color-mix(in srgb, var(--color-success) 10%, transparent));
  border-right: 1px solid var(--color-border);
}

.login-badge {
  width: fit-content;
  padding: 6px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
  color: var(--color-accent);
  background: color-mix(in srgb, var(--color-accent) 12%, transparent);
  border: 1px solid color-mix(in srgb, var(--color-accent) 18%, transparent);
}

.login-hero h1 {
  margin: 0;
  font-size: clamp(34px, 4.5vw, 54px);
  line-height: 1.1;
  letter-spacing: -0.05em;
}

.login-hero p {
  margin: 0;
  color: var(--color-text-secondary);
  font-size: 15px;
  line-height: 1.8;
}

.login-hero ul {
  display: grid;
  gap: 12px;
  padding: 0;
  margin: 8px 0 0;
  list-style: none;
}

.login-hero li {
  padding: 14px 16px;
  border-radius: 16px;
  background: var(--glass-bg-light);
  border: 1px solid var(--color-border-light);
  color: var(--color-text-primary);
}

.login-panel {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 26px;
}

.login-panel__header h2 {
  margin: 0 0 8px;
  font-size: 28px;
}

.login-panel__header p {
  margin: 0;
  color: var(--color-text-secondary);
  line-height: 1.7;
}

.login-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.form-group span {
  color: var(--color-text-secondary);
  font-size: 13px;
}

.submit-btn {
  justify-content: center;
  width: 100%;
  margin-top: 8px;
}

.error-alert {
  padding: 12px 14px;
  border-radius: 14px;
  color: var(--color-error);
  background: rgba(239, 68, 68, 0.1);
  border: 1px solid rgba(239, 68, 68, 0.18);
}

@media (max-width: 900px) {
  .login-frame {
    grid-template-columns: 1fr;
  }

  .login-hero {
    border-right: none;
    border-bottom: 1px solid var(--color-border);
  }
}

@media (max-width: 640px) {
  .login-page {
    padding: 16px;
  }

  .login-hero,
  .login-panel {
    padding: 28px 22px;
  }
}
</style>
