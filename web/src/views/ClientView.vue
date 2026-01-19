<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NCard, NButton, NSpace, NTag, NTable, NEmpty,
  NFormItem, NInput, NInputNumber, NSelect, NModal, NSwitch,
  NIcon, useMessage, useDialog, NSpin, NAlert
} from 'naive-ui'
import {
  ArrowBackOutline, CreateOutline, TrashOutline,
  PushOutline, PowerOutline, AddOutline, SaveOutline, CloseOutline,
  SettingsOutline, StorefrontOutline, RefreshOutline, StopOutline, PlayOutline, DocumentTextOutline
} from '@vicons/ionicons5'
import {
  getClient, updateClient, deleteClient, pushConfigToClient, disconnectClient, restartClient,
  getClientPluginConfig, updateClientPluginConfig,
  getStorePlugins, installStorePlugin, getRuleSchemas, startClientPlugin, restartClientPlugin, stopClientPlugin, deleteClientPlugin
} from '../api'
import type { ProxyRule, ClientPlugin, ConfigField, StorePluginInfo, RuleSchemasMap } from '../types'
import LogViewer from '../components/LogViewer.vue'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const dialog = useDialog()
const clientId = route.params.id as string

const online = ref(false)
const lastPing = ref('')
const remoteAddr = ref('')
const nickname = ref('')
const rules = ref<ProxyRule[]>([])
const clientPlugins = ref<ClientPlugin[]>([])
const editing = ref(false)
const editRules = ref<ProxyRule[]>([])

// 重命名相关
const showRenameModal = ref(false)
const renameValue = ref('')

// 内置类型
const builtinTypes = [
  { label: 'TCP', value: 'tcp' },
  { label: 'UDP', value: 'udp' },
  { label: 'HTTP', value: 'http' },
  { label: 'HTTPS', value: 'https' },
  { label: 'SOCKS5', value: 'socks5' }
]

// 规则类型选项（内置 + 插件）
const typeOptions = ref([...builtinTypes])

// 插件 RuleSchema 映射（包含内置类型和插件类型）
const pluginRuleSchemas = ref<RuleSchemasMap>({})

// 加载规则配置模式
const loadRuleSchemas = async () => {
  try {
    const { data } = await getRuleSchemas()
    pluginRuleSchemas.value = data || {}
  } catch (e) {
    console.error('Failed to load rule schemas', e)
  }
}

// 判断类型是否需要本地地址
const needsLocalAddr = (type: string) => {
  const schema = pluginRuleSchemas.value[type]
  return schema?.needs_local_addr ?? true // 默认需要
}

// 获取类型的额外字段
const getExtraFields = (type: string): ConfigField[] => {
  const schema = pluginRuleSchemas.value[type]
  return schema?.extra_fields || []
}

// 插件配置相关
const showConfigModal = ref(false)
const configPluginName = ref('')
const configSchema = ref<ConfigField[]>([])
const configValues = ref<Record<string, string>>({})
const configLoading = ref(false)

// 商店插件安装相关
const showStoreModal = ref(false)
const storePlugins = ref<StorePluginInfo[]>([])
const storeLoading = ref(false)
const storeInstalling = ref<string | null>(null) // 正在安装的插件名称

// 安装配置模态框
const showInstallConfigModal = ref(false)
const installPlugin = ref<StorePluginInfo | null>(null)
const installRemotePort = ref<number | null>(8080)
const installAuthEnabled = ref(false)
const installAuthUsername = ref('')
const installAuthPassword = ref('')

// 日志查看相关
const showLogViewer = ref(false)

// 商店插件相关函数
const openStoreModal = async () => {
  showStoreModal.value = true
  storeLoading.value = true
  try {
    const { data } = await getStorePlugins()
    storePlugins.value = (data.plugins || []).filter(p => p.download_url)
  } catch (e) {
    console.error('Failed to load store plugins', e)
    message.error('加载商店插件失败')
  } finally {
    storeLoading.value = false
  }
}

const handleInstallStorePlugin = async (plugin: StorePluginInfo) => {
  if (!plugin.download_url) {
    message.error('该插件没有下载地址')
    return
  }
  // 打开配置模态框
  installPlugin.value = plugin
  installRemotePort.value = 8080
  installAuthEnabled.value = false
  installAuthUsername.value = ''
  installAuthPassword.value = ''
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

const loadClient = async () => {
  try {
    const { data } = await getClient(clientId)
    online.value = data.online
    lastPing.value = data.last_ping || ''
    remoteAddr.value = data.remote_addr || ''
    nickname.value = data.nickname || ''
    rules.value = data.rules || []
    clientPlugins.value = data.plugins || []
  } catch (e) {
    console.error('Failed to load client', e)
  }
}

onMounted(() => {
  loadRuleSchemas() // 加载内置协议配置模式
  loadClient()
})

// 打开重命名弹窗
const openRenameModal = () => {
  renameValue.value = nickname.value
  showRenameModal.value = true
}

// 保存重命名
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

const startEdit = () => {
  editRules.value = rules.value.map(rule => ({
    ...rule,
    type: rule.type || 'tcp',
    enabled: rule.enabled !== false
  }))
  editing.value = true
}

const cancelEdit = () => {
  editing.value = false
}

const addRule = () => {
  editRules.value.push({
    name: '', local_ip: '127.0.0.1', local_port: 80, remote_port: 8080, type: 'tcp', enabled: true
  })
}

const removeRule = (index: number) => {
  editRules.value.splice(index, 1)
}

const saveEdit = async () => {
  try {
    // 合并插件管理的规则和编辑后的规则
    await updateClient(clientId, { id: clientId, nickname: nickname.value, rules: editRules.value })
    editing.value = false
    message.success('保存成功')
    await loadClient()
    // 如果客户端在线，自动推送配置
    if (online.value) {
      try {
        await pushConfigToClient(clientId)
        message.success('配置已自动推送到客户端')
      } catch (e: any) {
        message.warning('配置已保存，但推送失败: ' + (e.response?.data || '未知错误'))
      }
    }
  } catch (e) {
    message.error('保存失败')
  }
}

const confirmDelete = () => {
  dialog.warning({
    title: '确认删除',
    content: '确定要删除此客户端吗？',
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await deleteClient(clientId)
        message.success('删除成功')
        router.push('/')
      } catch (e) {
        message.error('删除失败')
      }
    }
  })
}

const pushConfig = async () => {
  try {
    await pushConfigToClient(clientId)
    message.success('配置已推送')
  } catch (e: any) {
    message.error(e.response?.data || '推送失败')
  }
}

const disconnect = () => {
  dialog.warning({
    title: '确认断开',
    content: '确定要断开此客户端连接吗？',
    positiveText: '断开',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await disconnectClient(clientId)
        online.value = false
        message.success('已断开连接')
      } catch (e: any) {
        message.error(e.response?.data || '断开失败')
      }
    }
  })
}

// 重启客户端
const handleRestartClient = () => {
  dialog.warning({
    title: '确认重启',
    content: '确定要重启此客户端吗？客户端将断开连接并自动重连。',
    positiveText: '重启',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await restartClient(clientId)
        message.success('重启命令已发送，客户端将自动重连')
        setTimeout(() => loadClient(), 3000)
      } catch (e: any) {
        message.error(e.response?.data || '重启失败')
      }
    }
  })
}

// 启动客户端插件
const handleStartPlugin = async (plugin: ClientPlugin) => {
  const rule = rules.value.find(r => r.type === plugin.name)
  const ruleName = rule?.name || plugin.name
  try {
    await startClientPlugin(clientId, plugin.id, ruleName)
    message.success(`已启动 ${plugin.name}`)
    plugin.running = true
  } catch (e: any) {
    message.error(e.message || '启动失败')
  }
}

// 重启客户端插件
const handleRestartPlugin = async (plugin: ClientPlugin) => {
  // 找到使用此插件的规则
  const rule = rules.value.find(r => r.type === plugin.name)
  const ruleName = rule?.name || plugin.name
  try {
    await restartClientPlugin(clientId, plugin.id, ruleName)
    message.success(`已重启 ${plugin.name}`)
    plugin.running = true
  } catch (e: any) {
    message.error(e.message || '重启失败')
  }
}

// 停止客户端插件
const handleStopPlugin = async (plugin: ClientPlugin) => {
  const rule = rules.value.find(r => r.type === plugin.name)
  const ruleName = rule?.name || plugin.name
  try {
    await stopClientPlugin(clientId, plugin.id, ruleName)
    message.success(`已停止 ${plugin.name}`)
    plugin.running = false
  } catch (e: any) {
    message.error(e.message || '停止失败')
  }
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

// 打开插件配置模态框
const openConfigModal = async (plugin: ClientPlugin) => {
  configPluginName.value = plugin.name
  configLoading.value = true
  showConfigModal.value = true

  try {
    const { data } = await getClientPluginConfig(clientId, plugin.name)
    configSchema.value = data.schema || []
    configValues.value = { ...data.config }
    // 填充默认值
    for (const field of configSchema.value) {
      if (field.default && !configValues.value[field.key]) {
        configValues.value[field.key] = field.default
      }
    }
  } catch (e: any) {
    message.error(e.response?.data || '加载配置失败')
    showConfigModal.value = false
  } finally {
    configLoading.value = false
  }
}

// 保存插件配置
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

// 删除客户端插件
const handleDeletePlugin = (plugin: ClientPlugin) => {
  dialog.warning({
    title: '确认删除',
    content: `确定要删除插件 ${plugin.name} 吗？`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await deleteClientPlugin(clientId, plugin.id)
        message.success(`已删除 ${plugin.name}`)
        await loadClient()
      } catch (e: any) {
        message.error(e.response?.data || '删除失败')
      }
    }
  })
}
</script>

<template>
  <div class="client-view">
    <!-- 头部信息卡片 -->
    <n-card style="margin-bottom: 16px;">
      <n-space justify="space-between" align="center" wrap>
        <n-space align="center">
          <n-button quaternary @click="router.push('/')">
            <template #icon><n-icon><ArrowBackOutline /></n-icon></template>
            返回
          </n-button>
          <h2 style="margin: 0; cursor: pointer;" @click="openRenameModal" title="点击重命名">
            {{ nickname || clientId }}
            <n-icon size="16" style="margin-left: 4px; opacity: 0.5;"><CreateOutline /></n-icon>
          </h2>
          <span v-if="nickname" style="color: #999; font-size: 12px;">{{ clientId }}</span>
          <n-tag :type="online ? 'success' : 'default'">
            {{ online ? '在线' : '离线' }}
          </n-tag>
          <span v-if="remoteAddr && online" style="color: #666; font-size: 14px;">
            IP: {{ remoteAddr }}
          </span>
          <span v-if="lastPing" style="color: #666; font-size: 14px;">
            最后心跳: {{ lastPing }}
          </span>
        </n-space>
        <n-space>
          <template v-if="online">
            <n-button type="info" @click="pushConfig">
              <template #icon><n-icon><PushOutline /></n-icon></template>
              推送配置
            </n-button>
            <n-button @click="showLogViewer = true">
              <template #icon><n-icon><DocumentTextOutline /></n-icon></template>
              查看日志
            </n-button>
            <n-button @click="openStoreModal">
              <template #icon><n-icon><StorefrontOutline /></n-icon></template>
              从商店安装
            </n-button>
            <n-button type="warning" @click="disconnect">
              <template #icon><n-icon><PowerOutline /></n-icon></template>
              断开连接
            </n-button>
            <n-button type="error" @click="handleRestartClient">
              <template #icon><n-icon><RefreshOutline /></n-icon></template>
              重启客户端
            </n-button>
          </template>
          <template v-if="!editing">
            <n-button type="primary" @click="startEdit">
              <template #icon><n-icon><CreateOutline /></n-icon></template>
              编辑规则
            </n-button>
            <n-button type="error" @click="confirmDelete">
              <template #icon><n-icon><TrashOutline /></n-icon></template>
              删除
            </n-button>
          </template>
        </n-space>
      </n-space>
    </n-card>

    <!-- 规则卡片 -->
    <n-card title="代理规则">
      <template #header-extra v-if="editing">
        <n-space>
          <n-button @click="cancelEdit">
            <template #icon><n-icon><CloseOutline /></n-icon></template>
            取消
          </n-button>
          <n-button type="primary" @click="saveEdit">
            <template #icon><n-icon><SaveOutline /></n-icon></template>
            保存
          </n-button>
        </n-space>
      </template>

      <!-- 查看模式 -->
      <template v-if="!editing">
        <n-empty v-if="rules.length === 0" description="暂无代理规则" />
        <n-table v-else :bordered="false" :single-line="false">
          <thead>
            <tr>
              <th>名称</th>
              <th>本地地址</th>
              <th>远程端口</th>
              <th>类型</th>
              <th>状态</th>
              <th>来源</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="rule in rules" :key="rule.name">
              <td>{{ rule.name || '未命名' }}</td>
              <td>
                <template v-if="needsLocalAddr(rule.type || 'tcp')">
                  {{ rule.local_ip }}:{{ rule.local_port }}
                </template>
                <span v-else style="color: #999;">-</span>
              </td>
              <td>{{ rule.remote_port }}</td>
              <td><n-tag size="small">{{ (rule.type || 'tcp').toUpperCase() }}</n-tag></td>
              <td>
                <n-tag size="small" :type="rule.enabled !== false ? 'success' : 'default'">
                  {{ rule.enabled !== false ? '启用' : '禁用' }}
                </n-tag>
              </td>
              <td>
                <n-tag v-if="rule.plugin_managed" size="small" type="info">插件</n-tag>
                <n-tag v-else size="small" type="default">手动</n-tag>
              </td>
            </tr>
          </tbody>
        </n-table>
      </template>

      <!-- 编辑模式 -->
      <template v-else>
        <n-space vertical :size="12">
          <n-card v-for="(rule, i) in editRules" :key="i" size="small">
            <n-alert v-if="rule.plugin_managed" type="info" show-icon style="margin-bottom: 12px">
              此规则由插件创建，禁止修改。如需修改请前往插件管理页面。
            </n-alert>
            <n-space align="center" wrap>
              <n-form-item label="启用" :show-feedback="false">
                <n-switch v-model:value="rule.enabled" :disabled="!!rule.plugin_managed" />
              </n-form-item>
              <n-form-item label="名称" :show-feedback="false">
                <n-input v-model:value="rule.name" placeholder="规则名称" :disabled="!!rule.plugin_managed" />
              </n-form-item>
              <n-form-item label="类型" :show-feedback="false">
                <n-select v-model:value="rule.type" :options="typeOptions" style="width: 140px;" :disabled="!!rule.plugin_managed" />
              </n-form-item>
              <!-- 仅 tcp/udp 显示本地地址 -->
              <template v-if="needsLocalAddr(rule.type || 'tcp')">
                <n-form-item label="本地IP" :show-feedback="false">
                  <n-input v-model:value="rule.local_ip" placeholder="127.0.0.1" :disabled="!!rule.plugin_managed" />
                </n-form-item>
                <n-form-item label="本地端口" :show-feedback="false">
                  <n-input-number v-model:value="rule.local_port" :show-button="false" :disabled="!!rule.plugin_managed" />
                </n-form-item>
              </template>
              <n-form-item label="远程端口" :show-feedback="false">
                <n-input-number v-model:value="rule.remote_port" :show-button="false" :disabled="!!rule.plugin_managed" />
              </n-form-item>
              <!-- 插件额外字段 -->
              <template v-for="field in getExtraFields(rule.type || '')" :key="field.key">
                <n-form-item :label="field.label" :show-feedback="false">
                  <!-- 字符串输入 -->
                  <n-input
                    v-if="field.type === 'string'"
                    :value="rule.plugin_config?.[field.key] || field.default || ''"
                    @update:value="(v: string) => { if (!rule.plugin_config) rule.plugin_config = {}; rule.plugin_config[field.key] = v }"
                    :placeholder="field.description"
                    :disabled="!!rule.plugin_managed"
                  />
                  <!-- 密码输入 -->
                  <n-input
                    v-else-if="field.type === 'password'"
                    :value="rule.plugin_config?.[field.key] || ''"
                    @update:value="(v: string) => { if (!rule.plugin_config) rule.plugin_config = {}; rule.plugin_config[field.key] = v }"
                    type="password"
                    show-password-on="click"
                    :placeholder="field.description"
                    :disabled="!!rule.plugin_managed"
                  />
                  <!-- 数字输入 -->
                  <n-input-number
                    v-else-if="field.type === 'number'"
                    :value="rule.plugin_config?.[field.key] ? Number(rule.plugin_config[field.key]) : undefined"
                    @update:value="(v: number | null) => { if (!rule.plugin_config) rule.plugin_config = {}; rule.plugin_config[field.key] = v !== null ? String(v) : '' }"
                    :placeholder="field.description"
                    :show-button="false"
                    style="width: 120px;"
                    :disabled="!!rule.plugin_managed"
                  />
                  <!-- 布尔开关 -->
                  <n-switch
                    v-else-if="field.type === 'bool'"
                    :value="rule.plugin_config?.[field.key] === 'true'"
                    @update:value="(v: boolean) => { if (!rule.plugin_config) rule.plugin_config = {}; rule.plugin_config[field.key] = String(v) }"
                    :disabled="!!rule.plugin_managed"
                  />
                  <!-- 下拉选择 -->
                  <n-select
                    v-else-if="field.type === 'select'"
                    :value="rule.plugin_config?.[field.key] || field.default"
                    @update:value="(v: string) => { if (!rule.plugin_config) rule.plugin_config = {}; rule.plugin_config[field.key] = v }"
                    :options="(field.options || []).map(o => ({ label: o, value: o }))"
                    style="width: 120px;"
                    :disabled="!!rule.plugin_managed"
                  />
                </n-form-item>
              </template>
              <n-button v-if="editRules.length > 1 && !rule.plugin_managed" quaternary type="error" @click="removeRule(i)">
                <template #icon><n-icon><TrashOutline /></n-icon></template>
              </n-button>
            </n-space>
          </n-card>
          <n-button dashed block @click="addRule">
            <template #icon><n-icon><AddOutline /></n-icon></template>
            添加规则
          </n-button>
        </n-space>
      </template>
    </n-card>

    <!-- 已安装插件卡片 -->
    <n-card title="已安装扩展" style="margin-top: 16px;">
      <n-empty v-if="clientPlugins.length === 0" description="暂无已安装扩展" />
      <n-table v-else :bordered="false" :single-line="false">
        <thead>
          <tr>
            <th>名称</th>
            <th>版本</th>
            <th>状态</th>
            <th>启用</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="plugin in clientPlugins" :key="plugin.id">
            <td>{{ plugin.name }}</td>
            <td>v{{ plugin.version }}</td>
            <td>
              <n-tag v-if="plugin.running" type="success" size="small">运行中</n-tag>
              <n-tag v-else type="default" size="small">已停止</n-tag>
            </td>
            <td>
              <n-switch :value="plugin.enabled" @update:value="toggleClientPlugin(plugin)" />
            </td>
            <td>
              <n-space :size="4">
                <n-button size="small" quaternary @click="openConfigModal(plugin)">
                  <template #icon><n-icon><SettingsOutline /></n-icon></template>
                  配置
                </n-button>
                <n-button v-if="online && plugin.enabled && plugin.running" size="small" quaternary type="info" @click="handleRestartPlugin(plugin)">
                  <template #icon><n-icon><RefreshOutline /></n-icon></template>
                  重启
                </n-button>
                <n-button v-if="online && plugin.enabled && !plugin.running" size="small" quaternary type="success" @click="handleStartPlugin(plugin)">
                  <template #icon><n-icon><PlayOutline /></n-icon></template>
                  启动
                </n-button>
                <n-button v-if="online && plugin.enabled && plugin.running" size="small" quaternary type="warning" @click="handleStopPlugin(plugin)">
                  <template #icon><n-icon><StopOutline /></n-icon></template>
                  停止
                </n-button>
                <n-button size="small" quaternary type="error" @click="handleDeletePlugin(plugin)">
                  <template #icon><n-icon><TrashOutline /></n-icon></template>
                  删除
                </n-button>
              </n-space>
            </td>
          </tr>
        </tbody>
      </n-table>
    </n-card>

    <!-- 插件配置模态框 -->
    <n-modal v-model:show="showConfigModal" preset="card" :title="`${configPluginName} 配置`" style="width: 500px;">
      <n-empty v-if="configLoading" description="加载中..." />
      <n-empty v-else-if="configSchema.length === 0" description="该插件暂无可配置项" />
      <n-space v-else vertical :size="16">
        <n-form-item v-for="field in configSchema" :key="field.key" :label="field.label">
          <!-- 字符串输入 -->
          <n-input
            v-if="field.type === 'string'"
            v-model:value="configValues[field.key]"
            :placeholder="field.description || field.label"
          />
          <!-- 密码输入 -->
          <n-input
            v-else-if="field.type === 'password'"
            v-model:value="configValues[field.key]"
            type="password"
            show-password-on="click"
            :placeholder="field.description || field.label"
          />
          <!-- 数字输入 -->
          <n-input-number
            v-else-if="field.type === 'number'"
            :value="configValues[field.key] ? Number(configValues[field.key]) : undefined"
            @update:value="(v: number | null) => configValues[field.key] = v !== null ? String(v) : ''"
            :placeholder="field.description"
            style="width: 100%;"
          />
          <!-- 下拉选择 -->
          <n-select
            v-else-if="field.type === 'select'"
            v-model:value="configValues[field.key]"
            :options="(field.options || []).map(o => ({ label: o, value: o }))"
          />
          <!-- 布尔开关 -->
          <n-switch
            v-else-if="field.type === 'bool'"
            :value="configValues[field.key] === 'true'"
            @update:value="(v: boolean) => configValues[field.key] = String(v)"
          />
          <template #feedback v-if="field.description && field.type !== 'string' && field.type !== 'password'">
            <span style="color: #999; font-size: 12px;">{{ field.description }}</span>
          </template>
        </n-form-item>
      </n-space>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showConfigModal = false">取消</n-button>
          <n-button type="primary" @click="savePluginConfig" :disabled="configSchema.length === 0">
            保存
          </n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- 重命名模态框 -->
    <n-modal v-model:show="showRenameModal" preset="card" title="重命名客户端" style="width: 400px;">
      <n-form-item label="昵称" :show-feedback="false">
        <n-input v-model:value="renameValue" placeholder="给客户端起个名字（可选）" />
      </n-form-item>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showRenameModal = false">取消</n-button>
          <n-button type="primary" @click="saveRename">保存</n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- 商店插件安装模态框 -->
    <n-modal v-model:show="showStoreModal" preset="card" title="从商店安装插件" style="width: 500px;">
      <n-spin :show="storeLoading">
        <n-empty v-if="!storeLoading && storePlugins.length === 0" description="商店暂无可用插件" />
        <n-space v-else vertical :size="12">
          <n-card v-for="plugin in storePlugins" :key="plugin.name" size="small">
            <n-space justify="space-between" align="center">
              <n-space vertical :size="4">
                <n-space align="center">
                  <span style="font-weight: 500;">{{ plugin.name }}</span>
                  <n-tag size="small">v{{ plugin.version }}</n-tag>
                </n-space>
                <span style="color: #666; font-size: 12px;">{{ plugin.description }}</span>
                <span style="color: #999; font-size: 12px;">作者: {{ plugin.author }}</span>
              </n-space>
              <n-button
                size="small"
                type="primary"
                :loading="storeInstalling === plugin.name"
                @click="handleInstallStorePlugin(plugin)"
              >
                安装
              </n-button>
            </n-space>
          </n-card>
        </n-space>
      </n-spin>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showStoreModal = false">关闭</n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- 安装配置模态框 -->
    <n-modal v-model:show="showInstallConfigModal" preset="card" title="安装配置" style="width: 450px;">
      <n-space vertical :size="16">
        <div v-if="installPlugin">
          <p style="margin: 0 0 8px 0;"><strong>插件:</strong> {{ installPlugin.name }}</p>
          <p style="margin: 0; color: #666;">{{ installPlugin.description }}</p>
        </div>
        <div>
          <p style="margin: 0 0 8px 0; color: #666; font-size: 13px;">远程端口:</p>
          <n-input-number
            v-model:value="installRemotePort"
            :min="1"
            :max="65535"
            placeholder="输入端口号"
            style="width: 100%;"
          />
        </div>
        <div>
          <n-space align="center" :size="8">
            <n-switch v-model:value="installAuthEnabled" />
            <span style="color: #666;">启用 HTTP Basic Auth</span>
          </n-space>
        </div>
        <template v-if="installAuthEnabled">
          <n-input v-model:value="installAuthUsername" placeholder="用户名" />
          <n-input v-model:value="installAuthPassword" type="password" placeholder="密码" show-password-on="click" />
        </template>
      </n-space>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showInstallConfigModal = false">取消</n-button>
          <n-button type="primary" :loading="!!storeInstalling" @click="confirmInstallPlugin">
            安装
          </n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- 日志查看模态框 -->
    <n-modal v-model:show="showLogViewer" preset="card" style="width: 900px; max-width: 95vw;">
      <LogViewer :client-id="clientId" :visible="showLogViewer" @close="showLogViewer = false" />
    </n-modal>
  </div>
</template>
