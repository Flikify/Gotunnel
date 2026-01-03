<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import {
  NCard, NButton, NSpace, NTag, NStatistic, NGrid, NGi,
  NEmpty, NSpin, NIcon, NSwitch, NTabs, NTabPane, useMessage,
  NSelect, NModal, NInput, NInputNumber
} from 'naive-ui'
import { ArrowBackOutline, ExtensionPuzzleOutline, StorefrontOutline, CodeSlashOutline, SettingsOutline } from '@vicons/ionicons5'
import {
  getPlugins, enablePlugin, disablePlugin, getStorePlugins, getJSPlugins,
  pushJSPluginToClient, getClients, installStorePlugin, updateJSPluginConfig, setJSPluginEnabled
} from '../api'
import type { PluginInfo, StorePluginInfo, JSPlugin, ClientStatus } from '../types'

const router = useRouter()
const message = useMessage()
const plugins = ref<PluginInfo[]>([])
const storePlugins = ref<StorePluginInfo[]>([])
const jsPlugins = ref<JSPlugin[]>([])
const clients = ref<ClientStatus[]>([])
const loading = ref(true)
const storeLoading = ref(false)
const jsLoading = ref(false)
const activeTab = ref('installed')

const loadPlugins = async () => {
  try {
    const { data } = await getPlugins()
    plugins.value = data || []
  } catch (e) {
    console.error('Failed to load plugins', e)
  } finally {
    loading.value = false
  }
}

const loadStorePlugins = async () => {
  storeLoading.value = true
  try {
    const { data } = await getStorePlugins()
    storePlugins.value = data.plugins || []
  } catch (e) {
    console.error('Failed to load store plugins', e)
  } finally {
    storeLoading.value = false
  }
}

const proxyPlugins = computed(() =>
  plugins.value.filter(p => p.type === 'proxy')
)

const appPlugins = computed(() =>
  plugins.value.filter(p => p.type === 'app')
)

const togglePlugin = async (plugin: PluginInfo) => {
  try {
    if (plugin.enabled) {
      await disablePlugin(plugin.name)
      message.success(`已禁用 ${plugin.name}`)
    } else {
      await enablePlugin(plugin.name)
      message.success(`已启用 ${plugin.name}`)
    }
    plugin.enabled = !plugin.enabled
  } catch (e) {
    message.error('操作失败')
  }
}

const getTypeLabel = (type: string) => {
  const labels: Record<string, string> = {
    proxy: '协议',
    app: '应用',
    service: '服务',
    tool: '工具'
  }
  return labels[type] || type
}

const getTypeColor = (type: string) => {
  const colors: Record<string, 'info' | 'success' | 'warning' | 'error' | 'default'> = {
    proxy: 'info',
    app: 'success',
    service: 'warning',
    tool: 'default'
  }
  return colors[type] || 'default'
}

const handleTabChange = (tab: string) => {
  if (tab === 'store' && storePlugins.value.length === 0) {
    loadStorePlugins()
  }
  if (tab === 'js' && jsPlugins.value.length === 0) {
    loadJSPlugins()
  }
}

// JS 插件相关
/* 安全加固：暂时禁用创建/删除功能
const showJSModal = ref(false)
const jsForm = ref<JSPlugin>({...})
const configItems = ref<Array<{ key: string; value: string }>>([])
const configToObject = () => {...}
const handleCreateJSPlugin = async () => {...}
const handleDeleteJSPlugin = async (name: string) => {...}
const resetJSForm = () => {...}
*/

const loadJSPlugins = async () => {
  jsLoading.value = true
  try {
    const { data } = await getJSPlugins()
    jsPlugins.value = data || []
  } catch (e) {
    console.error('Failed to load JS plugins', e)
  } finally {
    jsLoading.value = false
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

// JS 插件推送相关
const showPushModal = ref(false)
const selectedJSPlugin = ref<JSPlugin | null>(null)
const pushClientId = ref('')
const pushRemotePort = ref<number | null>(8080)
const pushing = ref(false)

const openPushModal = (plugin: JSPlugin) => {
  selectedJSPlugin.value = plugin
  pushClientId.value = ''
  pushRemotePort.value = 8080
  showPushModal.value = true
}

const handlePushJSPlugin = async () => {
  if (!selectedJSPlugin.value || !pushClientId.value) {
    message.warning('请选择要推送到的客户端')
    return
  }
  pushing.value = true
  try {
    await pushJSPluginToClient(selectedJSPlugin.value.name, pushClientId.value, pushRemotePort.value || 0)
    message.success(`已推送 ${selectedJSPlugin.value.name} 到 ${pushClientId.value}，监听端口: ${pushRemotePort.value || '未指定'}`)
    showPushModal.value = false
  } catch (e: any) {
    message.error(e.response?.data || '推送失败')
  } finally {
    pushing.value = false
  }
}

const onlineClients = computed(() => clients.value.filter(c => c.online))

// JS 插件配置相关
const showJSConfigModal = ref(false)
const currentJSPlugin = ref<JSPlugin | null>(null)
const jsConfigItems = ref<Array<{ key: string; value: string }>>([])
const jsConfigSaving = ref(false)

const openJSConfigModal = (plugin: JSPlugin) => {
  currentJSPlugin.value = plugin
  // 将 config 转换为数组形式便于编辑
  jsConfigItems.value = Object.entries(plugin.config || {}).map(([key, value]) => ({ key, value }))
  if (jsConfigItems.value.length === 0) {
    jsConfigItems.value.push({ key: '', value: '' })
  }
  showJSConfigModal.value = true
}

const addJSConfigItem = () => {
  jsConfigItems.value.push({ key: '', value: '' })
}

const removeJSConfigItem = (index: number) => {
  jsConfigItems.value.splice(index, 1)
}

const saveJSPluginConfig = async () => {
  if (!currentJSPlugin.value) return

  jsConfigSaving.value = true
  try {
    // 将数组转换回对象
    const config: Record<string, string> = {}
    for (const item of jsConfigItems.value) {
      if (item.key.trim()) {
        config[item.key.trim()] = item.value
      }
    }
    await updateJSPluginConfig(currentJSPlugin.value.name, config)
    // 更新本地数据
    const plugin = jsPlugins.value.find(p => p.name === currentJSPlugin.value!.name)
    if (plugin) {
      plugin.config = config
    }
    message.success('配置已保存')
    showJSConfigModal.value = false
  } catch (e: any) {
    message.error(e.response?.data || '保存失败')
  } finally {
    jsConfigSaving.value = false
  }
}

// 切换 JS 插件启用状态
const toggleJSPlugin = async (plugin: JSPlugin) => {
  try {
    await setJSPluginEnabled(plugin.name, !plugin.enabled)
    plugin.enabled = !plugin.enabled
    message.success(plugin.enabled ? `已启用 ${plugin.name}` : `已禁用 ${plugin.name}`)
  } catch (e: any) {
    message.error(e.response?.data || '操作失败')
  }
}

// 商店插件安装相关
const showInstallModal = ref(false)
const selectedStorePlugin = ref<StorePluginInfo | null>(null)
const selectedClientId = ref('')
const installing = ref(false)

const openInstallModal = (plugin: StorePluginInfo) => {
  selectedStorePlugin.value = plugin
  selectedClientId.value = ''
  showInstallModal.value = true
}

const handleInstallStorePlugin = async () => {
  if (!selectedStorePlugin.value || !selectedClientId.value) {
    message.warning('请选择要安装到的客户端')
    return
  }
  if (!selectedStorePlugin.value.download_url) {
    message.error('该插件没有下载地址')
    return
  }
  if (!selectedStorePlugin.value.signature_url) {
    message.error('该插件没有签名文件')
    return
  }
  installing.value = true
  try {
    await installStorePlugin(
      selectedStorePlugin.value.name,
      selectedStorePlugin.value.download_url,
      selectedStorePlugin.value.signature_url,
      selectedClientId.value,
      8080, // 默认端口，可在配置中修改
      selectedStorePlugin.value.version,
      selectedStorePlugin.value.config_schema
    )
    message.success(`已安装 ${selectedStorePlugin.value.name}，可在客户端配置中修改端口和其他设置`)
    showInstallModal.value = false
  } catch (e: any) {
    message.error(e.response?.data || '安装失败')
  } finally {
    installing.value = false
  }
}

onMounted(() => {
  loadPlugins()
  loadClients()
})
</script>

<template>
  <div class="plugins-view">
    <n-space justify="space-between" align="center" style="margin-bottom: 24px;">
      <div>
        <h2 style="margin: 0 0 8px 0;">扩展商店</h2>
        <p style="margin: 0; color: #666;">管理已安装扩展和浏览扩展商店</p>
      </div>
      <n-button quaternary @click="router.push('/')">
        <template #icon><n-icon><ArrowBackOutline /></n-icon></template>
        返回首页
      </n-button>
    </n-space>

    <n-tabs v-model:value="activeTab" type="line" @update:value="handleTabChange">
      <!-- 已安装扩展 -->
      <n-tab-pane name="installed" tab="已安装">
        <n-spin :show="loading">
          <n-grid :cols="3" :x-gap="16" :y-gap="16" style="margin-bottom: 24px;">
            <n-gi>
              <n-card>
                <n-statistic label="总插件数" :value="plugins.length" />
              </n-card>
            </n-gi>
            <n-gi>
              <n-card>
                <n-statistic label="协议插件" :value="proxyPlugins.length" />
              </n-card>
            </n-gi>
            <n-gi>
              <n-card>
                <n-statistic label="应用插件" :value="appPlugins.length" />
              </n-card>
            </n-gi>
          </n-grid>

          <n-empty v-if="!loading && plugins.length === 0" description="暂无已安装扩展" />

          <n-grid v-else :cols="3" :x-gap="16" :y-gap="16" responsive="screen" cols-s="1" cols-m="2">
            <n-gi v-for="plugin in plugins" :key="plugin.name">
              <n-card hoverable>
                <template #header>
                  <n-space align="center">
                    <img v-if="plugin.icon" :src="plugin.icon" style="width: 24px; height: 24px;" />
                    <n-icon v-else size="24" color="#18a058"><ExtensionPuzzleOutline /></n-icon>
                    <span>{{ plugin.name }}</span>
                  </n-space>
                </template>
                <template #header-extra>
                  <n-switch :value="plugin.enabled" @update:value="togglePlugin(plugin)" />
                </template>
                <n-space vertical :size="8">
                  <n-space>
                    <n-tag size="small">v{{ plugin.version }}</n-tag>
                    <n-tag size="small" :type="getTypeColor(plugin.type)">
                      {{ getTypeLabel(plugin.type) }}
                    </n-tag>
                    <n-tag size="small" :type="plugin.source === 'builtin' ? 'default' : 'info'">
                      {{ plugin.source === 'builtin' ? '内置' : 'JS' }}
                    </n-tag>
                  </n-space>
                  <p style="margin: 0; color: #666;">{{ plugin.description }}</p>
                </n-space>
              </n-card>
            </n-gi>
          </n-grid>
        </n-spin>
      </n-tab-pane>

      <!-- 扩展商店 -->
      <n-tab-pane name="store" tab="扩展商店">
        <n-spin :show="storeLoading">
          <n-empty v-if="!storeLoading && storePlugins.length === 0" description="扩展商店暂无可用扩展" />

          <n-grid v-else :cols="3" :x-gap="16" :y-gap="16" responsive="screen" cols-s="1" cols-m="2">
            <n-gi v-for="plugin in storePlugins" :key="plugin.name">
              <n-card hoverable>
                <template #header>
                  <n-space align="center">
                    <img v-if="plugin.icon" :src="plugin.icon" style="width: 24px; height: 24px;" />
                    <n-icon v-else size="24" color="#18a058"><StorefrontOutline /></n-icon>
                    <span>{{ plugin.name }}</span>
                  </n-space>
                </template>
                <template #header-extra>
                  <n-button
                    v-if="plugin.download_url && plugin.signature_url && onlineClients.length > 0"
                    size="small"
                    type="primary"
                    @click="openInstallModal(plugin)"
                  >
                    安装
                  </n-button>
                </template>
                <n-space vertical :size="8">
                  <n-space>
                    <n-tag size="small">v{{ plugin.version }}</n-tag>
                    <n-tag size="small" :type="getTypeColor(plugin.type)">
                      {{ getTypeLabel(plugin.type) }}
                    </n-tag>
                  </n-space>
                  <p style="margin: 0; color: #666;">{{ plugin.description }}</p>
                  <p style="margin: 0; color: #999; font-size: 12px;">作者: {{ plugin.author }}</p>
                </n-space>
              </n-card>
            </n-gi>
          </n-grid>
        </n-spin>
      </n-tab-pane>

      <!-- JS 插件 -->
      <n-tab-pane name="js" tab="JS 插件">
        <n-spin :show="jsLoading">
          <n-empty v-if="!jsLoading && jsPlugins.length === 0" description="暂无 JS 插件" />

          <n-grid v-else :cols="2" :x-gap="16" :y-gap="16" responsive="screen" cols-s="1">
            <n-gi v-for="plugin in jsPlugins" :key="plugin.name">
              <n-card hoverable>
                <template #header>
                  <n-space align="center">
                    <n-icon size="24" color="#f0a020"><CodeSlashOutline /></n-icon>
                    <span>{{ plugin.name }}</span>
                    <n-tag v-if="plugin.version" size="small">v{{ plugin.version }}</n-tag>
                  </n-space>
                </template>
                <template #header-extra>
                  <n-switch :value="plugin.enabled" @update:value="toggleJSPlugin(plugin)" />
                </template>
                <n-space vertical :size="8">
                  <n-space>
                    <n-tag size="small" type="warning">JS</n-tag>
                    <n-tag v-if="plugin.auto_start" size="small" type="success">自动启动</n-tag>
                    <n-tag v-if="plugin.signature" size="small" type="info">已签名</n-tag>
                  </n-space>
                  <p style="margin: 0; color: #666;">{{ plugin.description || '无描述' }}</p>
                  <p v-if="plugin.author" style="margin: 0; color: #999; font-size: 12px;">作者: {{ plugin.author }}</p>

                  <!-- 配置预览 -->
                  <div v-if="Object.keys(plugin.config || {}).length > 0" style="margin-top: 8px;">
                    <p style="margin: 0 0 4px 0; color: #999; font-size: 12px;">配置:</p>
                    <n-space :size="4" wrap>
                      <n-tag v-for="(value, key) in plugin.config" :key="key" size="small" type="default">
                        {{ key }}: {{ value.length > 10 ? value.slice(0, 10) + '...' : value }}
                      </n-tag>
                    </n-space>
                  </div>
                </n-space>
                <template #action>
                  <n-space justify="space-between">
                    <n-button size="small" quaternary @click="openJSConfigModal(plugin)">
                      <template #icon><n-icon><SettingsOutline /></n-icon></template>
                      配置
                    </n-button>
                    <n-button
                      v-if="onlineClients.length > 0"
                      size="small"
                      type="primary"
                      @click="openPushModal(plugin)"
                    >
                      推送到客户端
                    </n-button>
                  </n-space>
                </template>
              </n-card>
            </n-gi>
          </n-grid>
        </n-spin>
      </n-tab-pane>
    </n-tabs>

    <!-- 安全加固：暂时禁用创建 JS 插件 Modal
    <n-modal v-model:show="showJSModal" preset="card" title="新建 JS 插件" style="width: 600px;">
      ... 已屏蔽 ...
    </n-modal>
    -->

    <!-- 安装商店插件模态框 -->
    <n-modal v-model:show="showInstallModal" preset="card" title="安装插件" style="width: 400px;">
      <n-space vertical :size="16">
        <div v-if="selectedStorePlugin">
          <p style="margin: 0 0 8px 0;"><strong>插件:</strong> {{ selectedStorePlugin.name }}</p>
          <p style="margin: 0; color: #666;">{{ selectedStorePlugin.description }}</p>
        </div>
        <n-select
          v-model:value="selectedClientId"
          placeholder="选择要安装到的客户端"
          :options="onlineClients.map(c => ({ label: c.nickname || c.id, value: c.id }))"
        />
        <p style="margin: 0; color: #999; font-size: 12px;">安装后可在客户端详情页配置端口和其他设置</p>
      </n-space>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showInstallModal = false">取消</n-button>
          <n-button
            type="primary"
            :loading="installing"
            :disabled="!selectedClientId"
            @click="handleInstallStorePlugin"
          >
            安装
          </n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- JS 插件配置模态框 -->
    <n-modal v-model:show="showJSConfigModal" preset="card" :title="`${currentJSPlugin?.name || ''} 配置`" style="width: 500px;">
      <n-space vertical :size="12">
        <p style="margin: 0; color: #666; font-size: 13px;">编辑插件配置参数（键值对形式）</p>
        <div v-for="(item, index) in jsConfigItems" :key="index">
          <n-space :size="8" align="center">
            <n-input v-model:value="item.key" placeholder="参数名" style="width: 150px;" />
            <n-input v-model:value="item.value" placeholder="参数值" style="width: 200px;" />
            <n-button v-if="jsConfigItems.length > 1" quaternary type="error" size="small" @click="removeJSConfigItem(index)">
              删除
            </n-button>
          </n-space>
        </div>
        <n-button dashed size="small" @click="addJSConfigItem">添加配置项</n-button>
      </n-space>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showJSConfigModal = false">取消</n-button>
          <n-button type="primary" :loading="jsConfigSaving" @click="saveJSPluginConfig">保存</n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- JS 插件推送模态框 -->
    <n-modal v-model:show="showPushModal" preset="card" title="推送插件到客户端" style="width: 400px;">
      <n-space vertical :size="16">
        <div v-if="selectedJSPlugin">
          <p style="margin: 0 0 8px 0;"><strong>插件:</strong> {{ selectedJSPlugin.name }}</p>
          <p style="margin: 0; color: #666;">{{ selectedJSPlugin.description || '无描述' }}</p>
        </div>
        <n-select
          v-model:value="pushClientId"
          placeholder="选择要推送到的客户端"
          :options="onlineClients.map(c => ({ label: c.nickname || c.id, value: c.id }))"
        />
        <div>
          <p style="margin: 0 0 8px 0; color: #666; font-size: 13px;">远程端口（服务端监听端口）:</p>
          <n-input-number
            v-model:value="pushRemotePort"
            :min="1"
            :max="65535"
            placeholder="输入端口号"
            style="width: 100%;"
          />
          <p style="margin: 8px 0 0 0; color: #999; font-size: 12px;">用户可以通过 服务端IP:端口 访问此插件提供的服务</p>
        </div>
      </n-space>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showPushModal = false">取消</n-button>
          <n-button
            type="primary"
            :loading="pushing"
            :disabled="!pushClientId"
            @click="handlePushJSPlugin"
          >
            推送
          </n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>
