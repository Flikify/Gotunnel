<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import GlassModal from './GlassModal.vue'
import { createRemoteControlSocket } from '../api'
import { useToast } from '../composables/useToast'

const props = defineProps<{
  show: boolean
  clientId: string
  clientOs?: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

type RemoteControlState = 'idle' | 'connecting' | 'connected' | 'closed' | 'error'

interface RemoteControlMessage {
  type: string
  event_type?: string
  x?: number
  y?: number
  button?: string
  delta_x?: number
  delta_y?: number
  key?: string
  keys?: string[]
  text?: string
  data?: string
  width?: number
  height?: number
  timestamp?: number
  message?: string
  reason?: string
  frame_interval_ms?: number
}

interface PointerTarget {
  button: string
  x: number
  y: number
}

const message = useToast()
const remoteImage = ref<HTMLImageElement | null>(null)
const socket = ref<WebSocket | null>(null)
const state = ref<RemoteControlState>('idle')
const errorMessage = ref('')
const stopReason = ref('')
const frameSrc = ref('')
const desktopWidth = ref(0)
const desktopHeight = ref(0)
const frameIntervalMs = ref(150)
const fps = ref(0)
const manualClipboardInput = ref('')
const manualClipboardOutput = ref('')
const frameTimes = ref<number[]>([])

const shortcutModifier = computed(() => props.clientOs === 'darwin' ? 'cmd' : 'control')
const shortcutLabel = computed(() => props.clientOs === 'darwin' ? 'Cmd' : 'Ctrl')
const statusText = computed(() => {
  switch (state.value) {
    case 'connecting':
      return '连接中'
    case 'connected':
      return '控制中'
    case 'closed':
      return stopReason.value || '已断开'
    case 'error':
      return errorMessage.value || '连接异常'
    default:
      return '未连接'
  }
})

const shortcutButtons = computed(() => {
  const modifier = shortcutModifier.value
  const label = shortcutLabel.value
  return [
    { label: `${label}+C`, title: '复制', keys: [modifier, 'c'] },
    { label: `${label}+V`, title: '粘贴', keys: [modifier, 'v'] },
    { label: `${label}+X`, title: '剪切', keys: [modifier, 'x'] },
    { label: `${label}+A`, title: '全选', keys: [modifier, 'a'] },
    { label: `${label}+Z`, title: '撤销', keys: [modifier, 'z'] },
    props.clientOs === 'darwin'
      ? { label: `${label}+Shift+Z`, title: '重做', keys: [modifier, 'shift', 'z'] }
      : { label: `${label}+Y`, title: '重做', keys: [modifier, 'y'] },
    { label: `${label}+S`, title: '保存', keys: [modifier, 's'] },
    { label: 'Enter', title: '回车', keys: ['enter'] },
    { label: 'Tab', title: 'Tab', keys: ['tab'] },
  ]
})

const pressedKeys = new Set<string>()
let pointerMoveRaf: number | null = null
let pendingMove: { x: number, y: number } | null = null
let pendingPointer: PointerTarget | null = null
let clickTimer: number | null = null
let dragging = false

const singleClickDelayMs = 220

watch(() => props.show, (show) => {
  if (show) {
    openSession()
    bindWindowListeners()
    return
  }
  closeSession('modal closed')
  unbindWindowListeners()
})

onUnmounted(() => {
  closeSession('component unmounted')
  unbindWindowListeners()
})

function openSession() {
  closeSession('')
  frameSrc.value = ''
  frameTimes.value = []
  fps.value = 0
  errorMessage.value = ''
  stopReason.value = ''
  state.value = 'connecting'

  try {
    const ws = createRemoteControlSocket(props.clientId)
    socket.value = ws
    ws.onmessage = handleSocketMessage
    ws.onerror = () => {
      if (state.value !== 'closed') {
        state.value = 'error'
        errorMessage.value = '远控连接异常'
      }
    }
    ws.onclose = () => {
      socket.value = null
      if (state.value === 'connecting' || state.value === 'connected') {
        state.value = 'closed'
      }
    }
  } catch (error: any) {
    state.value = 'error'
    errorMessage.value = error?.message || '无法创建远控连接'
  }
}

function closeSession(reason: string) {
  dragging = false
  pendingPointer = null
  pressedKeys.clear()
  if (pointerMoveRaf !== null) {
    window.cancelAnimationFrame(pointerMoveRaf)
    pointerMoveRaf = null
  }
  pendingMove = null
  if (clickTimer !== null) {
    window.clearTimeout(clickTimer)
    clickTimer = null
  }

  const ws = socket.value
  socket.value = null
  if (ws) {
    if (ws.readyState === WebSocket.OPEN && reason) {
      ws.send(JSON.stringify({ type: 'stop', reason }))
    }
    ws.close()
  }
}

function bindWindowListeners() {
  window.addEventListener('keydown', handleWindowKeyDown)
  window.addEventListener('keyup', handleWindowKeyUp)
  window.addEventListener('paste', handleWindowPaste)
  window.addEventListener('mousemove', handleWindowMouseMove)
  window.addEventListener('mouseup', handleWindowMouseUp)
}

function unbindWindowListeners() {
  window.removeEventListener('keydown', handleWindowKeyDown)
  window.removeEventListener('keyup', handleWindowKeyUp)
  window.removeEventListener('paste', handleWindowPaste)
  window.removeEventListener('mousemove', handleWindowMouseMove)
  window.removeEventListener('mouseup', handleWindowMouseUp)
}

function handleSocketMessage(event: MessageEvent<string>) {
  const payload = JSON.parse(event.data) as RemoteControlMessage

  switch (payload.type) {
    case 'ready':
      state.value = 'connected'
      desktopWidth.value = payload.width || 0
      desktopHeight.value = payload.height || 0
      frameIntervalMs.value = payload.frame_interval_ms || 150
      return
    case 'frame':
      frameSrc.value = payload.data ? `data:image/jpeg;base64,${payload.data}` : ''
      desktopWidth.value = payload.width || desktopWidth.value
      desktopHeight.value = payload.height || desktopHeight.value
      recordFrame()
      if (state.value !== 'connected') {
        state.value = 'connected'
      }
      return
    case 'clipboard_data':
      manualClipboardOutput.value = payload.text || ''
      if (manualClipboardOutput.value) {
        navigator.clipboard?.writeText(manualClipboardOutput.value).catch(() => {})
        message.success('已读取客户端剪贴板')
      }
      return
    case 'error':
      state.value = 'error'
      errorMessage.value = payload.message || '远控发生错误'
      return
    case 'stopped':
      state.value = 'closed'
      stopReason.value = payload.reason || '远控已结束'
      return
  }
}

function recordFrame() {
  const now = Date.now()
  frameTimes.value = [...frameTimes.value.filter((value) => now - value < 1000), now]
  fps.value = frameTimes.value.length
}

function sendMessage(payload: RemoteControlMessage) {
  const ws = socket.value
  if (!ws || ws.readyState !== WebSocket.OPEN) {
    return
  }
  ws.send(JSON.stringify(payload))
}

function sendShortcut(keys: string[]) {
  sendMessage({ type: 'input', event_type: 'shortcut', keys })
}

function sendPasteText(text: string) {
  if (!text) return
  sendMessage({ type: 'clipboard_set', text })
  sendMessage({ type: 'input', event_type: 'paste_text', text })
}

function handleCopyFromClient() {
  sendMessage({ type: 'clipboard_get' })
}

async function handlePasteToClient() {
  let text = ''
  try {
    text = await navigator.clipboard.readText()
  } catch {
    text = manualClipboardInput.value.trim()
  }

  if (!text) {
    message.error('没有可粘贴的文本')
    return
  }

  manualClipboardInput.value = text
  sendPasteText(text)
  message.success('已发送粘贴内容')
}

function handleShortcutButton(keys: string[]) {
  sendShortcut(keys)
}

function handleWindowKeyDown(event: KeyboardEvent) {
  if (!props.show || shouldIgnoreTarget(event.target)) return

  const shortcut = toShortcut(event)
  if (shortcut) {
    event.preventDefault()
    sendShortcut(shortcut)
    return
  }

  const key = toRobotKey(event)
  if (!key) return
  if (shouldPreventDefaultKey(key)) {
    event.preventDefault()
  }
  if (pressedKeys.has(key)) {
    return
  }
  pressedKeys.add(key)
  sendMessage({ type: 'input', event_type: 'key_down', key })
}

function handleWindowKeyUp(event: KeyboardEvent) {
  if (!props.show || shouldIgnoreTarget(event.target)) return

  const shortcut = toShortcut(event)
  if (shortcut) {
    event.preventDefault()
    return
  }

  const key = toRobotKey(event)
  if (!key) return
  if (shouldPreventDefaultKey(key)) {
    event.preventDefault()
  }
  pressedKeys.delete(key)
  sendMessage({ type: 'input', event_type: 'key_up', key })
}

function handleWindowPaste(event: ClipboardEvent) {
  if (!props.show || shouldIgnoreTarget(event.target)) return
  const text = event.clipboardData?.getData('text/plain') || ''
  if (!text) return
  event.preventDefault()
  manualClipboardInput.value = text
  sendPasteText(text)
}

function handleMouseDown(event: MouseEvent) {
  if (!remoteImage.value) return
  event.preventDefault()
  const point = toNormalizedPoint(event)
  if (!point) return
  pendingPointer = {
    button: toMouseButton(event.button),
    x: point.x,
    y: point.y,
  }
}

function handleImageClick(event: MouseEvent) {
  if (!remoteImage.value || dragging) return
  event.preventDefault()
  const point = toNormalizedPoint(event)
  if (!point) return
  pendingPointer = null
  if (clickTimer !== null) {
    window.clearTimeout(clickTimer)
  }
  clickTimer = window.setTimeout(() => {
    clickTimer = null
    sendMessage({ type: 'input', event_type: 'mouse_move', x: point.x, y: point.y })
    sendMessage({ type: 'input', event_type: 'mouse_click', button: toMouseButton(event.button), x: point.x, y: point.y })
  }, singleClickDelayMs)
}

function handleImageDoubleClick(event: MouseEvent) {
  if (!remoteImage.value) return
  event.preventDefault()
  const point = toNormalizedPoint(event)
  if (!point) return
  if (clickTimer !== null) {
    window.clearTimeout(clickTimer)
    clickTimer = null
  }
  sendMessage({ type: 'input', event_type: 'mouse_move', x: point.x, y: point.y })
  sendMessage({ type: 'input', event_type: 'mouse_double_click', button: toMouseButton(event.button), x: point.x, y: point.y })
  pendingPointer = null
}

function handleImageMouseMove(event: MouseEvent) {
  if (!remoteImage.value) return
  const point = toNormalizedPoint(event)
  if (!point) return

  if (pendingPointer && !dragging && movedEnough(point, pendingPointer)) {
    dragging = true
    sendMessage({ type: 'input', event_type: 'mouse_move', x: pendingPointer.x, y: pendingPointer.y })
    sendMessage({ type: 'input', event_type: 'mouse_down', button: pendingPointer.button, x: pendingPointer.x, y: pendingPointer.y })
  }

  queueMouseMove(point.x, point.y)
}

function handleWindowMouseMove(event: MouseEvent) {
  if (!props.show || !remoteImage.value || (!pendingPointer && !dragging)) return
  const point = toNormalizedPoint(event)
  if (!point) return

  if (pendingPointer && !dragging && movedEnough(point, pendingPointer)) {
    dragging = true
    sendMessage({ type: 'input', event_type: 'mouse_move', x: pendingPointer.x, y: pendingPointer.y })
    sendMessage({ type: 'input', event_type: 'mouse_down', button: pendingPointer.button, x: pendingPointer.x, y: pendingPointer.y })
  }

  if (dragging) {
    queueMouseMove(point.x, point.y)
  }
}

function handleWindowMouseUp(event: MouseEvent) {
  if (!props.show || !remoteImage.value) return
  if (!dragging) {
    pendingPointer = null
    return
  }

  const point = toNormalizedPoint(event)
  const button = pendingPointer?.button || toMouseButton(event.button)
  if (point) {
    sendMessage({ type: 'input', event_type: 'mouse_move', x: point.x, y: point.y })
  }
  sendMessage({ type: 'input', event_type: 'mouse_up', button })
  dragging = false
  pendingPointer = null
}

function handleWheel(event: WheelEvent) {
  if (!remoteImage.value) return
  event.preventDefault()
  const deltaX = normalizeWheelDelta(event.deltaX)
  const deltaY = normalizeWheelDelta(event.deltaY)
  if (deltaX === 0 && deltaY === 0) return
  sendMessage({ type: 'input', event_type: 'mouse_wheel', delta_x: deltaX, delta_y: deltaY })
}

function queueMouseMove(x: number, y: number) {
  pendingMove = { x, y }
  if (pointerMoveRaf !== null) return
  pointerMoveRaf = window.requestAnimationFrame(() => {
    pointerMoveRaf = null
    if (!pendingMove) return
    sendMessage({ type: 'input', event_type: 'mouse_move', x: pendingMove.x, y: pendingMove.y })
    pendingMove = null
  })
}

function toNormalizedPoint(event: MouseEvent): { x: number, y: number } | null {
  const image = remoteImage.value
  if (!image) return null
  const rect = image.getBoundingClientRect()
  if (rect.width <= 0 || rect.height <= 0) return null

  const x = clamp((event.clientX - rect.left) / rect.width)
  const y = clamp((event.clientY - rect.top) / rect.height)
  return { x, y }
}

function movedEnough(point: { x: number, y: number }, origin: PointerTarget): boolean {
  const dx = Math.abs(point.x - origin.x)
  const dy = Math.abs(point.y - origin.y)
  return dx > 0.005 || dy > 0.005
}

function toMouseButton(button: number): string {
  switch (button) {
    case 1:
      return 'center'
    case 2:
      return 'right'
    default:
      return 'left'
  }
}

function normalizeWheelDelta(value: number): number {
  if (value === 0) return 0
  const amount = Math.max(1, Math.round(Math.abs(value) / 80))
  return value > 0 ? amount : -amount
}

function toShortcut(event: KeyboardEvent): string[] | null {
  const modifier = event.metaKey ? 'cmd' : event.ctrlKey ? 'control' : ''
  if (!modifier) return null

  const key = toRobotKey(event)
  if (!key) return null

  if (!['a', 'c', 's', 'v', 'x', 'y', 'z'].includes(key)) {
    return null
  }

  const keys = [modifier]
  if (event.shiftKey) {
    keys.push('shift')
  }
  keys.push(key)
  return keys
}

function toRobotKey(event: KeyboardEvent): string | null {
  const key = event.key
  if (!key) return null

  switch (key) {
    case 'Control':
      return 'control'
    case 'Shift':
      return 'shift'
    case 'Alt':
      return 'alt'
    case 'Meta':
      return 'cmd'
    case 'ArrowUp':
      return 'up'
    case 'ArrowDown':
      return 'down'
    case 'ArrowLeft':
      return 'left'
    case 'ArrowRight':
      return 'right'
    case 'Backspace':
      return 'backspace'
    case 'Delete':
      return 'delete'
    case 'Escape':
      return 'escape'
    case 'Enter':
      return 'enter'
    case 'Tab':
      return 'tab'
    case 'Home':
      return 'home'
    case 'End':
      return 'end'
    case 'PageUp':
      return 'pageup'
    case 'PageDown':
      return 'pagedown'
    case ' ':
      return 'space'
    default:
      return key.length === 1 ? key.toLowerCase() : null
  }
}

function shouldPreventDefaultKey(key: string): boolean {
  return ['control', 'shift', 'alt', 'cmd', 'tab', 'enter', 'backspace', 'delete', 'space', 'up', 'down', 'left', 'right', 'home', 'end', 'pageup', 'pagedown', 'escape'].includes(key)
}

function shouldIgnoreTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  return target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)
}

function clamp(value: number): number {
  if (value < 0) return 0
  if (value > 1) return 1
  return value
}
</script>

<template>
  <GlassModal :show="show" title="远程控制" width="1180px" @close="emit('close')">
    <div class="remote-control">
      <section class="remote-stage glass-card">
        <header class="remote-stage__header">
          <div>
            <h4>实时画面</h4>
            <p>{{ statusText }}</p>
          </div>
          <div class="remote-stage__meta">
            <span>{{ desktopWidth }}x{{ desktopHeight }}</span>
            <span>{{ fps }} FPS</span>
            <span>{{ frameIntervalMs }}ms</span>
          </div>
        </header>
        <div class="remote-stage__body">
          <img
            v-if="frameSrc"
            ref="remoteImage"
            :src="frameSrc"
            alt="Remote desktop"
            class="remote-stage__image"
            draggable="false"
            @mousedown="handleMouseDown"
            @mousemove="handleImageMouseMove"
            @click="handleImageClick"
            @dblclick="handleImageDoubleClick"
            @wheel="handleWheel"
          />
          <div v-else class="remote-stage__empty">
            <strong>{{ state === 'connecting' ? '正在建立远控会话…' : '等待首帧画面…' }}</strong>
            <span>弹窗已进入控制态，键盘与快捷键会直接发送到客户端。</span>
          </div>
        </div>
      </section>

      <aside class="remote-sidebar glass-card">
        <section class="remote-panel">
          <header>
            <h4>快捷键</h4>
            <p>浏览器保留快捷键请用按钮发送。</p>
          </header>
          <div class="shortcut-grid">
            <button
              v-for="item in shortcutButtons"
              :key="item.label"
              class="glass-btn small remote-action"
              @click="handleShortcutButton(item.keys)"
            >
              <strong>{{ item.label }}</strong>
              <span>{{ item.title }}</span>
            </button>
          </div>
        </section>

        <section class="remote-panel">
          <header>
            <h4>复制粘贴</h4>
            <p>支持双向文本剪贴板，权限失败时可手动回退。</p>
          </header>
          <div class="clipboard-actions">
            <button class="glass-btn small" @click="handleCopyFromClient">复制自客户端</button>
            <button class="glass-btn primary small" @click="handlePasteToClient">粘贴到客户端</button>
          </div>
          <label class="clipboard-label">本地待粘贴文本</label>
          <textarea
            v-model="manualClipboardInput"
            class="glass-input clipboard-box"
            placeholder="浏览器无法读取本地剪贴板时，在这里粘贴文本后再点“粘贴到客户端”。"
          />
          <label class="clipboard-label">客户端剪贴板文本</label>
          <textarea
            :value="manualClipboardOutput"
            class="glass-input clipboard-box clipboard-box--readonly"
            readonly
            placeholder="点击“复制自客户端”后，文本会显示在这里。"
          />
        </section>

        <section v-if="errorMessage || stopReason" class="remote-panel remote-panel--status">
          <header>
            <h4>会话状态</h4>
          </header>
          <p>{{ errorMessage || stopReason }}</p>
        </section>
      </aside>
    </div>

    <template #footer>
      <button class="glass-btn" @click="emit('close')">关闭</button>
    </template>
  </GlassModal>
</template>

<style scoped>
.remote-control {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(320px, 0.8fr);
  gap: 20px;
}

.remote-stage,
.remote-sidebar {
  background: rgba(11, 18, 32, 0.6);
  border: 1px solid rgba(113, 144, 185, 0.16);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

.remote-stage {
  padding: 18px;
}

.remote-stage__header,
.remote-panel header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.remote-stage__header h4,
.remote-panel h4 {
  margin: 0;
  font-size: 15px;
  color: var(--color-text-primary);
}

.remote-stage__header p,
.remote-panel p {
  margin: 6px 0 0;
  color: var(--color-text-secondary);
  font-size: 13px;
  line-height: 1.5;
}

.remote-stage__meta {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.remote-stage__meta span {
  padding: 6px 10px;
  border-radius: 999px;
  background: rgba(35, 52, 83, 0.62);
  color: #c8d7f0;
  font-size: 12px;
}

.remote-stage__body {
  margin-top: 16px;
  min-height: 520px;
  border-radius: 18px;
  background:
    radial-gradient(circle at top, rgba(49, 89, 145, 0.22), transparent 45%),
    linear-gradient(180deg, rgba(7, 12, 22, 0.98), rgba(10, 15, 26, 0.98));
  border: 1px solid rgba(133, 166, 218, 0.14);
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  padding: 14px;
}

.remote-stage__image {
  display: block;
  max-width: 100%;
  max-height: 100%;
  border-radius: 12px;
  box-shadow: 0 18px 40px rgba(0, 0, 0, 0.38);
  cursor: crosshair;
  user-select: none;
}

.remote-stage__empty {
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-items: center;
  justify-content: center;
  text-align: center;
  color: #b9c7dc;
  max-width: 360px;
}

.remote-sidebar {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 18px;
}

.remote-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.shortcut-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.remote-action {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
  min-height: 64px;
}

.remote-action strong {
  font-size: 13px;
}

.remote-action span {
  font-size: 12px;
  color: var(--color-text-secondary);
}

.clipboard-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.clipboard-label {
  font-size: 12px;
  color: var(--color-text-secondary);
}

.clipboard-box {
  width: 100%;
  min-height: 96px;
  resize: vertical;
}

.clipboard-box--readonly {
  opacity: 0.92;
}

.remote-panel--status {
  padding: 14px;
  border-radius: 14px;
  background: rgba(120, 52, 52, 0.16);
  border: 1px solid rgba(234, 111, 111, 0.18);
}

@media (max-width: 1080px) {
  .remote-control {
    grid-template-columns: 1fr;
  }

  .remote-stage__body {
    min-height: 380px;
  }
}

@media (max-width: 720px) {
  .shortcut-grid {
    grid-template-columns: 1fr 1fr;
  }

  .remote-stage__body {
    min-height: 280px;
  }
}
</style>
