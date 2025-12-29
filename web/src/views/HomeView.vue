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
    <div style="margin-bottom: 24px;">
      <h2 style="margin: 0 0 8px 0;">客户端管理</h2>
      <p style="margin: 0; color: #666;">查看已连接的隧道客户端</p>
    </div>

    <n-grid :cols="3" :x-gap="16" :y-gap="16" style="margin-bottom: 24px;">
      <n-gi>
        <n-card>
          <n-statistic label="总客户端" :value="clients.length" />
        </n-card>
      </n-gi>
      <n-gi>
        <n-card>
          <n-statistic label="在线客户端" :value="onlineClients" />
        </n-card>
      </n-gi>
      <n-gi>
        <n-card>
          <n-statistic label="总规则数" :value="totalRules" />
        </n-card>
      </n-gi>
    </n-grid>

    <n-empty v-if="clients.length === 0" description="暂无客户端连接" />

    <n-grid v-else :cols="3" :x-gap="16" :y-gap="16" responsive="screen" cols-s="1" cols-m="2">
      <n-gi v-for="client in clients" :key="client.id">
        <n-card hoverable style="cursor: pointer;" @click="viewClient(client.id)">
          <n-space justify="space-between" align="center">
            <div>
              <h3 style="margin: 0 0 4px 0;">{{ client.nickname || client.id }}</h3>
              <p v-if="client.nickname" style="margin: 0 0 4px 0; color: #999; font-size: 12px;">{{ client.id }}</p>
              <p v-if="client.remote_addr && client.online" style="margin: 0 0 8px 0; color: #666; font-size: 12px;">IP: {{ client.remote_addr }}</p>
              <n-space>
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
