<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import {
  NCard, NButton, NSpace, NTag, NStatistic, NGrid, NGi,
  NEmpty, NSpin, NIcon, NSwitch, NTabs, NTabPane, useMessage,
  NSelect, NModal
} from 'naive-ui'
import { ArrowBackOutline, ExtensionPuzzleOutline, StorefrontOutline, CodeSlashOutline } from '@vicons/ionicons5'
import {
  getPlugins, enablePlugin, disablePlugin, getStorePlugins, getJSPlugins,
  pushJSPluginToClient, getClients, installStorePlugin
} from '../api'
import type { PluginInfo, StorePluginInfo, JSPlugin, ClientStatus } from '../types'

const router = useRouter()
const message = useMessage()
const plugins = ref<PluginInfo[]>([])
const storePlugins = ref<StorePluginInfo[]>([])
const jsPlugins = ref<JSPlugin[]>([])
const clients = ref<ClientStatus[]>([])
const storeUrl = ref('')
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
    storeUrl.value = data.store_url || ''
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

const handlePushJSPlugin = async (pluginName: string, clientId: string) => {
  try {
    await pushJSPluginToClient(pluginName, clientId)
    message.success(`已推送 ${pluginName} 到 ${clientId}`)
  } catch (e) {
    message.error('推送失败')
  }
}

const onlineClients = computed(() => clients.value.filter(c => c.online))

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
  installing.value = true
  try {
    await installStorePlugin(
      selectedStorePlugin.value.name,
      selectedStorePlugin.value.download_url,
      selectedClientId.value
    )
    message.success(`已安装 ${selectedStorePlugin.value.name} 到客户端`)
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
                    <n-tag size="small" :type="plugin.source === 'builtin' ? 'default' : 'warning'">
                      {{ plugin.source === 'builtin' ? '内置' : 'WASM' }}
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
          <n-empty v-if="!storeUrl" description="未配置扩展商店URL，请在配置文件中设置 plugin_store.url" />
          <n-empty v-else-if="!storeLoading && storePlugins.length === 0" description="扩展商店暂无可用扩展" />

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
                    v-if="plugin.download_url && onlineClients.length > 0"
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
        <!-- 安全加固：暂时禁用 Web UI 创建功能
        <n-space justify="end" style="margin-bottom: 16px;">
          <n-button type="primary" @click="showJSModal = true">
            <template #icon><n-icon><AddOutline /></n-icon></template>
            新建 JS 插件
          </n-button>
        </n-space>
        -->

        <n-spin :show="jsLoading">
          <n-empty v-if="!jsLoading && jsPlugins.length === 0" description="暂无 JS 插件" />

          <n-grid v-else :cols="2" :x-gap="16" :y-gap="16" responsive="screen" cols-s="1">
            <n-gi v-for="plugin in jsPlugins" :key="plugin.name">
              <n-card hoverable>
                <template #header>
                  <n-space align="center">
                    <n-icon size="24" color="#f0a020"><CodeSlashOutline /></n-icon>
                    <span>{{ plugin.name }}</span>
                  </n-space>
                </template>
                <template #header-extra>
                  <n-space>
                    <n-select
                      v-if="onlineClients.length > 0"
                      placeholder="推送到..."
                      size="small"
                      style="width: 120px;"
                      :options="onlineClients.map(c => ({ label: c.nickname || c.id, value: c.id }))"
                      @update:value="(v: string) => handlePushJSPlugin(plugin.name, v)"
                    />
                    <!-- 安全加固：暂时禁用删除功能
                    <n-popconfirm @positive-click="handleDeleteJSPlugin(plugin.name)">
                      <template #trigger>
                        <n-button size="small" type="error" quaternary>删除</n-button>
                      </template>
                      确定删除此插件？
                    </n-popconfirm>
                    -->
                  </n-space>
                </template>
                <n-space vertical :size="8">
                  <n-space>
                    <n-tag size="small" type="warning">JS</n-tag>
                    <n-tag v-if="plugin.auto_start" size="small" type="success">自动启动</n-tag>
                  </n-space>
                  <p style="margin: 0; color: #666;">{{ plugin.description || '无描述' }}</p>
                  <p v-if="plugin.author" style="margin: 0; color: #999; font-size: 12px;">作者: {{ plugin.author }}</p>
                </n-space>
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
  </div>
</template>
