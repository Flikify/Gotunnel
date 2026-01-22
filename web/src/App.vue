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
  background: var(--gradient-bg);
  position: relative;
}

/* Header - 毛玻璃效果 */
.app-header {
  height: 64px;
  background: var(--glass-bg);
  backdrop-filter: var(--glass-blur);
  -webkit-backdrop-filter: var(--glass-blur);
  border-bottom: 1px solid var(--color-border);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  position: sticky;
  top: 0;
  z-index: 100;
  box-shadow: 0 4px 30px rgba(0, 0, 0, 0.1);
}

/* 头部顶部高光线 */
.app-header::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 1px;
  background: linear-gradient(90deg,
    transparent 0%,
    rgba(255, 255, 255, 0.1) 50%,
    transparent 100%);
}

.logo {
  font-size: 20px;
  font-weight: 700;
  background: var(--gradient-accent);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  letter-spacing: -0.5px;
}

/* Navigation */
.header-nav {
  display: flex;
  gap: 6px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 18px;
  color: var(--color-text-secondary);
  text-decoration: none;
  border-radius: 10px;
  font-size: 14px;
  font-weight: 500;
  transition: all 0.2s ease;
  position: relative;
}

.nav-item:hover {
  color: var(--color-text-primary);
  background: var(--glass-bg-light);
}

.nav-item.active {
  color: white;
  background: var(--gradient-accent);
  box-shadow: 0 4px 15px var(--color-accent-glow);
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
  width: 32px;
  height: 32px;
  color: var(--color-text-secondary);
  transition: all 0.2s ease;
  padding: 4px;
  border-radius: 8px;
}

.user-icon:hover {
  color: var(--color-text-primary);
  background: var(--glass-bg-light);
}

/* Theme Menu */
.header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.theme-menu {
  position: relative;
  cursor: pointer;
}

.theme-icon {
  width: 28px;
  height: 28px;
  padding: 4px;
  color: var(--color-text-secondary);
  transition: all 0.2s ease;
  border-radius: 8px;
}

.theme-icon:hover {
  color: var(--color-text-primary);
  background: var(--glass-bg-light);
}

.theme-dropdown {
  position: absolute;
  top: 100%;
  right: 0;
  margin-top: 8px;
  background: var(--glass-bg);
  backdrop-filter: var(--glass-blur);
  -webkit-backdrop-filter: var(--glass-blur);
  border: 1px solid var(--color-border);
  border-radius: 12px;
  padding: 6px;
  min-width: 130px;
  box-shadow: var(--shadow-lg);
}

.dropdown-item.active {
  background: var(--gradient-accent);
  color: white;
  box-shadow: 0 2px 8px var(--color-accent-glow);
}

.dropdown-item.active:hover {
  filter: brightness(1.1);
}

.user-dropdown {
  position: absolute;
  top: 100%;
  right: 0;
  margin-top: 8px;
  background: var(--glass-bg);
  backdrop-filter: var(--glass-blur);
  -webkit-backdrop-filter: var(--glass-blur);
  border: 1px solid var(--color-border);
  border-radius: 12px;
  padding: 6px;
  min-width: 150px;
  box-shadow: var(--shadow-lg);
}

.dropdown-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 10px 14px;
  background: none;
  border: none;
  color: var(--color-text-primary);
  font-size: 14px;
  cursor: pointer;
  border-radius: 8px;
  transition: all 0.2s ease;
}

.dropdown-item:hover {
  background: var(--glass-bg-hover);
}

.dropdown-icon {
  width: 16px;
  height: 16px;
}

.main-content {
  flex: 1;
  overflow-y: auto;
}

/* Footer - 毛玻璃效果 */
.app-footer {
  height: 52px;
  background: var(--glass-bg);
  backdrop-filter: var(--glass-blur);
  -webkit-backdrop-filter: var(--glass-blur);
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
  gap: 16px;
}

.brand {
  font-weight: 600;
  background: var(--gradient-accent);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.version {
  padding: 4px 10px;
  background: var(--glass-bg-light);
  color: var(--color-text-secondary);
  border-radius: 6px;
  font-size: 12px;
  border: 1px solid var(--color-border);
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
  padding: 4px 10px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;
  border: 1px solid transparent;
}

.update-status:hover {
  transform: translateY(-1px);
}

.update-status.latest {
  color: var(--color-success);
  background: rgba(16, 185, 129, 0.1);
  border-color: rgba(16, 185, 129, 0.2);
}

.update-status.has-update {
  color: var(--color-warning);
  background: rgba(245, 158, 11, 0.1);
  border-color: rgba(245, 158, 11, 0.2);
  animation: pulse-glow 2s ease-in-out infinite;
}

@keyframes pulse-glow {
  0%, 100% { box-shadow: 0 0 0 0 var(--color-warning-glow); }
  50% { box-shadow: 0 0 8px 2px var(--color-warning-glow); }
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

/* Update Modal - 毛玻璃效果 */
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.update-modal {
  background: var(--glass-bg);
  backdrop-filter: var(--glass-blur);
  -webkit-backdrop-filter: var(--glass-blur);
  border: 1px solid var(--color-border);
  border-radius: 16px;
  width: 90%;
  max-width: 480px;
  max-height: 80vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  box-shadow: var(--shadow-lg), var(--shadow-glow);
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
  padding: 10px 18px;
  border-radius: 10px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  background: var(--glass-bg);
  border: 1px solid var(--color-border);
  color: var(--color-text-primary);
}

.modal-btn:hover:not(:disabled) {
  background: var(--glass-bg-hover);
  transform: translateY(-1px);
}

.modal-btn.primary {
  background: var(--gradient-accent);
  border: none;
  color: white;
  box-shadow: 0 4px 15px var(--color-accent-glow);
}

.modal-btn.primary:hover:not(:disabled) {
  box-shadow: 0 6px 20px var(--color-accent-glow);
  filter: brightness(1.1);
}

.modal-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
