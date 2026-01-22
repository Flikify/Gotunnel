<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { RouterView, useRouter, useRoute } from 'vue-router'
import {
  HomeOutline, DesktopOutline, SettingsOutline,
  PersonCircleOutline, LogOutOutline, LogoGithub, ServerOutline, CheckmarkCircleOutline, ArrowUpCircleOutline
} from '@vicons/ionicons5'
import { getServerStatus, getVersionInfo, checkServerUpdate, removeToken, getToken, type UpdateInfo } from './api'

const router = useRouter()
const route = useRoute()
const serverInfo = ref({ bind_addr: '', bind_port: 0 })
const clientCount = ref(0)
const version = ref('')
const showUserMenu = ref(false)
const updateInfo = ref<UpdateInfo | null>(null)

const isLoginPage = computed(() => route.path === '/login')

const navItems = [
  { key: 'home', label: '首页', icon: HomeOutline, path: '/' },
  { key: 'clients', label: '客户端', icon: DesktopOutline, path: '/clients' },
  { key: 'settings', label: '设置', icon: SettingsOutline, path: '/settings' }
]

const activeNav = computed(() => {
  const path = route.path
  if (path === '/' || path === '/home') return 'home'
  if (path === '/clients' || path.startsWith('/client/')) return 'clients'
  if (path === '/settings') return 'settings'
  return 'home'
})

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

const checkUpdate = async () => {
  if (isLoginPage.value || !getToken()) return
  try {
    const { data } = await checkServerUpdate()
    updateInfo.value = data
  } catch (e) {
    console.error('Failed to check update', e)
  }
}

watch(() => route.path, (newPath, oldPath) => {
  if (oldPath === '/login' && newPath !== '/login') {
    fetchServerStatus()
    fetchVersion()
    checkUpdate()
  }
})

onMounted(() => {
  fetchServerStatus()
  fetchVersion()
  checkUpdate()
})

const logout = () => {
  removeToken()
  router.push('/login')
}

const toggleUserMenu = () => {
  showUserMenu.value = !showUserMenu.value
}
</script>

<template>
  <div v-if="!isLoginPage" class="app-layout">
    <!-- Header -->
    <header class="app-header">
      <div class="header-left">
        <span class="logo">GoTunnel</span>
      </div>
      <nav class="header-nav">
        <router-link
          v-for="item in navItems"
          :key="item.key"
          :to="item.path"
          class="nav-item"
          :class="{ active: activeNav === item.key }"
        >
          <component :is="item.icon" class="nav-icon" />
          <span>{{ item.label }}</span>
        </router-link>
      </nav>
      <div class="header-right">
        <div class="user-menu" @click="toggleUserMenu">
          <PersonCircleOutline class="user-icon" />
          <div v-if="showUserMenu" class="user-dropdown" @click.stop>
            <button class="dropdown-item" @click="logout">
              <LogOutOutline class="dropdown-icon" />
              <span>退出登录</span>
            </button>
          </div>
        </div>
      </div>
    </header>

    <!-- Main Content -->
    <main class="main-content">
      <RouterView />
    </main>

    <!-- Footer -->
    <footer class="app-footer">
      <div class="footer-left">
        <span class="brand">GoTunnel</span>
        <div v-if="version" class="version-info">
          <ServerOutline class="version-icon" />
          <span class="version">v{{ version }}</span>
          <span v-if="updateInfo" class="update-status" :class="{ latest: !updateInfo.available, 'has-update': updateInfo.available }">
            <template v-if="updateInfo.available">
              <ArrowUpCircleOutline class="status-icon" />
              <span>新版本 ({{ updateInfo.latest }})</span>
            </template>
            <template v-else>
              <CheckmarkCircleOutline class="status-icon" />
              <span>最新版本</span>
            </template>
          </span>
        </div>
      </div>
      <a href="https://github.com/user/gotunnel" target="_blank" class="footer-link">
        <LogoGithub class="footer-icon" />
        <span>GitHub</span>
      </a>
      <span class="copyright">© 2024 Flik. MIT License</span>
    </footer>
  </div>
  <RouterView v-else />
</template>

<style scoped>
.app-layout {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: linear-gradient(135deg, #1e1b4b 0%, #312e81 30%, #4c1d95 60%, #581c87 100%);
}

/* Header */
.app-header {
  height: 60px;
  background: rgba(15, 12, 41, 0.9);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  position: sticky;
  top: 0;
  z-index: 100;
}

.logo {
  font-size: 20px;
  font-weight: 700;
  color: white;
}

/* Navigation */
.header-nav {
  display: flex;
  gap: 4px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  color: rgba(255, 255, 255, 0.6);
  text-decoration: none;
  border-radius: 8px;
  font-size: 14px;
  transition: all 0.2s;
}

.nav-item:hover {
  color: white;
  background: rgba(255, 255, 255, 0.1);
}

.nav-item.active {
  color: white;
  background: linear-gradient(135deg, #60a5fa 0%, #a78bfa 100%);
}

.nav-icon {
  width: 18px;
  height: 18px;
}

/* User Menu */
.user-menu {
  position: relative;
  cursor: pointer;
}

.user-icon {
  width: 28px;
  height: 28px;
  color: rgba(255, 255, 255, 0.8);
  transition: color 0.2s;
}

.user-icon:hover {
  color: white;
}

.user-dropdown {
  position: absolute;
  top: 100%;
  right: 0;
  margin-top: 8px;
  background: rgba(30, 27, 75, 0.95);
  backdrop-filter: blur(20px);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  padding: 4px;
  min-width: 140px;
}

.dropdown-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 8px 12px;
  background: none;
  border: none;
  color: rgba(255, 255, 255, 0.8);
  font-size: 13px;
  cursor: pointer;
  border-radius: 6px;
  transition: all 0.2s;
}

.dropdown-item:hover {
  background: rgba(255, 255, 255, 0.1);
  color: white;
}

.dropdown-icon {
  width: 16px;
  height: 16px;
}

.main-content {
  flex: 1;
  overflow-y: auto;
}

/* Footer */
.app-footer {
  height: 48px;
  background: rgba(15, 12, 41, 0.9);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border-top: 1px solid rgba(255, 255, 255, 0.1);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  font-size: 13px;
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

.version-info {
  display: flex;
  align-items: center;
  gap: 6px;
}

.version-icon {
  width: 14px;
  height: 14px;
  color: rgba(255, 255, 255, 0.5);
}

.update-status {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  padding: 2px 8px;
  border-radius: 4px;
}

.update-status.latest {
  color: #34d399;
  background: rgba(52, 211, 153, 0.15);
}

.update-status.has-update {
  color: #fbbf24;
  background: rgba(251, 191, 36, 0.15);
}

.status-icon {
  width: 14px;
  height: 14px;
}

.footer-link {
  display: flex;
  align-items: center;
  gap: 6px;
  color: rgba(255, 255, 255, 0.6);
  text-decoration: none;
  transition: color 0.2s;
}

.footer-link:hover {
  color: white;
}

.footer-icon {
  width: 16px;
  height: 16px;
}

.copyright {
  color: rgba(255, 255, 255, 0.4);
}

/* Responsive */
@media (max-width: 768px) {
  .app-header {
    padding: 0 12px;
  }
  .header-nav {
    display: none;
  }
  .app-footer {
    padding: 0 12px;
  }
  .copyright {
    display: none;
  }
}
</style>
