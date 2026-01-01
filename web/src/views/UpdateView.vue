<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import {
  NCard, NButton, NSpace, NTag, NGrid, NGi, NEmpty, NSpin, NIcon,
  NAlert, NSelect, useMessage, useDialog
} from 'naive-ui'
import { ArrowBackOutline, CloudDownloadOutline, RefreshOutline, RocketOutline } from '@vicons/ionicons5'
import {
  getVersionInfo, checkServerUpdate, checkClientUpdate, applyServerUpdate, applyClientUpdate,
  getClients, type UpdateInfo, type VersionInfo
} from '../api'
import type { ClientStatus } from '../types'

const router = useRouter()
const message = useMessage()
const dialog = useDialog()

const versionInfo = ref<VersionInfo | null>(null)
const serverUpdate = ref<UpdateInfo | null>(null)
const clientUpdate = ref<UpdateInfo | null>(null)
const clients = ref<ClientStatus[]>([])
const loading = ref(true)
const checkingServer = ref(false)
const checkingClient = ref(false)
const updatingServer = ref(false)
const selectedClientId = ref('')

const onlineClients = computed(() => clients.value.filter(c => c.online))

const loadVersionInfo = async () => {
  try {
    const { data } = await getVersionInfo()
    versionInfo.value = data
  } catch (e) {
    console.error('Failed to load version info', e)
  }
}

const loadClients = async () => {
  try {
    const { data } = await getClients()
    clients.value = data || []
  } catch (e) {
    console.error('Failed to load clients', e)
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

const handleCheckClientUpdate = async () => {
  checkingClient.value = true
  try {
    const { data } = await checkClientUpdate()
    clientUpdate.value = data
    if (data.download_url) {
      message.success('找到客户端更新包: ' + data.latest)
    } else {
      message.warning('未找到对应平台的更新包')
    }
  } catch (e: any) {
    message.error(e.response?.data || '检查更新失败')
  } finally {
    checkingClient.value = false
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
        // 显示倒计时或等待
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

const handleApplyClientUpdate = async () => {
  if (!selectedClientId.value) {
    message.warning('请选择要更新的客户端')
    return
  }

  if (!clientUpdate.value?.download_url) {
    message.error('没有可用的下载链接')
    return
  }

  const clientName = onlineClients.value.find(c => c.id === selectedClientId.value)?.nickname || selectedClientId.value

  dialog.warning({
    title: '确认更新客户端',
    content: `即将更新客户端 "${clientName}" 到 ${clientUpdate.value.latest}，更新后客户端将自动重启。确定要继续吗？`,
    positiveText: '更新',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await applyClientUpdate(selectedClientId.value, clientUpdate.value!.download_url)
        message.success(`更新命令已发送到客户端 ${clientName}`)
      } catch (e: any) {
        message.error(e.response?.data || '更新失败')
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

onMounted(async () => {
  await Promise.all([loadVersionInfo(), loadClients()])
  loading.value = false
})
</script>

<template>
  <div class="update-view">
    <n-space justify="space-between" align="center" style="margin-bottom: 24px;">
      <div>
        <h2 style="margin: 0 0 8px 0;">系统更新</h2>
        <p style="margin: 0; color: #666;">检查并应用服务端和客户端更新</p>
      </div>
      <n-button quaternary @click="router.push('/')">
        <template #icon><n-icon><ArrowBackOutline /></n-icon></template>
        返回首页
      </n-button>
    </n-space>

    <n-spin :show="loading">
      <!-- 当前版本信息 -->
      <n-card title="当前版本" style="margin-bottom: 16px;">
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

      <n-grid :cols="2" :x-gap="16" responsive="screen" cols-s="1">
        <!-- 服务端更新 -->
        <n-gi>
          <n-card title="服务端更新">
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

                <div v-if="serverUpdate.release_note" style="max-height: 150px; overflow-y: auto;">
                  <p style="margin: 0 0 4px 0; color: #666; font-size: 12px;">更新日志:</p>
                  <pre style="margin: 0; white-space: pre-wrap; font-size: 12px; color: #333;">{{ serverUpdate.release_note }}</pre>
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
        </n-gi>

        <!-- 客户端更新 -->
        <n-gi>
          <n-card title="客户端更新">
            <template #header-extra>
              <n-button size="small" :loading="checkingClient" @click="handleCheckClientUpdate">
                <template #icon><n-icon><RefreshOutline /></n-icon></template>
                检查更新
              </n-button>
            </template>

            <n-empty v-if="!clientUpdate" description="点击检查更新按钮查看客户端更新" />

            <template v-else>
              <n-space vertical :size="12">
                <div v-if="clientUpdate.download_url">
                  <p style="margin: 0 0 8px 0; color: #666;">
                    最新版本: {{ clientUpdate.latest }}
                  </p>
                  <p style="margin: 0 0 8px 0; color: #666;">
                    下载文件: {{ clientUpdate.asset_name }}
                    <n-tag size="small" style="margin-left: 8px;">{{ formatBytes(clientUpdate.asset_size) }}</n-tag>
                  </p>
                </div>

                <n-empty v-if="onlineClients.length === 0" description="没有在线的客户端" />

                <template v-else>
                  <n-select
                    v-model:value="selectedClientId"
                    placeholder="选择要更新的客户端"
                    :options="onlineClients.map(c => ({ label: c.nickname || c.id, value: c.id }))"
                  />

                  <n-button
                    type="primary"
                    :disabled="!selectedClientId || !clientUpdate.download_url"
                    @click="handleApplyClientUpdate"
                  >
                    <template #icon><n-icon><RocketOutline /></n-icon></template>
                    推送更新到客户端
                  </n-button>
                </template>
              </n-space>
            </template>
          </n-card>
        </n-gi>
      </n-grid>
    </n-spin>
  </div>
</template>

<style scoped>
.info-item {
  display: flex;
  flex-direction: column;
  padding: 8px 0;
}

.info-item .label {
  font-size: 12px;
  color: #999;
  margin-bottom: 4px;
}

.info-item .value {
  font-size: 14px;
  color: #333;
  font-weight: 500;
}
</style>
