<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { RouterView, useRouter, useRoute } from 'vue-router'
import {
  HomeOutline, DesktopOutline, SettingsOutline,
  PersonCircleOutline, LogOutOutline, LogoGithub, ServerOutline, CheckmarkCircleOutline, ArrowUpCircleOutline, CloseOutline,
  SunnyOutline, MoonOutline, ContrastOutline
} from '@vicons/ionicons5'
import { getServerStatus, getVersionInfo, checkServerUpdate, applyServerUpdate, removeToken, getToken, type UpdateInfo } from './api'
import { useToast } from './composables/useToast'
import { useConfirm } from './composables/useConfirm'
import { useTheme, type ThemeMode } from './composables/useTheme'

const router = useRouter()
const route = useRoute()
const message = useToast()
const dialog = useConfirm()
const { themeMode, setTheme } = useTheme()
const showThemeMenu = ref(false)
const serverInfo = ref({ bind_addr: '', bind_port: 0 })
const clientCount = ref(0)
const version = ref('')
const showUserMenu = ref(false)
const updateInfo = ref<UpdateInfo | null>(null)
const showUpdateModal = ref(false)
const updatingServer = ref(false)

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

const toggleThemeMenu = () => {
  showThemeMenu.value = !showThemeMenu.value
}

const selectTheme = (mode: ThemeMode) => {
  setTheme(mode)
  showThemeMenu.value = false
}

const themeIcon = computed(() => {
  if (themeMode.value === 'light') return SunnyOutline
  if (themeMode.value === 'dark') return MoonOutline
  return ContrastOutline
})

const openUpdateModal = () => {
  if (updateInfo.value && updateInfo.value.available) {
    showUpdateModal.value = true
  }
}

const formatBytes = (bytes: number): string => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

const handleApplyServerUpdate = () => {
  if (!updateInfo.value?.download_url) {
    message.error('没有可用的下载链接')
    return
  }

  dialog.warning({
    title: '确认更新服务端',
    content: `即将更新服务端到 ${updateInfo.value.latest}，更新后服务器将自动重启。确定要继续吗？`,
    positiveText: '更新并重启',
    negativeText: '取消',
    onPositiveClick: async () => {
      updatingServer.value = true
      try {
        await applyServerUpdate(updateInfo.value!.download_url)
        message.success('更新已开始，服务器将在几秒后重启')
        showUpdateModal.value = false
        setTimeout(() => {
          window.location.reload()
        }, 5000)
      } catch (e: any) {
        message.error(e.response?.data || '更新失败')
        updatingServer.value = false
      }
    }
  })
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
        <!-- Theme Switcher -->
        <div class="theme-menu" @click="toggleThemeMenu">
          <component :is="themeIcon" class="theme-icon" />
          <div v-if="showThemeMenu" class="theme-dropdown" @click.stop>
            <button class="dropdown-item" :class="{ active: themeMode === 'light' }" @click="selectTheme('light')">
              <SunnyOutline class="dropdown-icon" />
              <span>浅色</span>
            </button>
            <button class="dropdown-item" :class="{ active: themeMode === 'dark' }" @click="selectTheme('dark')">
              <MoonOutline class="dropdown-icon" />
              <span>深色</span>
            </button>
            <button class="dropdown-item" :class="{ active: themeMode === 'auto' }" @click="selectTheme('auto')">
              <ContrastOutline class="dropdown-icon" />
              <span>自动</span>
            </button>
          </div>
        </div>
        <!-- User Menu -->
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
          <span class="version">{{ version.startsWith('v') ? version : 'v' + version }}</span>
          <span v-if="updateInfo" class="update-status" :class="{ latest: !updateInfo.available, 'has-update': updateInfo.available }" @click="openUpdateModal">
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

    <!-- Update Modal -->
    <div v-if="showUpdateModal" class="modal-overlay" @click.self="showUpdateModal = false">
      <div class="update-modal">
        <div class="modal-header">
          <h3>系统更新</h3>
          <button class="close-btn" @click="showUpdateModal = false">
            <CloseOutline />
          </button>
        </div>
        <div class="modal-body" v-if="updateInfo">
          <div class="update-info-grid">
            <div class="info-row">
              <span class="info-label">当前版本</span>
              <span class="info-value">{{ updateInfo.current }}</span>
            </div>
            <div class="info-row">
              <span class="info-label">最新版本</span>
              <span class="info-value highlight">{{ updateInfo.latest }}</span>
            </div>
            <div v-if="updateInfo.asset_name" class="info-row">
              <span class="info-label">文件名</span>
              <span class="info-value">{{ updateInfo.asset_name }}</span>
            </div>
            <div v-if="updateInfo.asset_size" class="info-row">
              <span class="info-label">文件大小</span>
              <span class="info-value">{{ formatBytes(updateInfo.asset_size) }}</span>
            </div>
          </div>
          <div v-if="updateInfo.release_note" class="release-note">
            <span class="note-label">更新日志</span>
            <pre>{{ updateInfo.release_note }}</pre>
          </div>
        </div>
        <div class="modal-footer">
          <button class="modal-btn" @click="showUpdateModal = false">取消</button>
          <button
            v-if="updateInfo?.available && updateInfo?.download_url"
            class="modal-btn primary"
            :disabled="updatingServer"
            @click="handleApplyServerUpdate"
          >
            {{ updatingServer ? '更新中...' : '立即更新' }}
          </button>
        </div>
      </div>
    </div>
  </div>
  <RouterView v-else />
</template>

<style scoped>
.app-layout {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--color-bg-primary);
}

/* Header */
.app-header {
  height: 56px;
  background: var(--color-bg-secondary);
  border-bottom: 1px solid var(--color-border);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  position: sticky;
  top: 0;
  z-index: 100;
}

.logo {
  font-size: 18px;
  font-weight: 600;
  color: var(--color-text-primary);
  letter-spacing: -0.5px;
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
  color: var(--color-text-secondary);
  text-decoration: none;
  border-radius: 6px;
  font-size: 14px;
  transition: all 0.15s;
}

.nav-item:hover {
  color: var(--color-text-primary);
  background: rgba(255, 255, 255, 0.06);
}

.nav-item.active {
  color: var(--color-text-primary);
  background: var(--color-accent);
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
  color: var(--color-text-secondary);
  transition: color 0.15s;
}

.user-icon:hover {
  color: var(--color-text-primary);
}

/* Theme Menu */
.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.theme-menu {
  position: relative;
  cursor: pointer;
}

.theme-icon {
  width: 24px;
  height: 24px;
  color: var(--color-text-secondary);
  transition: color 0.15s;
}

.theme-icon:hover {
  color: var(--color-text-primary);
}

.theme-dropdown {
  position: absolute;
  top: 100%;
  right: 0;
  margin-top: 8px;
  background: var(--color-bg-tertiary);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 4px;
  min-width: 120px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
}

.dropdown-item.active {
  background: var(--color-accent);
  color: white;
}

.dropdown-item.active:hover {
  background: var(--color-accent-hover);
}

.user-dropdown {
  position: absolute;
  top: 100%;
  right: 0;
  margin-top: 8px;
  background: var(--color-bg-tertiary);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 4px;
  min-width: 140px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
}

.dropdown-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 10px 12px;
  background: none;
  border: none;
  color: var(--color-text-primary);
  font-size: 14px;
  cursor: pointer;
  border-radius: 6px;
  transition: all 0.15s;
}

.dropdown-item:hover {
  background: var(--color-bg-elevated);
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
  background: var(--color-bg-secondary);
  border-top: 1px solid var(--color-border);
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
  color: var(--color-text-primary);
}

.version {
  padding: 2px 8px;
  background: var(--color-bg-elevated);
  color: var(--color-text-secondary);
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
  color: var(--color-text-secondary);
}

.update-status {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  padding: 2px 8px;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.15s;
}

.update-status:hover {
  opacity: 0.8;
}

.update-status.latest {
  color: #00ba7c;
  background: rgba(0, 186, 124, 0.1);
}

.update-status.has-update {
  color: #f7931a;
  background: rgba(247, 147, 26, 0.1);
}

.status-icon {
  width: 14px;
  height: 14px;
}

.footer-link {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--color-text-secondary);
  text-decoration: none;
  transition: color 0.15s;
}

.footer-link:hover {
  color: var(--color-text-primary);
}

.footer-icon {
  width: 16px;
  height: 16px;
}

.copyright {
  color: var(--color-text-muted);
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

/* Update Modal */
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.update-modal {
  background: var(--color-bg-tertiary);
  border: 1px solid var(--color-border);
  border-radius: 12px;
  width: 90%;
  max-width: 480px;
  max-height: 80vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5);
}

.modal-header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--color-border);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.modal-header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.close-btn {
  background: none;
  border: none;
  color: var(--color-text-secondary);
  cursor: pointer;
  padding: 4px;
  display: flex;
  transition: color 0.15s;
}

.close-btn:hover {
  color: var(--color-text-primary);
}

.modal-body {
  padding: 20px;
  overflow-y: auto;
  flex: 1;
}

.update-info-grid {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 16px;
}

.info-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.info-label {
  color: var(--color-text-secondary);
  font-size: 13px;
}

.info-value {
  color: var(--color-text-primary);
  font-size: 13px;
  font-weight: 500;
}

.info-value.highlight {
  color: var(--color-success);
}

.release-note {
  margin-top: 16px;
}

.note-label {
  display: block;
  font-size: 12px;
  color: var(--color-text-secondary);
  margin-bottom: 8px;
}

.release-note pre {
  margin: 0;
  white-space: pre-wrap;
  font-size: 12px;
  color: var(--color-text-secondary);
  background: var(--color-bg-elevated);
  padding: 12px;
  border-radius: 8px;
  max-height: 200px;
  overflow-y: auto;
}

.modal-footer {
  padding: 16px 20px;
  border-top: 1px solid var(--color-border);
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.modal-btn {
  padding: 8px 16px;
  border-radius: 6px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
  background: var(--color-bg-elevated);
  border: 1px solid var(--color-border);
  color: var(--color-text-primary);
}

.modal-btn:hover:not(:disabled) {
  background: var(--color-border);
}

.modal-btn.primary {
  background: var(--color-accent);
  border: none;
  color: white;
}

.modal-btn.primary:hover:not(:disabled) {
  background: var(--color-accent-hover);
}

.modal-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
