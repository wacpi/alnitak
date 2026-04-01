import { handleUnauthorizedResponse, type ApiResponse } from '@/api/http'

const API_BASE_URL = ((import.meta as any).env?.VITE_API_BASE_URL ?? '') as string

function normalizeBaseUrl(base: string) {
  return base.replace(/\/$/, '')
}

function getApiUrl(path: string) {
  const base = normalizeBaseUrl(API_BASE_URL)
  return `${base}${path.startsWith('/') ? path : `/${path}`}`
}

async function parseJsonSafely(res: Response) {
  try {
    return (await res.json()) as unknown
  } catch {
    return null
  }
}

export async function uploadImageApi(
  token: string,
  file: File,
): Promise<ApiResponse<{ url: string }>> {
  const form = new FormData()
  form.append('image', file)

  const res = await fetch(getApiUrl('/api/v1/upload/image'), {
    method: 'POST',
    credentials: 'include',
    headers: {
      Authorization: token,
    },
    body: form,
  })

  const json = await parseJsonSafely(res)
  handleUnauthorizedResponse(res, json)
  if (!json || typeof json !== 'object' || !('code' in json)) {
    return {
      code: res.status,
      data: null as any,
      msg: '响应解析失败',
    }
  }
  return json as ApiResponse<{ url: string }>
}

export type UploadVideoCreateResp = {
  resource: {
    vid: number
    title: string
    duration: number
    fileId?: number
  }
}

export async function checkVideoUploadApi(
  token: string,
  body: { hash: string; size: number },
): Promise<ApiResponse<{ chunks: number[]; fileID: number }>> {
  return fetchJsonWithAuth('/api/v1/upload/checkVideo', token, body)
}

export async function mergeVideoUploadApi(
  token: string,
  body: { hash: string; size: number; fileID?: number },
): Promise<ApiResponse<unknown>> {
  return fetchJsonWithAuth('/api/v1/upload/mergeVideo', token, body)
}

export async function createVideoUploadApi(
  token: string,
  body: { hash: string; size: number; fileID?: number },
): Promise<ApiResponse<UploadVideoCreateResp>> {
  return fetchJsonWithAuth('/api/v1/upload/video', token, body)
}

export async function uploadVideoChunkApi(
  token: string,
  args: {
    file: File
    hash: string
    chunkIndex: number
    totalChunks: number
    onProgress?: (percent: number) => void
  },
): Promise<ApiResponse<unknown>> {
  const form = new FormData()
  form.append('video', args.file)
  form.append('hash', args.hash)
  form.append('name', args.file.name)
  form.append('size', String(args.file.size))
  form.append('chunkIndex', String(args.chunkIndex))
  form.append('totalChunks', String(args.totalChunks))

  // 需要上传进度时使用 XHR（fetch 无法稳定拿到 upload progress）。
  if (typeof args.onProgress === 'function') {
    return new Promise<ApiResponse<unknown>>((resolve) => {
      const xhr = new XMLHttpRequest()
      xhr.open('POST', getApiUrl('/api/v1/upload/chunkVideo'), true)
      xhr.withCredentials = true
      xhr.setRequestHeader('Authorization', token)

      xhr.upload.onprogress = (evt) => {
        if (!evt.lengthComputable) return
        const percent = Math.max(0, Math.min(100, Math.round((evt.loaded / evt.total) * 100)))
        args.onProgress?.(percent)
      }

      xhr.onload = () => {
        let json: any = null
        try {
          json = xhr.responseText ? JSON.parse(xhr.responseText) : null
        } catch {
          json = null
        }
        if (!json || typeof json !== 'object' || !('code' in json)) {
          resolve({ code: xhr.status || 500, data: null as any, msg: '响应解析失败' })
          return
        }
        resolve(json as ApiResponse<unknown>)
      }

      xhr.onerror = () => {
        resolve({ code: xhr.status || 500, data: null as any, msg: '上传失败，请检查网络' })
      }

      xhr.send(form)
    })
  }

  const res = await fetch(getApiUrl('/api/v1/upload/chunkVideo'), {
    method: 'POST',
    credentials: 'include',
    headers: { Authorization: token },
    body: form,
  })
  const json = await parseJsonSafely(res)
  handleUnauthorizedResponse(res, json)
  if (!json || typeof json !== 'object' || !('code' in json)) {
    return { code: res.status, data: null as any, msg: '响应解析失败' }
  }
  return json as ApiResponse<unknown>
}

async function fetchJsonWithAuth<T>(path: string, token: string, body: unknown): Promise<ApiResponse<T>> {
  const res = await fetch(getApiUrl(path), {
    method: 'POST',
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      Authorization: token,
    },
    body: JSON.stringify(body),
  })
  const json = await parseJsonSafely(res)
  handleUnauthorizedResponse(res, json)
  if (!json || typeof json !== 'object' || !('code' in json)) {
    return { code: res.status, data: null as any, msg: '响应解析失败' }
  }
  return json as ApiResponse<T>
}

