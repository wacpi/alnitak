import { apiRequest, type ApiResponse } from '@/api/http'

export type PgcType = 1 | 2 | 3 | 4 | 5

// 后端目前直接返回 model.PGCContent，字段名可能是驼峰（PGCID/PGCType）或 snake_case（pgc_id/pgc_type）。
// 这里做一个“宽松”结构，页面侧再做字段兜底取值。
export type PgcContentLoose = Record<string, any>

export type PgcListRespData = {
  total: number
  list: PgcContentLoose[]
  page: number
  page_size: number
}

export async function getPgcListApi(args: {
  page: number
  pageSize: number
  pgcType?: number
  status?: number
  keyword?: string
  year?: number
  area?: string
  isOngoing?: boolean
}): Promise<ApiResponse<PgcListRespData>> {
  const q = new URLSearchParams()
  q.set('page', String(args.page))
  q.set('page_size', String(args.pageSize))
  if (args.pgcType != null && args.pgcType > 0) q.set('pgc_type', String(args.pgcType))
  if (args.status != null && args.status >= 0) q.set('status', String(args.status))
  if (args.keyword) q.set('keyword', args.keyword)
  if (args.year != null && args.year > 0) q.set('year', String(args.year))
  if (args.area) q.set('area', args.area)
  if (args.isOngoing) q.set('is_ongoing', 'true')

  return apiRequest<PgcListRespData>('GET', `/api/v1/pgc/list?${q.toString()}`)
}

export type CreatePgcEpisodeReq = {
  episode_number: number
  title: string
  vid: number
  duration: number
  publish_time: string
}

export type CreatePgcReq = {
  pgc_type: PgcType
  title: string
  cover: string
  desc: string
  year: number
  area: string
  rating: number
  is_ongoing: boolean
  /** 可选：创建时一起创建多集时传入；默认空数组，剧集在编辑页逐集添加 */
  episodes?: CreatePgcEpisodeReq[]
  /** 无剧集时可选：计划总集数（写入 total_episodes，current_episodes 为 0） */
  total_episodes?: number
}

export async function createPgcApi(token: string, body: CreatePgcReq): Promise<ApiResponse<{ pgc_id: string }>> {
  return apiRequest<{ pgc_id: string }>('POST', '/api/v1/pgc/create', body, {
    headers: {
      Authorization: token,
    },
  })
}

/** 从 create 响应取 pgc_id（应由后端以 JSON 字符串返回，避免 Snowflake 在 JS 中丢精度） */
export function pickPgcIdFromCreateData(data: unknown): string {
  if (data == null || typeof data !== 'object') return ''
  const raw = (data as Record<string, unknown>).pgc_id
  if (typeof raw === 'string') return raw.trim()
  // 仅信任安全整数；超大整数若以 number 返回已被 JSON 损坏，拒绝使用以免跳错页
  if (typeof raw === 'number' && Number.isSafeInteger(raw)) return String(raw)
  return ''
}

export type PgcDetailWithEpisodesData = {
  pgc: PgcContentLoose
  episodes: Record<string, any>[]
}

export async function getPgcDetailWithEpisodesApi(pgcId: string): Promise<ApiResponse<PgcDetailWithEpisodesData>> {
  const q = new URLSearchParams()
  q.set('pgc_id', pgcId)

  // 优先走后端聚合接口；若因状态过滤返回“PGC内容不存在”，自动降级为 detail + episodes 两次请求拼装
  const primary = await apiRequest<PgcDetailWithEpisodesData>('GET', `/api/v1/pgc/detail-with-episodes?${q.toString()}`)
  if (primary.code === 200) return primary

  const [detailRes, episodesRes] = await Promise.all([
    apiRequest<{ pgc: PgcContentLoose }>('GET', `/api/v1/pgc/detail?${q.toString()}`),
    apiRequest<{ episodes: Record<string, any>[] }>('GET', `/api/v1/pgc/${pgcId}/episodes?page=1&page_size=100`),
  ])

  if (detailRes.code !== 200) {
    return {
      code: detailRes.code,
      data: { pgc: {}, episodes: [] },
      msg: detailRes.msg || primary.msg || '加载失败',
    }
  }

  if (episodesRes.code !== 200) {
    return {
      code: episodesRes.code,
      data: { pgc: detailRes.data?.pgc ?? {}, episodes: [] },
      msg: episodesRes.msg || '加载剧集失败',
    }
  }

  return {
    code: 200,
    data: {
      pgc: detailRes.data?.pgc ?? {},
      episodes: episodesRes.data?.episodes ?? [],
    },
    msg: 'ok',
  }
}

export type UpdatePgcReq = {
  // Snowflake uint64，必须用 string 保真；后端用 `json:",string"` 接收
  pgc_id: string
  title: string
  cover: string
  desc: string
  year: number
  area: string
  rating: number
  is_ongoing: boolean
  total_episodes: number
}

export async function updatePgcApi(token: string, body: UpdatePgcReq): Promise<ApiResponse<unknown>> {
  return apiRequest<unknown>('PUT', '/api/v1/pgc/update', body, {
    headers: {
      Authorization: token,
    },
  })
}

export type AddPgcEpisodeReq = CreatePgcEpisodeReq

export async function addPgcEpisodeApi(
  token: string,
  pgcId: string,
  body: AddPgcEpisodeReq,
): Promise<ApiResponse<unknown>> {
  return apiRequest<unknown>('POST', `/api/v1/pgc/${pgcId}/episodes/add`, body, {
    headers: {
      Authorization: token,
    },
  })
}

export async function deletePgcEpisodeApi(
  token: string,
  pgcId: string,
  episodeId: number,
): Promise<ApiResponse<unknown>> {
  return apiRequest<unknown>('DELETE', `/api/v1/pgc/${pgcId}/episodes/${episodeId}`, undefined, {
    headers: {
      Authorization: token,
    },
  })
}

export async function deletePgcApi(token: string, pgcId: string): Promise<ApiResponse<unknown>> {
  return apiRequest<unknown>('DELETE', `/api/v1/pgc/${pgcId}`, undefined, {
    headers: {
      Authorization: token,
    },
  })
}

export async function updatePgcStatusApi(
  token: string,
  pgcId: string,
  status: number,
): Promise<ApiResponse<unknown>> {
  return apiRequest<unknown>('PUT', `/api/v1/pgc/${pgcId}/status`, { pgc_id: pgcId, status }, {
    headers: {
      Authorization: token,
    },
  })
}

export async function updatePgcEpisodeStatusApi(
  token: string,
  pgcId: string,
  episodeId: number,
  status: number,
): Promise<ApiResponse<unknown>> {
  return apiRequest<unknown>(
    'PUT',
    `/api/v1/pgc/${pgcId}/episodes/${episodeId}/status`,
    { status },
    {
      headers: {
        Authorization: token,
      },
    },
  )
}

export async function updatePgcEpisodeApi(
  token: string,
  pgcId: string,
  episodeId: number,
  body: { title: string },
): Promise<ApiResponse<unknown>> {
  return apiRequest<unknown>('PUT', `/api/v1/pgc/${pgcId}/episodes/${episodeId}`, body, {
    headers: {
      Authorization: token,
    },
  })
}

