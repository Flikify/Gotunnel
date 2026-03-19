<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { getClients, generateInstallCommand } from '../api'
import type { ClientStatus, InstallCommandResponse } from '../types'

const router = useRouter()
const clients = ref<ClientStatus[]>([])
const loading = ref(true)
const showInstallModal = ref(false)
const installData = ref<InstallCommandResponse | null>(null)
const generatingInstall = ref(false)

const loadClients = async () => {
  loading.value = true
  try {
    const { data } = await getClients()
    clients.value = data || []
  } catch (e) {
    console.error('Failed to load clients', e)
  } finally {
    loading.value = false
  }
}

const onlineClients = computed(() => clients.value.filter(c => c.online).length)

const viewClient = (id: string) => {
  router.push(`/client/${id}`)
}

const openInstallModal = async () => {
  generatingInstall.value = true
  try {
    const { data } = await generateInstallCommand()
    installData.value = data
    showInstallModal.value = true
  } catch (e) {
    console.error('Failed to generate install command', e)
  } finally {
    generatingInstall.value = false
  }
}

const copyCommand = (cmd: string) => {
  navigator.clipboard.writeText(cmd)
}

const closeInstallModal = () => {
  showInstallModal.value = false
  installData.value = null
}

onMounted(loadClients)
</script>

<template>
  <div class="clients-page">
    <!-- Particles -->
    <div class="particles">
      <div class="particle particle-1"></div>
      <div class="particle particle-2"></div>
      <div class="particle particle-3"></div>
    </div>

    <div class="clients-content">
      <!-- Header -->
      <div class="page-header">
        <h1 class="page-title">客户端管理</h1>
        <p class="page-subtitle">管理所有连接的客户端</p>
      </div>

      <!-- Stats Row -->
      <div class="stats-row">
        <div class="stat-card">
          <span class="stat-value">{{ clients.length }}</span>
          <span class="stat-label">总客户端</span>
        </div>
        <div class="stat-card">
          <span class="stat-value online">{{ onlineClients }}</span>
          <span class="stat-label">在线</span>
        </div>
        <div class="stat-card">
          <span class="stat-value offline">{{ clients.length - onlineClients }}</span>
          <span class="stat-label">离线</span>
        </div>
      </div>

      <!-- Client List -->
      <div class="glass-card">
        <div class="card-header">
          <h3>客户端列表</h3>
          <div style="display: flex; gap: 8px;">
            <button class="glass-btn small" @click="openInstallModal" :disabled="generatingInstall">
              {{ generatingInstall ? '生成中...' : '安装命令' }}
            </button>
            <button class="glass-btn small" @click="loadClients">刷新</button>
          </div>
        </div>
        <div class="card-body">
          <div v-if="loading" class="loading-state">加载中...</div>
          <div v-else-if="clients.length === 0" class="empty-state">
            <p>暂无客户端连接</p>
            <p class="empty-hint">等待客户端连接...</p>
          </div>
          <div v-else class="clients-grid">
            <div
              v-for="client in clients"
              :key="client.id"
              class="client-card"
              @click="viewClient(client.id)"
            >
              <div class="client-header">
                <div class="client-status" :class="{ online: client.online }"></div>
                <h4 class="client-name">{{ client.nickname || client.id }}</h4>
              </div>
              <p v-if="client.nickname" class="client-id">{{ client.id }}</p>
              <div class="client-info">
                <span v-if="client.remote_addr && client.online">{{ client.remote_addr }}</span>
                <span>{{ client.rule_count || 0 }} 条规则</span>
              </div>
              <div class="client-tag" :class="client.online ? 'online' : 'offline'">
                {{ client.online ? '在线' : '离线' }}
              </div>
              <!-- Heartbeat indicator -->
              <div class="heartbeat-indicator" :class="{ online: client.online, offline: !client.online }">
                <span class="heartbeat-dot"></span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Install Command Modal -->
    <div v-if="showInstallModal" class="modal-overlay" @click="closeInstallModal">
      <div class="modal-content" @click.stop>
        <div class="modal-header">
          <h3>客户端安装命令</h3>
          <button class="close-btn" @click="closeInstallModal">×</button>
        </div>
        <div class="modal-body" v-if="installData">
          <p class="install-hint">选择您的操作系统，复制命令并在目标机器上执行：</p>
          <div class="install-section">
            <h4>Linux</h4>
            <div class="command-box">
              <code>{{ installData.commands.linux }}</code>
              <button class="copy-btn" @click="copyCommand(installData.commands.linux)">复制</button>
            </div>
          </div>
          <div class="install-section">
            <h4>macOS</h4>
            <div class="command-box">
              <code>{{ installData.commands.macos }}</code>
              <button class="copy-btn" @click="copyCommand(installData.commands.macos)">复制</button>
            </div>
          </div>
          <div class="install-section">
            <h4>Windows</h4>
            <div class="command-box">
              <code>{{ installData.commands.windows }}</code>
              <button class="copy-btn" @click="copyCommand(installData.commands.windows)">复制</button>
            </div>
          </div>
          <p class="token-info">客户端 ID 会在目标机器上根据多种设备标识自动计算。</p>
          <p class="token-warning">⚠️ 此命令包含一次性token，使用后需重新生成</p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.clients-page {
  min-height: calc(100vh - 116px);
  position: relative;
  overflow: hidden;
  padding: 32px;
}

/* 动画背景粒子 */
.particles {
  position: absolute;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
  z-index: 0;
}

.particle {
  position: absolute;
  border-radius: 50%;
  opacity: 0.15;
  filter: blur(60px);
  animation: float 20s ease-in-out infinite;
}

.particle-1 {
  width: 350px;
  height: 350px;
  background: var(--color-accent);
  top: -80px;
  right: -80px;
}

.particle-2 {
  width: 280px;
  height: 280px;
  background: #8b5cf6;
  bottom: -40px;
  left: -40px;
  animation-delay: -5s;
}

.particle-3 {
  width: 220px;
  height: 220px;
  background: var(--color-success);
  top: 40%;
  left: 30%;
  animation-delay: -10s;
}

@keyframes float {
  0%, 100% { transform: translate(0, 0) scale(1); }
  25% { transform: translate(30px, -30px) scale(1.05); }
  50% { transform: translate(-20px, 20px) scale(0.95); }
  75% { transform: translate(-30px, -20px) scale(1.02); }
}

.clients-content {
  position: relative;
  z-index: 10;
  max-width: 1200px;
  margin: 0 auto;
}

.page-header { margin-bottom: 24px; }
.page-title {
  font-size: 28px;
  font-weight: 700;
  color: var(--color-text-primary);
  margin: 0 0 8px 0;
}
.page-subtitle {
  color: var(--color-text-secondary);
  margin: 0;
  font-size: 14px;
}

/* Stats */
.stats-row {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
  margin-bottom: 24px;
}

.stat-card {
  background: var(--glass-bg);
  backdrop-filter: var(--glass-blur);
  -webkit-backdrop-filter: var(--glass-blur);
  border-radius: 16px;
  border: 1px solid var(--color-border);
  padding: 20px;
  text-align: center;
  box-shadow: var(--shadow-card);
  transition: all 0.2s ease;
  position: relative;
}

.stat-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 20%;
  right: 20%;
  height: 1px;
  background: linear-gradient(90deg,
    transparent 0%,
    rgba(255, 255, 255, 0.1) 50%,
    transparent 100%);
}

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-lg), var(--shadow-glow);
}

.stat-value {
  display: block;
  font-size: 32px;
  font-weight: 700;
  color: var(--color-text-primary);
}
.stat-value.online { color: var(--color-success); }
.stat-value.offline { color: var(--color-text-muted); }
.stat-label {
  font-size: 13px;
  color: var(--color-text-secondary);
}

/* Glass Card */
.glass-card {
  background: var(--glass-bg);
  backdrop-filter: var(--glass-blur);
  -webkit-backdrop-filter: var(--glass-blur);
  border-radius: 16px;
  border: 1px solid var(--color-border);
  box-shadow: var(--shadow-card);
  position: relative;
}

.glass-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 10%;
  right: 10%;
  height: 1px;
  background: linear-gradient(90deg,
    transparent 0%,
    rgba(255, 255, 255, 0.1) 50%,
    transparent 100%);
}

.card-header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--color-border-light);
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.card-header h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text-primary);
}
.card-body { padding: 20px; }

.loading-state, .empty-state {
  text-align: center;
  padding: 48px;
  color: var(--color-text-muted);
}
.empty-hint {
  font-size: 13px;
  color: var(--color-text-muted);
  margin-top: 8px;
}

/* Clients Grid */
.clients-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}

@media (max-width: 900px) {
  .clients-grid { grid-template-columns: repeat(2, 1fr); }
  .stats-row { grid-template-columns: 1fr; }
}
@media (max-width: 600px) {
  .clients-grid { grid-template-columns: 1fr; }
}

.client-card {
  background: var(--glass-bg-light);
  border-radius: 12px;
  padding: 18px;
  border: 1px solid var(--color-border);
  cursor: pointer;
  transition: all 0.2s ease;
  position: relative;
}
.client-card:hover {
  background: var(--glass-bg-hover);
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
}

.client-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}

.client-status {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--color-text-muted);
}
.client-status.online {
  background: var(--color-success);
  box-shadow: 0 0 10px var(--color-success-glow);
}

.client-name {
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0;
}

.client-id {
  font-size: 12px;
  color: var(--color-text-muted);
  margin: 0 0 8px 0;
  font-family: monospace;
}

.client-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 13px;
  color: var(--color-text-secondary);
  margin-bottom: 12px;
}

.client-tag {
  display: inline-block;
  padding: 4px 10px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 500;
}
.client-tag.online {
  background: rgba(16, 185, 129, 0.15);
  color: var(--color-success);
  border: 1px solid rgba(16, 185, 129, 0.2);
}
.client-tag.offline {
  background: var(--glass-bg-light);
  color: var(--color-text-muted);
  border: 1px solid var(--color-border);
}

/* Button */
.glass-btn {
  background: var(--glass-bg);
  backdrop-filter: var(--glass-blur-light);
  border: 1px solid var(--color-border);
  border-radius: 10px;
  padding: 8px 16px;
  color: var(--color-text-primary);
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s ease;
}
.glass-btn:hover {
  background: var(--glass-bg-hover);
  transform: translateY(-1px);
}
.glass-btn.small { padding: 6px 12px; font-size: 12px; }

/* Heartbeat Indicator */
.heartbeat-indicator {
  position: absolute;
  top: 18px;
  right: 18px;
}

.heartbeat-dot {
  display: block;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--color-error);
}

.heartbeat-indicator.online .heartbeat-dot {
  background: var(--color-success);
  animation: heartbeat-pulse 2s ease-in-out infinite;
  box-shadow: 0 0 8px var(--color-success-glow);
}

.heartbeat-indicator.offline .heartbeat-dot {
  background: var(--color-error);
  animation: none;
}

@keyframes heartbeat-pulse {
  0%, 100% {
    box-shadow: 0 0 0 0 var(--color-success-glow);
    transform: scale(1);
  }
  50% {
    box-shadow: 0 0 0 8px rgba(16, 185, 129, 0);
    transform: scale(1.1);
  }
}

/* Install Modal */
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal-content {
  background: var(--glass-bg);
  backdrop-filter: blur(20px);
  border: 1px solid var(--glass-border);
  border-radius: 16px;
  max-width: 800px;
  width: 90%;
  max-height: 80vh;
  overflow-y: auto;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px 24px;
  border-bottom: 1px solid var(--glass-border);
}

.modal-header h3 {
  margin: 0;
  font-size: 18px;
}

.close-btn {
  background: none;
  border: none;
  font-size: 28px;
  cursor: pointer;
  color: var(--color-text-secondary);
  line-height: 1;
}

.modal-body {
  padding: 24px;
}

.install-hint {
  margin-bottom: 20px;
  color: var(--color-text-secondary);
}

.install-section {
  margin-bottom: 20px;
}

.install-section h4 {
  margin: 0 0 8px 0;
  font-size: 14px;
  color: var(--color-text-primary);
}

.command-box {
  display: flex;
  gap: 8px;
  background: rgba(0, 0, 0, 0.3);
  padding: 12px;
  border-radius: 8px;
  border: 1px solid var(--glass-border);
}

.command-box code {
  flex: 1;
  font-family: monospace;
  font-size: 12px;
  word-break: break-all;
  color: var(--color-text-primary);
}

.copy-btn {
  background: var(--glass-bg);
  border: 1px solid var(--glass-border);
  border-radius: 6px;
  padding: 6px 12px;
  font-size: 12px;
  cursor: pointer;
  color: var(--color-text-primary);
  white-space: nowrap;
}

.copy-btn:hover {
  background: var(--glass-bg-hover);
}

.token-info {
  margin-top: 20px;
  padding: 12px;
  background: rgba(59, 130, 246, 0.1);
  border-radius: 8px;
  font-size: 13px;
}

.token-warning {
  margin-top: 12px;
  padding: 12px;
  background: rgba(245, 158, 11, 0.1);
  border-radius: 8px;
  font-size: 13px;
  color: #f59e0b;
}

</style>
