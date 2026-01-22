<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ArrowBackOutline, CreateOutline, TrashOutline,
  PushOutline, AddOutline, StorefrontOutline, DocumentTextOutline,
  ExtensionPuzzleOutline, SettingsOutline, CloudDownloadOutline, RefreshOutline
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
  checkClientUpdate, applyClientUpdate, type UpdateInfo
} from '../api'
import type { ProxyRule, ClientPlugin, ConfigField, StorePluginInfo, RuleSchemasMap } from '../types'
import LogViewer from '../components/LogViewer.vue'
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

// 客户端更新相关
const clientUpdate = ref<UpdateInfo | null>(null)
const checkingUpdate = ref(false)
const updatingClient = ref(false)

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
  } catch (e) {
    message.error('加载客户端信息失败')
    console.error(e)
  } finally {
    loading.value = false
  }
}

// 客户端更新
const handleCheckClientUpdate = async () => {
  if (!online.value) {
    message.warning('客户端离线，无法检查更新')
    return
  }
  if (!clientOs.value || !clientArch.value) {
    message.warning('无法获取客户端平台信息')
    return
  }
  checkingUpdate.value = true
  try {
    const { data } = await checkClientUpdate(clientOs.value, clientArch.value)
    clientUpdate.value = data
    if (data.download_url) {
      message.success('找到客户端更新: ' + data.latest)
    } else {
      message.info('已是最新版本或未找到对应平台的更新包')
    }
  } catch (e: any) {
    message.error(e.response?.data || '检查更新失败')
  } finally {
    checkingUpdate.value = false
  }
}

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
onMounted(() => {
  loadRuleSchemas()
  loadClient()
})

// Log Viewer
const showLogViewer = ref(false)

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
            <n-icon size="20"><ArrowBackOutline /></n-icon>
          </button>
          <h1 class="page-title">{{ nickname || clientId }}</h1>
          <button class="edit-btn" @click="openRenameModal">
            <n-icon size="16"><CreateOutline /></n-icon>
          </button>
          <span class="status-tag" :class="{ online }">
            {{ online ? '在线' : '离线' }}
          </span>
        </div>
        <div class="header-actions">
          <button v-if="online" class="glass-btn primary" @click="pushConfigToClient(clientId).then(() => message.success('已推送'))">
            <n-icon size="16"><PushOutline /></n-icon>
            <span>推送配置</span>
          </button>
          <button class="glass-btn" @click="showLogViewer=true">
            <n-icon size="16"><DocumentTextOutline /></n-icon>
            <span>日志</span>
          </button>
          <button class="glass-btn danger" @click="confirmDelete">
            <n-icon size="16"><TrashOutline /></n-icon>
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

          <!-- Update Card -->
          <div class="glass-card">
            <div class="card-header">
              <h3>客户端更新</h3>
              <button class="glass-btn tiny" :disabled="!online || checkingUpdate" @click="handleCheckClientUpdate">
                <n-icon size="14"><RefreshOutline /></n-icon>
                检查
              </button>
            </div>
            <div class="card-body">
              <div v-if="clientOs && clientArch" class="platform-info">
                平台: {{ clientOs }}/{{ clientArch }}
              </div>
              <div v-if="!clientUpdate" class="empty-hint">点击检查更新</div>
              <template v-else>
                <div v-if="clientUpdate.download_url" class="update-available">
                  <p>发现新版本 {{ clientUpdate.latest }}</p>
                  <button class="glass-btn primary small" :disabled="updatingClient" @click="handleApplyClientUpdate">
                    <n-icon size="14"><CloudDownloadOutline /></n-icon>
                    更新
                  </button>
                </div>
                <div v-else class="empty-hint">已是最新版本</div>
              </template>
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
                <n-icon size="14"><AddOutline /></n-icon>
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
            <n-tag size="small">v{{ plugin.version }}</n-tag>
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

    <LogViewer :visible="showLogViewer" @close="showLogViewer = false" :client-id="clientId" />
  </div>
</template>

<style scoped>
.client-page {
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
  background: rgba(255, 255, 255, 0.1);
  border: none;
  border-radius: 8px;
  padding: 8px;
  color: rgba(255, 255, 255, 0.8);
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
}

.back-btn:hover, .edit-btn:hover {
  background: rgba(255, 255, 255, 0.2);
  color: white;
}

.page-title {
  font-size: 24px;
  font-weight: 700;
  color: white;
  margin: 0;
}

.status-tag {
  padding: 4px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;
  background: rgba(239, 68, 68, 0.2);
  color: #fca5a5;
}

.status-tag.online {
  background: rgba(52, 211, 153, 0.2);
  color: #34d399;
}

.header-actions {
  display: flex;
  gap: 8px;
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

.glass-btn.primary {
  background: linear-gradient(135deg, #60a5fa 0%, #a78bfa 100%);
  border: none;
}

.glass-btn.danger {
  background: rgba(239, 68, 68, 0.2);
  border-color: rgba(239, 68, 68, 0.3);
  color: #fca5a5;
}

.glass-btn.warning {
  background: rgba(251, 191, 36, 0.2);
  border-color: rgba(251, 191, 36, 0.3);
  color: #fcd34d;
}

.glass-btn.small { padding: 6px 12px; font-size: 12px; }
.glass-btn.tiny { padding: 4px 8px; font-size: 11px; }
.glass-btn.full { width: 100%; justify-content: center; }

/* Main Grid */
.main-grid {
  display: grid;
  grid-template-columns: 300px 1fr;
  gap: 24px;
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
  background: rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border-radius: 16px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  overflow: hidden;
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

.card-body { padding: 20px; }
.card-actions {
  padding: 16px 20px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  display: flex;
  gap: 8px;
}

/* Stat Items */
.stat-item {
  display: flex;
  justify-content: space-between;
  padding: 8px 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.stat-item:last-child { border-bottom: none; }

.stat-label {
  color: rgba(255, 255, 255, 0.6);
  font-size: 13px;
}

.stat-value {
  color: white;
  font-size: 13px;
}

.stat-value.mono {
  font-family: monospace;
  font-size: 12px;
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
  color: white;
}

.mini-stat-label {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.5);
}

/* Update Card */
.platform-info {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.5);
  margin-bottom: 8px;
}

.empty-hint {
  color: rgba(255, 255, 255, 0.4);
  font-size: 13px;
  text-align: center;
  padding: 16px 0;
}

.update-available p {
  margin: 0 0 8px 0;
  color: #34d399;
  font-size: 13px;
}

/* Rules Table */
.empty-state {
  text-align: center;
  padding: 32px;
  color: rgba(255, 255, 255, 0.4);
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
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  color: rgba(255, 255, 255, 0.5);
  font-size: 12px;
  font-weight: 500;
}

.table-row {
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  color: rgba(255, 255, 255, 0.8);
  font-size: 13px;
}

.rule-name { font-weight: 500; color: white; }
.rule-mapping { font-family: monospace; font-size: 12px; }
.rule-actions { display: flex; gap: 6px; justify-content: flex-end; }

/* Icon Button */
.icon-btn {
  background: rgba(255, 255, 255, 0.1);
  border: none;
  border-radius: 6px;
  padding: 4px 10px;
  color: rgba(255, 255, 255, 0.8);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
}

.icon-btn:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.2);
  color: white;
}

.icon-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.icon-btn.danger {
  color: #fca5a5;
}

.icon-btn.danger:hover:not(:disabled) {
  background: rgba(239, 68, 68, 0.2);
}

.icon-btn.success {
  color: #34d399;
}

.icon-btn.success:hover:not(:disabled) {
  background: rgba(52, 211, 153, 0.2);
}

/* Plugins List */
.plugins-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.plugin-item {
  background: rgba(255, 255, 255, 0.05);
  border-radius: 12px;
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
  color: white;
  font-size: 14px;
}

.plugin-version {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.5);
}

.plugin-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.6);
}

.plugin-actions {
  display: flex;
  gap: 8px;
  margin-top: 4px;
}

/* Store Plugin Card */
.store-plugin-card {
  background: rgba(255, 255, 255, 0.05);
  border-radius: 12px;
  padding: 16px;
  border: 1px solid rgba(255, 255, 255, 0.08);
}

.store-plugin-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.store-plugin-name {
  font-weight: 600;
  color: white;
  font-size: 14px;
}

.store-plugin-desc {
  color: rgba(255, 255, 255, 0.6);
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
  color: rgba(255, 255, 255, 0.7);
  margin-bottom: 6px;
}

.form-input {
  width: 100%;
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 8px;
  padding: 10px 12px;
  color: white;
  font-size: 14px;
  outline: none;
  transition: border-color 0.2s;
  box-sizing: border-box;
}

.form-input:focus {
  border-color: rgba(167, 139, 250, 0.5);
}

.form-input::placeholder {
  color: rgba(255, 255, 255, 0.4);
}

.form-input:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.form-select {
  width: 100%;
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 8px;
  padding: 10px 12px;
  color: white;
  font-size: 14px;
  outline: none;
  cursor: pointer;
}

.form-select option {
  background: #1e1b4b;
  color: white;
}

.form-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  color: rgba(255, 255, 255, 0.7);
  font-size: 13px;
  cursor: pointer;
}

.form-toggle input[type="checkbox"] {
  width: 18px;
  height: 18px;
  accent-color: #a78bfa;
}

.loading-state {
  text-align: center;
  padding: 32px;
  color: rgba(255, 255, 255, 0.5);
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
  background: rgba(30, 27, 75, 0.95);
  backdrop-filter: blur(20px);
  border: 1px solid rgba(255, 255, 255, 0.12);
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
  color: rgba(255, 255, 255, 0.8);
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  border-radius: 4px;
  transition: all 0.2s;
}

.dropdown-menu button:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.1);
  color: white;
}

.dropdown-menu button:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.dropdown-menu button.danger {
  color: #fca5a5;
}

.dropdown-menu button.danger:hover:not(:disabled) {
  background: rgba(239, 68, 68, 0.2);
}

/* Icon styles */
.btn-icon {
  width: 14px;
  height: 14px;
}

.plugin-icon {
  width: 18px;
  height: 18px;
  color: #a78bfa;
}

.settings-icon {
  width: 16px;
  height: 16px;
}
</style>