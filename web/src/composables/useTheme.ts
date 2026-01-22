import { ref, watch, onMounted } from 'vue'

export type ThemeMode = 'light' | 'dark' | 'auto'

const STORAGE_KEY = 'gotunnel-theme'

const themeMode = ref<ThemeMode>('auto')

function getSystemTheme(): 'light' | 'dark' {
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

function applyTheme(mode: ThemeMode) {
  const theme = mode === 'auto' ? getSystemTheme() : mode
  document.documentElement.setAttribute('data-theme', theme)
}

export function useTheme() {
  onMounted(() => {
    const saved = localStorage.getItem(STORAGE_KEY) as ThemeMode | null
    if (saved && ['light', 'dark', 'auto'].includes(saved)) {
      themeMode.value = saved
    }
    applyTheme(themeMode.value)

    // 监听系统主题变化
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
      if (themeMode.value === 'auto') {
        applyTheme('auto')
      }
    })
  })

  watch(themeMode, (mode) => {
    localStorage.setItem(STORAGE_KEY, mode)
    applyTheme(mode)
  })

  const setTheme = (mode: ThemeMode) => {
    themeMode.value = mode
  }

  return {
    themeMode,
    setTheme
  }
}
