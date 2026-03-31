import { defineStore } from 'pinia'
import { ref } from 'vue'

import { loginApi, logoutApi, meApi } from '@/api/auth'
import type { LoginRespData } from '@/api/auth'

export type UserInfo = {
  uid: number
  name: string
  sign: string
  avatar: string
  email: string
}

const TOKEN_KEY = 'pgc-studio-token'
const USER_ID_KEY = 'pgc-studio-userId'
let validatedToken = ''
let validatingAuthPromise: Promise<boolean> | null = null

function safeGetItem(key: string) {
  try {
    return localStorage.getItem(key) ?? ''
  } catch {
    return ''
  }
}

function safeGetNumber(key: string) {
  try {
    const v = localStorage.getItem(key)
    if (!v) return null
    const n = Number(v)
    return Number.isFinite(n) ? n : null
  } catch {
    return null
  }
}

export function isAuthenticated() {
  return Boolean(safeGetItem(TOKEN_KEY))
}

export async function verifyAuthentication() {
  const token = safeGetItem(TOKEN_KEY)
  if (!token) return false
  if (validatedToken === token) return true
  if (validatingAuthPromise) return validatingAuthPromise

  validatingAuthPromise = (async () => {
    try {
      const res = await meApi(token)
      const ok = res.code === 200
      if (ok) {
        validatedToken = token
        return true
      }
    } catch {
      // ignore and fallthrough to clear
    }

    validatedToken = ''
    try {
      localStorage.removeItem(TOKEN_KEY)
      localStorage.removeItem(USER_ID_KEY)
    } catch {
      // ignore
    }
    return false
  })()

  try {
    return await validatingAuthPromise
  } finally {
    validatingAuthPromise = null
  }
}

export type LoginResult =
  | { ok: true }
  | { ok: false; code: number; msg: string; captchaId?: string }

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string>(safeGetItem(TOKEN_KEY))
  const userId = ref<number | null>(safeGetNumber(USER_ID_KEY))
  const userInfo = ref<UserInfo | null>(null)

  const loading = ref(false)

  function setAuth(next: LoginRespData) {
    token.value = next.token
    userId.value = next.userId
    userInfo.value = null
    validatedToken = next.token

    try {
      localStorage.setItem(TOKEN_KEY, next.token)
      localStorage.setItem(USER_ID_KEY, String(next.userId))
    } catch {
      // ignore
    }
  }

  function clearAuth() {
    token.value = ''
    userId.value = null
    userInfo.value = null
    validatedToken = ''
    try {
      localStorage.removeItem(TOKEN_KEY)
      localStorage.removeItem(USER_ID_KEY)
    } catch {
      // ignore
    }
  }

  async function fetchMe() {
    if (!token.value) return null
    const res = await meApi(token.value)
    if (res.code !== 200) return null

    const next = (res.data as any)?.userInfo as UserInfo | undefined
    if (next) userInfo.value = next
    return userInfo.value
  }

  async function login(email: string, password: string, captchaId?: string): Promise<LoginResult> {
    loading.value = true
    try {
      const res = await loginApi({ email, password, captchaId })
      if (res.code === 200) {
        setAuth(res.data as LoginRespData)
        await fetchMe()
        return { ok: true }
      }

      if (res.code === -1) {
        const nextCaptchaId = (res.data as any)?.captchaId as string | undefined
        return { ok: false, code: res.code, msg: res.msg, captchaId: nextCaptchaId }
      }

      return {
        ok: false,
        code: res.code,
        msg: res.msg || '登录失败',
      }
    } finally {
      loading.value = false
    }
  }

  async function logout() {
    try {
      await logoutApi()
    } catch {
      // ignore: logout 失败不影响前端清理
    } finally {
      clearAuth()
    }
  }

  return { token, userId, userInfo, loading, login, logout, fetchMe, setAuth, clearAuth }
})

