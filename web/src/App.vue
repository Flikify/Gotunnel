<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { RouterView, useRouter, useRoute } from 'vue-router'
import {
  NLayout, NLayoutHeader, NLayoutContent, NLayoutFooter,
  NButton, NIcon, NConfigProvider, NMessageProvider,
  NDialogProvider, NGlobalStyle, NDropdown, NTabs, NTabPane,
  type GlobalThemeOverrides
} from 'naive-ui'
import {
  PersonCircleOutline, LogoGithub
} from '@vicons/ionicons5'
import { getServerStatus, getVersionInfo, removeToken, getToken } from './api'

const router = useRouter()
const route = useRoute()
const serverInfo = ref({ bind_addr: '', bind_port: 0 })
const clientCount = ref(0)
const version = ref('')

const isLoginPage = computed(() => route.path === '/login')

// 当前激活的 Tab
const activeTab = computed(() => {
  const path = route.path
  if (path === '/' || path === '/home') return 'home'
  if (path.startsWith('/client')) return 'clients'
  if (path === '/plugins') return 'plugins'
  if (path === '/settings') return 'settings'
  return 'home'
})

const handleTabChange = (tab: string) => {
  if (tab === 'home') router.push('/')
  else if (tab === 'clients') router.push('/')
  else if (tab === 'plugins') router.push('/plugins')
  else if (tab === 'settings') router.push('/settings')
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

const fetchVersion = async () => {
  if (isLoginPage.value || !getToken()) return
  try {
    const { data } = await getVersionInfo()
    version.value = data.version || ''
  } catch (e) {
    console.error('Failed to get version', e)
  }
}

watch(() => route.path, (newPath, oldPath) => {
  if (oldPath === '/login' && newPath !== '/login') {
    fetchServerStatus()
    fetchVersion()
  }
})

onMounted(() => {
  fetchServerStatus()
  fetchVersion()
})

const logout = () => {
  removeToken()
  router.push('/login')
}

const handleUserAction = (key: string) => {
  if (key === 'logout') logout()
}

// 紫色渐变主题
const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#6366f1',
    primaryColorHover: '#818cf8',
    primaryColorPressed: '#4f46e5',
  },
  Layout: {
    headerColor: '#ffffff'
  },
  Tabs: {
    tabTextColorActiveLine: '#6366f1',
    barColor: '#6366f1'
  }
}
</script>

<template>
  <n-config-provider :theme-overrides="themeOverrides">
    <n-global-style />
    <n-dialog-provider>
      <n-message-provider>
        <n-layout v-if="!isLoginPage" class="main-layout" position="absolute">
          <!-- 顶部导航栏 -->
          <n-layout-header bordered class="header">
            <div class="header-content">
              <div class="header-left">
                <div class="logo">
                  <span class="logo-text">GoTunnel</span>
                </div>
                <n-tabs
                  type="line"
                  :value="activeTab"
                  @update:value="handleTabChange"
                  class="nav-tabs"
                >
                  <n-tab-pane name="home" tab="首页" />
                  <n-tab-pane name="clients" tab="客户端管理" />
                  <n-tab-pane name="plugins" tab="插件商店" />
                  <n-tab-pane name="settings" tab="系统设置" />
                </n-tabs>
              </div>
              <div class="header-right">
                <n-dropdown
                  :options="[{ label: '退出登录', key: 'logout' }]"
                  @select="handleUserAction"
                >
                  <n-button quaternary circle size="large">
                    <template #icon>
                      <n-icon size="24"><PersonCircleOutline /></n-icon>
                    </template>
                  </n-button>
                </n-dropdown>
              </div>
            </div>
          </n-layout-header>

          <!-- 主内容区 -->
          <n-layout-content class="main-content">
            <RouterView />
          </n-layout-content>

          <!-- 底部页脚 -->
          <n-layout-footer bordered class="footer">
            <div class="footer-content">
              <div class="footer-left">
                <span class="brand">GoTunnel</span>
                <span class="version" v-if="version">v{{ version }}</span>
              </div>
              <div class="footer-center">
                <a href="https://github.com/user/gotunnel" target="_blank" class="footer-link">
                  <n-icon size="16"><LogoGithub /></n-icon>
                  <span>GitHub</span>
                </a>
              </div>
              <div class="footer-right">
                <span>© 2024 Flik. MIT License</span>
              </div>
            </div>
          </n-layout-footer>
        </n-layout>
        <RouterView v-else />
      </n-message-provider>
    </n-dialog-provider>
  </n-config-provider>
</template>

<style scoped>
.main-layout {
  height: 100vh;
  display: flex;
  flex-direction: column;
}

.header {
  height: 60px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  display: flex;
  align-items: center;
  padding: 0 24px;
  box-shadow: 0 2px 8px rgba(102, 126, 234, 0.15);
}

.header-content {
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 32px;
}

.logo {
  display: flex;
  align-items: center;
}

.logo-text {
  font-size: 20px;
  font-weight: 700;
  color: #ffffff;
}

.nav-tabs :deep(.n-tabs-tab) {
  color: rgba(255, 255, 255, 0.8);
  font-weight: 500;
}

.nav-tabs :deep(.n-tabs-tab--active) {
  color: #ffffff !important;
}

.nav-tabs :deep(.n-tabs-bar) {
  background-color: #ffffff !important;
}

.header-right :deep(.n-button) {
  color: rgba(255, 255, 255, 0.9);
}

.main-content {
  flex: 1;
  padding: 0;
  background-color: transparent;
  overflow-y: auto;
}

.footer {
  height: 48px;
  background: rgba(255, 255, 255, 0.05);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border-top: 1px solid rgba(255, 255, 255, 0.1);
}

.footer-content {
  height: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 24px;
  font-size: 13px;
  color: rgba(255, 255, 255, 0.6);
}

.footer-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.brand {
  font-weight: 600;
  color: rgba(255, 255, 255, 0.9);
}

.version {
  padding: 2px 8px;
  background: rgba(255, 255, 255, 0.1);
  color: rgba(255, 255, 255, 0.7);
  border-radius: 4px;
  font-size: 12px;
}

.footer-center {
  display: flex;
  gap: 16px;
}

.footer-link {
  display: flex;
  align-items: center;
  gap: 4px;
  color: rgba(255, 255, 255, 0.6);
  text-decoration: none;
  transition: color 0.2s;
}

.footer-link:hover {
  color: rgba(255, 255, 255, 0.9);
}

.footer-right {
  color: rgba(255, 255, 255, 0.4);
}

@media (max-width: 768px) {
  .header {
    padding: 0 12px;
  }
  .header-left {
    gap: 16px;
  }
  .logo-text {
    font-size: 16px;
  }
  .footer-content {
    padding: 0 12px;
    font-size: 12px;
  }
}
</style>
