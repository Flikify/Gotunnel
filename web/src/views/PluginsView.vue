<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { ExtensionPuzzleOutline, CodeSlashOutline, SettingsOutline } from '@vicons/ionicons5'
import GlassModal from '../components/GlassModal.vue'
import GlassTag from '../components/GlassTag.vue'
import GlassSwitch from '../components/GlassSwitch.vue'
import { useToast } from '../composables/useToast'
import {
  getPlugins, enablePlugin, disablePlugin, getJSPlugins,
  pushJSPluginToClient, getClients, updateJSPluginConfig, setJSPluginEnabled
} from '../api'
import type { PluginInfo, JSPlugin, ClientStatus } from '../types'

const message = useToast()
const plugins = ref<PluginInfo[]>([])
const jsPlugins = ref<JSPlugin[]>([])
const clients = ref<ClientStatus[]>([])
const loading = ref(true)
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
        <p class="page-subtitle">管理已安装插件和 JS 插件</p>
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
                  <ExtensionPuzzleOutline class="icon-purple" />
                </div>
                <span class="plugin-name">{{ plugin.name }}</span>
                <GlassSwitch :model-value="plugin.enabled" size="small" @update:model-value="togglePlugin(plugin)" />
              </div>
              <div class="plugin-tags">
                <GlassTag>v{{ plugin.version }}</GlassTag>
                <GlassTag :type="plugin.type === 'proxy' ? 'info' : 'success'">{{ getTypeLabel(plugin.type) }}</GlassTag>
                <GlassTag :type="plugin.source === 'builtin' ? 'default' : 'warning'">
                  {{ plugin.source === 'builtin' ? '内置' : 'JS' }}
                </GlassTag>
              </div>
              <p class="plugin-desc">{{ plugin.description }}</p>
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
                  <CodeSlashOutline class="icon-yellow" />
                </div>
                <span class="plugin-name">{{ plugin.name }}</span>
                <GlassTag v-if="plugin.version">v{{ plugin.version }}</GlassTag>
                <GlassSwitch :model-value="plugin.enabled" size="small" @update:model-value="toggleJSPlugin(plugin)" />
              </div>
              <div class="plugin-tags">
                <GlassTag type="warning">JS</GlassTag>
                <GlassTag v-if="plugin.auto_start" type="success">自动启动</GlassTag>
                <GlassTag v-if="plugin.signature" type="info">已签名</GlassTag>
              </div>
              <p class="plugin-desc">{{ plugin.description || '无描述' }}</p>
              <p v-if="plugin.author" class="plugin-author">作者: {{ plugin.author }}</p>
              <div v-if="Object.keys(plugin.config || {}).length > 0" class="plugin-config-preview">
                <span class="config-label">配置:</span>
                <div class="config-tags">
                  <GlassTag v-for="(value, key) in plugin.config" :key="key">
                    {{ key }}: {{ String(value).length > 10 ? String(value).slice(0, 10) + '...' : value }}
                  </GlassTag>
                </div>
              </div>
              <div class="plugin-actions">
                <button class="glass-btn tiny" @click="openJSConfigModal(plugin)">
                  <SettingsOutline class="btn-icon" />
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

    <!-- JS Config Modal -->
    <GlassModal :show="showJSConfigModal" :title="`${currentJSPlugin?.name || ''} 配置`" @close="showJSConfigModal = false">
      <p class="config-hint">编辑插件配置参数</p>
      <div v-for="(item, index) in jsConfigItems" :key="index" class="config-row">
        <input v-model="item.key" class="form-input config-key" placeholder="参数名" />
        <input v-model="item.value" class="form-input config-value" placeholder="参数值" />
        <button v-if="jsConfigItems.length > 1" class="icon-btn danger" @click="removeJSConfigItem(index)">删除</button>
      </div>
      <button class="glass-btn small dashed" @click="addJSConfigItem">添加配置项</button>
      <template #footer>
        <button class="glass-btn" @click="showJSConfigModal = false">取消</button>
        <button class="glass-btn primary" :disabled="jsConfigSaving" @click="saveJSPluginConfig">
          {{ jsConfigSaving ? '保存中...' : '保存' }}
        </button>
      </template>
    </GlassModal>

    <!-- Push Modal -->
    <GlassModal :show="showPushModal" title="推送插件到客户端" width="400px" @close="showPushModal = false">
      <div v-if="selectedJSPlugin" class="plugin-info-box">
        <p class="plugin-info-name">插件: {{ selectedJSPlugin.name }}</p>
        <p class="plugin-info-desc">{{ selectedJSPlugin.description || '无描述' }}</p>
      </div>
      <div class="form-group">
        <label class="form-label">选择客户端</label>
        <select v-model="pushClientId" class="form-select">
          <option value="" disabled>选择客户端</option>
          <option v-for="c in onlineClients" :key="c.id" :value="c.id">{{ c.nickname || c.id }}</option>
        </select>
      </div>
      <div class="form-group">
        <label class="form-label">远程端口</label>
        <input v-model.number="pushRemotePort" type="number" class="form-input" min="1" max="65535" />
      </div>
      <template #footer>
        <button class="glass-btn" @click="showPushModal = false">取消</button>
        <button class="glass-btn primary" :disabled="!pushClientId || pushing" @click="handlePushJSPlugin">
          {{ pushing ? '推送中...' : '推送' }}
        </button>
      </template>
    </GlassModal>
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

.glass-btn.small {
  padding: 6px 12px;
  font-size: 12px;
}

.glass-btn.dashed {
  border-style: dashed;
}

.glass-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
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

/* Plugin Info Box */
.plugin-info-box {
  background: rgba(255, 255, 255, 0.05);
  border-radius: 8px;
  padding: 12px;
  margin-bottom: 16px;
}

.plugin-info-name {
  margin: 0 0 4px 0;
  color: white;
  font-weight: 500;
}

.plugin-info-desc {
  margin: 0;
  color: rgba(255, 255, 255, 0.6);
  font-size: 13px;
}

/* Config Row */
.config-hint {
  margin: 0 0 12px 0;
  color: rgba(255, 255, 255, 0.5);
  font-size: 13px;
}

.config-row {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-bottom: 8px;
}

.config-key {
  width: 150px;
  flex-shrink: 0;
}

.config-value {
  flex: 1;
}

.icon-btn {
  background: rgba(255, 255, 255, 0.1);
  border: none;
  border-radius: 6px;
  padding: 6px 10px;
  color: rgba(255, 255, 255, 0.7);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
}

.icon-btn:hover {
  background: rgba(255, 255, 255, 0.2);
  color: white;
}

.icon-btn.danger {
  color: #fca5a5;
}

.icon-btn.danger:hover {
  background: rgba(239, 68, 68, 0.2);
}

/* Icon styles */
.icon-purple {
  width: 20px;
  height: 20px;
  color: #a78bfa;
}

.icon-blue {
  width: 20px;
  height: 20px;
  color: #60a5fa;
}

.icon-yellow {
  width: 20px;
  height: 20px;
  color: #fbbf24;
}

.btn-icon {
  width: 14px;
  height: 14px;
}

.config-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
</style>
