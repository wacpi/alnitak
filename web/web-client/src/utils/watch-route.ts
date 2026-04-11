import { navigateTo } from '#app';
import type { LocationQueryValueRaw } from 'vue-router';
import { getPGCEpisodeDetailAPI } from '@/api/pgc';
import { statusCode } from '@/utils/status-code';

function firstQueryValue(
  raw: LocationQueryValueRaw | LocationQueryValueRaw[] | undefined | null
): string {
  if (raw === undefined || raw === null) return '';
  if (Array.isArray(raw)) return String(raw[0] ?? '').trim();
  return String(raw).trim();
}

async function videoIdFromEpisodeId(epId: number): Promise<number> {
  const epRes = await getPGCEpisodeDetailAPI(epId);
  if (epRes?.data?.code !== statusCode.OK) {
    await navigateTo('/404');
    throw new Error('episode not found');
  }
  const ep = epRes.data.data?.episode || {};
  const resolvedVid = Number(ep?.vid || 0);
  if (!Number.isFinite(resolvedVid) || resolvedVid <= 0) {
    await navigateTo('/404');
    throw new Error('episode vid missing');
  }
  return resolvedVid;
}

export type ResolveWatchVideoIdResult =
  | { ok: true; videoId: string | number }
  | { ok: false; reason: 'invalid_ep' | 'episode_not_found' | 'no_identifier' };

/**
 * 播放页首次进入：必须存在有效 `ep` 或 `v`（与历史约定一致：`v` 须为 string）。
 */
export async function resolveWatchVideoIdForInitialLoad(
  query: Record<string, LocationQueryValueRaw | LocationQueryValueRaw[] | undefined>
): Promise<string | number> {
  const epRaw = firstQueryValue(query.ep);
  if (epRaw !== '') {
    const epId = Number(epRaw);
    if (!Number.isFinite(epId) || epId <= 0) {
      await navigateTo('/404');
      throw new Error('invalid ep');
    }
    return videoIdFromEpisodeId(epId);
  }
  const v = query.v;
  if (typeof v === 'string' && v.trim() !== '') {
    return v.trim();
  }
  await navigateTo('/404');
  throw new Error('missing v');
}

/**
 * 路由 query 变化时解析视频标识；无有效 `ep`/`v` 时返回 no_identifier（不跳转，由调用方决定）。
 */
export async function resolveWatchVideoIdOnQueryChange(
  query: Record<string, LocationQueryValueRaw | LocationQueryValueRaw[] | undefined>
): Promise<ResolveWatchVideoIdResult> {
  const epRaw = firstQueryValue(query.ep);
  if (epRaw !== '') {
    const epId = Number(epRaw);
    if (!Number.isFinite(epId) || epId <= 0) {
      await navigateTo('/404');
      return { ok: false, reason: 'invalid_ep' };
    }
    try {
      const vid = await videoIdFromEpisodeId(epId);
      return { ok: true, videoId: vid };
    } catch {
      return { ok: false, reason: 'episode_not_found' };
    }
  }

  const vRaw = query.v;
  const vStr = Array.isArray(vRaw) ? String(vRaw[0] ?? '').trim() : typeof vRaw === 'string' ? vRaw.trim() : '';
  if (vStr !== '') {
    return { ok: true, videoId: vStr };
  }
  return { ok: false, reason: 'no_identifier' };
}
