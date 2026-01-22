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
    const records = data.records || []
    // 如果没有数据，生成从当前时间开始的24小时空数据
    if (records.length === 0) {
      const now = new Date()
      const currentHour = new Date(now.getFullYear(), now.getMonth(), now.getDate(), now.getHours())
      const emptyRecords: TrafficRecord[] = []
      for (let i = 23; i >= 0; i--) {
        const ts = new Date(currentHour.getTime() - i * 3600 * 1000)
        emptyRecords.push({
          timestamp: Math.floor(ts.getTime() / 1000),
          inbound: 0,
          outbound: 0
        })
      }
      trafficHistory.value = emptyRecords
    } else {
      trafficHistory.value = records
    }
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
        <h1 class="text-3xl font-bold text-white mb-2">仪表盘</h1>
        <p class="text-white/70">监控隧道连接和流量状态</p>
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
            <div class="stat-value-row">
              <span class="stat-value">{{ formatted24hOutbound.value }}</span>
              <span class="stat-unit-inline">{{ formatted24hOutbound.unit }}</span>
            </div>
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
            <div class="stat-value-row">
              <span class="stat-value">{{ formatted24hInbound.value }}</span>
              <span class="stat-unit-inline">{{ formatted24hInbound.unit }}</span>
            </div>
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
            <div class="stat-value-row">
              <span class="stat-value">{{ formattedTotalOutbound.value }}</span>
              <span class="stat-unit-inline">{{ formattedTotalOutbound.unit }}</span>
            </div>
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
            <div class="stat-value-row">
              <span class="stat-value">{{ formattedTotalInbound.value }}</span>
              <span class="stat-unit-inline">{{ formattedTotalInbound.unit }}</span>
            </div>
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
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* Container */
.dashboard-container {
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
  background: var(--color-bg-tertiary);
  border-radius: 12px;
  border: 1px solid var(--color-border);
  padding: 20px;
  display: flex;
  align-items: flex-start;
  gap: 16px;
  position: relative;
  transition: all 0.15s;
}

.glass-stat:hover {
  background: var(--color-bg-elevated);
}

/* Stat icon */
.stat-icon {
  width: 44px;
  height: 44px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.stat-icon.outbound {
  background: var(--color-accent);
}

.stat-icon.inbound {
  background: #8b5cf6;
}

.stat-icon.clients {
  background: var(--color-success);
}

.stat-icon.rules {
  background: var(--color-warning);
}

.stat-icon.total-out {
  background: #0284c7;
}

.stat-icon.total-in {
  background: #9333ea;
}

.stat-icon svg {
  color: white;
}

/* Stat content */
.stat-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-height: 44px;
  justify-content: center;
}

.stat-label {
  font-size: 13px;
  color: var(--color-text-secondary);
  font-weight: 500;
  line-height: 1.2;
}

.stat-value {
  font-size: 26px;
  font-weight: 700;
  color: var(--color-text-primary);
  line-height: 1.2;
}

.stat-unit {
  font-size: 12px;
  color: var(--color-text-muted);
  line-height: 1.2;
}

.stat-value-row {
  display: flex;
  align-items: baseline;
  gap: 4px;
}

.stat-unit-inline {
  font-size: 14px;
  color: var(--color-text-muted);
  font-weight: 500;
}

/* Client count special styling */
.client-count {
  display: flex;
  align-items: baseline;
  gap: 4px;
}

.client-count .stat-value.online {
  color: var(--color-success);
}

.client-count .stat-value.total {
  font-size: 24px;
  color: var(--color-text-secondary);
}

.stat-separator {
  font-size: 20px;
  color: var(--color-text-muted);
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
  background: rgba(0, 186, 124, 0.15);
  color: var(--color-success);
}

/* Online indicator with pulse */
.online-indicator {
  position: absolute;
  top: 16px;
  right: 16px;
}

.online-indicator .pulse {
  display: block;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--color-text-muted);
}

.online-indicator.active .pulse {
  background: var(--color-success);
  animation: pulse-animation 2s ease-in-out infinite;
}

@keyframes pulse-animation {
  0%, 100% { box-shadow: 0 0 0 0 rgba(0, 186, 124, 0.5); }
  50% { box-shadow: 0 0 0 6px rgba(0, 186, 124, 0); }
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
  font-size: 16px;
  font-weight: 600;
  color: var(--color-text-primary);
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
  color: var(--color-text-secondary);
}

.legend-dot {
  width: 10px;
  height: 10px;
  border-radius: 2px;
}

.legend-item.inbound .legend-dot {
  background: #8b5cf6;
}

.legend-item.outbound .legend-dot {
  background: var(--color-accent);
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
  background: #8b5cf6;
}

.bar.outbound {
  background: var(--color-accent);
}

.bar-label {
  font-size: 10px;
  color: var(--color-text-muted);
  white-space: nowrap;
}

.chart-hint {
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--color-border);
  text-align: center;
  font-size: 12px;
  color: var(--color-text-muted);
}

/* Glass card base */
.glass-card {
  background: var(--color-bg-tertiary);
  border-radius: 12px;
  border: 1px solid var(--color-border);
}
</style>
