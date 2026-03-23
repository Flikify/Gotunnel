<script setup lang="ts">
import { onMounted, ref } from 'vue'
import MetricCard from '../components/MetricCard.vue'
import PageShell from '../components/PageShell.vue'
import SectionCard from '../components/SectionCard.vue'
import {
  getServerConfig,
  getVersionInfo,
  updateServerConfig,
  type ConfigUpdateResult,
  type ServerConfigResponse,
  type UpdateServerConfigRequest,
  type VersionInfo,
} from '../api'
import { useToast } from '../composables/useToast'

const message = useToast()
const versionInfo = ref<VersionInfo | null>(null)
const serverConfig = ref<ServerConfigResponse | null>(null)
const loadingVersion = ref(true)
const loadingConfig = ref(true)
const saving = ref(false)

const configForm = ref({
  heartbeat_sec: 30,
  heartbeat_timeout: 90,
  max_client_proxies: 0,
  client_response_timeout_sec: 15,
  web_username: '',
  web_password: '',
})

const loadVersionInfo = async () => {
  loadingVersion.value = true
  try {
    const { data } = await getVersionInfo()
    versionInfo.value = data
  } catch (error) {
    console.error('Failed to load version info', error)
  } finally {
    loadingVersion.value = false
  }
}

const loadServerConfig = async () => {
  loadingConfig.value = true
  try {
    const { data } = await getServerConfig()
    serverConfig.value = data
    configForm.value = {
      heartbeat_sec: data.server.heartbeat_sec,
      heartbeat_timeout: data.server.heartbeat_timeout,
      max_client_proxies: data.server.max_client_proxies,
      client_response_timeout_sec: data.server.client_response_timeout_sec,
      web_username: data.web.username,
      web_password: '',
    }
  } catch (error) {
    console.error('Failed to load server config', error)
    message.error('服务器配置加载失败')
  } finally {
    loadingConfig.value = false
  }
}

const handleSaveConfig = async () => {
  if (configForm.value.heartbeat_timeout < configForm.value.heartbeat_sec) {
    message.error('心跳超时必须大于或等于心跳间隔')
    return
  }
  if (configForm.value.max_client_proxies < 0) {
    message.error('单客户端最大代理数不能小于 0')
    return
  }

  saving.value = true
  try {
    const payload: UpdateServerConfigRequest = {
      server: {
        heartbeat_sec: configForm.value.heartbeat_sec,
        heartbeat_timeout: configForm.value.heartbeat_timeout,
        max_client_proxies: configForm.value.max_client_proxies,
        client_response_timeout_sec: configForm.value.client_response_timeout_sec,
      },
      web: {
        username: configForm.value.web_username,
      },
    }

    if (configForm.value.web_password) {
      payload.web = {
        ...payload.web,
        password: configForm.value.web_password,
      }
    }

    const { data } = await updateServerConfig(payload)
    configForm.value.web_password = ''
    const restartFields = (data as ConfigUpdateResult).restart_required_fields || []
    if (restartFields.length > 0) {
      message.success(`配置已保存，以下变更需重启生效：${restartFields.join(', ')}`)
      return
    }
    message.success('配置已保存并同步了可热生效项')
  } catch (error: any) {
    message.error(error.response?.data?.message || '保存配置失败')
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  loadVersionInfo()
  loadServerConfig()
})
</script>

<template>
  <PageShell title="系统设置" eyebrow="Settings" subtitle="统一整理运行版本与服务配置，减少样式重复并保留关键运维操作。">
    <template #actions>
      <button class="glass-btn" @click="loadVersionInfo">刷新版本</button>
      <button class="glass-btn primary" :disabled="saving" @click="handleSaveConfig">{{ saving ? '保存中...' : '保存配置' }}</button>
    </template>

    <template #metrics>
      <MetricCard label="当前版本" :value="versionInfo?.version || '—'" :hint="versionInfo?.git_commit?.slice(0, 8) || '未知提交'" />
      <MetricCard label="Go 版本" :value="versionInfo?.go_version || '—'" hint="运行时版本" tone="info" />
      <MetricCard label="运行平台" :value="versionInfo ? `${versionInfo.os}/${versionInfo.arch}` : '—'" hint="服务端当前平台" tone="success" />
      <MetricCard label="单客户端代理上限" :value="configForm.max_client_proxies === 0 ? '无限制' : String(configForm.max_client_proxies)" hint="0 表示不限制" tone="warning" />
    </template>

    <div class="settings-grid">
      <SectionCard title="版本信息" description="查看当前服务端构建信息，方便排查环境与升级状态。">
        <div v-if="loadingVersion" class="empty-state">正在加载版本信息...</div>
        <dl v-else-if="versionInfo" class="info-grid">
          <div><dt>版本号</dt><dd>{{ versionInfo.version }}</dd></div>
          <div><dt>Git 提交</dt><dd>{{ versionInfo.git_commit || 'N/A' }}</dd></div>
          <div><dt>构建时间</dt><dd>{{ versionInfo.build_time || 'N/A' }}</dd></div>
          <div><dt>Go 版本</dt><dd>{{ versionInfo.go_version }}</dd></div>
          <div><dt>操作系统</dt><dd>{{ versionInfo.os }}</dd></div>
          <div><dt>架构</dt><dd>{{ versionInfo.arch }}</dd></div>
        </dl>
        <div v-else class="empty-state">无法获取版本信息。</div>
      </SectionCard>

      <SectionCard title="服务配置" description="统一配置心跳、客户端响应超时和单客户端代理上限。带运行时效果的选项保存后会立即生效，Web 登录凭据仍需重启服务端。">
        <div v-if="loadingConfig" class="empty-state">正在加载服务器配置...</div>
        <form v-else class="config-form" @submit.prevent="handleSaveConfig">
          <label class="form-group">
            <span>心跳间隔（秒）</span>
            <input v-model.number="configForm.heartbeat_sec" class="glass-input" min="1" max="300" type="number" />
          </label>
          <label class="form-group">
            <span>心跳超时（秒）</span>
            <input v-model.number="configForm.heartbeat_timeout" class="glass-input" min="1" max="3600" type="number" />
          </label>
          <label class="form-group">
            <span>客户端响应超时（秒）</span>
            <input v-model.number="configForm.client_response_timeout_sec" class="glass-input" min="1" max="300" type="number" />
            <small class="form-hint">外部用户连入代理后，服务端等待客户端建立本地连接并返回结果的最长时间。</small>
          </label>
          <label class="form-group">
            <span>单客户端最大代理数</span>
            <input v-model.number="configForm.max_client_proxies" class="glass-input" min="0" max="10000" type="number" />
            <small class="form-hint">`0` 表示无限制。超过上限时，保存客户端规则会被拒绝。</small>
          </label>
          <label class="form-group form-group--full">
            <span>Web 用户名</span>
            <input v-model="configForm.web_username" class="glass-input" type="text" placeholder="admin" />
          </label>
          <label class="form-group form-group--full">
            <span>Web 密码</span>
            <input v-model="configForm.web_password" class="glass-input" type="password" placeholder="留空则保持不变" />
          </label>
        </form>
      </SectionCard>
    </div>
  </PageShell>
</template>

<style scoped>
.settings-grid {
  display: grid;
  gap: 20px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.info-grid {
  display: grid;
  gap: 14px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.info-grid div,
.form-group {
  padding: 16px;
  border-radius: 16px;
  background: var(--glass-bg-light);
  border: 1px solid var(--color-border-light);
}

.info-grid dt,
.form-group span {
  display: block;
  margin-bottom: 8px;
  color: var(--color-text-secondary);
  font-size: 13px;
}

.info-grid dd {
  color: var(--color-text-primary);
  word-break: break-word;
}

.config-form {
  display: grid;
  gap: 14px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.form-hint {
  color: var(--color-text-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.form-group--full {
  grid-column: 1 / -1;
}

.empty-state {
  padding: 48px 20px;
  text-align: center;
  color: var(--color-text-secondary);
  background: var(--glass-bg-light);
  border: 1px dashed var(--color-border);
  border-radius: 16px;
}

@media (max-width: 960px) {
  .settings-grid,
  .info-grid,
  .config-form {
    grid-template-columns: 1fr;
  }
}
</style>
