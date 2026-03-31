import { apiRequest } from '@/api/http'
import type { ApiResponse } from '@/api/http'

export type SliderCaptchaData = {
  slider_img: string
  bg_img: string
  y: number
}

export type GetSliderCaptchaRespData = {
  slider_captcha: SliderCaptchaData
}

export async function getSliderCaptchaApi(
  captchaId: string,
): Promise<ApiResponse<GetSliderCaptchaRespData>> {
  const q = encodeURIComponent(captchaId)
  return apiRequest<GetSliderCaptchaRespData>('GET', `/api/v1/verify/captcha/get?captchaId=${q}`)
}

export async function validateSliderApi(args: {
  captchaId: string
  x: number
}): Promise<ApiResponse<unknown>> {
  return apiRequest<unknown>('POST', '/api/v1/verify/captcha/validate', {
    captchaId: args.captchaId,
    x: args.x,
  })
}

export type SendEmailCodeResp200 = {
  countdown: number
}

export type SendEmailCodeRespNeedCaptcha = {
  captchaId: string
}

export async function sendEmailCodeApi(args: {
  email: string
  captchaId?: string
}): Promise<ApiResponse<SendEmailCodeResp200 | SendEmailCodeRespNeedCaptcha>> {
  return apiRequest<SendEmailCodeResp200 | SendEmailCodeRespNeedCaptcha>(
    'POST',
    '/api/v1/verify/getEmailCode',
    {
      email: args.email,
      captchaId: args.captchaId ?? '',
    },
  )
}

