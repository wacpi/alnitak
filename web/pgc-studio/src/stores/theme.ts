import { defineStore } from 'pinia'
import { ref, watch } from 'vue'

export type ThemeMode = 'light' | 'dark'

const STORAGE_KEY = 'pgc-studio-theme'

function readInitialMode(): ThemeMode {
  if (typeof document === 'undefined') return 'dark'
  return document.documentElement.classList.contains('dark') ? 'dark' : 'light'
}

function applyClass(mode: ThemeMode) {
  const isDark = mode === 'dark'
  document.documentElement.classList.toggle('dark', isDark)
  document.body?.classList.toggle('dark', isDark)
}

export const useThemeStore = defineStore('theme', () => {
  const mode = ref<ThemeMode>(readInitialMode())
  // 与首屏 inline 脚本对齐，避免偶发 class 与 pinia 状态不一致
  applyClass(mode.value)

  watch(
    mode,
    (m) => {
      applyClass(m)
      localStorage.setItem(STORAGE_KEY, m)
    },
    { flush: 'sync' },
  )

  function toggle() {
    mode.value = mode.value === 'dark' ? 'light' : 'dark'
  }

  function setTheme(m: ThemeMode) {
    mode.value = m
  }

  return { mode, toggle, setTheme }
})
