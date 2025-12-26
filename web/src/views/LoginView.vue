<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { NCard, NForm, NFormItem, NInput, NButton, NAlert } from 'naive-ui'
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
    <n-card class="login-card" :bordered="false">
      <template #header>
        <div class="login-header">
          <h1 class="logo">GoTunnel</h1>
          <p class="subtitle">安全的内网穿透工具</p>
        </div>
      </template>

      <n-form @submit.prevent="handleLogin">
        <n-form-item label="用户名">
          <n-input
            v-model:value="username"
            placeholder="请输入用户名"
            :disabled="loading"
          />
        </n-form-item>

        <n-form-item label="密码">
          <n-input
            v-model:value="password"
            type="password"
            placeholder="请输入密码"
            :disabled="loading"
            show-password-on="click"
          />
        </n-form-item>

        <n-alert v-if="error" type="error" :show-icon="true" style="margin-bottom: 16px;">
          {{ error }}
        </n-alert>

        <n-button
          type="primary"
          block
          :loading="loading"
          attr-type="submit"
        >
          {{ loading ? '登录中...' : '登录' }}
        </n-button>
      </n-form>

      <template #footer>
        <div class="login-footer">欢迎使用 GoTunnel</div>
      </template>
    </n-card>
  </div>
</template>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #e8f5e9 0%, #c8e6c9 100%);
  padding: 16px;
}

.login-card {
  width: 100%;
  max-width: 400px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
}

.login-header {
  text-align: center;
}

.logo {
  font-size: 28px;
  font-weight: 700;
  color: #18a058;
  margin: 0 0 8px 0;
}

.subtitle {
  color: #666;
  margin: 0;
  font-size: 14px;
}

.login-footer {
  text-align: center;
  color: #999;
  font-size: 14px;
}
</style>
