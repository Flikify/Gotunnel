<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { getClients, addClient } from '../api'
import type { ClientStatus, ProxyRule } from '../types'

const router = useRouter()
const clients = ref<ClientStatus[]>([])
const showModal = ref(false)
const newClientId = ref('')
const newRules = ref<ProxyRule[]>([])

const loadClients = async () => {
  try {
    const { data } = await getClients()
    clients.value = data || []
  } catch (e) {
    console.error('Failed to load clients', e)
  }
}

onMounted(loadClients)

const openAddModal = () => {
  newClientId.value = ''
  newRules.value = [{ name: '', local_ip: '127.0.0.1', local_port: 80, remote_port: 8080 }]
  showModal.value = true
}

const addRule = () => {
  newRules.value.push({ name: '', local_ip: '127.0.0.1', local_port: 80, remote_port: 8080 })
}

const removeRule = (index: number) => {
  newRules.value.splice(index, 1)
}

const saveClient = async () => {
  if (!newClientId.value) return
  try {
    await addClient({ id: newClientId.value, rules: newRules.value })
    showModal.value = false
    loadClients()
  } catch (e) {
    alert('添加失败')
  }
}

const viewClient = (id: string) => {
  router.push(`/client/${id}`)
}
</script>

<template>
  <div class="home">
    <div class="toolbar">
      <h2>客户端列表</h2>
      <button class="btn primary" @click="openAddModal">添加客户端</button>
    </div>

    <div class="client-grid">
      <div v-for="client in clients" :key="client.id" class="client-card" @click="viewClient(client.id)">
        <div class="card-header">
          <span class="client-id">{{ client.id }}</span>
          <span :class="['status', client.online ? 'online' : 'offline']"></span>
        </div>
        <div class="card-info">
          <span>{{ client.rule_count }} 条规则</span>
        </div>
      </div>
    </div>

    <div v-if="clients.length === 0" class="empty">暂无客户端配置</div>

    <!-- 添加客户端模态框 -->
    <div v-if="showModal" class="modal-overlay" @click.self="showModal = false">
      <div class="modal">
        <h3>添加客户端</h3>
        <div class="form-group">
          <label>客户端 ID</label>
          <input v-model="newClientId" placeholder="例如: client-a" />
        </div>
        <div class="form-group">
          <label>代理规则</label>
          <div v-for="(rule, i) in newRules" :key="i" class="rule-row">
            <input v-model="rule.name" placeholder="名称" />
            <input v-model="rule.local_ip" placeholder="本地IP" />
            <input v-model.number="rule.local_port" type="number" placeholder="本地端口" />
            <input v-model.number="rule.remote_port" type="number" placeholder="远程端口" />
            <button class="btn-icon" @click="removeRule(i)">×</button>
          </div>
          <button class="btn secondary" @click="addRule">+ 添加规则</button>
        </div>
        <div class="modal-actions">
          <button class="btn" @click="showModal = false">取消</button>
          <button class="btn primary" @click="saveClient">保存</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}
.toolbar h2 { font-size: 18px; color: #2c3e50; }
.btn {
  padding: 8px 16px;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 14px;
}
.btn.primary { background: #3498db; color: #fff; }
.btn.secondary { background: #95a5a6; color: #fff; }
.client-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}
.client-card {
  background: #fff;
  border-radius: 8px;
  padding: 16px;
  cursor: pointer;
  box-shadow: 0 2px 4px rgba(0,0,0,0.1);
  transition: transform 0.2s;
}
.client-card:hover { transform: translateY(-2px); }
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}
.client-id { font-weight: 600; }
.status { width: 10px; height: 10px; border-radius: 50%; }
.status.online { background: #27ae60; }
.status.offline { background: #95a5a6; }
.card-info { font-size: 14px; color: #666; }
.empty { text-align: center; color: #999; padding: 40px; }
.modal-overlay {
  position: fixed;
  top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,0.5);
  display: flex;
  align-items: center;
  justify-content: center;
}
.modal {
  background: #fff;
  border-radius: 8px;
  padding: 24px;
  width: 500px;
  max-width: 90%;
}
.modal h3 { margin-bottom: 16px; }
.form-group { margin-bottom: 16px; }
.form-group label { display: block; margin-bottom: 8px; font-weight: 500; }
.form-group input {
  width: 100%;
  padding: 8px;
  border: 1px solid #ddd;
  border-radius: 4px;
  box-sizing: border-box;
}
.rule-row {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
}
.rule-row input { flex: 1; width: auto; }
.btn-icon {
  background: #e74c3c;
  color: #fff;
  border: none;
  border-radius: 4px;
  width: 32px;
  cursor: pointer;
}
.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 16px;
}
</style>
