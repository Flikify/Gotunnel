<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NCard, NButton, NSpace, NTag, NTable, NEmpty,
  NForm, NFormItem, NInput, NInputNumber, NSelect, NModal, NSwitch,
  NIcon, useMessage, useDialog, NSpin, NGrid, NGridItem,
  NStatistic, NDivider, NTooltip, NDropdown, type FormInst, type FormRules
} from 'naive-ui'
import {
  ArrowBackOutline, CreateOutline, TrashOutline,
  PushOutline, AddOutline, StorefrontOutline, DocumentTextOutline,
  ExtensionPuzzleOutline, SettingsOutline, OpenOutline, CloudDownloadOutline, RefreshOutline
} from '@vicons/ionicons5'
import {
  getClient, updateClient, deleteClient, pushConfigToClient, disconnectClient, restartClient,
  getClientPluginConfig, updateClientPluginConfig,
  getStorePlugins, installStorePlugin, getRuleSchemas, startClientPlugin, restartClientPlugin, stopClientPlugin, deleteClientPlugin,
  checkClientUpdate, applyClientUpdate, type UpdateInfo
} from '../api'
import type { ProxyRule, ClientPlugin, ConfigField, StorePluginInfo, RuleSchemasMap } from '../types'
import LogViewer from '../components/LogViewer.vue'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const dialog = useDialog()
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
const ruleFormRef = ref<FormInst | null>(null)
// Default Rule Model
const defaultRule = {
  name: '',
  local_ip: '127.0.0.1',
  local_port: 80,
  remote_port: 0, // 0 means unset/placeholder
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

// Validation Rules
const ruleValidationRules: FormRules = {
  name: { required: true, message: '请输入规则名称', trigger: 'blur' },
  type: { required: true, message: '请选择类型', trigger: ['blur', 'change'] },
  remote_port: [
    { required: true, type: 'number', message: '请输入远程端口', trigger: ['blur', 'change'] },
    { type: 'number', min: 1, max: 65535, message: '端口范围 1-65535', trigger: ['blur', 'change'] }
  ],
  local_ip: {
    required: true,
    validator(_rule, value) {
      if (needsLocalAddr(ruleForm.value.type || 'tcp')) {
        if (!value) return new Error('请输入本地IP')
      }
      return true
    },
    trigger: 'blur'
  },
  local_port: {
    required: true,
    validator(_rule, value) {
      if (needsLocalAddr(ruleForm.value.type || 'tcp')) {
        if (!value && value !== 0) return new Error('请输入本地端口')
        if (typeof value === 'number' && (value < 1 || value > 65535)) return new Error('端口范围 1-65535')
      }
      return true
    },
    trigger: ['blur', 'change']
  }
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
  ruleForm.value = { ...defaultRule, remote_port: 8080 } // Reset
  showRuleModal.value = true
}

const openEditRule = (rule: ProxyRule) => {
  if (rule.plugin_managed) return
  ruleModalType.value = 'edit'
  // Deep copy to avoid modifying original until saved
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
    await loadClient() // Revert on failure
  }
}

const handleRuleSubmit = (e: MouseEvent) => {
  e.preventDefault()
  ruleFormRef.value?.validate(async (errors) => {
    if (!errors) {
      // Logic to merge rule
      let newRules = [...rules.value]
      if (ruleModalType.value === 'create') {
        // Check duplicate name
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
    } else {
      message.error('请检查表单填写')
    }
  })
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
    // Fill defaults
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
  <div class="client-view">
    <!-- Header Area -->
    <div class="page-header">
       <n-space align="center">
         <n-button quaternary circle @click="router.push('/')">
           <template #icon><n-icon><ArrowBackOutline /></n-icon></template>
         </n-button>
         <h1 class="page-title">{{ nickname || clientId }}</h1>
         <n-button text size="small" @click="openRenameModal">
           <template #icon><n-icon><CreateOutline /></n-icon></template>
         </n-button>
         <n-tag :type="online ? 'success' : 'error'" round size="small" style="margin-left: 8px;">
           {{ online ? '在线' : '离线' }}
         </n-tag>
       </n-space>
       <n-space>
         <n-button v-if="online" type="primary" secondary @click="pushConfigToClient(clientId).then(() => message.success('已推送'))" size="small">
           <template #icon><n-icon><PushOutline /></n-icon></template>
           推送配置
         </n-button>
         <n-button size="small" @click="showLogViewer=true">
            <template #icon><n-icon><DocumentTextOutline/></n-icon></template>
            日志
         </n-button>
         <n-button type="error" ghost size="small" @click="confirmDelete">
            <template #icon><n-icon><TrashOutline/></n-icon></template>
            删除客户端
         </n-button>
       </n-space>
    </div>

    <n-divider style="margin: 12px 0 24px 0;" />

    <n-grid :x-gap="24" :y-gap="24" cols="1 800:3" item-responsive>
      <!-- Left Column: Status & Info -->
      <n-grid-item span="1">
        <n-space vertical size="large">
          <n-card title="客户端状态" bordered size="small">
            <n-space vertical size="large" justify="space-between">
               <n-statistic label="连接 ID">
                 {{ clientId }}
               </n-statistic>
               <n-statistic label="远程 IP">
                 {{ remoteAddr || '-' }}
               </n-statistic>
               <n-statistic label="最后心跳">
                 {{ lastPing ? new Date(lastPing).toLocaleTimeString() : '-' }}
               </n-statistic>
            </n-space>
             <template #action>
               <n-space vertical>
                 <n-button block type="warning" dashed @click="disconnect" :disabled="!online">断开连接</n-button>
                 <n-button block type="error" dashed @click="handleRestartClient" :disabled="!online">重启客户端</n-button>
               </n-space>
             </template>
          </n-card>

          <n-card title="统计" bordered size="small">
            <n-space justify="space-around">
               <n-statistic label="规则数" :value="rules.length" />
               <n-statistic label="插件数" :value="clientPlugins.length" />
            </n-space>
          </n-card>

          <!-- 客户端更新 -->
          <n-card title="客户端更新" bordered size="small">
            <template #header-extra>
              <n-button size="tiny" :loading="checkingUpdate" @click="handleCheckClientUpdate" :disabled="!online">
                <template #icon><n-icon><RefreshOutline /></n-icon></template>
                检查
              </n-button>
            </template>
            <div v-if="clientOs && clientArch" style="margin-bottom: 8px; font-size: 12px; color: #666;">
              平台: {{ clientOs }}/{{ clientArch }}
            </div>
            <n-empty v-if="!clientUpdate" description="点击检查更新" size="small" />
            <template v-else>
              <div v-if="clientUpdate.download_url" style="font-size: 13px;">
                <p style="margin: 0 0 8px 0; color: #10b981;">发现新版本 {{ clientUpdate.latest }}</p>
                <n-button size="small" type="primary" :loading="updatingClient" @click="handleApplyClientUpdate">
                  <template #icon><n-icon><CloudDownloadOutline /></n-icon></template>
                  更新
                </n-button>
              </div>
              <div v-else style="font-size: 13px; color: #666;">
                已是最新版本
              </div>
            </template>
          </n-card>
        </n-space>
      </n-grid-item>

      <!-- Right Column: Rules & Plugins -->
      <n-grid-item span="2">
         <n-space vertical size="large">

           <!-- Rules Card -->
           <n-card title="代理规则" bordered>
             <template #header-extra>
               <n-button type="primary" size="small" @click="openCreateRule">
                 <template #icon><n-icon><AddOutline /></n-icon></template>
                 添加规则
               </n-button>
             </template>

             <n-empty v-if="rules.length === 0" description="暂无代理规则" style="padding: 24px;" />
             <n-table v-else :bordered="false" size="small">
               <thead>
                 <tr>
                   <th>名称</th>
                   <th>类型</th>
                   <th>映射</th>
                   <th>状态</th>
                   <th style="text-align: right;">操作</th>
                 </tr>
               </thead>
               <tbody>
                 <tr v-for="rule in rules" :key="rule.name">
                   <td><span style="font-weight: 500;">{{ rule.name }}</span></td>
                   <td><n-tag size="small" :type="rule.type==='websocket'?'info':'default'">{{ (rule.type || 'tcp').toUpperCase() }}</n-tag></td>
                   <td style="font-family: monospace; font-size: 12px; color: #666;">
                      {{ needsLocalAddr(rule.type||'tcp') ? `${rule.local_ip}:${rule.local_port}` : '-' }}
                      <n-icon><ArrowBackOutline style="transform: rotate(180deg); margin: 0 4px;" /></n-icon>
                      :{{ rule.remote_port }}
                   </td>
                   <td>
                     <n-switch :value="rule.enabled !== false" @update:value="(v: boolean) => { rule.enabled = v; saveRules(rules) }" size="small" />
                   </td>
                   <td style="text-align: right;">
                     <n-space justify="end" :size="8">
                       <n-tooltip v-if="rule.plugin_managed">
                         <template #trigger>
                            <n-tag type="info" size="small">插件托管</n-tag>
                         </template>
                         此规则由插件管理，无法手动编辑
                       </n-tooltip>
                       <template v-else>
                         <n-button size="tiny" secondary type="info" @click="openEditRule(rule)">编辑</n-button>
                         <n-button size="tiny" secondary type="error" @click="handleDeleteRule(rule)">删除</n-button>
                       </template>
                     </n-space>
                   </td>
                 </tr>
               </tbody>
             </n-table>
           </n-card>

           <!-- Plugins Card -->
           <n-card title="已安装扩展" bordered>
             <template #header-extra>
                <n-button secondary size="small" @click="openStoreModal">
                 <template #icon><n-icon><StorefrontOutline /></n-icon></template>
                 插件商店
               </n-button>
             </template>

             <n-empty v-if="clientPlugins.length === 0" description="暂无安装的扩展" style="padding: 24px;" />
             <n-table v-else :bordered="false" size="small">
                <thead>
                  <tr>
                    <th>名称</th>
                    <th>版本</th>
                    <th>端口</th>
                    <th>状态</th>
                    <th>启用</th>
                    <th style="text-align: right;">操作</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="plugin in clientPlugins" :key="plugin.id">
                    <td>
                      <div style="display: flex; align-items: center; gap: 8px;">
                        <n-icon size="18" color="#18a058"><ExtensionPuzzleOutline /></n-icon>
                        {{ plugin.name }}
                      </div>
                    </td>
                    <td>v{{ plugin.version }}</td>
                    <td>{{ plugin.remote_port || '-' }}</td>
                    <td>
                       <n-tag :type="plugin.running ? 'success' : 'default'" size="small" round>
                         {{ plugin.running ? '运行中' : '已停止' }}
                       </n-tag>
                    </td>
                    <td>
                      <n-switch :value="plugin.enabled" size="small" @update:value="toggleClientPlugin(plugin)" />
                    </td>
                    <td style="text-align: right;">
                      <n-space justify="end" :size="4">
                        <n-button v-if="plugin.running && plugin.remote_port" size="tiny" type="success" secondary @click="handleOpenPlugin(plugin)">
                          <template #icon><n-icon><OpenOutline /></n-icon></template>
                          打开
                        </n-button>
                        <n-button v-if="!plugin.running" size="tiny" @click="handleStartPlugin(plugin)" :disabled="!online || !plugin.enabled">启动</n-button>
                        <n-dropdown :options="[
                           { label: '重启', key: 'restart', disabled: !plugin.running },
                           { label: '配置', key: 'config' },
                           { label: '删除', key: 'delete', props: { style: 'color: red' } },
                           { label: '停止', key: 'stop', disabled: !plugin.running }
                        ]" @select="(k: string) => {
                             if(k==='restart') handleRestartPlugin(plugin);
                             if(k==='config') openConfigModal(plugin);
                             if(k==='delete') handleDeletePlugin(plugin);
                             if(k==='stop') handleStopPlugin(plugin);
                        }">
                          <n-button size="tiny" quaternary><template #icon><n-icon><SettingsOutline /></n-icon></template></n-button>
                        </n-dropdown>
                      </n-space>
                    </td>
                  </tr>
                </tbody>
             </n-table>
           </n-card>

         </n-space>
      </n-grid-item>
    </n-grid>

    <!-- Rule Edit Modal -->
    <n-modal v-model:show="showRuleModal" preset="card" :title="ruleModalType==='create'?'添加规则':'编辑规则'" style="width: 500px">
      <n-form ref="ruleFormRef" :model="ruleForm" :rules="ruleValidationRules" label-placement="left" label-width="80">
        <n-form-item label="名称" path="name">
          <n-input v-model:value="ruleForm.name" placeholder="请输入规则名称" :disabled="ruleModalType==='edit'" />
        </n-form-item>
        <n-form-item label="类型" path="type">
          <n-select v-model:value="ruleForm.type" :options="builtinTypes" />
        </n-form-item>

        <template v-if="needsLocalAddr(ruleForm.type || 'tcp')">
          <n-form-item label="本地IP" path="local_ip">
            <n-input v-model:value="ruleForm.local_ip" placeholder="127.0.0.1" />
          </n-form-item>
          <n-form-item label="本地端口" path="local_port">
            <n-input-number v-model:value="ruleForm.local_port" :min="1" :max="65535" style="width: 100%" />
          </n-form-item>
        </template>

        <n-form-item label="远程端口" path="remote_port">
          <n-input-number v-model:value="ruleForm.remote_port" :min="1" :max="65535" style="width: 100%" placeholder="将在服务器上监听的端口" />
        </n-form-item>

        <!-- Extra Fields -->
        <template v-for="field in getExtraFields(ruleForm.type || '')" :key="field.key">
           <n-form-item :label="field.label">
              <n-input v-if="field.type==='string'" v-model:value="ruleForm.plugin_config![field.key]" />
              <n-input v-if="field.type==='password'" type="password" v-model:value="ruleForm.plugin_config![field.key]" show-password-on="click" />
              <n-switch v-if="field.type==='bool'" :value="ruleForm.plugin_config![field.key]==='true'" @update:value="(v) => ruleForm.plugin_config![field.key] = String(v)" />
           </n-form-item>
        </template>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showRuleModal = false">取消</n-button>
          <n-button type="primary" @click="handleRuleSubmit">保存</n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- Plugin Config Modal -->
    <n-modal v-model:show="showConfigModal" preset="card" :title="`${configPluginName} 配置`" style="width: 500px;">
       <n-empty v-if="configLoading" description="加载中..." />
       <n-form v-else label-placement="left" label-width="100">
         <n-form-item v-for="field in configSchema" :key="field.key" :label="field.label">
            <n-input v-if="field.type==='string'" v-model:value="configValues[field.key]" />
            <n-input v-if="field.type==='password'" type="password" v-model:value="configValues[field.key]" show-password-on="click"/>
            <n-input-number v-if="field.type==='number'" :value="Number(configValues[field.key])" @update:value="(v) => configValues[field.key] = String(v)" />
            <n-switch v-if="field.type==='bool'" :value="configValues[field.key]==='true'" @update:value="(v) => configValues[field.key] = String(v)" />
         </n-form-item>
       </n-form>
       <template #footer>
         <n-space justify="end">
           <n-button @click="showConfigModal = false">取消</n-button>
           <n-button type="primary" @click="savePluginConfig">保存</n-button>
         </n-space>
       </template>
    </n-modal>

    <!-- Rename Modal -->
    <n-modal v-model:show="showRenameModal" preset="card" title="重命名客户端" style="width: 400px;">
      <n-input v-model:value="renameValue" placeholder="请输入新名称" />
      <template #footer>
        <n-space justify="end">
           <n-button @click="showRenameModal = false">取消</n-button>
           <n-button type="primary" @click="saveRename">保存</n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- Store Modal -->
    <n-modal v-model:show="showStoreModal" preset="card" title="插件商店" style="width: 600px;">
       <n-spin :show="storeLoading">
         <n-grid :x-gap="12" :y-gap="12" cols="1 600:2">
            <n-grid-item v-for="plugin in storePlugins" :key="plugin.name">
               <n-card size="small" hoverable>
                 <n-space align="center" justify="space-between">
                   <div style="font-weight: 600;">{{ plugin.name }}</div>
                   <n-tag size="small">v{{ plugin.version }}</n-tag>
                 </n-space>
                 <div style="color: #666; font-size: 12px; margin: 8px 0; height: 32px; overflow: hidden;">
                   {{ plugin.description }}
                 </div>
                 <n-button block type="primary" size="small" secondary @click="handleInstallStorePlugin(plugin)" :loading="storeInstalling === plugin.name">
                   安装
                 </n-button>
               </n-card>
            </n-grid-item>
         </n-grid>
       </n-spin>
    </n-modal>

    <!-- Install Config Modal -->
    <n-modal v-model:show="showInstallConfigModal" preset="card" title="安装配置" style="width: 400px;">
       <n-form label-placement="left">
          <n-form-item label="远程端口">
             <n-input-number v-model:value="installRemotePort" :min="1" :max="65535" style="width: 100%" placeholder="1-65535" />
          </n-form-item>
       </n-form>
       <template #footer>
          <n-space justify="end">
             <n-button @click="showInstallConfigModal = false">取消</n-button>
             <n-button type="primary" @click="confirmInstallPlugin">确认安装</n-button>
          </n-space>
       </template>
    </n-modal>

    <LogViewer :visible="showLogViewer" @close="showLogViewer = false" :client-id="clientId" />
  </div>
</template>

<style scoped>
.client-view {
  min-height: 100%;
}
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.page-title {
  font-size: 20px;
  font-weight: 600;
  margin: 0;
  color: #1f2937;
}
</style>
