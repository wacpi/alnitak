/** 用于跳转播放页的路径参数：与后端 ParseVideoID 一致，优先 opaque shortId */
export function videoPathId(v: Pick<VideoType, 'vid'> & { shortId?: string }): string {
  const sid = v.shortId?.trim()
  if (sid) return sid
  return String(v.vid)
}
