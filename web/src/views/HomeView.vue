<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { NCard, NButton, NSpace, NTag, NStatistic, NGrid, NGi, NEmpty } from 'naive-ui'
import { getClients } from '../api'
import type { ClientStatus } from '../types'

const router = useRouter()
const clients = ref<ClientStatus[]>([])

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
  return clients.value.reduce((sum, client) => sum + client.rule_count, 0)
})

onMounted(loadClients)

const viewClient = (id: string) => {
  router.push(`/client/${id}`)
}
</script>

<template>
  <div class="home">
    <div class="page-header">
      <h2>首页</h2>
      <p>查看已连接的隧道客户端</p>
    </div>

    <n-grid :cols="3" :x-gap="16" :y-gap="16" style="margin-bottom: 24px;" responsive="screen" cols-s="1" cols-m="3">
      <n-gi>
        <n-card class="stat-card">
          <n-statistic label="总客户端" :value="clients.length" />
        </n-card>
      </n-gi>
      <n-gi>
        <n-card class="stat-card">
          <n-statistic label="在线客户端" :value="onlineClients" />
        </n-card>
      </n-gi>
      <n-gi>
        <n-card class="stat-card">
          <n-statistic label="总规则数" :value="totalRules" />
        </n-card>
      </n-gi>
    </n-grid>

    <n-empty v-if="clients.length === 0" description="暂无客户端连接" />

    <n-grid v-else :cols="3" :x-gap="16" :y-gap="16" responsive="screen" cols-s="1" cols-m="2">
      <n-gi v-for="client in clients" :key="client.id">
        <n-card hoverable class="client-card" @click="viewClient(client.id)">
          <n-space justify="space-between" align="center">
            <div>
              <h3 class="client-name">{{ client.nickname || client.id }}</h3>
              <p v-if="client.nickname" class="client-id">{{ client.id }}</p>
              <p v-if="client.remote_addr && client.online" class="client-ip">IP: {{ client.remote_addr }}</p>
              <n-space style="margin-top: 8px;">
                <n-tag :type="client.online ? 'success' : 'default'" size="small">
                  {{ client.online ? '在线' : '离线' }}
                </n-tag>
                <n-tag type="info" size="small">{{ client.rule_count }} 条规则</n-tag>
              </n-space>
            </div>
            <n-button size="small" @click.stop="viewClient(client.id)">查看详情</n-button>
          </n-space>
        </n-card>
      </n-gi>
    </n-grid>
  </div>
</template>

<style scoped>
.home {
  max-width: 1200px;
  margin: 0 auto;
}

.page-header {
  margin-bottom: 24px;
}

.page-header h2 {
  margin: 0 0 8px 0;
  font-size: 24px;
  font-weight: 600;
  color: #1f2937;
}

.page-header p {
  margin: 0;
  color: #6b7280;
}

.stat-card {
  text-align: center;
}

.client-card {
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s;
}

.client-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.client-name {
  margin: 0 0 4px 0;
  font-size: 16px;
  font-weight: 600;
  color: #1f2937;
}

.client-id {
  margin: 0 0 4px 0;
  color: #9ca3af;
  font-size: 12px;
}

.client-ip {
  margin: 0;
  color: #6b7280;
  font-size: 12px;
}
</style>
