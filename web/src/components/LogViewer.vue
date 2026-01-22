<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import {
  PlayOutline, StopOutline, TrashOutline, DownloadOutline, CloseOutline
} from '@vicons/ionicons5'
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
  <div v-if="visible" class="log-overlay" @click.self="emit('close')">
    <div class="log-modal">
      <!-- Header -->
      <div class="log-header">
        <h3>客户端日志</h3>
        <div class="log-controls">
          <select v-model="levelFilter" class="log-select" @change="() => { stopStream(); logs = []; startStream(); }">
            <option value="">所有级别</option>
            <option value="info">Info</option>
            <option value="warn">Warning</option>
            <option value="error">Error</option>
            <option value="debug">Debug</option>
          </select>
          <input v-model="searchText" type="text" class="log-input" placeholder="搜索..." />
          <label class="log-toggle">
            <input type="checkbox" v-model="autoScroll" />
            <span>自动滚动</span>
          </label>
          <button class="icon-btn" @click="clearLogs" title="清空">
            <TrashOutline />
          </button>
          <button class="icon-btn" @click="downloadLogs" title="下载">
            <DownloadOutline />
          </button>
          <button class="action-btn" :class="isStreaming ? 'danger' : 'success'" @click="isStreaming ? stopStream() : startStream()">
            <StopOutline v-if="isStreaming" />
            <PlayOutline v-else />
            <span>{{ isStreaming ? '停止' : '开始' }}</span>
          </button>
        </div>
        <button class="close-btn" @click="emit('close')">
          <CloseOutline />
        </button>
      </div>

      <!-- Content -->
      <div class="log-body">
        <div v-if="loading && logs.length === 0" class="log-loading">加载中...</div>
        <div ref="logContainer" class="log-container">
          <div v-if="filteredLogs.length === 0" class="log-empty">暂无日志</div>
          <div v-for="(log, i) in filteredLogs" :key="i" class="log-line">
            <span class="log-time">{{ formatTime(log.ts) }}</span>
            <span class="log-level" :style="{ color: getLevelColor(log.level) }">[{{ log.level.toUpperCase() }}]</span>
            <span class="log-src">[{{ log.src }}]</span>
            <span class="log-msg">{{ log.msg }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* Overlay */
.log-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 24px;
}

/* Modal */
.log-modal {
  width: 100%;
  max-width: 900px;
  max-height: 80vh;
  background: rgba(30, 27, 75, 0.95);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border-radius: 16px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* Header */
.log-header {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 16px 20px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.log-header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: white;
  white-space: nowrap;
}

.log-controls {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

/* Select */
.log-select {
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 6px;
  padding: 6px 12px;
  color: white;
  font-size: 13px;
  cursor: pointer;
  outline: none;
}

.log-select option {
  background: #1e1b4b;
  color: white;
}

/* Input */
.log-input {
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 6px;
  padding: 6px 12px;
  color: white;
  font-size: 13px;
  width: 150px;
  outline: none;
}

.log-input::placeholder {
  color: rgba(255, 255, 255, 0.4);
}

.log-input:focus {
  border-color: rgba(167, 139, 250, 0.5);
}

/* Toggle */
.log-toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  color: rgba(255, 255, 255, 0.7);
  font-size: 13px;
  cursor: pointer;
  white-space: nowrap;
}

.log-toggle input[type="checkbox"] {
  width: 16px;
  height: 16px;
  accent-color: #a78bfa;
}

/* Icon Button */
.icon-btn {
  background: rgba(255, 255, 255, 0.1);
  border: none;
  border-radius: 6px;
  padding: 6px;
  color: rgba(255, 255, 255, 0.7);
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
}

.icon-btn:hover {
  background: rgba(255, 255, 255, 0.2);
  color: white;
}

.icon-btn svg {
  width: 18px;
  height: 18px;
}

/* Action Button */
.action-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  background: rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 6px;
  padding: 6px 12px;
  color: white;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s;
}

.action-btn:hover {
  background: rgba(255, 255, 255, 0.2);
}

.action-btn svg {
  width: 16px;
  height: 16px;
}

.action-btn.success {
  background: rgba(52, 211, 153, 0.2);
  border-color: rgba(52, 211, 153, 0.3);
  color: #34d399;
}

.action-btn.danger {
  background: rgba(239, 68, 68, 0.2);
  border-color: rgba(239, 68, 68, 0.3);
  color: #fca5a5;
}

/* Close Button */
.close-btn {
  background: rgba(255, 255, 255, 0.1);
  border: none;
  border-radius: 6px;
  padding: 6px;
  color: rgba(255, 255, 255, 0.6);
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-left: auto;
}

.close-btn:hover {
  background: rgba(239, 68, 68, 0.2);
  color: #fca5a5;
}

.close-btn svg {
  width: 20px;
  height: 20px;
}

/* Body */
.log-body {
  flex: 1;
  padding: 16px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.log-loading {
  text-align: center;
  padding: 32px;
  color: rgba(255, 255, 255, 0.5);
}

/* Container */
.log-container {
  flex: 1;
  height: 400px;
  overflow-y: auto;
  background: rgba(0, 0, 0, 0.3);
  padding: 12px;
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 12px;
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.05);
}

.log-empty {
  text-align: center;
  padding: 48px;
  color: rgba(255, 255, 255, 0.4);
}

/* Log Line */
.log-line {
  line-height: 1.8;
  white-space: pre-wrap;
  word-break: break-all;
  color: rgba(255, 255, 255, 0.8);
  padding: 2px 0;
}

.log-time {
  color: rgba(255, 255, 255, 0.4);
  margin-right: 8px;
}

.log-level {
  margin-right: 8px;
  font-weight: 500;
}

.log-src {
  color: rgba(167, 139, 250, 0.8);
  margin-right: 8px;
}

.log-msg {
  color: rgba(255, 255, 255, 0.85);
}

/* Scrollbar */
.log-container::-webkit-scrollbar {
  width: 6px;
}

.log-container::-webkit-scrollbar-track {
  background: rgba(255, 255, 255, 0.05);
  border-radius: 3px;
}

.log-container::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.2);
  border-radius: 3px;
}

.log-container::-webkit-scrollbar-thumb:hover {
  background: rgba(255, 255, 255, 0.3);
}

/* Responsive */
@media (max-width: 768px) {
  .log-overlay {
    padding: 12px;
  }

  .log-modal {
    max-height: 90vh;
  }

  .log-header {
    flex-wrap: wrap;
  }

  .log-controls {
    order: 1;
    width: 100%;
    margin-top: 12px;
  }

  .log-input {
    width: 100%;
  }
}
</style>
