import { apiRequest, type ApiResponse } from '@/api/http'

export type LoginRespData = {
  token: string
  refreshToken: string
  userId: number
}

type LoginCaptchaData = {
  captchaId: string
}

export async function loginApi(args: {
  email: string
  password: string
  captchaId?: string
}): Promise<ApiResponse<LoginRespData | LoginCaptchaData>> {
  return apiRequest<LoginRespData>('POST', '/api/v1/auth/login', {
    email: args.email,
    password: args.password,
    captchaId: args.captchaId ?? '',
  })
}

export async function meApi(token: string): Promise<ApiResponse<{ userInfo: unknown }>> {
  return apiRequest<{ userInfo: unknown }>('GET', '/api/v1/auth/me', undefined, {
    headers: {
      Authorization: token,
    },
  })
}

export async function logoutApi(): Promise<ApiResponse<unknown>> {
  // 后端同时支持从 HttpOnly refresh_token Cookie 读取
  return apiRequest<unknown>('POST', '/api/v1/auth/logout', {})
}

export async function modifyPwdApi(args: {
  email: string
  password: string
  code: string
  captchaId?: string
}): Promise<ApiResponse<unknown>> {
  return apiRequest<unknown>('POST', '/api/v1/auth/modifyPwd', {
    email: args.email,
    password: args.password,
    code: args.code,
    captchaId: args.captchaId ?? '',
  })
}

