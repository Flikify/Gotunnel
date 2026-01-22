import { ref, createApp, h } from 'vue'

interface DialogOptions {
  title: string
  content: string
  positiveText?: string
  negativeText?: string
  onPositiveClick?: () => void | Promise<void>
  onNegativeClick?: () => void
}

const dialogVisible = ref(false)
const dialogOptions = ref<DialogOptions | null>(null)

const DialogComponent = {
  setup() {
    const handlePositive = async () => {
      if (dialogOptions.value?.onPositiveClick) {
        await dialogOptions.value.onPositiveClick()
      }
      dialogVisible.value = false
    }

    const handleNegative = () => {
      dialogOptions.value?.onNegativeClick?.()
      dialogVisible.value = false
    }

    return () => {
      if (!dialogVisible.value || !dialogOptions.value) return null

      return h('div', { class: 'dialog-overlay', onClick: handleNegative },
        h('div', { class: 'dialog-container', onClick: (e: Event) => e.stopPropagation() }, [
          h('h3', { class: 'dialog-title' }, dialogOptions.value.title),
          h('p', { class: 'dialog-content' }, dialogOptions.value.content),
          h('div', { class: 'dialog-footer' }, [
            h('button', { class: 'dialog-btn', onClick: handleNegative },
              dialogOptions.value.negativeText || '取消'),
            h('button', { class: 'dialog-btn primary', onClick: handlePositive },
              dialogOptions.value.positiveText || '确定')
          ])
        ])
      )
    }
  }
}

let containerMounted = false

function ensureContainer() {
  if (containerMounted) return

  const container = document.createElement('div')
  container.id = 'dialog-root'
  document.body.appendChild(container)

  const app = createApp(DialogComponent)
  app.mount(container)
  containerMounted = true

  // Add styles
  const style = document.createElement('style')
  style.textContent = `
    .dialog-overlay {
      position: fixed;
      inset: 0;
      background: rgba(0, 0, 0, 0.6);
      backdrop-filter: blur(4px);
      display: flex;
      align-items: center;
      justify-content: center;
      z-index: 9998;
    }
    .dialog-container {
      background: rgba(30, 27, 75, 0.95);
      backdrop-filter: blur(20px);
      border-radius: 16px;
      border: 1px solid rgba(255, 255, 255, 0.12);
      padding: 24px;
      max-width: 400px;
      width: 90%;
    }
    .dialog-title {
      margin: 0 0 12px 0;
      font-size: 18px;
      font-weight: 600;
      color: white;
    }
    .dialog-content {
      margin: 0 0 20px 0;
      color: rgba(255, 255, 255, 0.7);
      font-size: 14px;
      line-height: 1.6;
    }
    .dialog-footer {
      display: flex;
      justify-content: flex-end;
      gap: 12px;
    }
    .dialog-btn {
      background: rgba(255, 255, 255, 0.1);
      border: 1px solid rgba(255, 255, 255, 0.15);
      border-radius: 8px;
      padding: 8px 16px;
      color: white;
      font-size: 14px;
      cursor: pointer;
      transition: all 0.2s;
    }
    .dialog-btn:hover {
      background: rgba(255, 255, 255, 0.2);
    }
    .dialog-btn.primary {
      background: linear-gradient(135deg, #60a5fa 0%, #a78bfa 100%);
      border: none;
    }
  `
  document.head.appendChild(style)
}

export function useConfirm() {
  return {
    warning: (options: DialogOptions) => {
      ensureContainer()
      dialogOptions.value = options
      dialogVisible.value = true
    }
  }
}
