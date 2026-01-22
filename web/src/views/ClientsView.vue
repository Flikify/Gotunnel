<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { getClients } from '../api'
import type { ClientStatus } from '../types'

const router = useRouter()
const clients = ref<ClientStatus[]>([])
const loading = ref(true)

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
          <button class="glass-btn small" @click="loadClients">刷新</button>
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
  </div>
</template>

<style scoped>
.clients-page {
  min-height: calc(100vh - 108px);
  background: var(--color-bg-primary);
  position: relative;
  overflow: hidden;
  padding: 32px;
}

/* Hide particles */
.particles {
  display: none;
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
  background: var(--color-bg-tertiary);
  border-radius: 12px;
  border: 1px solid var(--color-border);
  padding: 20px;
  text-align: center;
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
  background: var(--color-bg-tertiary);
  border-radius: 12px;
  border: 1px solid var(--color-border);
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
  background: var(--color-bg-elevated);
  border-radius: 10px;
  padding: 16px;
  border: 1px solid var(--color-border-light);
  cursor: pointer;
  transition: all 0.15s;
  position: relative;
}
.client-card:hover {
  background: var(--color-border);
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
  box-shadow: 0 0 8px rgba(0, 186, 124, 0.6);
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
  border-radius: 6px;
  font-size: 12px;
  font-weight: 500;
}
.client-tag.online {
  background: rgba(0, 186, 124, 0.15);
  color: var(--color-success);
}
.client-tag.offline {
  background: var(--color-bg-tertiary);
  color: var(--color-text-muted);
}

/* Button */
.glass-btn {
  background: var(--color-bg-elevated);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 8px 16px;
  color: var(--color-text-primary);
  font-size: 13px;
  cursor: pointer;
  transition: all 0.15s;
}
.glass-btn:hover { background: var(--color-border); }
.glass-btn.small { padding: 6px 12px; font-size: 12px; }

/* Heartbeat Indicator */
.heartbeat-indicator {
  position: absolute;
  top: 16px;
  right: 16px;
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
}

.heartbeat-indicator.offline .heartbeat-dot {
  background: var(--color-error);
  animation: none;
}

@keyframes heartbeat-pulse {
  0%, 100% {
    box-shadow: 0 0 0 0 rgba(0, 186, 124, 0.5);
    transform: scale(1);
  }
  50% {
    box-shadow: 0 0 0 6px rgba(0, 186, 124, 0);
    transform: scale(1.1);
  }
}
</style>
