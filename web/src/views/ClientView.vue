<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ArrowBackOutline, CreateOutline, TrashOutline,
  PushOutline, AddOutline, RefreshOutline
} from '@vicons/ionicons5'
import GlassModal from '../components/GlassModal.vue'
import GlassTag from '../components/GlassTag.vue'
import GlassSwitch from '../components/GlassSwitch.vue'
import { useToast } from '../composables/useToast'
import { useConfirm } from '../composables/useConfirm'
import {
  getClient, updateClient, deleteClient, pushConfigToClient, disconnectClient, restartClient,
  checkClientUpdate, applyClientUpdate, getClientSystemStats, getVersionInfo, getServerConfig,
  getClientScreenshot,
  type UpdateInfo, type SystemStats, type ScreenshotData
} from '../api'
import type { ProxyRule } from '../types'

const route = useRoute()
const router = useRouter()
const message = useToast()
const dialog = useConfirm()
const clientId = route.params.id as string

// Data
const online = ref(false)
const lastPing = ref('')
const lastOfflineAt = ref(0)
const remoteAddr = ref('')
const nickname = ref('')
const rules = ref<ProxyRule[]>([])
const loading = ref(false)
const clientOs = ref('')
const clientArch = ref('')
const clientVersion = ref('')
const serverHeartbeatSec = ref(0)
const serverHeartbeatTimeout = ref(0)
const serverResponseTimeout = ref(0)

// 客户端更新相关
const clientUpdate = ref<UpdateInfo | null>(null)
const updatingClient = ref(false)
const serverVersion = ref('')

// 系统状态相关
  // 系统状态相关
  const systemStats = ref<SystemStats | null>(null)
  const loadingStats = ref(false)
  
  // 截图相关
  const screenshotData = ref<ScreenshotData | null>(null)
  const loadingScreenshot = ref(false)
  const autoRefreshScreenshot = ref(false)
  const screenshotInterval = ref(5) // 默认 5s
  const screenshotTimer = ref<number | null>(null)
  
// Built-in Types (Added WebSocket)
const builtinTypes = [
  { label: 'TCP', value: 'tcp' },
  { label: 'UDP', value: 'udp' },
  { label: 'HTTP', value: 'http' },
  { label: 'HTTPS', value: 'https' },
  { label: 'SOCKS5', value: 'socks5' },
  { label: 'WebSocket', value: 'websocket' }
]

// Modal Control for Rules
const showRuleModal = ref(false)
const ruleModalType = ref<'create' | 'edit'>('create')
// Default Rule Model
const defaultRule = {
  name: '',
  local_ip: '127.0.0.1',
  local_port: 80,
  remote_port: 0,
  type: 'tcp',
  enabled: true
}
const ruleForm = ref<ProxyRule>({ ...defaultRule })

// Helper: Check if type needs local addr
const needsLocalAddr = () => {
  return true
}

const canRemoteControl = computed(() => online.value && clientOs.value === 'windows')

const openRemoteControlPage = () => {
  if (!canRemoteControl.value) return
  router.push({ name: 'remote-control', params: { id: clientId } })
}

// 加载服务端版本
const loadServerVersion = async () => {
  try {
    const { data } = await getVersionInfo()
    serverVersion.value = data.version || ''
  } catch (e) {
    console.error('Failed to load server version', e)
  }
}

const loadServerRuntimeConfig = async () => {
  try {
    const { data } = await getServerConfig()
    serverHeartbeatSec.value = data.server.heartbeat_sec
    serverHeartbeatTimeout.value = data.server.heartbeat_timeout
    serverResponseTimeout.value = data.server.client_response_timeout_sec
  } catch (e) {
    console.error('Failed to load server runtime config', e)
  }
}

// 版本比较函数：返回 -1 (v1 < v2), 0 (v1 == v2), 1 (v1 > v2)
const compareVersions = (v1: string, v2: string): number => {
  const normalize = (v: string) => v.replace(/^v/, '').split('.').map(n => parseInt(n, 10) || 0)
  const parts1 = normalize(v1)
  const parts2 = normalize(v2)
  const len = Math.max(parts1.length, parts2.length)
  for (let i = 0; i < len; i++) {
    const p1 = parts1[i] || 0
    const p2 = parts2[i] || 0
    if (p1 < p2) return -1
    if (p1 > p2) return 1
  }
  return 0
}

// 判断客户端是否需要更新
// 逻辑：如果客户端最新版>=服务端版本，则目标版本为服务端版本；否则为客户端最新版
const needsUpdate = (): boolean => {
  if (!clientUpdate.value?.latest || !clientVersion.value) return false
  const latestClientVer = clientUpdate.value.latest
  const currentClientVer = clientVersion.value
  const serverVer = serverVersion.value

  // 确定目标版本
  let targetVersion = latestClientVer
  if (serverVer && compareVersions(latestClientVer, serverVer) >= 0) {
    targetVersion = serverVer
  }

  // 比较当前客户端版本和目标版本
  return compareVersions(currentClientVer, targetVersion) < 0
}

// 获取目标更新版本
const getTargetVersion = (): string => {
  if (!clientUpdate.value?.latest) return ''
  const latestClientVer = clientUpdate.value.latest
  const serverVer = serverVersion.value

  if (serverVer && compareVersions(latestClientVer, serverVer) >= 0) {
    return serverVer
  }
  return latestClientVer
}

// Actions
const loadClient = async () => {
  loading.value = true
  try {
    const { data } = await getClient(clientId)
    online.value = data.online
    lastPing.value = data.last_ping || ''
    lastOfflineAt.value = data.last_offline_at || 0
    remoteAddr.value = data.remote_addr || ''
    nickname.value = data.nickname || ''
    rules.value = data.rules || []
    clientOs.value = data.os || ''
    clientArch.value = data.arch || ''
    clientVersion.value = data.version || ''

    // 如果客户端在线且有平台信息，自动检测更新
    if (data.online && data.os && data.arch) {
      autoCheckClientUpdate()
      loadSystemStats()
    }
  } catch (e) {
    message.error('加载客户端信息失败')
    console.error(e)
  } finally {
    loading.value = false
  }
}

const formatDateTime = (value?: string | number) => {
  if (!value) return '-'
  const date = typeof value === 'number' ? new Date(value * 1000) : new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString()
}

const heartbeatSummary = () => {
  if (!serverHeartbeatSec.value || !serverHeartbeatTimeout.value) return '-'
  return `${serverHeartbeatSec.value}s / ${serverHeartbeatTimeout.value}s`
}

const connectionStateText = () => {
  if (online.value) return '在线'
  if (lastOfflineAt.value) return `离线于 ${formatDateTime(lastOfflineAt.value)}`
  return '离线'
}

// 自动检测客户端更新（静默）
const autoCheckClientUpdate = async () => {
  try {
    const { data } = await checkClientUpdate(clientVersion.value, clientOs.value, clientArch.value)
    clientUpdate.value = data
  } catch (e) {
    console.error('Auto check update failed', e)
  }
}

// 加载系统状态
const loadSystemStats = async () => {
  if (!online.value) return
  loadingStats.value = true
  try {
    const { data } = await getClientSystemStats(clientId)
    systemStats.value = data
  } catch (e) {
    console.error('Failed to load system stats', e)
  } finally {
    loadingStats.value = false
  }
}

// 截图相关方法
const loadScreenshot = async () => {
  if (!online.value) return
  loadingScreenshot.value = true
  try {
    const { data } = await getClientScreenshot(clientId, 70) // 默认质量 70
    screenshotData.value = data
  } catch (e: any) {
    message.error(e.response?.data?.message || '获取截图失败')
  } finally {
    loadingScreenshot.value = false
  }
}

const toggleAutoRefresh = () => {
  if (autoRefreshScreenshot.value) {
    // 开启自动刷新
    loadScreenshot()
    screenshotTimer.value = window.setInterval(loadScreenshot, screenshotInterval.value * 1000)
  } else {
    // 关闭自动刷新
    if (screenshotTimer.value) {
      clearInterval(screenshotTimer.value)
      screenshotTimer.value = null
    }
  }
}

// 格式化字节大小
const formatBytes = (bytes: number): string => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

// 客户端更新
const handleApplyClientUpdate = () => {
  if (!clientUpdate.value?.download_url) {
    message.error('没有可用的下载链接')
    return
  }
  dialog.warning({
    title: '确认更新客户端',
    content: `即将更新客户端到 ${clientUpdate.value.latest}，更新后客户端将自动重启。确定要继续吗？`,
    positiveText: '更新',
    negativeText: '取消',
    onPositiveClick: async () => {
      updatingClient.value = true
      try {
        await applyClientUpdate(clientId, clientUpdate.value!.download_url!)
        message.success('更新命令已发送，客户端将自动重启')
        clientUpdate.value = null
      } catch (e: any) {
        message.error(e.response?.data || '更新失败')
      } finally {
        updatingClient.value = false
      }
    }
  })
}

// Client Rename
const showRenameModal = ref(false)
const renameValue = ref('')
const openRenameModal = () => {
  renameValue.value = nickname.value
  showRenameModal.value = true
}
const saveRename = async () => {
  try {
    await updateClient(clientId, {
      nickname: renameValue.value,
      rules: rules.value
    })
    nickname.value = renameValue.value
    showRenameModal.value = false
    message.success('重命名成功')
  } catch (e) {
    message.error('重命名失败')
  }
}

// Rule Management
const openCreateRule = () => {
  ruleModalType.value = 'create'
  ruleForm.value = { ...defaultRule, remote_port: 8080 }
  showRuleModal.value = true
}

const openEditRule = (rule: ProxyRule) => {
  ruleModalType.value = 'edit'
  ruleForm.value = JSON.parse(JSON.stringify(rule))
  showRuleModal.value = true
}

const handleDeleteRule = (rule: ProxyRule) => {
  dialog.warning({
    title: '确认删除',
    content: `确定要删除规则 "${rule.name}" 吗？`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      const newRules = rules.value.filter(r => r.name !== rule.name)
      await saveRules(newRules)
    }
  })
}

const saveRules = async (newRules: ProxyRule[]) => {
  try {
    await updateClient(clientId, {
      nickname: nickname.value,
      rules: newRules
    })
    rules.value = newRules
    message.success('规则保存成功')
    if (online.value) {
      await pushConfigToClient(clientId)
      message.success('配置已推送到客户端')
    }
  } catch (e: any) {
    message.error('保存失败: ' + (e.response?.data?.message || e.message))
    await loadClient()
  }
}

const handleRuleSubmit = async () => {
  // Simple validation
  if (!ruleForm.value.name) {
    message.error('请输入规则名称')
    return
  }
  if (!ruleForm.value.remote_port || ruleForm.value.remote_port < 1 || ruleForm.value.remote_port > 65535) {
    message.error('请输入有效的远程端口 (1-65535)')
    return
  }
  if (needsLocalAddr()) {
    if (!ruleForm.value.local_ip) {
      message.error('请输入本地IP')
      return
    }
    if (!ruleForm.value.local_port || ruleForm.value.local_port < 1 || ruleForm.value.local_port > 65535) {
      message.error('请输入有效的本地端口 (1-65535)')
      return
    }
  }

  let newRules = [...rules.value]
  if (ruleModalType.value === 'create') {
    if (newRules.some(r => r.name === ruleForm.value.name)) {
      message.error('规则名称已存在')
      return
    }
    newRules.push({ ...ruleForm.value })
  } else {
    const index = newRules.findIndex(r => r.name === ruleForm.value.name)
    if (index > -1) {
      newRules[index] = { ...ruleForm.value }
    }
  }
  await saveRules(newRules)
  showRuleModal.value = false
}

// Standard Client Actions
const confirmDelete = () => {
    dialog.warning({
        title: '确认删除', content: '确定要删除此客户端吗？',
        positiveText: '删除', negativeText: '取消',
        onPositiveClick: async () => {
            await deleteClient(clientId); router.push('/')
        }
    })
}
const disconnect = () => {
     dialog.warning({
        title: '确认断开', content: '确定要断开连接吗？',
        positiveText: '断开', negativeText: '取消',
        onPositiveClick: async () => {
            await disconnectClient(clientId); loadClient()
        }
    })
}
const handleRestartClient = () => {
     dialog.warning({
        title: '确认重启', content: '确定要重启客户端吗？',
        positiveText: '重启', negativeText: '取消',
        onPositiveClick: async () => {
            await restartClient(clientId); message.success('重启命令已发送'); setTimeout(loadClient, 3000)
        }
    })
}

// Lifecycle
const pollTimer = ref<number | null>(null)

onMounted(() => {
  loadServerVersion()
  loadServerRuntimeConfig()
  loadClient()
  // 启动自动轮询，每 10 秒刷新一次
  pollTimer.value = window.setInterval(() => {
    loadClient()
  }, 10000)
})

onUnmounted(() => {
  if (pollTimer.value) {
    clearInterval(pollTimer.value)
    pollTimer.value = null
  }
  if (screenshotTimer.value) {
    clearInterval(screenshotTimer.value)
    screenshotTimer.value = null
  }
})
</script>

<template>
  <div class="client-page">
    <!-- Particles -->
    <div class="particles">
      <div class="particle particle-1"></div>
      <div class="particle particle-2"></div>
      <div class="particle particle-3"></div>
    </div>

    <div class="client-content">
      <!-- Header -->
      <div class="page-header">
        <div class="header-left">
          <button class="back-btn" @click="router.push('/')">
            <ArrowBackOutline class="btn-icon-lg" />
          </button>
          <h1 class="page-title">{{ nickname || clientId }}</h1>
          <button class="edit-btn" @click="openRenameModal">
            <CreateOutline class="btn-icon" />
          </button>
          <span class="status-tag" :class="{ online }">
            {{ online ? '在线' : '离线' }}
          </span>
        </div>
        <div class="header-actions">
          <button v-if="online" class="glass-btn primary" @click="pushConfigToClient(clientId).then(() => message.success('已推送'))">
            <PushOutline class="btn-icon" />
            <span>推送配置</span>
          </button>
          <button class="glass-btn danger" @click="confirmDelete">
            <TrashOutline class="btn-icon" />
            <span>删除</span>
          </button>
        </div>
      </div>

      <!-- Main Grid -->
      <div class="main-grid">
        <div class="side-column">
          <!-- Status Card -->
          <div class="glass-card">
            <div class="card-header">
              <h3>客户端状态</h3>
              <!-- Heartbeat indicator -->
              <div class="heartbeat-indicator" :class="{ online: online, offline: !online }">
                <span class="heartbeat-dot"></span>
              </div>
            </div>
            <div class="card-body">
              <div class="stat-item">
                <span class="stat-label">连接 ID</span>
                <span class="stat-value mono">{{ clientId }}</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">连接状态</span>
                <span class="stat-value">{{ connectionStateText() }}</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">{{ online ? '当前远程 IP' : '离线时 IP' }}</span>
                <span class="stat-value">{{ remoteAddr || '-' }}</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">客户端版本</span>
                <span class="stat-value">
                  {{ clientVersion || '-' }}
                  <span v-if="needsUpdate()" class="update-badge" @click="handleApplyClientUpdate">
                    可更新 → {{ getTargetVersion() }}
                  </span>
                  <span v-else-if="clientVersion" class="latest-badge">
                    最新版本
                  </span>
                </span>
              </div>
              <div class="stat-item">
                <span class="stat-label">{{ online ? '最后心跳' : '离线时间' }}</span>
                <span class="stat-value">{{ online ? formatDateTime(lastPing) : formatDateTime(lastOfflineAt) }}</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">系统平台</span>
                <span class="stat-value">{{ clientOs && clientArch ? `${clientOs}/${clientArch}` : '-' }}</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">服务端心跳策略</span>
                <span class="stat-value">{{ heartbeatSummary() }}</span>
              </div>
              <div class="stat-item">
                <span class="stat-label">客户端响应超时</span>
                <span class="stat-value">{{ serverResponseTimeout ? `${serverResponseTimeout}s` : '-' }}</span>
              </div>
            </div>
            <div class="card-actions">
              <button class="glass-btn warning small" @click="disconnect" :disabled="!online">断开连接</button>
              <button class="glass-btn danger small" @click="handleRestartClient" :disabled="!online">重启客户端</button>
            </div>
          </div>

          <!-- System Stats Card -->
          <div class="glass-card" v-if="online">
            <div class="card-header">
              <h3>系统状态</h3>
              <button class="glass-btn tiny" :disabled="loadingStats" @click="loadSystemStats">
                <RefreshOutline class="btn-icon-sm" />
                刷新
              </button>
            </div>
            <div class="card-body system-stats-body">
              <Transition name="fade-slide" mode="out-in">
                <div v-if="!systemStats" class="empty-hint" key="empty">
                  {{ loadingStats ? '加载中...' : '点击刷新获取状态' }}
                </div>
                <div v-else class="system-stats-content" key="stats">
                <div class="system-stat-item">
                  <span class="system-stat-label">CPU</span>
                  <div class="progress-bar">
                    <div class="progress-fill" :style="{ width: systemStats.cpu_usage + '%' }"></div>
                  </div>
                  <span class="system-stat-value">{{ systemStats.cpu_usage.toFixed(1) }}%</span>
                </div>
                <div class="system-stat-item">
                  <span class="system-stat-label">内存</span>
                  <div class="progress-bar">
                    <div class="progress-fill" :style="{ width: systemStats.memory_usage + '%' }"></div>
                  </div>
                  <span class="system-stat-value">{{ systemStats.memory_usage.toFixed(1) }}%</span>
                </div>
                <div class="system-stat-detail">
                  {{ formatBytes(systemStats.memory_used) }} / {{ formatBytes(systemStats.memory_total) }}
                </div>
                <div class="system-stat-item">
                  <span class="system-stat-label">磁盘</span>
                  <div class="progress-bar">
                    <div class="progress-fill" :style="{ width: systemStats.disk_usage + '%' }"></div>
                  </div>
                  <span class="system-stat-value">{{ systemStats.disk_usage.toFixed(1) }}%</span>
                </div>
                <div class="system-stat-detail">
                  {{ formatBytes(systemStats.disk_used) }} / {{ formatBytes(systemStats.disk_total) }}
                </div>
                </div>
              </Transition>
            </div>
          </div>
        </div>

        <div class="content-column">
          <!-- Rules Card -->
          <div class="glass-card">
            <div class="card-header">
              <h3>代理规则</h3>
              <button class="glass-btn primary small" @click="openCreateRule">
                <AddOutline class="btn-icon-sm" />
                添加规则
              </button>
            </div>
            <div class="card-body">
              <div v-if="rules.length === 0" class="empty-state">
                <p>暂无代理规则</p>
              </div>
              <div v-else class="rules-table">
                <div class="table-header">
                  <span>名称</span>
                  <span>类型</span>
                  <span>映射</span>
                  <span>状态</span>
                  <span>操作</span>
                </div>
                <div v-for="rule in rules" :key="rule.name" class="table-row">
                  <span class="rule-name">{{ rule.name }}</span>
                  <span><GlassTag :type="rule.type==='websocket'?'info':'default'">{{ (rule.type || 'tcp').toUpperCase() }}</GlassTag></span>
                  <span class="rule-mapping">
                    {{ needsLocalAddr() ? `${rule.local_ip}:${rule.local_port}` : '-' }}
                    →
                    :{{ rule.remote_port }}
                  </span>
                  <span>
                    <GlassSwitch :model-value="rule.enabled !== false" @update:model-value="(v: boolean) => { rule.enabled = v; saveRules(rules) }" size="small" />
                  </span>
                  <span class="rule-actions">
                    <button class="icon-btn" @click="openEditRule(rule)">编辑</button>
                    <button class="icon-btn danger" @click="handleDeleteRule(rule)">删除</button>
                  </span>
                </div>
              </div>
            </div>
          </div>

        </div>
      </div>

      <div v-if="online" class="bottom-workspace-grid">
        <div class="glass-card workspace-card workspace-card--screenshot">
          <div class="card-header">
            <h3>屏幕截图</h3>
            <div class="header-controls">
              <button v-if="canRemoteControl" class="glass-btn tiny" @click="openRemoteControlPage">
                远程控制
              </button>
              <GlassSwitch :model-value="autoRefreshScreenshot" @update:model-value="(v: boolean) => { autoRefreshScreenshot = v; toggleAutoRefresh() }" size="small">
                自动刷新
              </GlassSwitch>
              <button class="glass-btn tiny" :disabled="loadingScreenshot" @click="loadScreenshot">
                <RefreshOutline class="btn-icon-sm" />
              </button>
            </div>
          </div>
          <div class="card-body screenshot-body">
            <div class="screenshot-container" v-if="screenshotData">
              <img :src="`data:image/jpeg;base64,${screenshotData.data}`" alt="Screenshot" class="screenshot-img" />
              <div class="screenshot-meta">
                {{ new Date(screenshotData.timestamp).toLocaleTimeString() }} ({{ screenshotData.width }}x{{ screenshotData.height }})
              </div>
            </div>
            <button
              v-else
              type="button"
              class="empty-hint empty-hint--interactive screenshot-empty-state"
              :disabled="loadingScreenshot"
              @click="loadScreenshot"
            >
              {{ loadingScreenshot ? '截图中...' : '点击获取截图' }}
            </button>
          </div>
        </div>

      </div>
    </div>

    <!-- Rule Modal -->
    <GlassModal :show="showRuleModal" :title="ruleModalType==='create'?'添加规则':'编辑规则'" @close="showRuleModal = false">
      <div class="form-group">
        <label class="form-label">名称</label>
        <input v-model="ruleForm.name" class="form-input" placeholder="请输入规则名称" :disabled="ruleModalType==='edit'" />
      </div>
      <div class="form-group">
        <label class="form-label">类型</label>
        <select v-model="ruleForm.type" class="form-select">
          <option v-for="t in builtinTypes" :key="t.value" :value="t.value">{{ t.label }}</option>
        </select>
      </div>
      <template v-if="needsLocalAddr()">
        <div class="form-group">
          <label class="form-label">本地IP</label>
          <input v-model="ruleForm.local_ip" class="form-input" placeholder="127.0.0.1" />
        </div>
        <div class="form-group">
          <label class="form-label">本地端口</label>
          <input v-model.number="ruleForm.local_port" type="number" class="form-input" min="1" max="65535" />
        </div>
      </template>
      <div class="form-group">
        <label class="form-label">远程端口</label>
        <input v-model.number="ruleForm.remote_port" type="number" class="form-input" min="1" max="65535" />
      </div>
      <template #footer>
        <button class="glass-btn" @click="showRuleModal = false">取消</button>
        <button class="glass-btn primary" @click="handleRuleSubmit">保存</button>
      </template>
    </GlassModal>


    <!-- Rename Modal -->
    <GlassModal :show="showRenameModal" title="重命名客户端" width="400px" @close="showRenameModal = false">
      <div class="form-group">
        <label class="form-label">新名称</label>
        <input v-model="renameValue" class="form-input" placeholder="请输入新名称" />
      </div>
      <template #footer>
        <button class="glass-btn" @click="showRenameModal = false">取消</button>
        <button class="glass-btn primary" @click="saveRename">保存</button>
      </template>
    </GlassModal>
  </div>
</template>

<style scoped>
.client-page {
  min-height: calc(100vh - 108px);
  background: transparent;
  position: relative;
  overflow: hidden;
  padding: 32px;
}

/* Particles */
.particles {
  position: absolute;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
  z-index: 0;
}

.particle {
  position: absolute;
  border-radius: 50%;
  opacity: 0.15;
  filter: blur(60px);
  animation: float 20s ease-in-out infinite;
}

.particle-1 {
  width: 350px;
  height: 350px;
  background: var(--color-accent);
  top: -80px;
  right: -80px;
}

.particle-2 {
  width: 280px;
  height: 280px;
  background: var(--color-warning);
  bottom: -40px;
  left: -40px;
  animation-delay: -5s;
}

.particle-3 {
  width: 220px;
  height: 220px;
  background: var(--color-success);
  top: 40%;
  left: 30%;
  animation-delay: -10s;
}

@keyframes float {
  0%, 100% { transform: translate(0, 0) scale(1); }
  25% { transform: translate(30px, -30px) scale(1.05); }
  50% { transform: translate(-20px, 20px) scale(0.95); }
  75% { transform: translate(-30px, -20px) scale(1.02); }
}

.client-content {
  position: relative;
  z-index: 10;
  max-width: 1480px;
  margin: 0 auto;
}

/* Page Header */
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 28px;
  flex-wrap: wrap;
  gap: 16px;
}

.header-left {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  min-width: 0;
}

.back-btn, .edit-btn {
  background: var(--color-bg-tertiary);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 8px;
  color: var(--color-text-secondary);
  cursor: pointer;
  transition: all 0.15s;
  display: flex;
  align-items: center;
}

.back-btn:hover, .edit-btn:hover {
  background: var(--color-bg-elevated);
  color: var(--color-text-primary);
}

.page-title {
  font-size: clamp(24px, 3vw, 34px);
  font-weight: 700;
  color: var(--color-text-primary);
  margin: 0;
  min-width: 0;
  word-break: break-word;
}

.status-tag {
  padding: 4px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;
  background: rgba(239, 68, 68, 0.15);
  color: var(--color-error);
}

.status-tag.online {
  background: rgba(36, 166, 122, 0.16);
  color: var(--color-success);
}

.header-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

/* Glass Button */
.glass-btn {
  background: var(--color-bg-tertiary);
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
  background: var(--color-bg-elevated);
}

.glass-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.glass-btn.primary {
  background: var(--color-accent);
  border: none;
}

.glass-btn.primary:hover:not(:disabled) {
  background: var(--color-accent-hover);
}

.glass-btn.danger {
  background: rgba(244, 33, 46, 0.15);
  border-color: rgba(244, 33, 46, 0.3);
  color: var(--color-error);
}

.glass-btn.warning {
  background: rgba(213, 138, 45, 0.15);
  border-color: rgba(213, 138, 45, 0.3);
  color: var(--color-warning);
}

.glass-btn.small { padding: 6px 12px; font-size: 12px; }
.glass-btn.tiny { padding: 4px 8px; font-size: 11px; }
.glass-btn.full { width: 100%; justify-content: center; }

/* Main Grid */
.main-grid {
  display: grid;
  grid-template-columns: minmax(280px, 360px) minmax(0, 1fr);
  gap: 20px;
  align-items: start;
}

.side-column,
.content-column {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.bottom-workspace-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 20px;
  align-items: stretch;
  margin-top: 20px;
}

.workspace-card {
  min-height: 100%;
}

/* Glass Card */
.glass-card {
  background: var(--color-bg-tertiary);
  border-radius: 12px;
  border: 1px solid var(--color-border);
  overflow: hidden;
}

.card-header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--color-border-light);
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.card-header h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text-primary);
}

/* Heartbeat Indicator */
.heartbeat-indicator {
  position: relative;
}

.heartbeat-dot {
  display: block;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--color-error);
}

.heartbeat-indicator.online .heartbeat-dot {
  background: var(--color-success);
  animation: heartbeat-pulse 2s ease-in-out infinite;
}

.heartbeat-indicator.offline .heartbeat-dot {
  background: var(--color-error);
  animation: none;
}

@keyframes heartbeat-pulse {
  0%, 100% {
    box-shadow: 0 0 0 0 rgba(36, 166, 122, 0.5);
    transform: scale(1);
  }
  50% {
    box-shadow: 0 0 0 6px rgba(36, 166, 122, 0);
    transform: scale(1.1);
  }
}

.card-body { padding: 20px; }
.card-actions {
  padding: 16px 20px;
  border-top: 1px solid var(--color-border-light);
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

/* Stat Items */
.stat-item {
  display: flex;
  justify-content: space-between;
  padding: 8px 0;
  border-bottom: 1px solid var(--color-border-light);
}

.stat-item:last-child { border-bottom: none; }

.stat-label {
  color: var(--color-text-secondary);
  font-size: 13px;
}

.stat-value {
  color: var(--color-text-primary);
  font-size: 13px;
}

.stat-value.mono {
  font-family: monospace;
  font-size: 12px;
}

.update-badge {
  display: inline-block;
  margin-left: 8px;
  padding: 2px 8px;
  font-size: 11px;
  background: rgba(213, 138, 45, 0.15);
  color: var(--color-warning);
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.15s;
}

.update-badge:hover {
  background: rgba(213, 138, 45, 0.25);
}

.latest-badge {
  display: inline-block;
  margin-left: 8px;
  padding: 2px 8px;
  font-size: 11px;
  background: rgba(36, 166, 122, 0.16);
  color: var(--color-success);
  border-radius: 10px;
}

/* Update Card */
.platform-info {
  font-size: 12px;
  color: var(--color-text-muted);
  margin-bottom: 8px;
}

.empty-hint {
  color: var(--color-text-muted);
  font-size: 13px;
  text-align: center;
  padding: 16px 0;
}

.empty-hint--interactive {
  border: none;
  background: none;
  width: 100%;
  font: inherit;
}

.screenshot-empty-state {
  min-height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}

.screenshot-empty-state:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.04);
  color: var(--color-text-primary);
}

.screenshot-empty-state:disabled {
  cursor: wait;
}

.update-available p {
  margin: 0 0 8px 0;
  color: var(--color-success);
  font-size: 13px;
}

/* Rules Table */
.empty-state {
  text-align: center;
  padding: 32px;
  color: var(--color-text-muted);
}

.rules-table {
  overflow-x: auto;
}

.table-header, .table-row {
  display: grid;
  grid-template-columns: 1fr 80px 1.5fr 60px 100px;
  gap: 12px;
  padding: 10px 0;
  align-items: center;
  min-width: 680px;
}

.table-header {
  border-bottom: 1px solid var(--color-border);
  color: var(--color-text-muted);
  font-size: 12px;
  font-weight: 500;
}

.table-row {
  border-bottom: 1px solid var(--color-border-light);
  color: var(--color-text-secondary);
  font-size: 13px;
}

.rule-name { font-weight: 500; color: var(--color-text-primary); }
.rule-mapping { font-family: monospace; font-size: 12px; }
.rule-actions { display: flex; gap: 6px; justify-content: flex-end; flex-wrap: wrap; }

/* Icon Button */
.icon-btn {
  background: var(--color-bg-elevated);
  border: none;
  border-radius: 6px;
  padding: 4px 10px;
  color: var(--color-text-secondary);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s;
}

.icon-btn:hover:not(:disabled) {
  background: var(--color-border);
  color: var(--color-text-primary);
}

.icon-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.icon-btn.danger {
  color: var(--color-error);
}

.icon-btn.danger:hover:not(:disabled) {
  background: rgba(244, 33, 46, 0.15);
}

.icon-btn.success {
  color: var(--color-success);
}

.icon-btn.success:hover:not(:disabled) {
  background: rgba(36, 166, 122, 0.16);
}

/* Form Styles */
.form-group {
  margin-bottom: 16px;
}

.form-label {
  display: block;
  font-size: 13px;
  color: var(--color-text-secondary);
  margin-bottom: 6px;
}

.form-input {
  width: 100%;
  background: var(--color-bg-elevated);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 10px 12px;
  color: var(--color-text-primary);
  font-size: 14px;
  outline: none;
  transition: border-color 0.15s;
  box-sizing: border-box;
}

.form-input:focus {
  border-color: var(--color-accent);
}

.form-input::placeholder {
  color: var(--color-text-muted);
}

.form-input:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.form-select {
  width: 100%;
  background: var(--color-bg-elevated);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 10px 12px;
  color: var(--color-text-primary);
  font-size: 14px;
  outline: none;
  cursor: pointer;
}

.form-select option {
  background: var(--color-bg-tertiary);
  color: var(--color-text-primary);
}

.form-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--color-text-secondary);
  font-size: 13px;
  cursor: pointer;
}

.form-toggle input[type="checkbox"] {
  width: 18px;
  height: 18px;
  accent-color: var(--color-accent);
}

.loading-state {
  text-align: center;
  padding: 32px;
  color: var(--color-text-muted);
}

/* Dropdown Menu */
.dropdown-wrapper {
  position: relative;
}

.dropdown-menu {
  position: absolute;
  top: 100%;
  right: 0;
  margin-top: 4px;
  background: var(--color-bg-tertiary);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 4px;
  min-width: 100px;
  z-index: 100;
}

.dropdown-menu button {
  display: block;
  width: 100%;
  padding: 8px 12px;
  background: none;
  border: none;
  color: var(--color-text-secondary);
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  border-radius: 4px;
  transition: all 0.15s;
}

.dropdown-menu button:hover:not(:disabled) {
  background: var(--color-bg-elevated);
  color: var(--color-text-primary);
}

.dropdown-menu button:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.dropdown-menu button.danger {
  color: var(--color-error);
}

.dropdown-menu button.danger:hover:not(:disabled) {
  background: rgba(244, 33, 46, 0.15);
}

/* Icon styles */
.btn-icon {
  width: 14px;
  height: 14px;
}

.btn-icon-lg {
  width: 20px;
  height: 20px;
}

.btn-icon-sm {
  width: 14px;
  height: 14px;
}

.settings-icon {
  width: 16px;
  height: 16px;
}

/* System Stats Transition */
.system-stats-body {
  overflow: hidden;
}

.system-stats-content {
  display: flex;
  flex-direction: column;
}


.fade-slide-enter-active,
.fade-slide-leave-active {
  transition: all 0.3s ease;
}

.fade-slide-enter-from,
.fade-slide-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}

/* Screenshot Card */
.screenshot-body {
  padding: 0;
  display: flex;
  justify-content: center;
  align-items: stretch;
  min-height: 320px;
  background:
    radial-gradient(circle at top right, color-mix(in srgb, var(--color-accent) 14%, transparent), transparent 42%),
    rgba(0, 0, 0, 0.18);
  border-radius: 0 0 16px 16px;
  overflow: hidden;
  position: relative;
}

.screenshot-container {
  width: 100%;
  height: 100%;
  position: relative;
}

.screenshot-img {
  width: 100%;
  height: 100%;
  display: block;
  object-fit: contain;
  background: rgba(0, 0, 0, 0.14);
}

.screenshot-meta {
  position: absolute;
  bottom: 0;
  right: 0;
  background: rgba(7, 19, 26, 0.8);
  color: #fff;
  padding: 8px 10px;
  font-size: 12px;
  border-top-left-radius: 8px;
  font-family: monospace;
}

.header-controls {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}


.fade-slide-enter-from {
  opacity: 0;
  transform: translateY(-10px);
}

.fade-slide-leave-to {
  opacity: 0;
  transform: translateY(10px);
}

/* System Stats */
.system-stat-item {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}

.system-stat-label {
  width: 40px;
  font-size: 12px;
  color: var(--color-text-secondary);
}

.progress-bar {
  flex: 1;
  height: 8px;
  background: var(--color-border);
  border-radius: 4px;
  overflow: hidden;
}

.progress-fill {
  height: 100%;
  background: var(--color-accent);
  border-radius: 4px;
  transition: width 0.3s ease;
}

.system-stat-value {
  width: 50px;
  text-align: right;
  font-size: 12px;
  color: var(--color-text-primary);
  font-family: monospace;
}

.system-stat-detail {
  font-size: 11px;
  color: var(--color-text-muted);
  text-align: right;
  margin-bottom: 12px;
  margin-top: -4px;
}

@media (max-width: 1180px) {
  .bottom-workspace-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 980px) {
  .main-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .client-page {
    padding: 16px;
  }

  .header-actions {
    width: 100%;
  }

  .header-actions .glass-btn {
    flex: 1 1 0;
    justify-content: center;
  }

  .card-header,
  .card-body,
  .card-actions {
    padding-left: 16px;
    padding-right: 16px;
  }

  .stat-item {
    flex-direction: column;
    gap: 6px;
    align-items: flex-start;
  }

  .screenshot-body {
    min-height: 240px;
  }

  .screenshot-meta {
    left: 0;
    right: 0;
    border-top-left-radius: 0;
    text-align: center;
  }

  .system-stat-item {
    flex-wrap: wrap;
  }

  .system-stat-label,
  .system-stat-value {
    width: auto;
  }
}
</style>
