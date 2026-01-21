<script setup lang="ts">
import { ref, onMounted, computed, h, watch } from 'vue'
import { RouterView, useRouter, useRoute } from 'vue-router'
import {
  NLayout, NLayoutHeader, NLayoutContent, NLayoutSider, NMenu,
  NButton, NIcon, NConfigProvider, NMessageProvider,
  NDialogProvider, NGlobalStyle, NDropdown, NAvatar, type GlobalThemeOverrides
} from 'naive-ui'
import {
  HomeOutline, ExtensionPuzzleOutline, LogOutOutline,
  ServerOutline, MenuOutline, PersonCircleOutline
} from '@vicons/ionicons5'
import type { MenuOption } from 'naive-ui'
import { getServerStatus, removeToken, getToken } from './api'

const router = useRouter()
const route = useRoute()
const serverInfo = ref({ bind_addr: '', bind_port: 0 })
const clientCount = ref(0)
const collapsed = ref(false)

const isLoginPage = computed(() => route.path === '/login')

const menuOptions: MenuOption[] = [
  {
    label: 'Dashboard',
    key: '/',
    icon: () => h(NIcon, null, { default: () => h(HomeOutline) })
  },
  {
    label: 'Plugins Store',
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

// User dropdown menu options
const userDropdownOptions = [
  {
    label: '退出登录',
    key: 'logout',
    icon: () => h(NIcon, null, { default: () => h(LogOutOutline) })
  }
]

const handleUserDropdown = (key: string) => {
  if (key === 'logout') {
    logout()
  }
}

// Theme Overrides
const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#18a058',
    primaryColorHover: '#36ad6a',
    primaryColorPressed: '#0c7a43',
  },
  Layout: {
    siderColor: '#f7fcf9',
    headerColor: '#ffffff'
  }
}
</script>

<template>
  <n-config-provider :theme-overrides="themeOverrides">
    <n-global-style />
    <n-dialog-provider>
      <n-message-provider>
        <n-layout v-if="!isLoginPage" class="main-layout" has-sider position="absolute">
          <n-layout-sider
            bordered
            collapse-mode="width"
            :collapsed-width="64"
            :width="240"
            :collapsed="collapsed"
            show-trigger
            @collapse="collapsed = true"
            @expand="collapsed = false"
            style="background: #f9fafb;"
          >
            <div class="logo-container">
              <n-icon size="32" color="#18a058"><ServerOutline /></n-icon>
              <span v-if="!collapsed" class="logo-text">GoTunnel</span>
            </div>
            <n-menu
              :collapsed="collapsed"
              :collapsed-width="64"
              :collapsed-icon-size="22"
              :options="menuOptions"
              :value="activeKey"
              @update:value="handleMenuUpdate"
            />
            <div v-if="!collapsed" class="server-status-card">
               <div class="status-item">
                 <span class="label">Server:</span>
                 <span class="value">{{ serverInfo.bind_addr }}:{{ serverInfo.bind_port }}</span>
               </div>
               <div class="status-item">
                 <span class="label">Clients:</span>
                 <span class="value">{{ clientCount }}</span>
               </div>
            </div>
          </n-layout-sider>

          <n-layout>
            <n-layout-header bordered class="header">
              <div class="header-content">
                <n-button quaternary circle size="large" @click="collapsed = !collapsed" class="mobile-toggle">
                  <template #icon><n-icon><MenuOutline /></n-icon></template>
                </n-button>
                <div class="header-right">
                  <n-dropdown :options="userDropdownOptions" @select="handleUserDropdown">
                    <n-button quaternary circle size="large">
                      <template #icon>
                        <n-icon size="24"><PersonCircleOutline /></n-icon>
                      </template>
                    </n-button>
                  </n-dropdown>
                </div>
              </div>
            </n-layout-header>
            <n-layout-content content-style="padding: 24px; background-color: #f0f2f5; min-height: calc(100vh - 64px);">
              <RouterView />
            </n-layout-content>
          </n-layout>
        </n-layout>
        <RouterView v-else />
      </n-message-provider>
    </n-dialog-provider>
  </n-config-provider>
</template>

<style scoped>
.main-layout {
  height: 100vh;
}

.logo-container {
  height: 64px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  border-bottom: 1px solid #efeff5;
  overflow: hidden;
}

.logo-text {
  font-size: 20px;
  font-weight: 700;
  color: #18a058;
  white-space: nowrap;
}

.header {
  height: 64px;
  background: white;
  display: flex;
  align-items: center;
  padding: 0 24px;
}

.header-content {
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.server-status-card {
  position: absolute;
  bottom: 0;
  width: 100%;
  padding: 20px;
  background: #f0fdf4;
  border-top: 1px solid #d1fae5;
}

.status-item {
  display: flex;
  justify-content: space-between;
  margin-bottom: 8px;
  font-size: 12px;
}

.status-item .label {
  color: #64748b;
}

.status-item .value {
  font-weight: 600;
  color: #0f172a;
}

.mobile-toggle {
  display: none;
}

@media (max-width: 768px) {
  .mobile-toggle {
    display: inline-flex;
  }
}
</style>
