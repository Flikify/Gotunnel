<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterView, useRoute, useRouter } from 'vue-router'
import {
  ContrastOutline,
  DesktopOutline,
  HomeOutline,
  LogOutOutline,
  MoonOutline,
  PersonCircleOutline,
  SettingsOutline,
  SunnyOutline,
  SyncOutline,
} from '@vicons/ionicons5'
import GlassModal from './components/GlassModal.vue'
import { applyServerUpdate, checkServerUpdate, getServerStatus, getToken, getVersionInfo, removeToken, type UpdateInfo } from './api'
import { useConfirm } from './composables/useConfirm'
import { useTheme, type ThemeMode } from './composables/useTheme'
import { useToast } from './composables/useToast'

const router = useRouter()
const route = useRoute()
const message = useToast()
const dialog = useConfirm()
const { themeMode, setTheme } = useTheme()

const shellInfo = ref({ bind_addr: '', bind_port: 0, client_count: 0, version: '' })
const updateInfo = ref<UpdateInfo | null>(null)
const showThemeMenu = ref(false)
const showUserMenu = ref(false)
const showUpdateModal = ref(false)
const updatingServer = ref(false)
const themeMenuRef = ref<HTMLElement | null>(null)
const userMenuRef = ref<HTMLElement | null>(null)

const isLoginPage = computed(() => route.path === '/login')
const navItems = [
  { key: 'home', label: '控制台', icon: HomeOutline, path: '/' },
  { key: 'clients', label: '客户端', icon: DesktopOutline, path: '/clients' },
  { key: 'settings', label: '设置', icon: SettingsOutline, path: '/settings' },
]

const activeNav = computed(() => {
  if (route.path.startsWith('/client') || route.path === '/clients') return 'clients'
  if (route.path === '/settings') return 'settings'
  return 'home'
})

const themeIcon = computed(() => {
  if (themeMode.value === 'light') return SunnyOutline
  if (themeMode.value === 'dark') return MoonOutline
  return ContrastOutline
})

const updateBadgeText = computed(() => {
  if (!updateInfo.value) return '未检查更新'
  return updateInfo.value.available ? `可升级到 ${updateInfo.value.latest}` : '已是最新版本'
})

const loadShellInfo = async () => {
  if (!getToken() || isLoginPage.value) return
  try {
    const [statusResult, versionResult] = await Promise.allSettled([
      getServerStatus(),
      getVersionInfo(),
    ])

    if (statusResult.status === 'fulfilled') {
      shellInfo.value.bind_addr = statusResult.value.data.server.bind_addr
      shellInfo.value.bind_port = statusResult.value.data.server.bind_port
      shellInfo.value.client_count = statusResult.value.data.client_count
    }

    if (versionResult.status === 'fulfilled') {
      const versionInfo = versionResult.value.data
      shellInfo.value.version = versionInfo.version || ''
      try {
        const { data } = await checkServerUpdate(versionInfo.version, versionInfo.os, versionInfo.arch)
        updateInfo.value = data
      } catch (error) {
        console.error('Failed to check server update directly from GitHub Releases', error)
      }
    }
  } catch (error) {
    console.error('Failed to load shell info', error)
  }
}

const selectTheme = (mode: ThemeMode) => {
  setTheme(mode)
  showThemeMenu.value = false
}

const logout = () => {
  removeToken()
  router.push('/login')
}

const formatBytes = (bytes: number) => {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / Math.pow(1024, index)).toFixed(index === 0 ? 0 : 1)} ${units[index] ?? 'B'}`
}

const handleApplyServerUpdate = () => {
  if (!updateInfo.value?.download_url) {
    message.error('没有可用的更新包')
    return
  }

  dialog.warning({
    title: '确认升级服务端',
    content: `即将升级到 ${updateInfo.value.latest}，服务端会自动重启。是否继续？`,
    positiveText: '立即升级',
    negativeText: '取消',
    onPositiveClick: async () => {
      updatingServer.value = true
      try {
        await applyServerUpdate(updateInfo.value!.download_url!)
        message.success('升级任务已提交，页面将在 5 秒后尝试刷新')
        showUpdateModal.value = false
        window.setTimeout(() => window.location.reload(), 5000)
      } catch (error: any) {
        message.error(error.response?.data || '升级失败')
        updatingServer.value = false
      }
    },
  })
}

const closeMenus = (event: MouseEvent) => {
  const target = event.target as Node
  if (themeMenuRef.value && !themeMenuRef.value.contains(target)) showThemeMenu.value = false
  if (userMenuRef.value && !userMenuRef.value.contains(target)) showUserMenu.value = false
}

watch(() => route.fullPath, () => {
  showThemeMenu.value = false
  showUserMenu.value = false
  if (!isLoginPage.value) loadShellInfo()
})

onMounted(() => {
  document.addEventListener('click', closeMenus)
  loadShellInfo()
})

onUnmounted(() => {
  document.removeEventListener('click', closeMenus)
})
</script>

<template>
  <RouterView v-if="isLoginPage" />
  <div v-else class="app-shell">
    <aside class="app-sidebar glass-card">
      <div class="brand-block">
        <span class="brand-mark">GT</span>
        <div>
          <strong>GoTunnel</strong>
          <p>内网穿透控制台</p>
        </div>
      </div>

      <nav class="sidebar-nav">
        <router-link
          v-for="item in navItems"
          :key="item.key"
          :to="item.path"
          class="nav-link"
          :class="{ active: activeNav === item.key }"
        >
          <component :is="item.icon" class="nav-link__icon" />
          <span>{{ item.label }}</span>
        </router-link>
      </nav>

      <div class="sidebar-card">
        <span class="sidebar-card__label">服务监听</span>
        <strong>{{ shellInfo.bind_addr || '0.0.0.0' }}:{{ shellInfo.bind_port || '—' }}</strong>
        <p>在线客户端 {{ shellInfo.client_count }}</p>
      </div>

      <div class="sidebar-card update-card" @click="showUpdateModal = true">
        <span class="sidebar-card__label">更新状态</span>
        <strong>{{ updateBadgeText }}</strong>
        <p>{{ shellInfo.version ? `当前 ${shellInfo.version}` : '点击查看详情' }}</p>
      </div>
    </aside>

    <div class="app-main">
      <header class="app-topbar glass-card">
        <div class="topbar-intro">
          <span class="topbar-label">Workspace</span>
          <h1>{{ navItems.find((item) => item.key === activeNav)?.label || '控制台' }}</h1>
        </div>

        <div class="topbar-actions">
          <button class="topbar-icon-btn" @click="loadShellInfo">
            <SyncOutline />
          </button>

          <div ref="themeMenuRef" class="menu-wrap">
            <button class="topbar-icon-btn" @click.stop="showThemeMenu = !showThemeMenu">
              <component :is="themeIcon" />
            </button>
            <div v-if="showThemeMenu" class="floating-menu">
              <button class="floating-menu__item" :class="{ active: themeMode === 'light' }" @click="selectTheme('light')">
                <SunnyOutline /> 浅色
              </button>
              <button class="floating-menu__item" :class="{ active: themeMode === 'dark' }" @click="selectTheme('dark')">
                <MoonOutline /> 深色
              </button>
              <button class="floating-menu__item" :class="{ active: themeMode === 'auto' }" @click="selectTheme('auto')">
                <ContrastOutline /> 自动
              </button>
            </div>
          </div>

          <div ref="userMenuRef" class="menu-wrap">
            <button class="profile-button" @click.stop="showUserMenu = !showUserMenu">
              <PersonCircleOutline />
              <span>管理员</span>
            </button>
            <div v-if="showUserMenu" class="floating-menu floating-menu--right">
              <button class="floating-menu__item" @click="logout">
                <LogOutOutline /> 退出登录
              </button>
            </div>
          </div>
        </div>
      </header>

      <main class="app-content">
        <RouterView />
      </main>
    </div>

    <GlassModal :show="showUpdateModal" title="服务端更新" width="560px" @close="showUpdateModal = false">
      <div v-if="updateInfo" class="update-grid">
        <div><span>当前版本</span><strong>{{ updateInfo.current }}</strong></div>
        <div><span>最新版本</span><strong>{{ updateInfo.latest }}</strong></div>
        <div><span>文件名</span><strong>{{ updateInfo.asset_name || '未提供' }}</strong></div>
        <div><span>文件大小</span><strong>{{ formatBytes(updateInfo.asset_size || 0) }}</strong></div>
      </div>
      <div v-if="updateInfo?.release_note" class="release-note">{{ updateInfo.release_note }}</div>
      <template #footer>
        <button class="glass-btn" @click="showUpdateModal = false">关闭</button>
        <button v-if="updateInfo?.available" class="glass-btn primary" :disabled="updatingServer" @click="handleApplyServerUpdate">
          {{ updatingServer ? '升级中...' : '立即升级' }}
        </button>
      </template>
    </GlassModal>
  </div>
</template>

<style scoped>
.app-shell {
  min-height: 100vh;
  display: grid;
  grid-template-columns: 260px minmax(0, 1fr);
  gap: clamp(14px, 2vw, 20px);
  padding: clamp(12px, 2vw, 20px);
}

.app-sidebar,
.app-topbar {
  padding: 20px;
}

.app-sidebar {
  display: flex;
  flex-direction: column;
  gap: 18px;
  position: sticky;
  top: 20px;
  height: calc(100vh - 40px);
}

.brand-block {
  display: flex;
  align-items: center;
  gap: 14px;
}

.brand-mark {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: 14px;
  background: var(--gradient-accent);
  color: white;
  font-weight: 700;
  box-shadow: 0 8px 20px var(--color-accent-glow);
}

.brand-block strong {
  color: var(--color-text-primary);
  font-size: 18px;
}

.brand-block p,
.sidebar-card p,
.topbar-label {
  color: var(--color-text-secondary);
  font-size: 13px;
}

.sidebar-nav {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.nav-link {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  border-radius: 14px;
  text-decoration: none;
  color: var(--color-text-secondary);
  transition: all 0.2s ease;
}

.nav-link:hover,
.nav-link.active {
  color: var(--color-text-primary);
  background: var(--glass-bg-light);
}

.nav-link.active {
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--color-accent) 26%, transparent);
}

.nav-link__icon {
  width: 18px;
  height: 18px;
}

.sidebar-card {
  padding: 18px;
  border-radius: 18px;
  background: var(--glass-bg-light);
  border: 1px solid var(--color-border-light);
}

.sidebar-card__label {
  display: block;
  margin-bottom: 10px;
  color: var(--color-text-muted);
  font-size: 12px;
}

.sidebar-card strong {
  display: block;
  color: var(--color-text-primary);
  font-size: 16px;
}

.update-card {
  margin-top: auto;
  cursor: pointer;
}

.app-main {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.app-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  position: relative;
  overflow: visible;
  z-index: 30;
  padding: 18px 22px;
  border-radius: 22px;
  background:
    radial-gradient(circle at top right, var(--color-accent-glow), transparent 38%),
    linear-gradient(135deg, var(--glass-bg) 0%, var(--glass-bg-light) 100%);
  border-color: rgba(255, 255, 255, 0.1);
}

.app-topbar::after {
  content: '';
  position: absolute;
  inset: auto 18px -18px auto;
  width: 120px;
  height: 120px;
  border-radius: 999px;
  background: var(--color-accent-glow);
  opacity: 0.18;
  filter: blur(28px);
  pointer-events: none;
}

.topbar-intro {
  position: relative;
  z-index: 1;
  min-width: 0;
}

.app-topbar h1 {
  margin: 6px 0 0;
  font-size: 24px;
  color: var(--color-text-primary);
}

.topbar-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  position: relative;
  z-index: 2;
  flex-shrink: 0;
}

.topbar-icon-btn,
.profile-button {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  height: 42px;
  padding: 0 14px;
  border-radius: 12px;
  border: 1px solid var(--color-border);
  background: var(--glass-bg-light);
  color: var(--color-text-primary);
  cursor: pointer;
  transition: transform 0.2s ease, border-color 0.2s ease, background 0.2s ease, box-shadow 0.2s ease;
}

.topbar-icon-btn:hover,
.profile-button:hover {
  transform: translateY(-1px);
  border-color: color-mix(in srgb, var(--color-accent) 28%, transparent);
  background: rgba(255, 255, 255, 0.08);
  box-shadow: var(--shadow-sm);
}

.topbar-icon-btn svg,
.profile-button svg,
.floating-menu__item svg {
  width: 18px;
  height: 18px;
}

.menu-wrap {
  position: relative;
  z-index: 4;
}

.floating-menu {
  position: absolute;
  top: calc(100% + 10px);
  left: 0;
  min-width: 168px;
  padding: 8px;
  border-radius: 16px;
  background: var(--glass-bg);
  border: 1px solid var(--color-border);
  box-shadow: var(--shadow-lg);
  backdrop-filter: var(--glass-blur-light);
  -webkit-backdrop-filter: var(--glass-blur-light);
  z-index: 50;
}

.floating-menu--right {
  right: 0;
  left: auto;
}

.floating-menu__item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 10px 12px;
  border: none;
  border-radius: 12px;
  background: transparent;
  color: var(--color-text-primary);
  cursor: pointer;
}

.floating-menu__item.active,
.floating-menu__item:hover {
  background: var(--glass-bg-light);
}

.app-content {
  min-width: 0;
}

.update-grid {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.update-grid div {
  padding: 14px;
  border-radius: 14px;
  background: var(--glass-bg-light);
  border: 1px solid var(--color-border-light);
}

.update-grid span {
  display: block;
  margin-bottom: 8px;
  color: var(--color-text-secondary);
  font-size: 12px;
}

.update-grid strong {
  color: var(--color-text-primary);
  word-break: break-word;
}

.release-note {
  margin-top: 16px;
  padding: 14px;
  border-radius: 14px;
  background: var(--glass-bg-light);
  border: 1px solid var(--color-border-light);
  color: var(--color-text-secondary);
  white-space: pre-wrap;
  line-height: 1.7;
}

@media (max-width: 1080px) {
  .app-shell {
    grid-template-columns: 1fr;
  }

  .app-sidebar {
    position: static;
    height: auto;
    flex-direction: row;
    flex-wrap: wrap;
    align-items: stretch;
  }

  .brand-block {
    flex: 1 1 220px;
  }

  .sidebar-nav {
    flex: 2 1 320px;
    flex-direction: row;
    flex-wrap: wrap;
  }

  .nav-link {
    flex: 1 1 140px;
  }

  .sidebar-card {
    flex: 1 1 220px;
  }

  .update-card {
    margin-top: 0;
  }
}

@media (max-width: 768px) {
  .app-shell {
    padding: 12px;
  }

  .app-sidebar,
  .app-topbar {
    padding: 16px;
  }

  .app-topbar {
    flex-direction: column;
    align-items: flex-start;
    overflow: visible;
  }

  .topbar-actions {
    width: 100%;
    flex-wrap: wrap;
    justify-content: stretch;
  }

  .topbar-actions > * {
    flex: 1 1 0;
    min-width: 0;
  }

  .menu-wrap > button,
  .profile-button,
  .topbar-icon-btn {
    width: 100%;
    justify-content: center;
  }

  .floating-menu,
  .floating-menu--right {
    left: 0;
    right: 0;
    min-width: 0;
  }

  .update-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 520px) {
  .app-sidebar,
  .app-topbar {
    padding: 14px;
  }

  .brand-block {
    width: 100%;
  }

  .sidebar-nav {
    flex-direction: column;
  }

  .nav-link {
    width: 100%;
  }

  .app-topbar h1 {
    font-size: 20px;
  }
}
</style>
