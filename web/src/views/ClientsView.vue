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
  background: linear-gradient(135deg, #1e1b4b 0%, #312e81 30%, #4c1d95 60%, #581c87 100%);
  position: relative;
  overflow: hidden;
  padding: 32px;
}

.particles {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.particle {
  position: absolute;
  border-radius: 50%;
  background: linear-gradient(135deg, rgba(255,255,255,0.15), rgba(255,255,255,0.05));
  animation: float-particle 20s ease-in-out infinite;
}

.particle-1 { width: 250px; height: 250px; top: -80px; right: -50px; }
.particle-2 { width: 180px; height: 180px; bottom: 10%; left: 5%; animation-delay: -7s; }
.particle-3 { width: 120px; height: 120px; top: 50%; right: 15%; animation-delay: -12s; }

@keyframes float-particle {
  0%, 100% { transform: translate(0, 0) scale(1); opacity: 0.3; }
  50% { transform: translate(-20px, -60px) scale(0.95); opacity: 0.4; }
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
  color: white;
  margin: 0 0 8px 0;
}
.page-subtitle {
  color: rgba(255,255,255,0.6);
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
  background: rgba(255,255,255,0.08);
  backdrop-filter: blur(20px);
  border-radius: 16px;
  border: 1px solid rgba(255,255,255,0.12);
  padding: 20px;
  text-align: center;
}

.stat-value {
  display: block;
  font-size: 32px;
  font-weight: 700;
  color: white;
}
.stat-value.online { color: #34d399; }
.stat-value.offline { color: rgba(255,255,255,0.5); }
.stat-label {
  font-size: 13px;
  color: rgba(255,255,255,0.6);
}

/* Glass Card */
.glass-card {
  background: rgba(255,255,255,0.08);
  backdrop-filter: blur(20px);
  border-radius: 16px;
  border: 1px solid rgba(255,255,255,0.12);
}

.card-header {
  padding: 16px 20px;
  border-bottom: 1px solid rgba(255,255,255,0.08);
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.card-header h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: white;
}
.card-body { padding: 20px; }

.loading-state, .empty-state {
  text-align: center;
  padding: 48px;
  color: rgba(255,255,255,0.5);
}
.empty-hint {
  font-size: 13px;
  color: rgba(255,255,255,0.3);
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
  background: rgba(255,255,255,0.05);
  border-radius: 12px;
  padding: 16px;
  border: 1px solid rgba(255,255,255,0.08);
  cursor: pointer;
  transition: all 0.2s;
  position: relative;
}
.client-card:hover {
  background: rgba(255,255,255,0.1);
  transform: translateY(-2px);
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
  background: rgba(255,255,255,0.3);
}
.client-status.online {
  background: #34d399;
  box-shadow: 0 0 8px rgba(52,211,153,0.6);
}

.client-name {
  font-size: 15px;
  font-weight: 600;
  color: white;
  margin: 0;
}

.client-id {
  font-size: 12px;
  color: rgba(255,255,255,0.4);
  margin: 0 0 8px 0;
  font-family: monospace;
}

.client-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 13px;
  color: rgba(255,255,255,0.6);
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
  background: rgba(52,211,153,0.2);
  color: #34d399;
}
.client-tag.offline {
  background: rgba(255,255,255,0.1);
  color: rgba(255,255,255,0.5);
}

/* Button */
.glass-btn {
  background: rgba(255,255,255,0.1);
  border: 1px solid rgba(255,255,255,0.15);
  border-radius: 8px;
  padding: 8px 16px;
  color: white;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s;
}
.glass-btn:hover { background: rgba(255,255,255,0.2); }
.glass-btn.small { padding: 6px 12px; font-size: 12px; }
</style>
