<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import {
  NButton, NSpace, NTag, NIcon, NSwitch, NModal, NInput, NInputNumber, NSelect,
  useMessage
} from 'naive-ui'
import { ExtensionPuzzleOutline, StorefrontOutline, CodeSlashOutline, SettingsOutline } from '@vicons/ionicons5'
import {
  getPlugins, enablePlugin, disablePlugin, getStorePlugins, getJSPlugins,
  pushJSPluginToClient, getClients, installStorePlugin, updateJSPluginConfig, setJSPluginEnabled
} from '../api'
import type { PluginInfo, StorePluginInfo, JSPlugin, ClientStatus } from '../types'

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

const proxyPlugins = computed(() => plugins.value.filter(p => p.type === 'proxy'))
const appPlugins = computed(() => plugins.value.filter(p => p.type === 'app'))

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
  const labels: Record<string, string> = { proxy: '协议', app: '应用', service: '服务', tool: '工具' }
  return labels[type] || type
}

const handleTabChange = (tab: string) => {
  if (tab === 'store' && storePlugins.value.length === 0) loadStorePlugins()
  if (tab === 'js' && jsPlugins.value.length === 0) loadJSPlugins()
}

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

// JS Plugin Push
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
    message.success(`已推送 ${selectedJSPlugin.value.name}`)
    showPushModal.value = false
  } catch (e: any) {
    message.error(e.response?.data || '推送失败')
  } finally {
    pushing.value = false
  }
}

const onlineClients = computed(() => clients.value.filter(c => c.online))

// JS Plugin Config
const showJSConfigModal = ref(false)
const currentJSPlugin = ref<JSPlugin | null>(null)
const jsConfigItems = ref<Array<{ key: string; value: string }>>([])
const jsConfigSaving = ref(false)

const openJSConfigModal = (plugin: JSPlugin) => {
  currentJSPlugin.value = plugin
  jsConfigItems.value = Object.entries(plugin.config || {}).map(([key, value]) => ({ key, value }))
  if (jsConfigItems.value.length === 0) jsConfigItems.value.push({ key: '', value: '' })
  showJSConfigModal.value = true
}

const addJSConfigItem = () => jsConfigItems.value.push({ key: '', value: '' })
const removeJSConfigItem = (index: number) => jsConfigItems.value.splice(index, 1)

const saveJSPluginConfig = async () => {
  if (!currentJSPlugin.value) return
  jsConfigSaving.value = true
  try {
    const config: Record<string, string> = {}
    for (const item of jsConfigItems.value) {
      if (item.key.trim()) config[item.key.trim()] = item.value
    }
    await updateJSPluginConfig(currentJSPlugin.value.name, config)
    const plugin = jsPlugins.value.find(p => p.name === currentJSPlugin.value!.name)
    if (plugin) plugin.config = config
    message.success('配置已保存')
    showJSConfigModal.value = false
  } catch (e: any) {
    message.error(e.response?.data || '保存失败')
  } finally {
    jsConfigSaving.value = false
  }
}

const toggleJSPlugin = async (plugin: JSPlugin) => {
  try {
    await setJSPluginEnabled(plugin.name, !plugin.enabled)
    plugin.enabled = !plugin.enabled
    message.success(plugin.enabled ? `已启用 ${plugin.name}` : `已禁用 ${plugin.name}`)
  } catch (e: any) {
    message.error(e.response?.data || '操作失败')
  }
}

// Store Plugin Install
const showInstallModal = ref(false)
const selectedStorePlugin = ref<StorePluginInfo | null>(null)
const selectedClientId = ref('')
const installing = ref(false)
const installRemotePort = ref<number | null>(8080)
const installAuthEnabled = ref(false)
const installAuthUsername = ref('')
const installAuthPassword = ref('')

const openInstallModal = (plugin: StorePluginInfo) => {
  selectedStorePlugin.value = plugin
  selectedClientId.value = ''
  installRemotePort.value = 8080
  installAuthEnabled.value = false
  installAuthUsername.value = ''
  installAuthPassword.value = ''
  showInstallModal.value = true
}

const handleInstallStorePlugin = async () => {
  if (!selectedStorePlugin.value || !selectedClientId.value) {
    message.warning('请选择要安装到的客户端')
    return
  }
  if (!selectedStorePlugin.value.download_url || !selectedStorePlugin.value.signature_url) {
    message.error('该插件缺少下载地址或签名')
    return
  }
  installing.value = true
  try {
    await installStorePlugin(
      selectedStorePlugin.value.name,
      selectedStorePlugin.value.download_url,
      selectedStorePlugin.value.signature_url,
      selectedClientId.value,
      installRemotePort.value || 8080,
      selectedStorePlugin.value.version,
      selectedStorePlugin.value.config_schema,
      installAuthEnabled.value,
      installAuthUsername.value,
      installAuthPassword.value
    )
    message.success(`已安装 ${selectedStorePlugin.value.name}`)
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
  <div class="plugins-page">
    <!-- Particles -->
    <div class="particles">
      <div class="particle particle-1"></div>
      <div class="particle particle-2"></div>
      <div class="particle particle-3"></div>
    </div>

    <div class="plugins-content">
      <!-- Header -->
      <div class="page-header">
        <h1 class="page-title">插件管理</h1>
        <p class="page-subtitle">管理已安装插件和浏览插件商店</p>
      </div>

      <!-- Stats Row -->
      <div class="stats-row">
        <div class="stat-card">
          <span class="stat-value">{{ plugins.length }}</span>
          <span class="stat-label">总插件数</span>
        </div>
        <div class="stat-card">
          <span class="stat-value">{{ proxyPlugins.length }}</span>
          <span class="stat-label">协议插件</span>
        </div>
        <div class="stat-card">
          <span class="stat-value">{{ appPlugins.length }}</span>
          <span class="stat-label">应用插件</span>
        </div>
      </div>

      <!-- Tabs -->
      <div class="glass-card">
        <div class="tabs-header">
          <button class="tab-btn" :class="{ active: activeTab === 'installed' }" @click="activeTab = 'installed'">
            已安装插件
          </button>
          <button class="tab-btn" :class="{ active: activeTab === 'store' }" @click="activeTab = 'store'; handleTabChange('store')">
            插件商店
          </button>
          <button class="tab-btn" :class="{ active: activeTab === 'js' }" @click="activeTab = 'js'; handleTabChange('js')">
            JS 插件
          </button>
        </div>

        <!-- Installed Plugins Tab -->
        <div v-if="activeTab === 'installed'" class="tab-content">
          <div v-if="loading" class="loading-state">加载中...</div>
          <div v-else-if="plugins.length === 0" class="empty-state">暂无已安装插件</div>
          <div v-else class="plugins-grid">
            <div v-for="plugin in plugins" :key="plugin.name" class="plugin-card">
              <div class="plugin-header">
                <div class="plugin-icon">
                  <n-icon size="20" color="#a78bfa"><ExtensionPuzzleOutline /></n-icon>
                </div>
                <span class="plugin-name">{{ plugin.name }}</span>
                <n-switch :value="plugin.enabled" size="small" @update:value="togglePlugin(plugin)" />
              </div>
              <div class="plugin-tags">
                <n-tag size="small">v{{ plugin.version }}</n-tag>
                <n-tag size="small" :type="plugin.type === 'proxy' ? 'info' : 'success'">{{ getTypeLabel(plugin.type) }}</n-tag>
                <n-tag size="small" :type="plugin.source === 'builtin' ? 'default' : 'warning'">
                  {{ plugin.source === 'builtin' ? '内置' : 'JS' }}
                </n-tag>
              </div>
              <p class="plugin-desc">{{ plugin.description }}</p>
            </div>
          </div>
        </div>

        <!-- Store Tab -->
        <div v-if="activeTab === 'store'" class="tab-content">
          <div v-if="storeLoading" class="loading-state">加载中...</div>
          <div v-else-if="storePlugins.length === 0" class="empty-state">插件商店暂无可用插件</div>
          <div v-else class="plugins-grid">
            <div v-for="plugin in storePlugins" :key="plugin.name" class="plugin-card">
              <div class="plugin-header">
                <div class="plugin-icon store">
                  <n-icon size="20" color="#60a5fa"><StorefrontOutline /></n-icon>
                </div>
                <span class="plugin-name">{{ plugin.name }}</span>
                <button
                  v-if="plugin.download_url && plugin.signature_url && onlineClients.length > 0"
                  class="glass-btn primary tiny"
                  @click="openInstallModal(plugin)"
                >安装</button>
              </div>
              <div class="plugin-tags">
                <n-tag size="small">v{{ plugin.version }}</n-tag>
                <n-tag size="small" :type="plugin.type === 'proxy' ? 'info' : 'success'">{{ getTypeLabel(plugin.type) }}</n-tag>
              </div>
              <p class="plugin-desc">{{ plugin.description }}</p>
              <p class="plugin-author">作者: {{ plugin.author }}</p>
            </div>
          </div>
        </div>

        <!-- JS Plugins Tab -->
        <div v-if="activeTab === 'js'" class="tab-content">
          <div v-if="jsLoading" class="loading-state">加载中...</div>
          <div v-else-if="jsPlugins.length === 0" class="empty-state">暂无 JS 插件</div>
          <div v-else class="plugins-grid wide">
            <div v-for="plugin in jsPlugins" :key="plugin.name" class="plugin-card js">
              <div class="plugin-header">
                <div class="plugin-icon js">
                  <n-icon size="20" color="#fbbf24"><CodeSlashOutline /></n-icon>
                </div>
                <span class="plugin-name">{{ plugin.name }}</span>
                <n-tag v-if="plugin.version" size="small">v{{ plugin.version }}</n-tag>
                <n-switch :value="plugin.enabled" size="small" @update:value="toggleJSPlugin(plugin)" />
              </div>
              <div class="plugin-tags">
                <n-tag size="small" type="warning">JS</n-tag>
                <n-tag v-if="plugin.auto_start" size="small" type="success">自动启动</n-tag>
                <n-tag v-if="plugin.signature" size="small" type="info">已签名</n-tag>
              </div>
              <p class="plugin-desc">{{ plugin.description || '无描述' }}</p>
              <p v-if="plugin.author" class="plugin-author">作者: {{ plugin.author }}</p>
              <div v-if="Object.keys(plugin.config || {}).length > 0" class="plugin-config-preview">
                <span class="config-label">配置:</span>
                <n-space :size="4" wrap>
                  <n-tag v-for="(value, key) in plugin.config" :key="key" size="small">
                    {{ key }}: {{ String(value).length > 10 ? String(value).slice(0, 10) + '...' : value }}
                  </n-tag>
                </n-space>
              </div>
              <div class="plugin-actions">
                <button class="glass-btn tiny" @click="openJSConfigModal(plugin)">
                  <n-icon size="14"><SettingsOutline /></n-icon>
                  配置
                </button>
                <button v-if="onlineClients.length > 0" class="glass-btn primary tiny" @click="openPushModal(plugin)">
                  推送到客户端
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Install Modal -->
    <n-modal v-model:show="showInstallModal" preset="card" title="安装插件" style="width: 450px;">
      <n-space vertical :size="16">
        <div v-if="selectedStorePlugin">
          <p style="margin: 0 0 8px 0;"><strong>插件:</strong> {{ selectedStorePlugin.name }}</p>
          <p style="margin: 0; color: #666;">{{ selectedStorePlugin.description }}</p>
        </div>
        <n-select v-model:value="selectedClientId" placeholder="选择客户端"
          :options="onlineClients.map(c => ({ label: c.nickname || c.id, value: c.id }))" />
        <div>
          <p style="margin: 0 0 8px 0; color: #666; font-size: 13px;">远程端口:</p>
          <n-input-number v-model:value="installRemotePort" :min="1" :max="65535" style="width: 100%;" />
        </div>
        <n-space align="center" :size="8">
          <n-switch v-model:value="installAuthEnabled" />
          <span style="color: #666;">启用 HTTP Basic Auth</span>
        </n-space>
        <template v-if="installAuthEnabled">
          <n-input v-model:value="installAuthUsername" placeholder="用户名" />
          <n-input v-model:value="installAuthPassword" type="password" placeholder="密码" show-password-on="click" />
        </template>
      </n-space>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showInstallModal = false">取消</n-button>
          <n-button type="primary" :loading="installing" :disabled="!selectedClientId" @click="handleInstallStorePlugin">安装</n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- JS Config Modal -->
    <n-modal v-model:show="showJSConfigModal" preset="card" :title="`${currentJSPlugin?.name || ''} 配置`" style="width: 500px;">
      <n-space vertical :size="12">
        <p style="margin: 0; color: #666; font-size: 13px;">编辑插件配置参数</p>
        <div v-for="(item, index) in jsConfigItems" :key="index">
          <n-space :size="8" align="center">
            <n-input v-model:value="item.key" placeholder="参数名" style="width: 150px;" />
            <n-input v-model:value="item.value" placeholder="参数值" style="width: 200px;" />
            <n-button v-if="jsConfigItems.length > 1" quaternary type="error" size="small" @click="removeJSConfigItem(index)">删除</n-button>
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

    <!-- Push Modal -->
    <n-modal v-model:show="showPushModal" preset="card" title="推送插件到客户端" style="width: 400px;">
      <n-space vertical :size="16">
        <div v-if="selectedJSPlugin">
          <p style="margin: 0 0 8px 0;"><strong>插件:</strong> {{ selectedJSPlugin.name }}</p>
          <p style="margin: 0; color: #666;">{{ selectedJSPlugin.description || '无描述' }}</p>
        </div>
        <n-select v-model:value="pushClientId" placeholder="选择客户端"
          :options="onlineClients.map(c => ({ label: c.nickname || c.id, value: c.id }))" />
        <div>
          <p style="margin: 0 0 8px 0; color: #666; font-size: 13px;">远程端口:</p>
          <n-input-number v-model:value="pushRemotePort" :min="1" :max="65535" style="width: 100%;" />
        </div>
      </n-space>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showPushModal = false">取消</n-button>
          <n-button type="primary" :loading="pushing" :disabled="!pushClientId" @click="handlePushJSPlugin">推送</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.plugins-page {
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

.plugins-content {
  position: relative;
  z-index: 10;
  max-width: 1200px;
  margin: 0 auto;
}

.page-header {
  margin-bottom: 24px;
}

.page-title {
  font-size: 28px;
  font-weight: 700;
  color: white;
  margin: 0 0 8px 0;
}

.page-subtitle {
  color: rgba(255, 255, 255, 0.6);
  margin: 0;
  font-size: 14px;
}

/* Stats Row */
.stats-row {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
  margin-bottom: 24px;
}

.stat-card {
  background: rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border-radius: 16px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  padding: 20px;
  text-align: center;
}

.stat-value {
  display: block;
  font-size: 32px;
  font-weight: 700;
  color: white;
}

.stat-label {
  font-size: 13px;
  color: rgba(255, 255, 255, 0.6);
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

/* Tabs */
.tabs-header {
  display: flex;
  gap: 4px;
  padding: 16px 20px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.tab-btn {
  background: transparent;
  border: none;
  padding: 8px 16px;
  color: rgba(255, 255, 255, 0.6);
  font-size: 14px;
  cursor: pointer;
  border-radius: 8px;
  transition: all 0.2s;
}

.tab-btn:hover {
  color: white;
  background: rgba(255, 255, 255, 0.1);
}

.tab-btn.active {
  color: white;
  background: linear-gradient(135deg, #60a5fa 0%, #a78bfa 100%);
}

.tab-content {
  padding: 20px;
}

.loading-state, .empty-state {
  text-align: center;
  padding: 48px;
  color: rgba(255, 255, 255, 0.5);
}

/* Plugins Grid */
.plugins-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}

.plugins-grid.wide {
  grid-template-columns: repeat(2, 1fr);
}

@media (max-width: 900px) {
  .plugins-grid { grid-template-columns: repeat(2, 1fr); }
  .plugins-grid.wide { grid-template-columns: 1fr; }
  .stats-row { grid-template-columns: 1fr; }
}

@media (max-width: 600px) {
  .plugins-grid { grid-template-columns: 1fr; }
}

/* Plugin Card */
.plugin-card {
  background: rgba(255, 255, 255, 0.05);
  border-radius: 12px;
  padding: 16px;
  border: 1px solid rgba(255, 255, 255, 0.08);
}

.plugin-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}

.plugin-icon {
  width: 36px;
  height: 36px;
  border-radius: 8px;
  background: rgba(167, 139, 250, 0.2);
  display: flex;
  align-items: center;
  justify-content: center;
}

.plugin-icon.store {
  background: rgba(96, 165, 250, 0.2);
}

.plugin-icon.js {
  background: rgba(251, 191, 36, 0.2);
}

.plugin-name {
  flex: 1;
  font-weight: 600;
  color: white;
  font-size: 14px;
}

.plugin-tags {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}

.plugin-desc {
  margin: 0;
  color: rgba(255, 255, 255, 0.6);
  font-size: 13px;
  line-height: 1.5;
}

.plugin-author {
  margin: 8px 0 0 0;
  color: rgba(255, 255, 255, 0.4);
  font-size: 12px;
}

.plugin-config-preview {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}

.config-label {
  display: block;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.5);
  margin-bottom: 6px;
}

.plugin-actions {
  display: flex;
  gap: 8px;
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}

/* Glass Button */
.glass-btn {
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 6px;
  padding: 6px 12px;
  color: white;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  gap: 4px;
}

.glass-btn:hover {
  background: rgba(255, 255, 255, 0.2);
}

.glass-btn.primary {
  background: linear-gradient(135deg, #60a5fa 0%, #a78bfa 100%);
  border: none;
}

.glass-btn.tiny {
  padding: 4px 10px;
  font-size: 11px;
}
</style>
