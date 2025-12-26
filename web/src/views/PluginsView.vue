<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import {
  NCard, NButton, NSpace, NTag, NStatistic, NGrid, NGi,
  NEmpty, NSpin, NIcon, NSwitch, useMessage
} from 'naive-ui'
import { ArrowBackOutline, ExtensionPuzzleOutline } from '@vicons/ionicons5'
import { getPlugins, enablePlugin, disablePlugin } from '../api'
import type { PluginInfo } from '../types'

const router = useRouter()
const message = useMessage()
const plugins = ref<PluginInfo[]>([])
const loading = ref(true)

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

onMounted(loadPlugins)
</script>

<template>
  <div class="plugins-view">
    <n-space justify="space-between" align="center" style="margin-bottom: 24px;">
      <div>
        <h2 style="margin: 0 0 8px 0;">插件管理</h2>
        <p style="margin: 0; color: #666;">查看和管理已注册的插件</p>
      </div>
      <n-button quaternary @click="router.push('/')">
        <template #icon><n-icon><ArrowBackOutline /></n-icon></template>
        返回首页
      </n-button>
    </n-space>

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

      <n-empty v-if="!loading && plugins.length === 0" description="暂无插件" />

      <n-grid v-else :cols="3" :x-gap="16" :y-gap="16" responsive="screen" cols-s="1" cols-m="2">
        <n-gi v-for="plugin in plugins" :key="plugin.name">
          <n-card hoverable>
            <template #header>
              <n-space align="center">
                <n-icon size="24" color="#18a058"><ExtensionPuzzleOutline /></n-icon>
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
  </div>
</template>
