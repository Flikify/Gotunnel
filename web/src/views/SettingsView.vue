<script setup lang="ts">
import { ref, onMounted } from 'vue'
import {
  NCard, NButton, NSpace, NTag, NGrid, NGi, NEmpty, NSpin, NIcon,
  NAlert, useMessage, useDialog
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
  <div class="settings-view">
    <div class="page-header">
      <h2>系统设置</h2>
      <p>管理服务端配置和系统更新</p>
    </div>

    <n-spin :show="loading">
      <!-- 当前版本信息 -->
      <n-card title="版本信息" class="settings-card">
        <template #header-extra>
          <n-icon size="20" color="#6366f1"><ServerOutline /></n-icon>
        </template>
        <n-grid v-if="versionInfo" :cols="6" :x-gap="16" responsive="screen" cols-s="2" cols-m="3">
          <n-gi>
            <div class="info-item">
              <span class="label">版本号</span>
              <span class="value">{{ versionInfo.version }}</span>
            </div>
          </n-gi>
          <n-gi>
            <div class="info-item">
              <span class="label">Git 提交</span>
              <span class="value">{{ versionInfo.git_commit?.slice(0, 8) || 'N/A' }}</span>
            </div>
          </n-gi>
          <n-gi>
            <div class="info-item">
              <span class="label">构建时间</span>
              <span class="value">{{ versionInfo.build_time || 'N/A' }}</span>
            </div>
          </n-gi>
          <n-gi>
            <div class="info-item">
              <span class="label">Go 版本</span>
              <span class="value">{{ versionInfo.go_version }}</span>
            </div>
          </n-gi>
          <n-gi>
            <div class="info-item">
              <span class="label">操作系统</span>
              <span class="value">{{ versionInfo.os }}</span>
            </div>
          </n-gi>
          <n-gi>
            <div class="info-item">
              <span class="label">架构</span>
              <span class="value">{{ versionInfo.arch }}</span>
            </div>
          </n-gi>
        </n-grid>
        <n-empty v-else description="加载中..." />
      </n-card>

      <!-- 服务端更新 -->
      <n-card title="服务端更新" class="settings-card">
        <template #header-extra>
          <n-button size="small" :loading="checkingServer" @click="handleCheckServerUpdate">
            <template #icon><n-icon><RefreshOutline /></n-icon></template>
            检查更新
          </n-button>
        </template>

        <n-empty v-if="!serverUpdate" description="点击检查更新按钮查看是否有新版本" />

        <template v-else>
          <n-alert v-if="serverUpdate.available" type="success" style="margin-bottom: 16px;">
            发现新版本 {{ serverUpdate.latest }}，当前版本 {{ serverUpdate.current }}
          </n-alert>
          <n-alert v-else type="info" style="margin-bottom: 16px;">
            当前已是最新版本 {{ serverUpdate.current }}
          </n-alert>

          <n-space vertical :size="12">
            <div v-if="serverUpdate.download_url">
              <p style="margin: 0 0 8px 0; color: #666;">
                下载文件: {{ serverUpdate.asset_name }}
                <n-tag size="small" style="margin-left: 8px;">{{ formatBytes(serverUpdate.asset_size) }}</n-tag>
              </p>
            </div>

            <div v-if="serverUpdate.release_note" class="release-note">
              <p style="margin: 0 0 4px 0; color: #666; font-size: 12px;">更新日志:</p>
              <pre>{{ serverUpdate.release_note }}</pre>
            </div>

            <n-button
              v-if="serverUpdate.available && serverUpdate.download_url"
              type="primary"
              :loading="updatingServer"
              @click="handleApplyServerUpdate"
            >
              <template #icon><n-icon><CloudDownloadOutline /></n-icon></template>
              下载并更新服务端
            </n-button>
          </n-space>
        </template>
      </n-card>
    </n-spin>
  </div>
</template>

<style scoped>
.settings-view {
  max-width: 900px;
  margin: 0 auto;
}

.page-header {
  margin-bottom: 24px;
}

.page-header h2 {
  margin: 0 0 8px 0;
  font-size: 24px;
  font-weight: 600;
  color: #1f2937;
}

.page-header p {
  margin: 0;
  color: #6b7280;
}

.settings-card {
  margin-bottom: 16px;
}

.info-item {
  display: flex;
  flex-direction: column;
  padding: 8px 0;
}

.info-item .label {
  font-size: 12px;
  color: #9ca3af;
  margin-bottom: 4px;
}

.info-item .value {
  font-size: 14px;
  color: #1f2937;
  font-weight: 500;
}

.release-note {
  max-height: 150px;
  overflow-y: auto;
}

.release-note pre {
  margin: 0;
  white-space: pre-wrap;
  font-size: 12px;
  color: #374151;
  background: #f9fafb;
  padding: 12px;
  border-radius: 6px;
}
</style>
