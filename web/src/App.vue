<script setup lang="ts">
import { ref, onMounted, computed, h, watch } from 'vue'
import { RouterView, useRouter, useRoute } from 'vue-router'
import { NLayout, NLayoutHeader, NLayoutContent, NMenu, NButton, NSpace, NTag, NIcon, NConfigProvider, NMessageProvider, NDialogProvider } from 'naive-ui'
import { HomeOutline, ExtensionPuzzleOutline, LogOutOutline } from '@vicons/ionicons5'
import type { MenuOption } from 'naive-ui'
import { getServerStatus, removeToken, getToken } from './api'

const router = useRouter()
const route = useRoute()
const serverInfo = ref({ bind_addr: '', bind_port: 0 })
const clientCount = ref(0)

const isLoginPage = computed(() => route.path === '/login')

const menuOptions: MenuOption[] = [
  {
    label: '客户端',
    key: '/',
    icon: () => h(NIcon, null, { default: () => h(HomeOutline) })
  },
  {
    label: '插件',
    key: '/plugins',
    icon: () => h(NIcon, null, { default: () => h(ExtensionPuzzleOutline) })
  }
]

const activeKey = computed(() => {
  if (route.path.startsWith('/client/')) return '/'
  return route.path
})

const handleMenuUpdate = (key: string) => {
  router.push(key)
}

const fetchServerStatus = async () => {
  if (isLoginPage.value || !getToken()) return
  try {
    const { data } = await getServerStatus()
    serverInfo.value = data.server
    clientCount.value = data.client_count
  } catch (e) {
    console.error('Failed to get server status', e)
  }
}

// 监听路由变化，离开登录页时获取状态
watch(() => route.path, (newPath, oldPath) => {
  if (oldPath === '/login' && newPath !== '/login') {
    fetchServerStatus()
  }
})

onMounted(() => {
  fetchServerStatus()
})

const logout = () => {
  removeToken()
  router.push('/login')
}
</script>

<template>
  <n-config-provider>
    <n-dialog-provider>
      <n-message-provider>
        <n-layout v-if="!isLoginPage" style="min-height: 100vh;">
        <n-layout-header bordered style="height: 64px; padding: 0 24px; display: flex; align-items: center; justify-content: space-between;">
          <div style="display: flex; align-items: center; gap: 32px;">
            <div style="font-size: 20px; font-weight: 600; color: #18a058; cursor: pointer;" @click="router.push('/')">
              GoTunnel
            </div>
            <n-menu
              mode="horizontal"
              :options="menuOptions"
              :value="activeKey"
              @update:value="handleMenuUpdate"
            />
          </div>
          <n-space align="center" :size="16">
            <n-tag type="info" round>
              {{ serverInfo.bind_addr }}:{{ serverInfo.bind_port }}
            </n-tag>
            <n-tag type="success" round>
              {{ clientCount }} 客户端
            </n-tag>
            <n-button quaternary circle @click="logout">
              <template #icon>
                <n-icon><LogOutOutline /></n-icon>
              </template>
            </n-button>
          </n-space>
        </n-layout-header>
        <n-layout-content content-style="padding: 24px; max-width: 1200px; margin: 0 auto; width: 100%;">
          <RouterView />
        </n-layout-content>
      </n-layout>
      <RouterView v-else />
    </n-message-provider>
    </n-dialog-provider>
  </n-config-provider>
</template>
