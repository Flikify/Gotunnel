import { ref, createApp, h } from 'vue'

interface ToastOptions {
  message: string
  type: 'success' | 'error' | 'warning' | 'info'
  duration?: number
}

const toasts = ref<Array<ToastOptions & { id: number }>>([])
let toastId = 0

const ToastContainer = {
  setup() {
    return () => h('div', { class: 'toast-container' },
      toasts.value.map(toast =>
        h('div', {
          key: toast.id,
          class: ['toast-item', toast.type]
        }, toast.message)
      )
    )
  }
}

let containerMounted = false

function ensureContainer() {
  if (containerMounted) return

  const container = document.createElement('div')
  container.id = 'toast-root'
  document.body.appendChild(container)

  const app = createApp(ToastContainer)
  app.mount(container)
  containerMounted = true

  // Add styles
  const style = document.createElement('style')
  style.textContent = `
    .toast-container {
      position: fixed;
      top: 20px;
      right: 20px;
      z-index: 9999;
      display: flex;
      flex-direction: column;
      gap: 8px;
    }
    .toast-item {
      padding: 12px 20px;
      border-radius: 8px;
      font-size: 14px;
      color: white;
      backdrop-filter: blur(20px);
      animation: toast-in 0.3s ease;
      max-width: 350px;
    }
    .toast-item.success {
      background: rgba(36, 166, 122, 0.92);
    }
    .toast-item.error {
      background: rgba(239, 68, 68, 0.9);
    }
    .toast-item.warning {
      background: rgba(213, 138, 45, 0.92);
      color: #fff;
    }
    .toast-item.info {
      background: rgba(47, 143, 187, 0.92);
    }
    @keyframes toast-in {
      from { opacity: 0; transform: translateX(20px); }
      to { opacity: 1; transform: translateX(0); }
    }
  `
  document.head.appendChild(style)
}

function showToast(options: ToastOptions) {
  ensureContainer()

  const id = ++toastId
  toasts.value.push({ ...options, id })

  setTimeout(() => {
    const index = toasts.value.findIndex(t => t.id === id)
    if (index > -1) {
      toasts.value.splice(index, 1)
    }
  }, options.duration || 3000)
}

export function useToast() {
  return {
    success: (message: string) => showToast({ message, type: 'success' }),
    error: (message: string) => showToast({ message, type: 'error' }),
    warning: (message: string) => showToast({ message, type: 'warning' }),
    info: (message: string) => showToast({ message, type: 'info' })
  }
}
