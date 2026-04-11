# 历史 `video_codec` 对齐（不重编码）

## 目的

旧数据里 `video_index_file.video_codec` 可能长期为默认占位（如 `avc1.640028`），与真实 H.264 profile/level 不一致，会影响 MPD `codecs` 等。本工具用 **ffprobe** 对现有 `*_video.m4s` 探测，**仅当探测结果与当前库值不同** 时更新字段。

## 行为

- 只处理 **SegmentBase** 行：`video_file` 与 `video_init_range` 非空。
- 优先读本地：`{upload}/{DirName}/{VideoFile}`（默认 `upload=./upload/video`）。
- 默认仅用本地文件探测；若本地缺失且你显式加 `-use-oss`：从 `video/{DirName}/{VideoFile}` 下载到临时文件再探测。
- **ffprobe 失败**：跳过并打印，不写库。
- **探测值与库中相同**：静默跳过（大量时可每 500 条打进度）。

## 用法

在 **`server` 目录** 下执行（与 `./upload/video` 相对路径一致）：

```bash
# 先看会改哪些（强烈推荐）
go run ./cmd/fix_video_codec -env=prod -dry-run

# 真正写库（需交互确认 y）
go run ./cmd/fix_video_codec -env=prod

# 无人值守
go run ./cmd/fix_video_codec -env=prod -yes
```

常用参数：

| 参数 | 说明 |
|------|------|
| `-env` | `dev` / `prod` |
| `-dry-run` | 只打印 `old -> new`，不更新 |
| `-yes` | 跳过确认（非 dry-run） |
| `-upload` | 本地根目录，默认 `./upload/video` |
| `-use-oss` | 本地缺失时从 OSS 下载探测 |
| `-limit` | 最多处理条数，`0` 不限制 |

## 依赖

- 本机可执行 `ffprobe`（与转码环境一致）。
- 数据库与 `conf` 与线上一致；OSS 非 local 时需能访问存储。
