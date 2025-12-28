<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NCard, NButton, NSpace, NTag, NTable, NEmpty,
  NFormItem, NInput, NInputNumber, NSelect, NModal, NCheckbox, NSwitch,
  NIcon, useMessage, useDialog
} from 'naive-ui'
import {
  ArrowBackOutline, CreateOutline, TrashOutline,
  PushOutline, PowerOutline, AddOutline, SaveOutline, CloseOutline,
  DownloadOutline, SettingsOutline
} from '@vicons/ionicons5'
import {
  getClient, updateClient, deleteClient, pushConfigToClient, disconnectClient,
  getPlugins, installPluginsToClient, getClientPluginConfig, updateClientPluginConfig
} from '../api'
import type { ProxyRule, PluginInfo, ClientPlugin, ConfigField, RuleSchema } from '../types'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const dialog = useDialog()
const clientId = route.params.id as string

const online = ref(false)
const lastPing = ref('')
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
  { label: 'HTTPS', value: 'https' }
]

// 规则类型选项（内置 + 插件）
const typeOptions = ref([...builtinTypes])

// 插件 RuleSchema 映射
const pluginRuleSchemas = ref<Record<string, RuleSchema>>({})

// 判断类型是否需要本地地址
const needsLocalAddr = (type: string) => {
  // 内置类型
  if (['tcp', 'udp'].includes(type)) return true
  // 插件类型：查询 RuleSchema
  const schema = pluginRuleSchemas.value[type]
  return schema?.needs_local_addr ?? false
}

// 获取类型的额外字段
const getExtraFields = (type: string): ConfigField[] => {
  const schema = pluginRuleSchemas.value[type]
  return schema?.extra_fields || []
}

// 插件安装相关
const showInstallModal = ref(false)
const availablePlugins = ref<PluginInfo[]>([])
const selectedPlugins = ref<string[]>([])

// 插件配置相关
const showConfigModal = ref(false)
const configPluginName = ref('')
const configSchema = ref<ConfigField[]>([])
const configValues = ref<Record<string, string>>({})
const configLoading = ref(false)

const loadPlugins = async () => {
  try {
    const { data } = await getPlugins()
    availablePlugins.value = (data || []).filter(p => p.enabled)

    // 更新类型选项：内置类型 + proxy 类型插件
    const proxyPlugins = availablePlugins.value
      .filter(p => p.type === 'proxy')
      .map(p => ({ label: `${p.name.toUpperCase()} (插件)`, value: p.name }))
    typeOptions.value = [...builtinTypes, ...proxyPlugins]

    // 保存插件的 RuleSchema
    const schemas: Record<string, RuleSchema> = {}
    for (const p of availablePlugins.value) {
      if (p.rule_schema) {
        schemas[p.name] = p.rule_schema
      }
    }
    pluginRuleSchemas.value = schemas
  } catch (e) {
    console.error('Failed to load plugins', e)
  }
}

const openInstallModal = async () => {
  await loadPlugins()
  // 过滤掉已安装的插件
  const installedNames = clientPlugins.value.map(p => p.name)
  availablePlugins.value = availablePlugins.value.filter(p => !installedNames.includes(p.name))
  selectedPlugins.value = []
  showInstallModal.value = true
}

const getTypeLabel = (type: string) => {
  const labels: Record<string, string> = { proxy: '协议', app: '应用', service: '服务', tool: '工具' }
  return labels[type] || type
}

const loadClient = async () => {
  try {
    const { data } = await getClient(clientId)
    online.value = data.online
    lastPing.value = data.last_ping || ''
    nickname.value = data.nickname || ''
    rules.value = data.rules || []
    clientPlugins.value = data.plugins || []
  } catch (e) {
    console.error('Failed to load client', e)
  }
}

onMounted(() => {
  loadClient()
  loadPlugins()
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
    await updateClient(clientId, { id: clientId, nickname: nickname.value, rules: editRules.value })
    editing.value = false
    message.success('保存成功')
    loadClient()
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

const installPlugins = async () => {
  if (selectedPlugins.value.length === 0) {
    message.warning('请选择要安装的插件')
    return
  }
  try {
    await installPluginsToClient(clientId, selectedPlugins.value)
    message.success(`已推送 ${selectedPlugins.value.length} 个插件到客户端`)
    showInstallModal.value = false
    await loadClient() // 刷新客户端数据
  } catch (e: any) {
    message.error(e.response?.data || '安装失败')
  }
}

const toggleClientPlugin = async (plugin: ClientPlugin) => {
  const newEnabled = !plugin.enabled
  const updatedPlugins = clientPlugins.value.map(p =>
    p.name === plugin.name ? { ...p, enabled: newEnabled } : p
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
            <n-button type="success" @click="openInstallModal">
              <template #icon><n-icon><DownloadOutline /></n-icon></template>
              安装插件
            </n-button>
            <n-button type="warning" @click="disconnect">
              <template #icon><n-icon><PowerOutline /></n-icon></template>
              断开连接
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
            </tr>
          </tbody>
        </n-table>
      </template>

      <!-- 编辑模式 -->
      <template v-else>
        <n-space vertical :size="12">
          <n-card v-for="(rule, i) in editRules" :key="i" size="small">
            <n-space align="center" wrap>
              <n-form-item label="启用" :show-feedback="false">
                <n-switch v-model:value="rule.enabled" />
              </n-form-item>
              <n-form-item label="名称" :show-feedback="false">
                <n-input v-model:value="rule.name" placeholder="规则名称" />
              </n-form-item>
              <n-form-item label="类型" :show-feedback="false">
                <n-select v-model:value="rule.type" :options="typeOptions" style="width: 140px;" />
              </n-form-item>
              <!-- 仅 tcp/udp 显示本地地址 -->
              <template v-if="needsLocalAddr(rule.type || 'tcp')">
                <n-form-item label="本地IP" :show-feedback="false">
                  <n-input v-model:value="rule.local_ip" placeholder="127.0.0.1" />
                </n-form-item>
                <n-form-item label="本地端口" :show-feedback="false">
                  <n-input-number v-model:value="rule.local_port" :show-button="false" />
                </n-form-item>
              </template>
              <n-form-item label="远程端口" :show-feedback="false">
                <n-input-number v-model:value="rule.remote_port" :show-button="false" />
              </n-form-item>
              <!-- 插件额外字段 -->
              <template v-for="field in getExtraFields(rule.type || '')" :key="field.key">
                <n-form-item :label="field.label" :show-feedback="false">
                  <n-input
                    v-if="field.type === 'string'"
                    :value="rule.plugin_config?.[field.key] || field.default || ''"
                    @update:value="(v: string) => { if (!rule.plugin_config) rule.plugin_config = {}; rule.plugin_config[field.key] = v }"
                    :placeholder="field.description"
                  />
                </n-form-item>
              </template>
              <n-button v-if="editRules.length > 1" quaternary type="error" @click="removeRule(i)">
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
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="plugin in clientPlugins" :key="plugin.name">
            <td>{{ plugin.name }}</td>
            <td>v{{ plugin.version }}</td>
            <td>
              <n-switch :value="plugin.enabled" @update:value="toggleClientPlugin(plugin)" />
            </td>
            <td>
              <n-button size="small" quaternary @click="openConfigModal(plugin)">
                <template #icon><n-icon><SettingsOutline /></n-icon></template>
                配置
              </n-button>
            </td>
          </tr>
        </tbody>
      </n-table>
    </n-card>

    <!-- 安装插件模态框 -->
    <n-modal v-model:show="showInstallModal" preset="card" title="安装插件到客户端" style="width: 500px;">
      <n-empty v-if="availablePlugins.length === 0" description="暂无可用插件" />
      <n-space v-else vertical :size="12">
        <n-card v-for="plugin in availablePlugins" :key="plugin.name" size="small">
          <n-space justify="space-between" align="center">
            <n-space vertical :size="4">
              <n-space align="center">
                <span style="font-weight: 500;">{{ plugin.name }}</span>
                <n-tag size="small">{{ getTypeLabel(plugin.type) }}</n-tag>
              </n-space>
              <span style="color: #666; font-size: 12px;">{{ plugin.description }}</span>
            </n-space>
            <n-checkbox
              :checked="selectedPlugins.includes(plugin.name)"
              @update:checked="(v: boolean) => {
                if (v) selectedPlugins.push(plugin.name)
                else selectedPlugins = selectedPlugins.filter(n => n !== plugin.name)
              }"
            />
          </n-space>
        </n-card>
      </n-space>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showInstallModal = false">取消</n-button>
          <n-button type="primary" @click="installPlugins" :disabled="selectedPlugins.length === 0">
            安装 ({{ selectedPlugins.length }})
          </n-button>
        </n-space>
      </template>
    </n-modal>

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
  </div>
</template>
