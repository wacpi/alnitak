export type ApiResponse<T = unknown> = {
  code: number
  data: T
  msg: string
}

type HttpMethod = 'GET' | 'POST' | 'PUT' | 'DELETE'

const API_BASE_URL = ((import.meta as any).env?.VITE_API_BASE_URL ?? '') as string
const TOKEN_KEY = 'pgc-studio-token'
const USER_ID_KEY = 'pgc-studio-userId'
let redirectingToLogin = false

function normalizeBaseUrl(base: string) {
  return base.replace(/\/$/, '')
}

function getApiUrl(path: string) {
  const base = normalizeBaseUrl(API_BASE_URL)
  // path 建议传入以 / 开头的完整后端路径，如 /api/v1/auth/login
  return `${base}${path.startsWith('/') ? path : `/${path}`}`
}

async function parseJsonSafely(res: Response) {
  try {
    return (await res.json()) as unknown
  } catch {
    return null
  }
}

function clearLocalAuth() {
  try {
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(USER_ID_KEY)
  } catch {
    // ignore
  }
}

function redirectToLogin() {
  if (redirectingToLogin) return
  redirectingToLogin = true
  clearLocalAuth()
  const current = window.location.pathname + window.location.search
  const next = current && current !== '/login' ? `/login?redirect=${encodeURIComponent(current)}` : '/login'
  window.location.replace(next)
}

export function handleUnauthorizedResponse(res: Response, payload?: unknown) {
  const body = payload as Partial<ApiResponse<unknown>> | null | undefined
  const code = typeof body?.code === 'number' ? body.code : undefined
  const msg = String(body?.msg ?? '')
  const unauthorized =
    res.status === 401 ||
    code === 401 ||
    code === 403 ||
    msg.includes('未登录') ||
    msg.includes('登录过期') ||
    msg.includes('token')

  if (unauthorized) {
    redirectToLogin()
  }
}

export async function apiRequest<T = unknown>(
  method: HttpMethod,
  path: string,
  body?: unknown,
  init?: Omit<RequestInit, 'method' | 'body' | 'headers'> & { headers?: Record<string, string> },
): Promise<ApiResponse<T>> {
  const url = getApiUrl(path)

  const headers: Record<string, string> = {
    ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
    ...(init?.headers ?? {}),
  }

  const res = await fetch(url, {
    method,
    credentials: 'include', // 给 refresh_token cookie 的写入/读取留出通道
    ...init,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })

  const json = await parseJsonSafely(res)
  handleUnauthorizedResponse(res, json)
  if (!json || typeof json !== 'object' || !('code' in json)) {
    return {
      code: res.status,
      data: null as T,
      msg: '响应解析失败',
    }
  }

  return json as ApiResponse<T>
}

