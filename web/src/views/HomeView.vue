<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { getClients, getTrafficHourly, getTrafficStats, type TrafficRecord } from '../api'
import type { ClientStatus } from '../types'
import MetricCard from '../components/MetricCard.vue'
import PageFrame from '../components/PageFrame.vue'
import SectionCard from '../components/SectionCard.vue'

const clients = ref<ClientStatus[]>([])
const traffic24h = ref({ inbound: 0, outbound: 0 })
const trafficTotal = ref({ inbound: 0, outbound: 0 })
const trafficHistory = ref<TrafficRecord[]>([])
const loading = ref(true)

const formatBytes = (bytes: number): { value: string; unit: string } => {
  if (bytes === 0) return { value: '0', unit: 'B' }
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return {
    value: (bytes / Math.pow(1024, index)).toFixed(index === 0 ? 0 : 1),
    unit: units[index] ?? 'B',
  }
}

const loadDashboard = async () => {
  loading.value = true
  try {
    const [{ data: clientData }, { data: statsData }, { data: hourlyData }] = await Promise.all([
      getClients(),
      getTrafficStats(),
      getTrafficHourly(),
    ])

    clients.value = clientData || []
    traffic24h.value = statsData.traffic_24h
    trafficTotal.value = statsData.traffic_total

    const records = hourlyData.records || []
    if (records.length) {
      trafficHistory.value = records.slice(-12)
      return
    }

    const now = new Date()
    trafficHistory.value = Array.from({ length: 12 }, (_, index) => {
      const slot = new Date(now.getTime() - (11 - index) * 3600 * 1000)
      return {
        timestamp: Math.floor(slot.getTime() / 1000),
        inbound: 0,
        outbound: 0,
      }
    })
  } catch (error) {
    console.error('Failed to load dashboard', error)
  } finally {
    loading.value = false
  }
}

const onlineClients = computed(() => clients.value.filter((client) => client.online).length)
const offlineClients = computed(() => Math.max(clients.value.length - onlineClients.value, 0))
const totalRules = computed(() => clients.value.reduce((sum, client) => sum + (client.rule_count || 0), 0))
const topClients = computed(() => [...clients.value].sort((a, b) => Number(b.online) - Number(a.online)).slice(0, 6))
const chartMax = computed(() => Math.max(...trafficHistory.value.flatMap((item) => [item.inbound, item.outbound]), 1))
const formatted24hInbound = computed(() => formatBytes(traffic24h.value.inbound))
const formatted24hOutbound = computed(() => formatBytes(traffic24h.value.outbound))
const formattedTotalInbound = computed(() => formatBytes(trafficTotal.value.inbound))
const formattedTotalOutbound = computed(() => formatBytes(trafficTotal.value.outbound))
const formatHour = (timestamp: number) => new Date(timestamp * 1000).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })

onMounted(loadDashboard)
</script>

<template>
  <PageFrame title="控制台" eyebrow="Overview" subtitle="统一查看连接状态、流量趋势与客户端健康情况，减少页面层级并突出关键数据。">
    <template #actions>
      <button class="glass-btn" @click="loadDashboard">{{ loading ? '刷新中...' : '刷新数据' }}</button>
    </template>

    <template #metrics>
      <MetricCard label="在线客户端" :value="onlineClients" :hint="`离线 ${offlineClients} 台`" tone="success" />
      <MetricCard label="代理规则" :value="totalRules" hint="全部客户端规则总数" />
      <MetricCard
        label="24H 出站"
        :value="formatted24hOutbound.value"
        :hint="formatted24hOutbound.unit"
        tone="info"
      />
      <MetricCard
        label="总入站"
        :value="formattedTotalInbound.value"
        :hint="formattedTotalInbound.unit"
        tone="warning"
      />
    </template>

    <div class="dashboard-grid">
      <SectionCard title="流量趋势" description="近 12 小时入站 / 出站流量概览。">
        <div class="traffic-summary">
          <div class="traffic-pill">
            <span>24H 入站</span>
            <strong>{{ formatted24hInbound.value }} {{ formatted24hInbound.unit }}</strong>
          </div>
          <div class="traffic-pill">
            <span>24H 出站</span>
            <strong>{{ formatted24hOutbound.value }} {{ formatted24hOutbound.unit }}</strong>
          </div>
          <div class="traffic-pill">
            <span>总出站</span>
            <strong>{{ formattedTotalOutbound.value }} {{ formattedTotalOutbound.unit }}</strong>
          </div>
        </div>

        <div class="traffic-chart">
          <div v-for="item in trafficHistory" :key="item.timestamp" class="traffic-chart__item">
            <div class="traffic-chart__bars">
              <span class="bar bar--inbound" :style="{ height: `${(item.inbound / chartMax) * 100}%` }"></span>
              <span class="bar bar--outbound" :style="{ height: `${(item.outbound / chartMax) * 100}%` }"></span>
            </div>
            <span class="traffic-chart__label">{{ formatHour(item.timestamp) }}</span>
          </div>
        </div>
      </SectionCard>

      <SectionCard title="客户端概况" description="优先展示在线客户端，并保留连接来源与规则数量。">
        <div v-if="topClients.length" class="client-list">
          <article v-for="client in topClients" :key="client.id" class="client-row">
            <div>
              <div class="client-row__title">
                <span class="client-dot" :class="{ online: client.online }"></span>
                <strong>{{ client.nickname || client.id }}</strong>
              </div>
              <p>{{ client.remote_addr || '等待连接地址' }}</p>
            </div>
            <div class="client-row__meta">
              <span>{{ client.rule_count || 0 }} 条规则</span>
              <span class="state-pill" :class="client.online ? 'online' : 'offline'">{{ client.online ? '在线' : '离线' }}</span>
            </div>
          </article>
        </div>
        <div v-else class="empty-state">暂无客户端数据。</div>
      </SectionCard>
    </div>
  </PageFrame>
</template>

<style scoped>
.dashboard-grid {
  display: grid;
  gap: 20px;
  grid-template-columns: minmax(0, 1.6fr) minmax(320px, 1fr);
}

.traffic-summary {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
}

.traffic-pill {
  padding: 14px 16px;
  border-radius: 16px;
  background: var(--glass-bg-light);
  border: 1px solid var(--color-border-light);
}

.traffic-pill span {
  display: block;
  color: var(--color-text-secondary);
  font-size: 12px;
}

.traffic-pill strong {
  display: block;
  margin-top: 8px;
  color: var(--color-text-primary);
  font-size: 18px;
}

.traffic-chart {
  display: grid;
  grid-template-columns: repeat(12, minmax(0, 1fr));
  gap: 10px;
  align-items: end;
  min-height: 240px;
}

.traffic-chart__item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
}

.traffic-chart__bars {
  display: flex;
  align-items: end;
  gap: 4px;
  height: 200px;
  width: 100%;
}

.bar {
  flex: 1;
  min-height: 4px;
  border-radius: 999px 999px 6px 6px;
}

.bar--inbound {
  background: linear-gradient(180deg, rgba(6, 182, 212, 0.95), rgba(6, 182, 212, 0.25));
}

.bar--outbound {
  background: linear-gradient(180deg, rgba(59, 130, 246, 0.95), rgba(59, 130, 246, 0.25));
}

.traffic-chart__label {
  font-size: 11px;
  color: var(--color-text-muted);
}

.client-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.client-row {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  padding: 16px;
  border-radius: 16px;
  background: var(--glass-bg-light);
  border: 1px solid var(--color-border-light);
}

.client-row__title {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}

.client-row p {
  color: var(--color-text-secondary);
  font-size: 13px;
}

.client-row__meta {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 10px;
  color: var(--color-text-secondary);
  font-size: 13px;
}

.client-dot {
  width: 10px;
  height: 10px;
  border-radius: 999px;
  background: var(--color-error);
  box-shadow: 0 0 0 6px rgba(239, 68, 68, 0.08);
}

.client-dot.online {
  background: var(--color-success);
  box-shadow: 0 0 0 6px rgba(16, 185, 129, 0.1);
}

.state-pill {
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 12px;
  border: 1px solid transparent;
}

.state-pill.online {
  color: var(--color-success);
  background: rgba(16, 185, 129, 0.12);
  border-color: rgba(16, 185, 129, 0.2);
}

.state-pill.offline {
  color: var(--color-text-secondary);
  background: rgba(148, 163, 184, 0.12);
  border-color: rgba(148, 163, 184, 0.2);
}

.empty-state {
  padding: 36px 16px;
  text-align: center;
  color: var(--color-text-secondary);
  background: var(--glass-bg-light);
  border: 1px dashed var(--color-border);
  border-radius: 16px;
}

@media (max-width: 1024px) {
  .dashboard-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 768px) {
  .traffic-chart {
    overflow-x: auto;
    padding-bottom: 8px;
  }

  .traffic-chart__bars {
    width: 28px;
  }

  .client-row {
    flex-direction: column;
  }

  .client-row__meta {
    align-items: flex-start;
  }
}
</style>
