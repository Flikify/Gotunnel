<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { RouterView } from 'vue-router'
import { getServerStatus } from './api'

const serverInfo = ref({ bind_addr: '', bind_port: 0 })
const clientCount = ref(0)

onMounted(async () => {
  try {
    const { data } = await getServerStatus()
    serverInfo.value = data.server
    clientCount.value = data.client_count
  } catch (e) {
    console.error('Failed to get server status', e)
  }
})
</script>

<template>
  <div class="app">
    <header class="header">
      <h1>GoTunnel 控制台</h1>
      <div class="server-info">
        <span>{{ serverInfo.bind_addr }}:{{ serverInfo.bind_port }}</span>
        <span class="badge">{{ clientCount }} 客户端</span>
      </div>
    </header>
    <main class="main">
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.app { min-height: 100vh; background: #f5f7fa; }
.header {
  background: #fff;
  padding: 16px 24px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  box-shadow: 0 2px 4px rgba(0,0,0,0.1);
}
.header h1 { font-size: 20px; color: #2c3e50; }
.server-info { display: flex; align-items: center; gap: 12px; color: #666; }
.badge {
  background: #3498db;
  color: #fff;
  padding: 4px 12px;
  border-radius: 12px;
  font-size: 12px;
}
.main { padding: 24px; max-width: 1200px; margin: 0 auto; }
</style>
