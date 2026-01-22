<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { getClients } from '../api'
import type { ClientStatus } from '../types'

const router = useRouter()
const clients = ref<ClientStatus[]>([])

// Mock data for traffic (API not implemented yet)
const trafficStats = ref({
  inbound: 0,
  outbound: 0,
  inboundUnit: 'GB',
  outboundUnit: 'GB'
})

const loadClients = async () => {
  try {
    const { data } = await getClients()
    clients.value = data || []
  } catch (e) {
    console.error('Failed to load clients', e)
  }
}

const onlineClients = computed(() => {
  return clients.value.filter(client => client.online).length
})

onMounted(loadClients)

const viewClient = (id: string) => {
  router.push(`/client/${id}`)
}
</script>

<template>
  <div class="dashboard-container">
    <!-- Animated background particles -->
    <div class="particles">
      <div class="particle particle-1"></div>
      <div class="particle particle-2"></div>
      <div class="particle particle-3"></div>
      <div class="particle particle-4"></div>
      <div class="particle particle-5"></div>
    </div>

    <!-- Main content -->
    <div class="dashboard-content">
      <!-- Header -->
      <div class="dashboard-header">
        <h1 class="text-3xl font-bold text-white mb-2">Dashboard</h1>
        <p class="text-white/70">Monitor your tunnel connections and traffic</p>
      </div>

      <!-- Stats Grid -->
      <div class="stats-grid">
        <!-- Outbound Traffic -->
        <div class="stat-card glass-stat">
          <div class="stat-icon outbound">
            <svg xmlns="http://www.w3.org/2000/svg" class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 11l5-5m0 0l5 5m-5-5v12" />
            </svg>
          </div>
          <div class="stat-content">
            <span class="stat-label">Outbound Traffic</span>
            <span class="stat-value">{{ trafficStats.outbound.toFixed(2) }}</span>
            <span class="stat-unit">{{ trafficStats.outboundUnit }}</span>
          </div>
          <div class="stat-trend up">
            <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
            </svg>
          </div>
        </div>

        <!-- Inbound Traffic -->
        <div class="stat-card glass-stat">
          <div class="stat-icon inbound">
            <svg xmlns="http://www.w3.org/2000/svg" class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 13l-5 5m0 0l-5-5m5 5V6" />
            </svg>
          </div>
          <div class="stat-content">
            <span class="stat-label">Inbound Traffic</span>
            <span class="stat-value">{{ trafficStats.inbound.toFixed(2) }}</span>
            <span class="stat-unit">{{ trafficStats.inboundUnit }}</span>
          </div>
          <div class="stat-trend up">
            <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
            </svg>
          </div>
        </div>

        <!-- Client Count -->
        <div class="stat-card glass-stat">
          <div class="stat-icon clients">
            <svg xmlns="http://www.w3.org/2000/svg" class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
            </svg>
          </div>
          <div class="stat-content">
            <span class="stat-label">Clients</span>
            <div class="client-count">
              <span class="stat-value online">{{ onlineClients }}</span>
              <span class="stat-separator">/</span>
              <span class="stat-value total">{{ clients.length }}</span>
            </div>
            <span class="stat-unit">online / total</span>
          </div>
          <div class="online-indicator" :class="{ active: onlineClients > 0 }">
            <span class="pulse"></span>
          </div>
        </div>
      </div>

      <!-- Client List Section -->
      <div class="clients-section">
        <div class="section-header">
          <h2 class="text-xl font-semibold text-white">Connected Clients</h2>
          <span class="client-badge">{{ clients.length }} clients</span>
        </div>

        <!-- Empty State -->
        <div v-if="clients.length === 0" class="empty-state glass-card">
          <svg xmlns="http://www.w3.org/2000/svg" class="w-16 h-16 text-white/30 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
          </svg>
          <p class="text-white/50 text-lg">No clients connected</p>
          <p class="text-white/30 text-sm mt-2">Waiting for tunnel connections...</p>
        </div>

        <!-- Client Cards Grid -->
        <div v-else class="clients-grid">
          <div
            v-for="client in clients"
            :key="client.id"
            class="client-card glass-card"
            @click="viewClient(client.id)"
          >
            <div class="client-header">
              <div class="client-status" :class="{ online: client.online }">
                <span class="status-dot"></span>
              </div>
              <h3 class="client-name">{{ client.nickname || client.id }}</h3>
            </div>

            <p v-if="client.nickname" class="client-id">{{ client.id }}</p>

            <div class="client-info">
              <div v-if="client.remote_addr && client.online" class="info-item">
                <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9" />
                </svg>
                <span>{{ client.remote_addr }}</span>
              </div>
              <div class="info-item">
                <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 10h16M4 14h16M4 18h16" />
                </svg>
                <span>{{ client.rule_count }} rules</span>
              </div>
            </div>

            <div class="client-tags">
              <span class="tag" :class="client.online ? 'tag-online' : 'tag-offline'">
                {{ client.online ? 'Online' : 'Offline' }}
              </span>
            </div>

            <div class="card-arrow">
              <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
              </svg>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* Container with gradient background */
.dashboard-container {
  min-height: calc(100vh - 108px);
  background: linear-gradient(135deg, #1e1b4b 0%, #312e81 30%, #4c1d95 60%, #581c87 100%);
  position: relative;
  overflow: hidden;
  padding: 32px;
}

/* Floating particles */
.particles {
  position: absolute;
  inset: 0;
  pointer-events: none;
  overflow: hidden;
}

.particle {
  position: absolute;
  border-radius: 50%;
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.15), rgba(255, 255, 255, 0.05));
  animation: float-particle 20s ease-in-out infinite;
}

.particle-1 {
  width: 300px;
  height: 300px;
  top: -100px;
  right: -50px;
  animation-delay: 0s;
}

.particle-2 {
  width: 200px;
  height: 200px;
  bottom: 10%;
  left: 5%;
  animation-delay: -5s;
}

.particle-3 {
  width: 150px;
  height: 150px;
  top: 40%;
  right: 20%;
  animation-delay: -10s;
}

.particle-4 {
  width: 100px;
  height: 100px;
  top: 20%;
  left: 30%;
  animation-delay: -15s;
}

.particle-5 {
  width: 80px;
  height: 80px;
  bottom: 30%;
  right: 10%;
  animation-delay: -8s;
}

@keyframes float-particle {
  0%, 100% { transform: translate(0, 0) scale(1); opacity: 0.3; }
  25% { transform: translate(30px, -40px) scale(1.1); opacity: 0.5; }
  50% { transform: translate(-20px, -80px) scale(0.9); opacity: 0.4; }
  75% { transform: translate(-40px, -40px) scale(1.05); opacity: 0.35; }
}

/* Main content */
.dashboard-content {
  position: relative;
  z-index: 10;
  max-width: 1200px;
  margin: 0 auto;
}

.dashboard-header {
  margin-bottom: 32px;
}

/* Stats Grid */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 24px;
  margin-bottom: 40px;
}

@media (max-width: 768px) {
  .stats-grid {
    grid-template-columns: 1fr;
  }
}

/* Glass stat card */
.glass-stat {
  background: rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border-radius: 20px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.2);
  padding: 24px;
  display: flex;
  align-items: flex-start;
  gap: 16px;
  position: relative;
  transition: all 0.25s ease;
}

.glass-stat:hover {
  background: rgba(255, 255, 255, 0.12);
  transform: translateY(-4px);
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.3);
}

/* Stat icon */
.stat-icon {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.stat-icon.outbound {
  background: linear-gradient(135deg, #60a5fa 0%, #3b82f6 100%);
  box-shadow: 0 4px 16px rgba(59, 130, 246, 0.4);
}

.stat-icon.inbound {
  background: linear-gradient(135deg, #a78bfa 0%, #8b5cf6 100%);
  box-shadow: 0 4px 16px rgba(139, 92, 246, 0.4);
}

.stat-icon.clients {
  background: linear-gradient(135deg, #34d399 0%, #10b981 100%);
  box-shadow: 0 4px 16px rgba(16, 185, 129, 0.4);
}

.stat-icon svg {
  color: white;
}

/* Stat content */
.stat-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.stat-label {
  font-size: 13px;
  color: rgba(255, 255, 255, 0.6);
  font-weight: 500;
}

.stat-value {
  font-size: 32px;
  font-weight: 700;
  color: white;
  line-height: 1.1;
}

.stat-unit {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.5);
}

/* Client count special styling */
.client-count {
  display: flex;
  align-items: baseline;
  gap: 4px;
}

.client-count .stat-value.online {
  color: #34d399;
}

.client-count .stat-value.total {
  font-size: 24px;
  color: rgba(255, 255, 255, 0.7);
}

.stat-separator {
  font-size: 20px;
  color: rgba(255, 255, 255, 0.4);
}

/* Stat trend indicator */
.stat-trend {
  position: absolute;
  top: 16px;
  right: 16px;
  padding: 4px 8px;
  border-radius: 8px;
  font-size: 12px;
}

.stat-trend.up {
  background: rgba(52, 211, 153, 0.2);
  color: #34d399;
}

/* Online indicator with pulse */
.online-indicator {
  position: absolute;
  top: 16px;
  right: 16px;
}

.online-indicator .pulse {
  display: block;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.3);
}

.online-indicator.active .pulse {
  background: #34d399;
  animation: pulse-animation 2s ease-in-out infinite;
}

@keyframes pulse-animation {
  0%, 100% { box-shadow: 0 0 0 0 rgba(52, 211, 153, 0.5); }
  50% { box-shadow: 0 0 0 8px rgba(52, 211, 153, 0); }
}

/* Clients Section */
.clients-section {
  margin-top: 16px;
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
}

.client-badge {
  background: rgba(255, 255, 255, 0.1);
  color: rgba(255, 255, 255, 0.7);
  padding: 6px 12px;
  border-radius: 20px;
  font-size: 13px;
  border: 1px solid rgba(255, 255, 255, 0.1);
}

/* Empty state */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 64px 32px;
  text-align: center;
}

/* Clients grid */
.clients-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 20px;
}

@media (max-width: 1024px) {
  .clients-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 640px) {
  .clients-grid {
    grid-template-columns: 1fr;
  }
}

/* Client card */
.client-card {
  background: rgba(255, 255, 255, 0.06);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border-radius: 16px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  padding: 20px;
  cursor: pointer;
  position: relative;
  transition: all 0.2s ease;
}

.client-card:hover {
  background: rgba(255, 255, 255, 0.1);
  transform: translateY(-2px);
  border-color: rgba(255, 255, 255, 0.2);
}

.client-card:active {
  transform: translateY(0) scale(0.98);
}

/* Client header */
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
  background: rgba(255, 255, 255, 0.3);
}

.client-status.online {
  background: #34d399;
  box-shadow: 0 0 8px rgba(52, 211, 153, 0.6);
}

.client-name {
  font-size: 16px;
  font-weight: 600;
  color: white;
  margin: 0;
}

.client-id {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.4);
  margin: 0 0 12px 0;
}

/* Client info */
.client-info {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 12px;
}

.info-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: rgba(255, 255, 255, 0.6);
}

.info-item svg {
  flex-shrink: 0;
  opacity: 0.6;
}

/* Client tags */
.client-tags {
  display: flex;
  gap: 8px;
}

.tag {
  padding: 4px 10px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 500;
}

.tag-online {
  background: rgba(52, 211, 153, 0.2);
  color: #34d399;
}

.tag-offline {
  background: rgba(255, 255, 255, 0.1);
  color: rgba(255, 255, 255, 0.5);
}

/* Card arrow */
.card-arrow {
  position: absolute;
  right: 16px;
  top: 50%;
  transform: translateY(-50%);
  color: rgba(255, 255, 255, 0.3);
  transition: all 0.2s ease;
}

.client-card:hover .card-arrow {
  color: rgba(255, 255, 255, 0.6);
  transform: translateY(-50%) translateX(4px);
}

/* Glass card base */
.glass-card {
  background: rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border-radius: 16px;
  border: 1px solid rgba(255, 255, 255, 0.12);
}
</style>
