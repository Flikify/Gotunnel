<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getClient, updateClient, deleteClient } from '../api'
import type { ProxyRule } from '../types'

const route = useRoute()
const router = useRouter()
const clientId = route.params.id as string

const online = ref(false)
const lastPing = ref('')
const rules = ref<ProxyRule[]>([])
const editing = ref(false)
const editRules = ref<ProxyRule[]>([])

const loadClient = async () => {
  try {
    const { data } = await getClient(clientId)
    online.value = data.online
    lastPing.value = data.last_ping || ''
    rules.value = data.rules || []
  } catch (e) {
    console.error('Failed to load client', e)
  }
}

onMounted(loadClient)

const startEdit = () => {
  editRules.value = JSON.parse(JSON.stringify(rules.value))
  editing.value = true
}

const cancelEdit = () => {
  editing.value = false
}

const addRule = () => {
  editRules.value.push({
    name: '', local_ip: '127.0.0.1', local_port: 80, remote_port: 8080
  })
}

const removeRule = (index: number) => {
  editRules.value.splice(index, 1)
}

const saveEdit = async () => {
  try {
    await updateClient(clientId, { id: clientId, rules: editRules.value })
    editing.value = false
    loadClient()
  } catch (e) {
    alert('保存失败')
  }
}

const confirmDelete = async () => {
  if (!confirm('确定删除此客户端?')) return
  try {
    await deleteClient(clientId)
    router.push('/')
  } catch (e) {
    alert('删除失败')
  }
}
</script>

<template>
  <div class="client-view">
    <div class="header">
      <button class="btn" @click="router.push('/')">← 返回</button>
      <h2>{{ clientId }}</h2>
      <span :class="['status-badge', online ? 'online' : 'offline']">
        {{ online ? '在线' : '离线' }}
      </span>
    </div>

    <div v-if="lastPing" class="ping-info">最后心跳: {{ lastPing }}</div>

    <div class="rules-section">
      <div class="section-header">
        <h3>代理规则</h3>
        <div v-if="!editing">
          <button class="btn primary" @click="startEdit">编辑</button>
          <button class="btn danger" @click="confirmDelete">删除</button>
        </div>
      </div>

      <!-- 查看模式 -->
      <table v-if="!editing" class="rules-table">
        <thead>
          <tr>
            <th>名称</th>
            <th>本地地址</th>
            <th>远程端口</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="rule in rules" :key="rule.name">
            <td>{{ rule.name }}</td>
            <td>{{ rule.local_ip }}:{{ rule.local_port }}</td>
            <td>{{ rule.remote_port }}</td>
          </tr>
        </tbody>
      </table>

      <!-- 编辑模式 -->
      <div v-if="editing" class="edit-form">
        <div v-for="(rule, i) in editRules" :key="i" class="rule-row">
          <input v-model="rule.name" placeholder="名称" />
          <input v-model="rule.local_ip" placeholder="本地IP" />
          <input v-model.number="rule.local_port" type="number" placeholder="本地端口" />
          <input v-model.number="rule.remote_port" type="number" placeholder="远程端口" />
          <button class="btn-icon" @click="removeRule(i)">×</button>
        </div>
        <button class="btn secondary" @click="addRule">+ 添加规则</button>
        <div class="edit-actions">
          <button class="btn" @click="cancelEdit">取消</button>
          <button class="btn primary" @click="saveEdit">保存</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.header {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 20px;
}
.header h2 { margin: 0; }
.status-badge {
  padding: 4px 12px;
  border-radius: 12px;
  font-size: 12px;
}
.status-badge.online { background: #d4edda; color: #155724; }
.status-badge.offline { background: #f8d7da; color: #721c24; }
.ping-info { color: #666; margin-bottom: 20px; }
.rules-section {
  background: #fff;
  border-radius: 8px;
  padding: 20px;
  box-shadow: 0 2px 4px rgba(0,0,0,0.1);
}
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}
.section-header h3 { margin: 0; }
.section-header .btn { margin-left: 8px; }
.btn {
  padding: 8px 16px;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 14px;
}
.btn.primary { background: #3498db; color: #fff; }
.btn.secondary { background: #95a5a6; color: #fff; }
.btn.danger { background: #e74c3c; color: #fff; }
.rules-table {
  width: 100%;
  border-collapse: collapse;
}
.rules-table th, .rules-table td {
  padding: 12px;
  text-align: left;
  border-bottom: 1px solid #eee;
}
.rules-table th { font-weight: 600; }
.rule-row {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
}
.rule-row input {
  flex: 1;
  padding: 8px;
  border: 1px solid #ddd;
  border-radius: 4px;
}
.btn-icon {
  background: #e74c3c;
  color: #fff;
  border: none;
  border-radius: 4px;
  width: 32px;
  cursor: pointer;
}
.edit-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 16px;
}
</style>
