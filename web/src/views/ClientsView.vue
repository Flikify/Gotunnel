<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import GlassModal from '../components/GlassModal.vue'
import MetricCard from '../components/MetricCard.vue'
import PageShell from '../components/PageShell.vue'
import SectionCard from '../components/SectionCard.vue'
import { generateInstallCommand, getClients } from '../api'
import { useToast } from '../composables/useToast'
import type { ClientStatus, InstallCommandResponse } from '../types'

const router = useRouter()
const message = useToast()
const clients = ref<ClientStatus[]>([])
const loading = ref(true)
const showInstallModal = ref(false)
const installData = ref<InstallCommandResponse | null>(null)
const generatingInstall = ref(false)
const search = ref('')
const installScriptUrl = 'https://raw.githubusercontent.com/gotunnel/gotunnel/main/scripts/install.sh'
const installPs1Url = 'https://raw.githubusercontent.com/gotunnel/gotunnel/main/scripts/install.ps1'

const quoteShellArg = (value: string) => `'${value.replace(/'/g, `'\"'\"'`)}'`

const resolveTunnelHost = () => window.location.hostname || 'localhost'

const formatServerAddr = (host: string, port: number) => {
  const normalizedHost = host.includes(':') && !host.startsWith('[') ? `[${host}]` : host
  return `${normalizedHost}:${port}`
}

const buildInstallCommands = (data: InstallCommandResponse) => {
  const serverAddr = formatServerAddr(resolveTunnelHost(), data.tunnel_port)

  return {
    linux: `bash <(curl -fsSL ${installScriptUrl}) -s ${quoteShellArg(serverAddr)} -t ${quoteShellArg(data.token)}`,
    macos: `bash <(curl -fsSL ${installScriptUrl}) -s ${quoteShellArg(serverAddr)} -t ${quoteShellArg(data.token)}`,
    windows: `powershell -c \"irm ${installPs1Url} | iex; Install-GoTunnel -Server '${serverAddr}' -Token '${data.token}'\"`,
  }
}

const loadClients = async () => {
  loading.value = true
  try {
    const { data } = await getClients()
    clients.value = data || []
  } catch (error) {
    console.error('Failed to load clients', error)
    message.error('客户端列表加载失败')
  } finally {
    loading.value = false
  }
}

const openInstallModal = async () => {
  generatingInstall.value = true
  try {
    const { data } = await generateInstallCommand()
    installData.value = data
    showInstallModal.value = true
  } catch (error) {
    console.error('Failed to generate install command', error)
    message.error('安装命令生成失败')
  } finally {
    generatingInstall.value = false
  }
}

const copyCommand = async (command: string) => {
  try {
    await navigator.clipboard.writeText(command)
    message.success('命令已复制')
  } catch (error) {
    console.error('Failed to copy command', error)
    message.error('复制失败，请手动复制')
  }
}

const filteredClients = computed(() => {
  const keyword = search.value.trim().toLowerCase()
  if (!keyword) return clients.value
  return clients.value.filter((client) => {
    return [client.id, client.nickname, client.remote_addr, client.os, client.arch]
      .filter(Boolean)
      .some((value) => String(value).toLowerCase().includes(keyword))
  })
})

const onlineClients = computed(() => clients.value.filter((client) => client.online).length)
const offlineClients = computed(() => Math.max(clients.value.length - onlineClients.value, 0))
const installCommands = computed(() => (installData.value ? buildInstallCommands(installData.value) : null))

onMounted(loadClients)
</script>

<template>
  <PageShell title="客户端" eyebrow="Clients" subtitle="统一管理已注册节点、连接状态与快速安装命令，减少操作跳转。">
    <template #actions>
      <button class="glass-btn" :disabled="generatingInstall" @click="openInstallModal">
        {{ generatingInstall ? '生成中...' : '安装命令' }}
      </button>
      <button class="glass-btn primary" @click="loadClients">{{ loading ? '刷新中...' : '刷新列表' }}</button>
    </template>

    <template #metrics>
      <MetricCard label="客户端总数" :value="clients.length" hint="已接入的全部节点" />
      <MetricCard label="在线节点" :value="onlineClients" hint="可立即推送配置" tone="success" />
      <MetricCard label="离线节点" :value="offlineClients" hint="等待心跳恢复" tone="warning" />
      <MetricCard label="当前筛选结果" :value="filteredClients.length" hint="支持 ID / 昵称 / 地址搜索" tone="info" />
    </template>

    <SectionCard title="节点列表" description="使用统一卡片样式展示连接信息，便于快速判断状态与进入详情页。">
      <template #header>
        <input v-model="search" class="glass-input search-input" type="search" placeholder="搜索 ID / 昵称 / 地址" />
      </template>

      <div v-if="loading" class="empty-state">正在加载客户端列表...</div>
      <div v-else-if="filteredClients.length === 0" class="empty-state">未找到匹配的客户端。</div>
      <div v-else class="client-grid">
        <article v-for="client in filteredClients" :key="client.id" class="client-card" @click="router.push(`/client/${client.id}`)">
          <div class="client-card__header">
            <div>
              <div class="client-card__title">
                <span class="status-dot" :class="{ online: client.online }"></span>
                <strong>{{ client.nickname || client.id }}</strong>
              </div>
              <p>{{ client.nickname ? client.id : client.remote_addr || '等待首次连接' }}</p>
            </div>
            <span class="state-pill" :class="client.online ? 'online' : 'offline'">{{ client.online ? '在线' : '离线' }}</span>
          </div>

          <dl class="client-card__meta">
            <div>
              <dt>地址</dt>
              <dd>{{ client.remote_addr || '未上报' }}</dd>
            </div>
            <div>
              <dt>规则数</dt>
              <dd>{{ client.rule_count || 0 }}</dd>
            </div>
            <div>
              <dt>平台</dt>
              <dd>{{ [client.os, client.arch].filter(Boolean).join(' / ') || '未知' }}</dd>
            </div>
          </dl>
        </article>
      </div>
    </SectionCard>

    <GlassModal :show="showInstallModal" title="安装命令" width="760px" @close="showInstallModal = false">
      <div v-if="installCommands" class="install-grid">
        <article v-for="item in [
          { label: 'Linux', value: installCommands.linux },
          { label: 'macOS', value: installCommands.macos },
          { label: 'Windows', value: installCommands.windows },
        ]" :key="item.label" class="install-card">
          <header>
            <strong>{{ item.label }}</strong>
            <button class="glass-btn small" @click="copyCommand(item.value)">复制</button>
          </header>
          <code>{{ item.value }}</code>
        </article>
      </div>
      <template #footer>
        <span class="install-footnote">命令内含一次性 token，使用后请重新生成。</span>
      </template>
    </GlassModal>
  </PageShell>
</template>

<style scoped>
.search-input {
  min-width: min(320px, 100%);
}

.client-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
}

.client-card {
  padding: 18px;
  border-radius: 18px;
  background: var(--glass-bg-light);
  border: 1px solid var(--color-border-light);
  cursor: pointer;
  transition: transform 0.2s ease, border-color 0.2s ease, box-shadow 0.2s ease;
}

.client-card:hover {
  transform: translateY(-2px);
  border-color: rgba(59, 130, 246, 0.24);
  box-shadow: var(--shadow-md);
}

.client-card__header {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 18px;
}

.client-card__title {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}

.client-card__header p {
  color: var(--color-text-secondary);
  font-size: 13px;
}

.status-dot {
  width: 10px;
  height: 10px;
  border-radius: 999px;
  background: var(--color-error);
}

.status-dot.online {
  background: var(--color-success);
}

.state-pill {
  height: fit-content;
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

.client-card__meta {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.client-card__meta dt {
  margin-bottom: 6px;
  color: var(--color-text-muted);
  font-size: 12px;
}

.client-card__meta dd {
  color: var(--color-text-primary);
  font-size: 13px;
  word-break: break-word;
}

.install-grid {
  display: grid;
  gap: 14px;
}

.install-card {
  padding: 16px;
  border-radius: 16px;
  background: var(--glass-bg-light);
  border: 1px solid var(--color-border-light);
}

.install-card header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.install-card code {
  display: block;
  color: var(--color-text-primary);
  white-space: pre-wrap;
  word-break: break-all;
  font-size: 12px;
  line-height: 1.7;
}

.install-footnote {
  color: var(--color-text-secondary);
  font-size: 12px;
}

.empty-state {
  padding: 48px 20px;
  text-align: center;
  color: var(--color-text-secondary);
  background: var(--glass-bg-light);
  border: 1px dashed var(--color-border);
  border-radius: 16px;
}

@media (max-width: 768px) {
  .client-card__header {
    flex-direction: column;
  }

  .client-card__meta {
    grid-template-columns: 1fr;
  }
}
</style>
