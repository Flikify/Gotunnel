<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { NCard, NSpace, NButton, NSelect, NSwitch, NInput, NIcon, NEmpty, NSpin } from 'naive-ui'
import { PlayOutline, StopOutline, TrashOutline, DownloadOutline } from '@vicons/ionicons5'
import { createLogStream } from '../api'
import type { LogEntry } from '../types'

const props = defineProps<{
  clientId: string
  visible: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const logs = ref<LogEntry[]>([])
const isStreaming = ref(false)
const autoScroll = ref(true)
const levelFilter = ref<string>('')
const searchText = ref('')
const loading = ref(false)

let eventSource: EventSource | null = null
const logContainer = ref<HTMLElement | null>(null)

const levelOptions = [
  { label: '所有级别', value: '' },
  { label: 'Info', value: 'info' },
  { label: 'Warning', value: 'warn' },
  { label: 'Error', value: 'error' },
  { label: 'Debug', value: 'debug' }
]

const startStream = () => {
  if (eventSource) {
    eventSource.close()
  }

  loading.value = true
  isStreaming.value = true

  eventSource = createLogStream(
    props.clientId,
    { lines: 500, follow: true, level: levelFilter.value },
    (entry) => {
      logs.value.push(entry)
      // 限制内存中的日志数量
      if (logs.value.length > 2000) {
        logs.value = logs.value.slice(-1000)
      }
      if (autoScroll.value) {
        nextTick(() => scrollToBottom())
      }
      loading.value = false
    },
    () => {
      isStreaming.value = false
      loading.value = false
    }
  )
}

const stopStream = () => {
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
  isStreaming.value = false
}

const clearLogs = () => {
  logs.value = []
}

const scrollToBottom = () => {
  if (logContainer.value) {
    logContainer.value.scrollTop = logContainer.value.scrollHeight
  }
}

const downloadLogs = () => {
  const content = logs.value.map(l =>
    `${new Date(l.ts).toISOString()} [${l.level.toUpperCase()}] [${l.src}] ${l.msg}`
  ).join('\n')

  const blob = new Blob([content], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${props.clientId}-logs-${new Date().toISOString().slice(0, 10)}.txt`
  a.click()
  URL.revokeObjectURL(url)
}

const filteredLogs = computed(() => {
  if (!searchText.value) return logs.value
  const search = searchText.value.toLowerCase()
  return logs.value.filter(l => l.msg.toLowerCase().includes(search))
})

const getLevelColor = (level: string): string => {
  switch (level) {
    case 'error': return '#e88080'
    case 'warn': return '#e8b880'
    case 'info': return '#80b8e8'
    case 'debug': return '#808080'
    default: return '#ffffff'
  }
}

const formatTime = (ts: number): string => {
  return new Date(ts).toLocaleTimeString('en-US', { hour12: false })
}

watch(() => props.visible, (visible) => {
  if (visible) {
    startStream()
  } else {
    stopStream()
  }
})

onMounted(() => {
  if (props.visible) {
    startStream()
  }
})

onUnmounted(() => {
  stopStream()
})
</script>

<template>
  <n-card title="客户端日志" :closable="true" @close="emit('close')">
    <template #header-extra>
      <n-space :size="8">
        <n-select
          v-model:value="levelFilter"
          :options="levelOptions"
          size="small"
          style="width: 110px;"
          @update:value="() => { stopStream(); logs = []; startStream(); }"
        />
        <n-input
          v-model:value="searchText"
          placeholder="搜索..."
          size="small"
          style="width: 120px;"
          clearable
        />
        <n-switch v-model:value="autoScroll" size="small">
          <template #checked>自动滚动</template>
          <template #unchecked>手动</template>
        </n-switch>
        <n-button size="small" quaternary @click="clearLogs">
          <template #icon><n-icon><TrashOutline /></n-icon></template>
        </n-button>
        <n-button size="small" quaternary @click="downloadLogs">
          <template #icon><n-icon><DownloadOutline /></n-icon></template>
        </n-button>
        <n-button
          size="small"
          :type="isStreaming ? 'error' : 'success'"
          @click="isStreaming ? stopStream() : startStream()"
        >
          <template #icon>
            <n-icon><StopOutline v-if="isStreaming" /><PlayOutline v-else /></n-icon>
          </template>
          {{ isStreaming ? '停止' : '开始' }}
        </n-button>
      </n-space>
    </template>

    <n-spin :show="loading && logs.length === 0">
      <div
        ref="logContainer"
        class="log-container"
      >
        <n-empty v-if="filteredLogs.length === 0" description="暂无日志" />
        <div
          v-for="(log, i) in filteredLogs"
          :key="i"
          class="log-line"
        >
          <span class="log-time">{{ formatTime(log.ts) }}</span>
          <span class="log-level" :style="{ color: getLevelColor(log.level) }">[{{ log.level.toUpperCase() }}]</span>
          <span class="log-src">[{{ log.src }}]</span>
          <span class="log-msg">{{ log.msg }}</span>
        </div>
      </div>
    </n-spin>
  </n-card>
</template>

<style scoped>
.log-container {
  height: 400px;
  overflow-y: auto;
  background: #1e1e1e;
  padding: 8px;
  font-family: 'Consolas', 'Monaco', monospace;
  font-size: 12px;
  border-radius: 4px;
}

.log-line {
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
  color: #d4d4d4;
}

.log-time {
  color: #808080;
  margin-right: 8px;
}

.log-level {
  margin-right: 8px;
}

.log-src {
  color: #a0a0a0;
  margin-right: 8px;
}

.log-msg {
  color: #d4d4d4;
}
</style>
