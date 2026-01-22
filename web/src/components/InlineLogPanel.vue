<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { PlayOutline, StopOutline, TrashOutline } from '@vicons/ionicons5'
import { createLogStream } from '../api'
import type { LogEntry } from '../types'

const props = defineProps<{
  clientId: string
}>()

const logs = ref<LogEntry[]>([])
const isStreaming = ref(false)
const autoScroll = ref(true)
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
    { lines: 100, follow: true, level: '' },
    (entry) => {
      logs.value.push(entry)
      if (logs.value.length > 500) {
        logs.value = logs.value.slice(-300)
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

const getLevelColor = (level: string): string => {
  switch (level) {
    case 'error': return '#fca5a5'
    case 'warn': return '#fcd34d'
    case 'info': return '#60a5fa'
    case 'debug': return '#9ca3af'
    default: return 'rgba(255,255,255,0.7)'
  }
}

const formatTime = (ts: number): string => {
  return new Date(ts).toLocaleTimeString('en-US', { hour12: false })
}

onMounted(() => {
  startStream()
})

onUnmounted(() => {
  stopStream()
})
</script>

<template>
  <div class="inline-log-panel">
    <div class="log-toolbar">
      <div class="toolbar-left">
        <span class="log-title">实时日志</span>
        <span v-if="isStreaming" class="streaming-badge">
          <span class="streaming-dot"></span>
          实时
        </span>
      </div>
      <div class="toolbar-right">
        <button v-if="!isStreaming" class="tool-btn" @click="startStream" title="开始">
          <PlayOutline class="tool-icon" />
        </button>
        <button v-else class="tool-btn" @click="stopStream" title="停止">
          <StopOutline class="tool-icon" />
        </button>
        <button class="tool-btn" @click="clearLogs" title="清空">
          <TrashOutline class="tool-icon" />
        </button>
        <label class="auto-scroll-toggle">
          <input type="checkbox" v-model="autoScroll" />
          <span>自动滚动</span>
        </label>
      </div>
    </div>
    <div ref="logContainer" class="log-content">
      <div v-if="loading && logs.length === 0" class="log-loading">
        连接中...
      </div>
      <div v-else-if="logs.length === 0" class="log-empty">
        暂无日志
      </div>
      <div v-else class="log-lines">
        <div v-for="(log, index) in logs" :key="index" class="log-line">
          <span class="log-time">{{ formatTime(log.ts) }}</span>
          <span class="log-level" :style="{ color: getLevelColor(log.level) }">
            [{{ log.level.toUpperCase() }}]
          </span>
          <span class="log-src">[{{ log.src }}]</span>
          <span class="log-msg">{{ log.msg }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.inline-log-panel {
  background: var(--glass-bg);
  border-radius: 12px;
  border: 1px solid var(--color-border);
  overflow: hidden;
}

.log-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid var(--color-border);
  background: var(--glass-bg-light);
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.log-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.streaming-badge {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--color-success);
  background: rgba(16, 185, 129, 0.15);
  padding: 2px 8px;
  border-radius: 10px;
}

.streaming-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--color-success);
  animation: pulse 1.5s ease-in-out infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.tool-btn {
  background: var(--color-border);
  border: none;
  border-radius: 6px;
  padding: 6px;
  color: var(--color-text-secondary);
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
}

.tool-btn:hover {
  background: var(--color-bg-elevated);
  color: var(--color-text-primary);
}

.tool-icon {
  width: 16px;
  height: 16px;
}

.auto-scroll-toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--color-text-secondary);
  cursor: pointer;
}

.auto-scroll-toggle input {
  width: 14px;
  height: 14px;
  accent-color: var(--color-accent);
}

.log-content {
  height: 200px;
  overflow-y: auto;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 12px;
  line-height: 1.6;
  background: var(--color-bg-elevated);
}

.log-loading,
.log-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--color-text-muted);
  font-size: 13px;
}

.log-lines {
  padding: 8px 12px;
}

.log-line {
  display: flex;
  gap: 8px;
  padding: 2px 0;
  white-space: nowrap;
}

.log-time {
  color: var(--color-text-muted);
  flex-shrink: 0;
}

.log-level {
  flex-shrink: 0;
  font-weight: 500;
}

.log-src {
  color: var(--color-text-muted);
  flex-shrink: 0;
}

.log-msg {
  color: var(--color-text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
}

/* Scrollbar */
.log-content::-webkit-scrollbar {
  width: 6px;
}

.log-content::-webkit-scrollbar-track {
  background: transparent;
}

.log-content::-webkit-scrollbar-thumb {
  background: var(--color-border);
  border-radius: 3px;
}

.log-content::-webkit-scrollbar-thumb:hover {
  background: var(--color-text-muted);
}
</style>
