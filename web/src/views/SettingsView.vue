<script setup lang="ts">
import { ref, onMounted } from 'vue'
import {
  NTag, NIcon, useMessage, useDialog
} from 'naive-ui'
import { CloudDownloadOutline, RefreshOutline, ServerOutline } from '@vicons/ionicons5'
import {
  getVersionInfo, checkServerUpdate, applyServerUpdate,
  type UpdateInfo, type VersionInfo
} from '../api'

const message = useMessage()
const dialog = useDialog()

const versionInfo = ref<VersionInfo | null>(null)
const serverUpdate = ref<UpdateInfo | null>(null)
const loading = ref(true)
const checkingServer = ref(false)
const updatingServer = ref(false)

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

const handleCheckServerUpdate = async () => {
  checkingServer.value = true
  try {
    const { data } = await checkServerUpdate()
    serverUpdate.value = data
    if (data.available) {
      message.success('发现新版本: ' + data.latest)
    } else {
      message.info('已是最新版本')
    }
  } catch (e: any) {
    message.error(e.response?.data || '检查更新失败')
  } finally {
    checkingServer.value = false
  }
}

const handleApplyServerUpdate = () => {
  if (!serverUpdate.value?.download_url) {
    message.error('没有可用的下载链接')
    return
  }

  dialog.warning({
    title: '确认更新服务端',
    content: `即将更新服务端到 ${serverUpdate.value.latest}，更新后服务器将自动重启。确定要继续吗？`,
    positiveText: '更新并重启',
    negativeText: '取消',
    onPositiveClick: async () => {
      updatingServer.value = true
      try {
        await applyServerUpdate(serverUpdate.value!.download_url)
        message.success('更新已开始，服务器将在几秒后重启')
        setTimeout(() => {
          window.location.reload()
        }, 5000)
      } catch (e: any) {
        message.error(e.response?.data || '更新失败')
        updatingServer.value = false
      }
    }
  })
}

const formatBytes = (bytes: number): string => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

onMounted(() => {
  loadVersionInfo()
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
          <n-icon size="20" color="#a78bfa"><ServerOutline /></n-icon>
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

      <!-- Server Update Card -->
      <div class="glass-card">
        <div class="card-header">
          <h3>服务端更新</h3>
          <button class="glass-btn small" :disabled="checkingServer" @click="handleCheckServerUpdate">
            <n-icon size="14"><RefreshOutline /></n-icon>
            检查更新
          </button>
        </div>
        <div class="card-body">
          <div v-if="!serverUpdate" class="empty-state">
            点击检查更新按钮查看是否有新版本
          </div>
          <template v-else>
            <div v-if="serverUpdate.available" class="update-alert success">
              发现新版本 {{ serverUpdate.latest }}，当前版本 {{ serverUpdate.current }}
            </div>
            <div v-else class="update-alert info">
              当前已是最新版本 {{ serverUpdate.current }}
            </div>

            <div v-if="serverUpdate.download_url" class="download-info">
              下载文件: {{ serverUpdate.asset_name }}
              <n-tag size="small" style="margin-left: 8px;">{{ formatBytes(serverUpdate.asset_size) }}</n-tag>
            </div>

            <div v-if="serverUpdate.release_note" class="release-note">
              <span class="note-label">更新日志:</span>
              <pre>{{ serverUpdate.release_note }}</pre>
            </div>

            <button
              v-if="serverUpdate.available && serverUpdate.download_url"
              class="glass-btn primary"
              :disabled="updatingServer"
              @click="handleApplyServerUpdate"
            >
              <n-icon size="16"><CloudDownloadOutline /></n-icon>
              下载并更新服务端
            </button>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.settings-page {
  min-height: calc(100vh - 108px);
  background: linear-gradient(135deg, #1e1b4b 0%, #312e81 30%, #4c1d95 60%, #581c87 100%);
  position: relative;
  overflow: hidden;
  padding: 32px;
}

.particles {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.particle {
  position: absolute;
  border-radius: 50%;
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.15), rgba(255, 255, 255, 0.05));
  animation: float-particle 20s ease-in-out infinite;
}

.particle-1 { width: 250px; height: 250px; top: -80px; right: -50px; }
.particle-2 { width: 180px; height: 180px; bottom: 10%; left: 5%; animation-delay: -7s; }
.particle-3 { width: 120px; height: 120px; top: 50%; right: 15%; animation-delay: -12s; }

@keyframes float-particle {
  0%, 100% { transform: translate(0, 0) scale(1); opacity: 0.3; }
  50% { transform: translate(-20px, -60px) scale(0.95); opacity: 0.4; }
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
  color: white;
  margin: 0 0 8px 0;
}

.page-subtitle {
  color: rgba(255, 255, 255, 0.6);
  margin: 0;
  font-size: 14px;
}

/* Glass Card */
.glass-card {
  background: rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border-radius: 16px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  margin-bottom: 20px;
}

.card-header {
  padding: 16px 20px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-header h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: white;
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
  color: rgba(255, 255, 255, 0.5);
}

.info-value {
  font-size: 14px;
  color: white;
  font-weight: 500;
}

.info-value.mono {
  font-family: monospace;
}

/* States */
.loading-state, .empty-state {
  text-align: center;
  padding: 32px;
  color: rgba(255, 255, 255, 0.5);
}

/* Update Alert */
.update-alert {
  padding: 12px 16px;
  border-radius: 8px;
  margin-bottom: 16px;
  font-size: 13px;
}

.update-alert.success {
  background: rgba(52, 211, 153, 0.15);
  border: 1px solid rgba(52, 211, 153, 0.3);
  color: #34d399;
}

.update-alert.info {
  background: rgba(96, 165, 250, 0.15);
  border: 1px solid rgba(96, 165, 250, 0.3);
  color: #60a5fa;
}

/* Download Info */
.download-info {
  color: rgba(255, 255, 255, 0.6);
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
  color: rgba(255, 255, 255, 0.5);
  margin-bottom: 6px;
}

.release-note pre {
  margin: 0;
  white-space: pre-wrap;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.7);
  background: rgba(0, 0, 0, 0.2);
  padding: 12px;
  border-radius: 8px;
  max-height: 150px;
  overflow-y: auto;
}

/* Glass Button */
.glass-btn {
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 8px;
  padding: 8px 16px;
  color: white;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  gap: 6px;
}

.glass-btn:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.2);
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
  background: linear-gradient(135deg, #60a5fa 0%, #a78bfa 100%);
  border: none;
}
</style>
