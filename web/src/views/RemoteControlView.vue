<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ExpandOutline, ArrowBackOutline, RefreshOutline } from '@vicons/ionicons5'
import { createRemoteControlSocket, getClient } from '../api'
import { useToast } from '../composables/useToast'

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

const route = useRoute()
const router = useRouter()
const toast = useToast()

const clientId = computed(() => route.params.id as string)
const remoteImage = ref<HTMLImageElement | null>(null)
const fullscreenHost = ref<HTMLElement | null>(null)
const socket = ref<WebSocket | null>(null)
const state = ref<RemoteControlState>('idle')
const errorMessage = ref('')
const stopReason = ref('')
const frameSrc = ref('')
const desktopWidth = ref(0)
const desktopHeight = ref(0)
const frameIntervalMs = ref(150)
const fps = ref(0)
const frameTimes = ref<number[]>([])
const clientName = ref('')
const clientOs = ref('')
const online = ref(false)

const pressedKeys = new Set<string>()
let pointerMoveRaf: number | null = null
let pendingMove: { x: number, y: number } | null = null
let pendingPointer: PointerTarget | null = null
let clickTimer: number | null = null
let dragging = false

const singleClickDelayMs = 220

const statusText = computed(() => {
  switch (state.value) {
    case 'connecting':
      return '正在建立远控会话'
    case 'connected':
      return '键盘和鼠标已直连到客户端'
    case 'closed':
      return stopReason.value || '会话已结束'
    case 'error':
      return errorMessage.value || '远控发生异常'
    default:
      return '等待连接'
  }
})

const connectionBadge = computed(() => {
  if (state.value === 'connected') return 'LIVE'
  if (state.value === 'connecting') return 'SYNC'
  if (state.value === 'error') return 'ERR'
  return 'IDLE'
})

const isUnsupportedClient = computed(() => clientOs.value !== '' && clientOs.value !== 'windows')

onMounted(() => {
  bindWindowListeners()
  void loadClientAndOpenSession()
})

onUnmounted(() => {
  closeSession('page closed')
  unbindWindowListeners()
})

async function loadClientAndOpenSession() {
  closeSession('')
  errorMessage.value = ''
  stopReason.value = ''
  frameSrc.value = ''
  frameTimes.value = []
  fps.value = 0
  state.value = 'idle'

  try {
    const { data } = await getClient(clientId.value)
    clientName.value = data.nickname || data.id || clientId.value
    clientOs.value = data.os || ''
    online.value = !!data.online

    if (!online.value) {
      state.value = 'closed'
      stopReason.value = '客户端离线，无法建立远控会话'
      return
    }
    if (clientOs.value !== 'windows') {
      state.value = 'error'
      errorMessage.value = '当前远控页仅支持 Windows 客户端'
      return
    }

    openSession()
  } catch (error: any) {
    state.value = 'error'
    errorMessage.value = error?.message || '加载客户端信息失败'
  }
}

function openSession() {
  closeSession('')
  frameSrc.value = ''
  frameTimes.value = []
  fps.value = 0
  errorMessage.value = ''
  stopReason.value = ''
  state.value = 'connecting'

  try {
    const ws = createRemoteControlSocket(clientId.value)
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
        stopReason.value = stopReason.value || '远控连接已关闭'
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

function handleWindowKeyDown(event: KeyboardEvent) {
  if (!shouldCaptureInput(event)) return

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
  if (!shouldCaptureInput(event)) return

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
  if (!shouldCaptureInput(event)) return
  const text = event.clipboardData?.getData('text/plain') || ''
  if (!text) return
  event.preventDefault()
  sendPasteText(text)
  toast.success('已发送粘贴内容')
}

function handleMouseDown(event: MouseEvent) {
  if (!remoteImage.value || state.value !== 'connected') return
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
  if (!remoteImage.value || dragging || state.value !== 'connected') return
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
  if (!remoteImage.value || state.value !== 'connected') return
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
  if (!remoteImage.value || state.value !== 'connected') return
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
  if (!remoteImage.value || state.value !== 'connected' || (!pendingPointer && !dragging)) return
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
  if (!remoteImage.value || state.value !== 'connected') return
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
  if (!remoteImage.value || state.value !== 'connected') return
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
  const modifier = event.metaKey || event.ctrlKey ? 'control' : ''
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

function shouldCaptureInput(event: Event): boolean {
  if (state.value !== 'connected') return false
  return !shouldIgnoreTarget(event.target)
}

function clamp(value: number): number {
  if (value < 0) return 0
  if (value > 1) return 1
  return value
}

async function toggleFullscreen() {
  try {
    if (!document.fullscreenElement) {
      await fullscreenHost.value?.requestFullscreen()
    } else {
      await document.exitFullscreen()
    }
  } catch {
    toast.error('浏览器拒绝进入全屏')
  }
}

function goBack() {
  router.push({ name: 'client', params: { id: clientId.value } })
}
</script>

<template>
  <div ref="fullscreenHost" class="remote-page">
    <header class="remote-page__bar">
      <button class="remote-page__back" @click="goBack">
        <ArrowBackOutline />
        <span>返回客户端</span>
      </button>

      <div class="remote-page__title">
        <span class="remote-page__eyebrow">Remote Control</span>
        <h1>{{ clientName || clientId }}</h1>
        <p>{{ statusText }}</p>
      </div>

      <div class="remote-page__stats">
        <span class="remote-page__badge">{{ connectionBadge }}</span>
        <span>{{ desktopWidth || '—' }}x{{ desktopHeight || '—' }}</span>
        <span>{{ fps }} FPS</span>
        <span>{{ frameIntervalMs }}ms</span>
      </div>

      <div class="remote-page__actions">
        <button class="remote-page__action" @click="loadClientAndOpenSession">
          <RefreshOutline />
          <span>重连</span>
        </button>
        <button class="remote-page__action remote-page__action--primary" @click="toggleFullscreen">
          <ExpandOutline />
          <span>全屏</span>
        </button>
      </div>
    </header>

    <main class="remote-page__stage">
      <div class="remote-stage">
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
          <strong v-if="isUnsupportedClient">当前远控页仅支持 Windows 客户端</strong>
          <strong v-else-if="state === 'connecting'">正在等待首帧画面…</strong>
          <strong v-else-if="state === 'closed'">{{ stopReason || '会话已结束' }}</strong>
          <strong v-else>{{ errorMessage || '暂时无法建立远控画面' }}</strong>
          <span>聚焦当前页面后，键盘、鼠标和粘贴内容会直接发送到远端客户端。</span>
        </div>
      </div>
    </main>
  </div>
</template>

<style scoped>
.remote-page {
  --remote-bg: #07111e;
  --remote-panel: rgba(10, 18, 32, 0.82);
  --remote-panel-border: rgba(122, 157, 215, 0.16);
  --remote-text: #eff6ff;
  --remote-text-dim: rgba(210, 225, 246, 0.72);
  --remote-accent: #71a7ff;
  min-height: 100vh;
  background:
    radial-gradient(circle at top left, rgba(44, 101, 190, 0.22), transparent 36%),
    radial-gradient(circle at top right, rgba(49, 161, 132, 0.16), transparent 28%),
    linear-gradient(180deg, #07111e 0%, #030813 100%);
  color: var(--remote-text);
  padding: 18px;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  gap: 18px;
}

.remote-page__bar {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto auto;
  gap: 16px;
  align-items: center;
  padding: 18px 20px;
  border-radius: 24px;
  background: var(--remote-panel);
  border: 1px solid var(--remote-panel-border);
  box-shadow:
    0 28px 80px rgba(0, 0, 0, 0.42),
    inset 0 1px 0 rgba(255, 255, 255, 0.04);
  backdrop-filter: blur(18px);
}

.remote-page__back,
.remote-page__action {
  border: 1px solid rgba(141, 175, 230, 0.18);
  background: rgba(18, 30, 51, 0.82);
  color: var(--remote-text);
  border-radius: 16px;
  padding: 12px 16px;
  display: inline-flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
  transition: transform 0.18s ease, border-color 0.18s ease, background 0.18s ease;
}

.remote-page__back:hover,
.remote-page__action:hover {
  transform: translateY(-1px);
  border-color: rgba(152, 196, 255, 0.38);
  background: rgba(24, 39, 66, 0.92);
}

.remote-page__action--primary {
  background: linear-gradient(135deg, rgba(52, 112, 208, 0.95), rgba(34, 73, 144, 0.96));
  border-color: rgba(137, 182, 255, 0.36);
}

.remote-page__title h1 {
  margin: 4px 0 6px;
  font-size: 30px;
  line-height: 1.05;
  letter-spacing: -0.03em;
}

.remote-page__title p,
.remote-stage__empty span {
  margin: 0;
  color: var(--remote-text-dim);
  font-size: 14px;
  line-height: 1.6;
}

.remote-page__eyebrow {
  display: inline-block;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.26em;
  color: rgba(148, 187, 244, 0.78);
}

.remote-page__stats,
.remote-page__actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.remote-page__stats span {
  border-radius: 999px;
  padding: 8px 12px;
  background: rgba(19, 31, 53, 0.88);
  border: 1px solid rgba(124, 159, 216, 0.14);
  color: #d9e8ff;
  font-size: 12px;
}

.remote-page__badge {
  background: rgba(21, 69, 120, 0.88) !important;
  color: #9ed6ff !important;
  letter-spacing: 0.12em;
}

.remote-page__stage {
  min-height: 0;
}

.remote-stage {
  height: 100%;
  min-height: calc(100vh - 138px);
  border-radius: 30px;
  background:
    radial-gradient(circle at top, rgba(58, 100, 163, 0.22), transparent 42%),
    linear-gradient(180deg, rgba(5, 10, 19, 0.98), rgba(3, 8, 15, 0.98));
  border: 1px solid rgba(126, 161, 219, 0.16);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.03),
    0 32px 90px rgba(0, 0, 0, 0.38);
  padding: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

.remote-stage__image {
  display: block;
  max-width: 100%;
  max-height: 100%;
  width: auto;
  height: auto;
  border-radius: 18px;
  box-shadow: 0 24px 80px rgba(0, 0, 0, 0.45);
  cursor: crosshair;
  user-select: none;
}

.remote-stage__empty {
  display: flex;
  flex-direction: column;
  gap: 10px;
  align-items: center;
  justify-content: center;
  text-align: center;
  max-width: 420px;
}

.remote-stage__empty strong {
  font-size: 24px;
  line-height: 1.2;
  letter-spacing: -0.03em;
}

@media (max-width: 1100px) {
  .remote-page {
    padding: 12px;
    gap: 12px;
  }

  .remote-page__bar {
    grid-template-columns: 1fr;
    justify-items: stretch;
  }

  .remote-page__stats,
  .remote-page__actions {
    justify-content: flex-start;
  }

  .remote-stage {
    min-height: calc(100vh - 188px);
  }
}
</style>
