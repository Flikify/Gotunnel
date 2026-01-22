<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { getClients, getTrafficStats, getTrafficHourly, type TrafficRecord } from '../api'
import type { ClientStatus } from '../types'

const clients = ref<ClientStatus[]>([])

// 流量统计数据
const traffic24h = ref({ inbound: 0, outbound: 0 })
const trafficTotal = ref({ inbound: 0, outbound: 0 })
const trafficHistory = ref<TrafficRecord[]>([])

// 格式化字节数
const formatBytes = (bytes: number): { value: string; unit: string } => {
  if (bytes === 0) return { value: '0', unit: 'B' }
  const k = 1024
  const sizes: string[] = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), sizes.length - 1)
  return {
    value: parseFloat((bytes / Math.pow(k, i)).toFixed(2)).toString(),
    unit: sizes[i] as string
  }
}

// 加载流量统计
const loadTrafficStats = async () => {
  try {
    const { data } = await getTrafficStats()
    traffic24h.value = data.traffic_24h
    trafficTotal.value = data.traffic_total
  } catch (e) {
    console.error('Failed to load traffic stats', e)
  }
}

// 加载每小时流量
const loadTrafficHourly = async () => {
  try {
    const { data } = await getTrafficHourly()
    trafficHistory.value = data.records || []
  } catch (e) {
    console.error('Failed to load hourly traffic', e)
  }
}

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

const totalRules = computed(() => {
  return clients.value.reduce((sum, c) => sum + (c.rule_count || 0), 0)
})

// 格式化后的流量统计
const formatted24hInbound = computed(() => formatBytes(traffic24h.value.inbound))
const formatted24hOutbound = computed(() => formatBytes(traffic24h.value.outbound))
const formattedTotalInbound = computed(() => formatBytes(trafficTotal.value.inbound))
const formattedTotalOutbound = computed(() => formatBytes(trafficTotal.value.outbound))

// Chart helpers
const maxTraffic = computed(() => {
  const max = Math.max(
    ...trafficHistory.value.map(d => Math.max(d.inbound, d.outbound))
  )
  return max || 100
})

const getBarHeight = (value: number) => {
  return (value / maxTraffic.value) * 100
}

// 格式化时间戳为小时
const formatHour = (timestamp: number) => {
  const date = new Date(timestamp * 1000)
  return date.getHours().toString().padStart(2, '0') + ':00'
}

onMounted(() => {
  loadClients()
  loadTrafficStats()
  loadTrafficHourly()
})
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
            <span class="stat-label">24h出站</span>
            <span class="stat-value">{{ formatted24hOutbound.value }}</span>
            <span class="stat-unit">{{ formatted24hOutbound.unit }}</span>
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
            <span class="stat-label">24h入站</span>
            <span class="stat-value">{{ formatted24hInbound.value }}</span>
            <span class="stat-unit">{{ formatted24hInbound.unit }}</span>
          </div>
        </div>

        <!-- Total Outbound Traffic -->
        <div class="stat-card glass-stat">
          <div class="stat-icon total-out">
            <svg xmlns="http://www.w3.org/2000/svg" class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 11l5-5m0 0l5 5m-5-5v12" />
            </svg>
          </div>
          <div class="stat-content">
            <span class="stat-label">总出站</span>
            <span class="stat-value">{{ formattedTotalOutbound.value }}</span>
            <span class="stat-unit">{{ formattedTotalOutbound.unit }}</span>
          </div>
        </div>

        <!-- Total Inbound Traffic -->
        <div class="stat-card glass-stat">
          <div class="stat-icon total-in">
            <svg xmlns="http://www.w3.org/2000/svg" class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 13l-5 5m0 0l-5-5m5 5V6" />
            </svg>
          </div>
          <div class="stat-content">
            <span class="stat-label">总入站</span>
            <span class="stat-value">{{ formattedTotalInbound.value }}</span>
            <span class="stat-unit">{{ formattedTotalInbound.unit }}</span>
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
            <span class="stat-label">客户端</span>
            <div class="client-count">
              <span class="stat-value online">{{ onlineClients }}</span>
              <span class="stat-separator">/</span>
              <span class="stat-value total">{{ clients.length }}</span>
            </div>
            <span class="stat-unit">在线 / 总数</span>
          </div>
          <div class="online-indicator" :class="{ active: onlineClients > 0 }">
            <span class="pulse"></span>
          </div>
        </div>

        <!-- Rules Count -->
        <div class="stat-card glass-stat">
          <div class="stat-icon rules">
            <svg xmlns="http://www.w3.org/2000/svg" class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 10h16M4 14h16M4 18h16" />
            </svg>
          </div>
          <div class="stat-content">
            <span class="stat-label">代理规则</span>
            <span class="stat-value">{{ totalRules }}</span>
            <span class="stat-unit">条规则</span>
          </div>
        </div>
      </div>

      <!-- Traffic Chart Section -->
      <div class="chart-section">
        <div class="section-header">
          <h2 class="section-title">24小时流量趋势</h2>
          <div class="chart-legend">
            <span class="legend-item inbound"><span class="legend-dot"></span>入站</span>
            <span class="legend-item outbound"><span class="legend-dot"></span>出站</span>
          </div>
        </div>

        <div class="chart-card glass-card">
          <div class="chart-container">
            <div class="chart-bars">
              <div v-for="(data, index) in trafficHistory" :key="index" class="bar-group">
                <div class="bar-wrapper">
                  <div class="bar inbound" :style="{ height: getBarHeight(data.inbound) + '%' }"></div>
                  <div class="bar outbound" :style="{ height: getBarHeight(data.outbound) + '%' }"></div>
                </div>
                <span class="bar-label">{{ formatHour(data.timestamp) }}</span>
              </div>
            </div>
          </div>
          <div class="chart-hint" v-if="trafficHistory.length === 0">
            <span>暂无流量数据</span>
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
  gap: 20px;
  margin-bottom: 32px;
}

@media (max-width: 1024px) {
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 640px) {
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

.stat-icon.rules {
  background: linear-gradient(135deg, #fbbf24 0%, #f59e0b 100%);
  box-shadow: 0 4px 16px rgba(245, 158, 11, 0.4);
}

.stat-icon.total-out {
  background: linear-gradient(135deg, #38bdf8 0%, #0284c7 100%);
  box-shadow: 0 4px 16px rgba(2, 132, 199, 0.4);
}

.stat-icon.total-in {
  background: linear-gradient(135deg, #c084fc 0%, #9333ea 100%);
  box-shadow: 0 4px 16px rgba(147, 51, 234, 0.4);
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

/* Chart Section */
.chart-section {
  margin-top: 16px;
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
}

.section-title {
  font-size: 18px;
  font-weight: 600;
  color: white;
  margin: 0;
}

.chart-legend {
  display: flex;
  gap: 16px;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: rgba(255, 255, 255, 0.7);
}

.legend-dot {
  width: 10px;
  height: 10px;
  border-radius: 2px;
}

.legend-item.inbound .legend-dot {
  background: #a78bfa;
}

.legend-item.outbound .legend-dot {
  background: #60a5fa;
}

/* Chart Card */
.chart-card {
  padding: 24px;
}

.chart-container {
  height: 200px;
  overflow-x: auto;
}

.chart-bars {
  display: flex;
  gap: 4px;
  height: 100%;
  min-width: 600px;
  align-items: flex-end;
}

.bar-group {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.bar-wrapper {
  flex: 1;
  width: 100%;
  display: flex;
  gap: 2px;
  align-items: flex-end;
}

.bar {
  flex: 1;
  border-radius: 3px 3px 0 0;
  min-height: 2px;
  transition: height 0.3s ease;
}

.bar.inbound {
  background: linear-gradient(180deg, #a78bfa 0%, #8b5cf6 100%);
}

.bar.outbound {
  background: linear-gradient(180deg, #60a5fa 0%, #3b82f6 100%);
}

.bar-label {
  font-size: 10px;
  color: rgba(255, 255, 255, 0.4);
  white-space: nowrap;
}

.chart-hint {
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  text-align: center;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.4);
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
