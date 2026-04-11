import { apiRequest, type ApiResponse } from '@/api/http'

export type UploadVideoItem = {
  vid: number
  title: string
  duration: number
  createdAt?: string
  status?: number
}

export async function getUploadVideoListApi(
  token: string,
  args: { page?: number; pageSize?: number; category?: string } = {},
): Promise<ApiResponse<{ total: number; videos: UploadVideoItem[] }>> {
  const q = new URLSearchParams()
  q.set('page', String(args.page ?? 1))
  q.set('pageSize', String(args.pageSize ?? 50))
  q.set('category', args.category ?? 'all')
  return apiRequest<{ total: number; videos: UploadVideoItem[] }>('GET', `/api/v1/video/getUploadVideo?${q.toString()}`, undefined, {
    headers: {
      Authorization: token,
    },
  })
}

