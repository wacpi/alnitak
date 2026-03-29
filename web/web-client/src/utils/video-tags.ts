/**
 * 兼容后端 tags：JSON 数组 或 历史 CSV 字符串。
 */
export function normalizeVideoTags(tags: unknown): string[] {
  if (Array.isArray(tags)) {
    return tags.map((t) => String(t).trim()).filter(Boolean);
  }
  if (typeof tags === 'string' && tags.trim()) {
    return tags.split(',').map((t) => t.trim()).filter(Boolean);
  }
  return [];
}
