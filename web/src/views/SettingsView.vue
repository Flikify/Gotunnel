<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ServerOutline, SettingsOutline, SaveOutline } from '@vicons/ionicons5'
import { useToast } from '../composables/useToast'
import {
  getVersionInfo, getServerConfig, updateServerConfig,
  type VersionInfo, type ServerConfigResponse
} from '../api'

const message = useToast()

const versionInfo = ref<VersionInfo | null>(null)
const loading = ref(true)

// 服务器配置
const serverConfig = ref<ServerConfigResponse | null>(null)
const configLoading = ref(false)
const savingConfig = ref(false)

// 配置表单
const configForm = ref({
  heartbeat_sec: 30,
  heartbeat_timeout: 90,
  web_username: '',
  web_password: '',
  plugin_store_url: ''
})

const loadVersionInfo = async () => {
  try {
    const { data } = await getVersionInfo()
    versionInfo.value = data
  } catch (e) {
    console.error('Failed to load version info', e)
  } finally {
    loading.value = false
  }
}

const loadServerConfig = async () => {
  configLoading.value = true
  try {
    const { data } = await getServerConfig()
    serverConfig.value = data
    // 填充表单
    configForm.value = {
      heartbeat_sec: data.server.heartbeat_sec,
      heartbeat_timeout: data.server.heartbeat_timeout,
      web_username: data.web.username,
      web_password: '',
      plugin_store_url: data.plugin_store.url
    }
  } catch (e) {
    console.error('Failed to load server config', e)
  } finally {
    configLoading.value = false
  }
}

const handleSaveConfig = async () => {
  savingConfig.value = true
  try {
    const updateReq: any = {
      server: {
        heartbeat_sec: configForm.value.heartbeat_sec,
        heartbeat_timeout: configForm.value.heartbeat_timeout
      },
      web: {
        username: configForm.value.web_username
      },
      plugin_store: {
        url: configForm.value.plugin_store_url
      }
    }
    // 只有填写了密码才更新
    if (configForm.value.web_password) {
      updateReq.web.password = configForm.value.web_password
    }
    await updateServerConfig(updateReq)
    message.success('配置已保存，部分配置需要重启服务后生效')
    configForm.value.web_password = ''
  } catch (e: any) {
    message.error(e.response?.data || '保存配置失败')
  } finally {
    savingConfig.value = false
  }
}

onMounted(() => {
  loadVersionInfo()
  loadServerConfig()
})
</script>

<template>
  <div class="settings-page">
    <!-- Particles -->
    <div class="particles">
      <div class="particle particle-1"></div>
      <div class="particle particle-2"></div>
      <div class="particle particle-3"></div>
    </div>

    <div class="settings-content">
      <!-- Header -->
      <div class="page-header">
        <h1 class="page-title">系统设置</h1>
        <p class="page-subtitle">管理服务端配置和系统更新</p>
      </div>

      <!-- Version Info Card -->
      <div class="glass-card">
        <div class="card-header">
          <h3>版本信息</h3>
          <ServerOutline class="header-icon" />
        </div>
        <div class="card-body">
          <div v-if="loading" class="loading-state">加载中...</div>
          <div v-else-if="versionInfo" class="info-grid">
            <div class="info-item">
              <span class="info-label">版本号</span>
              <span class="info-value">{{ versionInfo.version }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">Git 提交</span>
              <span class="info-value mono">{{ versionInfo.git_commit?.slice(0, 8) || 'N/A' }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">构建时间</span>
              <span class="info-value">{{ versionInfo.build_time || 'N/A' }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">Go 版本</span>
              <span class="info-value">{{ versionInfo.go_version }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">操作系统</span>
              <span class="info-value">{{ versionInfo.os }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">架构</span>
              <span class="info-value">{{ versionInfo.arch }}</span>
            </div>
          </div>
          <div v-else class="empty-state">无法加载版本信息</div>
        </div>
      </div>

      <!-- Server Config Card -->
      <div class="glass-card">
        <div class="card-header">
          <h3>服务器配置</h3>
          <SettingsOutline class="header-icon" />
        </div>
        <div class="card-body">
          <div v-if="configLoading" class="loading-state">加载中...</div>
          <div v-else-if="serverConfig" class="config-form">
            <div class="form-row">
              <div class="form-group">
                <label class="form-label">心跳间隔 (秒)</label>
                <input
                  v-model.number="configForm.heartbeat_sec"
                  type="number"
                  class="glass-input"
                  min="1"
                  max="300"
                />
              </div>
              <div class="form-group">
                <label class="form-label">心跳超时 (秒)</label>
                <input
                  v-model.number="configForm.heartbeat_timeout"
                  type="number"
                  class="glass-input"
                  min="1"
                  max="600"
                />
              </div>
            </div>

            <div class="form-divider"></div>

            <div class="form-row">
              <div class="form-group">
                <label class="form-label">Web 用户名</label>
                <input
                  v-model="configForm.web_username"
                  type="text"
                  class="glass-input"
                  placeholder="admin"
                />
              </div>
              <div class="form-group">
                <label class="form-label">Web 密码</label>
                <input
                  v-model="configForm.web_password"
                  type="password"
                  class="glass-input"
                  placeholder="留空则不修改"
                />
              </div>
            </div>

            <div class="form-divider"></div>

            <div class="form-group">
              <label class="form-label">插件商店地址</label>
              <input
                v-model="configForm.plugin_store_url"
                type="text"
                class="glass-input"
                placeholder="https://git.92coco.cn/flik/GoTunnel-Plugins/raw/branch/main/store.json"
              />
              <span class="form-hint">插件商店的 API 地址，留空使用默认地址</span>
            </div>

            <div class="form-actions">
              <button
                class="glass-btn primary"
                :disabled="savingConfig"
                @click="handleSaveConfig"
              >
                <SaveOutline class="btn-icon" />
                保存配置
              </button>
            </div>
          </div>
          <div v-else class="empty-state">无法加载配置信息</div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.settings-page {
  min-height: calc(100vh - 108px);
  background: var(--color-bg-primary);
  position: relative;
  overflow: hidden;
  padding: 32px;
}

/* Hide particles */
.particles {
  display: none;
}

.settings-content {
  position: relative;
  z-index: 10;
  max-width: 900px;
  margin: 0 auto;
}

.page-header {
  margin-bottom: 24px;
}

.page-title {
  font-size: 28px;
  font-weight: 700;
  color: var(--color-text-primary);
  margin: 0 0 8px 0;
}

.page-subtitle {
  color: var(--color-text-secondary);
  margin: 0;
  font-size: 14px;
}

/* Glass Card */
.glass-card {
  background: var(--color-bg-tertiary);
  border-radius: 12px;
  border: 1px solid var(--color-border);
  margin-bottom: 20px;
}

.card-header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--color-border-light);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-header h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.card-body {
  padding: 20px;
}

/* Info Grid */
.info-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}

@media (max-width: 600px) {
  .info-grid { grid-template-columns: repeat(2, 1fr); }
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.info-label {
  font-size: 12px;
  color: var(--color-text-muted);
}

.info-value {
  font-size: 14px;
  color: var(--color-text-primary);
  font-weight: 500;
}

.info-value.mono {
  font-family: monospace;
}

/* States */
.loading-state, .empty-state {
  text-align: center;
  padding: 32px;
  color: var(--color-text-muted);
}

/* Update Alert */
.update-alert {
  padding: 12px 16px;
  border-radius: 8px;
  margin-bottom: 16px;
  font-size: 13px;
}

.update-alert.success {
  background: rgba(0, 186, 124, 0.15);
  border: 1px solid rgba(0, 186, 124, 0.3);
  color: var(--color-success);
}

.update-alert.info {
  background: rgba(29, 155, 240, 0.15);
  border: 1px solid rgba(29, 155, 240, 0.3);
  color: var(--color-info);
}

/* Download Info */
.download-info {
  color: var(--color-text-secondary);
  font-size: 13px;
  margin-bottom: 12px;
}

/* Release Note */
.release-note {
  margin-bottom: 16px;
}

.note-label {
  display: block;
  font-size: 12px;
  color: var(--color-text-muted);
  margin-bottom: 6px;
}

.release-note pre {
  margin: 0;
  white-space: pre-wrap;
  font-size: 12px;
  color: var(--color-text-secondary);
  background: var(--color-bg-elevated);
  padding: 12px;
  border-radius: 8px;
  max-height: 150px;
  overflow-y: auto;
}

/* Glass Button */
.glass-btn {
  background: var(--color-bg-elevated);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 8px 16px;
  color: var(--color-text-primary);
  font-size: 13px;
  cursor: pointer;
  transition: all 0.15s;
  display: flex;
  align-items: center;
  gap: 6px;
}

.glass-btn:hover:not(:disabled) {
  background: var(--color-border);
}

.glass-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.glass-btn.small {
  padding: 6px 12px;
  font-size: 12px;
}

.glass-btn.primary {
  background: var(--color-accent);
  border: none;
}

.glass-btn.primary:hover:not(:disabled) {
  background: var(--color-accent-hover);
}

/* Icon styles */
.header-icon {
  width: 20px;
  height: 20px;
  color: var(--color-text-muted);
}

.btn-icon {
  width: 14px;
  height: 14px;
}

/* Config Form */
.config-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-label {
  font-size: 13px;
  color: var(--color-text-secondary);
  font-weight: 500;
}

.form-hint {
  font-size: 11px;
  color: var(--color-text-muted);
}

.form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

@media (max-width: 500px) {
  .form-row {
    grid-template-columns: 1fr;
  }
}

.form-divider {
  height: 1px;
  background: var(--color-border-light);
  margin: 8px 0;
}

.form-actions {
  margin-top: 8px;
}

/* Glass Input */
.glass-input {
  background: var(--color-bg-elevated);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 10px 14px;
  color: var(--color-text-primary);
  font-size: 14px;
  outline: none;
  transition: all 0.15s;
}

.glass-input:focus {
  border-color: var(--color-accent);
}

.glass-input::placeholder {
  color: var(--color-text-muted);
}
</style>
