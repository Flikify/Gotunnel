<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ArrowBackOutline, CreateOutline, TrashOutline,
  PushOutline, AddOutline, StorefrontOutline,
  ExtensionPuzzleOutline, SettingsOutline, RefreshOutline,
  ImageOutline, TerminalOutline, PlayOutline
} from '@vicons/ionicons5'
import GlassModal from '../components/GlassModal.vue'
import GlassTag from '../components/GlassTag.vue'
import GlassSwitch from '../components/GlassSwitch.vue'
import { useToast } from '../composables/useToast'
import { useConfirm } from '../composables/useConfirm'
import {
  getClient, updateClient, deleteClient, pushConfigToClient, disconnectClient, restartClient,
  getClientPluginConfig, updateClientPluginConfig,
  getStorePlugins, installStorePlugin, getRuleSchemas, startClientPlugin, restartClientPlugin, stopClientPlugin, deleteClientPlugin,
  checkClientUpdate, applyClientUpdate, getClientSystemStats, getVersionInfo,
  getClientScreenshot, executeClientShell,
  type UpdateInfo, type SystemStats, type ScreenshotData, type ShellResult
} from '../api'
import type { ProxyRule, ClientPlugin, ConfigField, StorePluginInfo, RuleSchemasMap } from '../types'
import InlineLogPanel from '../components/InlineLogPanel.vue'

const route = useRoute()
const router = useRouter()
const message = useToast()
const dialog = useConfirm()
const clientId = route.params.id as string

// Data
const online = ref(false)
const lastPing = ref('')
const remoteAddr = ref('')
const nickname = ref('')
const rules = ref<ProxyRule[]>([])
const clientPlugins = ref<ClientPlugin[]>([])
const loading = ref(false)
const clientOs = ref('')
const clientArch = ref('')
const clientVersion = ref('')

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
  
  // Shell 相关
  const shellCommand = ref('')
  const shellOutput = ref('')
  const executingShell = ref(false)
  const shellHistory = ref<string[]>([])
  const historyIndex = ref(-1)

// Rule Schemas
const pluginRuleSchemas = ref<RuleSchemasMap>({})
const loadRuleSchemas = async () => {
  try {
    const { data } = await getRuleSchemas()
    pluginRuleSchemas.value = data || {}
  } catch (e) {
    console.error('Failed to load rule schemas', e)
  }
}

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
  enabled: true,
  plugin_config: {} as Record<string, string>
}
const ruleForm = ref<ProxyRule>({ ...defaultRule })

// Helper: Check if type needs local addr
const needsLocalAddr = (type: string) => {
  const schema = pluginRuleSchemas.value[type]
  return schema?.needs_local_addr ?? true
}

const getExtraFields = (type: string): ConfigField[] => {
  const schema = pluginRuleSchemas.value[type]
  return schema?.extra_fields || []
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
    remoteAddr.value = data.remote_addr || ''
    nickname.value = data.nickname || ''
    rules.value = data.rules || []
    clientPlugins.value = data.plugins || []
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

// 自动检测客户端更新（静默）
const autoCheckClientUpdate = async () => {
  try {
    const { data } = await checkClientUpdate(clientOs.value, clientArch.value)
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

// Shell 相关方法
const executeShell = async () => {
  if (!shellCommand.value.trim()) return
  
  const cmd = shellCommand.value.trim()
  shellCommand.value = ''
  executingShell.value = true
  
  // 添加到历史记录
  shellHistory.value.unshift(cmd)
  if (shellHistory.value.length > 50) shellHistory.value.pop()
  historyIndex.value = -1
  
  shellOutput.value += `\n> ${cmd}\n`
  
  try {
    const { data } = await executeClientShell(clientId, cmd)
    if (data.error) {
       shellOutput.value += `Error: ${data.error}\n`
    } else {
       shellOutput.value += data.output + '\n'
    }
    if (data.exit_code !== 0) {
       shellOutput.value += `Exit Code: ${data.exit_code}\n`
    }
  } catch (e: any) {
    shellOutput.value += `Error: ${e.message}\n`
  } finally {
    executingShell.value = false
    // 滚动到底部 (需要 nextTick 和 ref)
    setTimeout(() => {
        const textarea = document.getElementById('shell-output')
        if (textarea) textarea.scrollTop = textarea.scrollHeight
    }, 100)
  }
}

const handleShellHistory = (direction: 'up' | 'down') => {
    if (shellHistory.value.length === 0) return
    
    if (direction === 'up') {
        if (historyIndex.value < shellHistory.value.length - 1) {
            historyIndex.value++
            shellCommand.value = shellHistory.value[historyIndex.value]
        }
    } else {
        if (historyIndex.value > 0) {
            historyIndex.value--
            shellCommand.value = shellHistory.value[historyIndex.value]
        } else if (historyIndex.value === 0) {
            historyIndex.value = -1
            shellCommand.value = ''
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
        await applyClientUpdate(clientId, clientUpdate.value!.download_url)
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
      id: clientId,
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
  if (rule.plugin_managed) return
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
      id: clientId,
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
    message.error('保存失败: ' + (e.response?.data || e.message))
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
  if (needsLocalAddr(ruleForm.value.type || 'tcp')) {
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

// Store & Plugin Logic
const showStoreModal = ref(false)
const storePlugins = ref<StorePluginInfo[]>([])
const storeLoading = ref(false)
const storeInstalling = ref<string | null>(null)
const showInstallConfigModal = ref(false)
const installPlugin = ref<StorePluginInfo | null>(null)
const installRemotePort = ref<number | null>(8080)
const installAuthEnabled = ref(false)
const installAuthUsername = ref('')
const installAuthPassword = ref('')

const openStoreModal = async () => {
  showStoreModal.value = true
  storeLoading.value = true
  try {
    const { data } = await getStorePlugins()
    storePlugins.value = (data.plugins || []).filter((p: any) => p.download_url)
  } catch (e) {
    message.error('加载商店失败')
  } finally {
    storeLoading.value = false
  }
}
const handleInstallStorePlugin = (plugin: StorePluginInfo) => {
  installPlugin.value = plugin
  installRemotePort.value = 8080
  showInstallConfigModal.value = true
}
const confirmInstallPlugin = async () => {
  if (!installPlugin.value) return
  storeInstalling.value = installPlugin.value.name
  try {
    await installStorePlugin(
      installPlugin.value.name,
      installPlugin.value.download_url || '',
      installPlugin.value.signature_url || '',
      clientId,
      installRemotePort.value || 8080,
      installPlugin.value.version,
      installPlugin.value.config_schema,
      installAuthEnabled.value,
      installAuthUsername.value,
      installAuthPassword.value
    )
    message.success(`已安装 ${installPlugin.value.name}`)
    showInstallConfigModal.value = false
    showStoreModal.value = false
    await loadClient()
  } catch (e: any) {
    message.error(e.response?.data || '安装失败')
  } finally {
    storeInstalling.value = null
  }
}

// Plugin Actions
const handleOpenPlugin = (plugin: ClientPlugin) => {
  if (!plugin.remote_port) return
  const hostname = window.location.hostname
  const url = `http://${hostname}:${plugin.remote_port}`
  window.open(url, '_blank')
}

const toggleClientPlugin = async (plugin: ClientPlugin) => {
  const newEnabled = !plugin.enabled
  const updatedPlugins = clientPlugins.value.map(p =>
    p.id === plugin.id ? { ...p, enabled: newEnabled } : p
  )
  try {
    await updateClient(clientId, {
      id: clientId,
      nickname: nickname.value,
      rules: rules.value,
      plugins: updatedPlugins
    })
    plugin.enabled = newEnabled
    message.success(newEnabled ? `已启用 ${plugin.name}` : `已禁用 ${plugin.name}`)
  } catch (e) {
    message.error('操作失败')
  }
}

// Plugin Config Modal
const showConfigModal = ref(false)
const configPluginName = ref('')
const configSchema = ref<ConfigField[]>([])
const configValues = ref<Record<string, string>>({})
const configLoading = ref(false)
const openConfigModal = async (plugin: ClientPlugin) => {
  configPluginName.value = plugin.name
  configLoading.value = true
  showConfigModal.value = true
  try {
    const { data } = await getClientPluginConfig(clientId, plugin.name)
    configSchema.value = data.schema || []
    configValues.value = { ...data.config }
    configSchema.value.forEach(f => {
      if (f.default && !configValues.value[f.key]) {
        configValues.value[f.key] = f.default
      }
    })
  } catch (e) {
    message.error('加载配置失败')
    showConfigModal.value = false
  } finally {
    configLoading.value = false
  }
}
const savePluginConfig = async () => {
   try {
    await updateClientPluginConfig(clientId, configPluginName.value, configValues.value)
    message.success('配置已保存')
    showConfigModal.value = false
    loadClient()
  } catch (e: any) {
    message.error(e.response?.data || '保存失败')
  }
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
  loadRuleSchemas()
  loadServerVersion()
  loadClient()
  // 启动自动轮询，每 5 秒刷新一次
  pollTimer.value = window.setInterval(() => {
    loadClient()
  }, 5000)
})

onUnmounted(() => {
  if (pollTimer.value) {
    clearInterval(pollTimer.value)
    pollTimer.value = null
  }
})

// Plugin Menu
const activePluginMenu = ref('')
const togglePluginMenu = (pluginId: string) => {
  activePluginMenu.value = activePluginMenu.value === pluginId ? '' : pluginId
}

// Plugin Status Actions
const handleStartPlugin = async (plugin: ClientPlugin) => {
    const rule = rules.value.find(r => r.type === plugin.name)
    const ruleName = rule?.name || plugin.name
    try { await startClientPlugin(clientId, plugin.id, ruleName); message.success('已启动'); plugin.running = true } catch(e:any){ message.error(e.message) }
}
const handleRestartPlugin = async (plugin: ClientPlugin) => {
    const rule = rules.value.find(r => r.type === plugin.name)
    const ruleName = rule?.name || plugin.name
    try { await restartClientPlugin(clientId, plugin.id, ruleName); message.success('已重启'); plugin.running = true } catch(e:any){ message.error(e.message)}
}
const handleStopPlugin = async (plugin: ClientPlugin) => {
    const rule = rules.value.find(r => r.type === plugin.name)
    const ruleName = rule?.name || plugin.name
    try { await stopClientPlugin(clientId, plugin.id, ruleName); message.success('已停止'); plugin.running = false } catch(e:any){ message.error(e.message)}
}
const handleDeletePlugin = (plugin: ClientPlugin) => {
    dialog.warning({
        title: '确认删除', content: `确定要删除插件 ${plugin.name} 吗？`,
        positiveText: '删除', negativeText: '取消',
        onPositiveClick: async () => {
             await deleteClientPlugin(clientId, plugin.id); message.success('已删除'); loadClient()
        }
    })
}
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
        <!-- Left Column -->
        <div class="left-column">
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
                <span class="stat-label">远程 IP</span>
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
                <span class="stat-label">最后心跳</span>
                <span class="stat-value">{{ lastPing ? new Date(lastPing).toLocaleTimeString() : '-' }}</span>
              </div>
            </div>
            <div class="card-actions">
              <button class="glass-btn warning small" @click="disconnect" :disabled="!online">断开连接</button>
              <button class="glass-btn danger small" @click="handleRestartClient" :disabled="!online">重启客户端</button>
            </div>
          </div>

          <!-- Stats Card -->
          <div class="glass-card">
            <div class="card-header">
              <h3>统计</h3>
            </div>
            <div class="card-body stats-row">
              <div class="mini-stat">
                <span class="mini-stat-value">{{ rules.length }}</span>
                <span class="mini-stat-label">规则数</span>
              </div>
              <div class="mini-stat">
                <span class="mini-stat-value">{{ clientPlugins.length }}</span>
                <span class="mini-stat-label">插件数</span>
              </div>
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

          <!-- Screenshot Card -->
          <div class="glass-card" v-if="online">
            <div class="card-header">
              <h3>屏幕截图</h3>
              <div class="header-controls">
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
              <div v-else class="empty-hint" @click="loadScreenshot">
                {{ loadingScreenshot ? '截图中...' : '点击获取截图' }}
              </div>
            </div>
          </div>

          <!-- Shell Terminal Card -->
          <div class="glass-card" v-if="online">
            <div class="card-header">
              <h3>远程 Shell</h3>
            </div>
            <div class="card-body shell-body">
              <textarea 
                id="shell-output" 
                class="shell-output" 
                readonly 
                v-model="shellOutput"
              ></textarea>
              <div class="shell-input-group">
                <input 
                  type="text" 
                  class="glass-input shell-input" 
                  v-model="shellCommand" 
                  @keydown.enter="executeShell"
                  @keydown.up.prevent="handleShellHistory('up')"
                  @keydown.down.prevent="handleShellHistory('down')"
                  placeholder="输入命令..."
                  :disabled="executingShell"
                />
                <button class="glass-btn primary small" :disabled="executingShell" @click="executeShell">
                  <PlayOutline class="btn-icon-sm" />
                </button>
              </div>
            </div>
          </div>

        </div>

        <!-- Right Column -->
        <div class="right-column">
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
                    {{ needsLocalAddr(rule.type||'tcp') ? `${rule.local_ip}:${rule.local_port}` : '-' }}
                    →
                    :{{ rule.remote_port }}
                  </span>
                  <span>
                    <GlassSwitch :model-value="rule.enabled !== false" @update:model-value="(v: boolean) => { rule.enabled = v; saveRules(rules) }" size="small" />
                  </span>
                  <span class="rule-actions">
                    <GlassTag v-if="rule.plugin_managed" type="info" title="此规则由插件管理">插件托管</GlassTag>
                    <template v-else>
                      <button class="icon-btn" @click="openEditRule(rule)">编辑</button>
                      <button class="icon-btn danger" @click="handleDeleteRule(rule)">删除</button>
                    </template>
                  </span>
                </div>
              </div>
            </div>
          </div>

          <!-- Plugins Card -->
          <div class="glass-card">
            <div class="card-header">
              <h3>已安装扩展</h3>
              <button class="glass-btn small" @click="openStoreModal">
                <StorefrontOutline class="btn-icon" />
                插件商店
              </button>
            </div>
            <div class="card-body">
              <div v-if="clientPlugins.length === 0" class="empty-state">
                <p>暂无安装的扩展</p>
              </div>
              <div v-else class="plugins-list">
                <div v-for="plugin in clientPlugins" :key="plugin.id" class="plugin-item">
                  <div class="plugin-info">
                    <ExtensionPuzzleOutline class="plugin-icon" />
                    <span class="plugin-name">{{ plugin.name }}</span>
                    <span class="plugin-version">v{{ plugin.version }}</span>
                  </div>
                  <div class="plugin-meta">
                    <span>端口: {{ plugin.remote_port || '-' }}</span>
                    <GlassTag :type="plugin.running ? 'success' : 'default'" round>
                      {{ plugin.running ? '运行中' : '已停止' }}
                    </GlassTag>
                    <GlassSwitch :model-value="plugin.enabled" size="small" @update:model-value="toggleClientPlugin(plugin)" />
                  </div>
                  <div class="plugin-actions">
                    <button v-if="plugin.running && plugin.remote_port" class="icon-btn success" @click="handleOpenPlugin(plugin)">打开</button>
                    <button v-if="!plugin.running" class="icon-btn" @click="handleStartPlugin(plugin)" :disabled="!online || !plugin.enabled">启动</button>
                    <div class="dropdown-wrapper">
                      <button class="icon-btn" @click="togglePluginMenu(plugin.id)">
                        <SettingsOutline class="settings-icon" />
                      </button>
                      <div v-if="activePluginMenu === plugin.id" class="dropdown-menu">
                        <button @click="handleRestartPlugin(plugin); activePluginMenu = ''" :disabled="!plugin.running">重启</button>
                        <button @click="openConfigModal(plugin); activePluginMenu = ''">配置</button>
                        <button @click="handleStopPlugin(plugin); activePluginMenu = ''" :disabled="!plugin.running">停止</button>
                        <button class="danger" @click="handleDeletePlugin(plugin); activePluginMenu = ''">删除</button>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Inline Log Panel -->
          <div class="glass-card">
            <InlineLogPanel :client-id="clientId" />
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
      <template v-if="needsLocalAddr(ruleForm.type || 'tcp')">
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
      <template v-for="field in getExtraFields(ruleForm.type || '')" :key="field.key">
        <div class="form-group">
          <label class="form-label">{{ field.label }}</label>
          <input v-if="field.type==='string'" v-model="ruleForm.plugin_config![field.key]" class="form-input" />
          <input v-if="field.type==='password'" type="password" v-model="ruleForm.plugin_config![field.key]" class="form-input" />
          <label v-if="field.type==='bool'" class="form-toggle">
            <input type="checkbox" :checked="ruleForm.plugin_config![field.key]==='true'" @change="(e: Event) => ruleForm.plugin_config![field.key] = String((e.target as HTMLInputElement).checked)" />
            <span>启用</span>
          </label>
        </div>
      </template>
      <template #footer>
        <button class="glass-btn" @click="showRuleModal = false">取消</button>
        <button class="glass-btn primary" @click="handleRuleSubmit">保存</button>
      </template>
    </GlassModal>

    <!-- Config Modal -->
    <GlassModal :show="showConfigModal" :title="`${configPluginName} 配置`" @close="showConfigModal = false">
      <div v-if="configLoading" class="loading-state">加载中...</div>
      <template v-else>
        <div v-for="field in configSchema" :key="field.key" class="form-group">
          <label class="form-label">{{ field.label }}</label>
          <input v-if="field.type==='string'" v-model="configValues[field.key]" class="form-input" />
          <input v-if="field.type==='password'" type="password" v-model="configValues[field.key]" class="form-input" />
          <input v-if="field.type==='number'" type="number" :value="Number(configValues[field.key])" @input="(e: Event) => configValues[field.key] = (e.target as HTMLInputElement).value" class="form-input" />
          <label v-if="field.type==='bool'" class="form-toggle">
            <input type="checkbox" :checked="configValues[field.key]==='true'" @change="(e: Event) => configValues[field.key] = String((e.target as HTMLInputElement).checked)" />
            <span>启用</span>
          </label>
        </div>
      </template>
      <template #footer>
        <button class="glass-btn" @click="showConfigModal = false">取消</button>
        <button class="glass-btn primary" @click="savePluginConfig">保存</button>
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

    <!-- Store Modal -->
    <GlassModal :show="showStoreModal" title="插件商店" width="600px" @close="showStoreModal = false">
      <div v-if="storeLoading" class="loading-state">加载中...</div>
      <div v-else class="store-grid">
        <div v-for="plugin in storePlugins" :key="plugin.name" class="store-plugin-card">
          <div class="store-plugin-header">
            <span class="store-plugin-name">{{ plugin.name }}</span>
            <GlassTag>v{{ plugin.version }}</GlassTag>
          </div>
          <p class="store-plugin-desc">{{ plugin.description }}</p>
          <button class="glass-btn primary small full" @click="handleInstallStorePlugin(plugin)">
            安装
          </button>
        </div>
      </div>
    </GlassModal>

    <!-- Install Config Modal -->
    <GlassModal :show="showInstallConfigModal" title="安装配置" width="400px" @close="showInstallConfigModal = false">
      <div class="form-group">
        <label class="form-label">远程端口</label>
        <input v-model.number="installRemotePort" type="number" class="form-input" min="1" max="65535" />
      </div>
      <template #footer>
        <button class="glass-btn" @click="showInstallConfigModal = false">取消</button>
        <button class="glass-btn primary" @click="confirmInstallPlugin">确认安装</button>
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
  background: #8b5cf6;
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
  max-width: 1400px;
  margin: 0 auto;
}

/* Page Header */
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
  flex-wrap: wrap;
  gap: 16px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
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
  font-size: 24px;
  font-weight: 700;
  color: var(--color-text-primary);
  margin: 0;
}

.status-tag {
  padding: 4px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;
  background: rgba(244, 33, 46, 0.15);
  color: var(--color-error);
}

.status-tag.online {
  background: rgba(0, 186, 124, 0.15);
  color: var(--color-success);
}

.header-actions {
  display: flex;
  gap: 8px;
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
  background: rgba(247, 147, 26, 0.15);
  border-color: rgba(247, 147, 26, 0.3);
  color: var(--color-warning);
}

.glass-btn.small { padding: 6px 12px; font-size: 12px; }
.glass-btn.tiny { padding: 4px 8px; font-size: 11px; }
.glass-btn.full { width: 100%; justify-content: center; }

/* Main Grid */
.main-grid {
  display: grid;
  grid-template-columns: 300px 1fr;
  gap: 24px;
  align-items: start;
}

@media (max-width: 900px) {
  .main-grid { grid-template-columns: 1fr; }
}

.left-column, .right-column {
  display: flex;
  flex-direction: column;
  gap: 20px;
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
    box-shadow: 0 0 0 0 rgba(0, 186, 124, 0.5);
    transform: scale(1);
  }
  50% {
    box-shadow: 0 0 0 6px rgba(0, 186, 124, 0);
    transform: scale(1.1);
  }
}

.card-body { padding: 20px; }
.card-actions {
  padding: 16px 20px;
  border-top: 1px solid var(--color-border-light);
  display: flex;
  gap: 8px;
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
  background: rgba(247, 147, 26, 0.15);
  color: var(--color-warning);
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.15s;
}

.update-badge:hover {
  background: rgba(247, 147, 26, 0.25);
}

.latest-badge {
  display: inline-block;
  margin-left: 8px;
  padding: 2px 8px;
  font-size: 11px;
  background: rgba(0, 186, 124, 0.15);
  color: var(--color-success);
  border-radius: 10px;
}

/* Mini Stats */
.stats-row {
  display: flex;
  justify-content: space-around;
}

.mini-stat {
  text-align: center;
}

.mini-stat-value {
  display: block;
  font-size: 28px;
  font-weight: 700;
  color: var(--color-text-primary);
}

.mini-stat-label {
  font-size: 12px;
  color: var(--color-text-muted);
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
.rule-actions { display: flex; gap: 6px; justify-content: flex-end; }

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
  background: rgba(0, 186, 124, 0.15);
}

/* Plugins List */
.plugins-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.plugin-item {
  background: var(--color-bg-elevated);
  border-radius: 10px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.plugin-info {
  display: flex;
  align-items: center;
  gap: 10px;
}

.plugin-name {
  font-weight: 600;
  color: var(--color-text-primary);
  font-size: 14px;
}

.plugin-version {
  font-size: 12px;
  color: var(--color-text-muted);
}

.plugin-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 12px;
  color: var(--color-text-secondary);
}

.plugin-actions {
  display: flex;
  gap: 8px;
  margin-top: 4px;
}

/* Store Plugin Card */
.store-plugin-card {
  background: var(--color-bg-elevated);
  border-radius: 10px;
  padding: 16px;
  border: 1px solid var(--color-border-light);
}

.store-plugin-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.store-plugin-name {
  font-weight: 600;
  color: var(--color-text-primary);
  font-size: 14px;
}

.store-plugin-desc {
  color: var(--color-text-secondary);
  font-size: 12px;
  margin: 0 0 12px 0;
  line-height: 1.5;
}

/* Store Grid */
.store-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
}

@media (max-width: 500px) {
  .store-grid { grid-template-columns: 1fr; }
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

.plugin-icon {
  width: 18px;
  height: 18px;
  color: var(--color-accent);
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
  align-items: center;
  min-height: 200px;
  background: rgba(0, 0, 0, 0.2);
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
  height: auto;
  display: block;
}

.screenshot-meta {
  position: absolute;
  bottom: 0;
  right: 0;
  background: rgba(0, 0, 0, 0.6);
  color: #fff;
  padding: 4px 8px;
  font-size: 12px;
  border-top-left-radius: 8px;
  font-family: monospace;
}

/* Shell Terminal Card */
.shell-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.shell-output {
  width: 100%;
  height: 300px;
  background: #1a1a1a;
  color: #0f0;
  font-family: 'Consolas', 'Monaco', monospace;
  font-size: 13px;
  padding: 12px;
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  resize: vertical;
  overflow-y: auto;
  white-space: pre-wrap;
  word-wrap: break-word;
}

.shell-input-group {
  display: flex;
  gap: 8px;
}

.shell-input {
  flex: 1;
  font-family: 'Consolas', 'Monaco', monospace;
}

.header-controls {
  display: flex;
  align-items: center;
  gap: 12px;
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
</style>