# 开发纪要

> 最后更新: 2026-06-09
> 项目: alnitak (Go + Vue 3)

---

## 1. 最近完成的工作

### 1.1 OSS 上传可靠性改进

**核心改动：** 为 OSS 上传增加超时控制、指数退避重试、部分失败回滚。

#### 改动清单

| 文件 | 改动 |
|------|------|
| `pkg/oss/config.go` | +`Timeout int` 配置字段 |
| `internal/config/storage.go` | +`UploadTimeout int` (映射到 `oss_upload_timeout`) |
| `pkg/oss/index.go` | 传递 `c.UploadTimeout` → `config.Timeout` |
| `pkg/oss/aliyun.go` | `oss.Timeout(t, t)` via `[]oss.ClientOption` |
| `pkg/oss/minio.go` | `Transport` 设置 `ResponseHeaderTimeout` + 自定义 dialer |
| `pkg/oss/cloudflare.go` | `http.Client.Timeout` on `aws.Config` |
| `pkg/oss/local.go` | 添加 `DeleteObject(objectKey string) error` 空实现 |
| `internal/service/transcoding_constants.go` | 新建常量：`ossUploadRetryDelay=500ms`、`ossUploadMaxRetries=3`、`ossUploadBackoff=[1s,2s,4s]`、`ossUploadMaxConcurrency=3` |
| `internal/service/transcoding.go` | `uploadToOSS` 重写 — 3 次退避重试 + 部分失败调用 `DeleteObject` 回滚；新增 `setUploadProgress` |

**限制条件（已确认）：**
- 本地存储跳过上传
- MinIO v7.0.86 不支持 `HTTPClient`，用 `Transport` 替代
- 阿里云用 `oss.ClientOption` 类型而非 `oss.Option`
- OSS 类型值：`aliyun`、`minio`、`cloudflare`(R2)、`local`

### 1.2 上传进度追踪

- **后端：** `vo/video.go` 新增 `UploadProgressInfo{ossType, progress, status}`，注入到 `UploadVideoResp` 和 `VideoInfoManageResp`。
- **存储：** `resourceTranscodingProgress` 结构体通过 `setUploadProgress()` 原子写入，`GetVideoTranscodingProgress` 返回 `(*vo.UploadProgressInfo, ...)`。
- **前端：** `typings/video.d.ts` 新增 `UploadProgressInfo`；`index.vue` 新增"上传状态"列 + 展开行 OSS 上传进度条。
  - 中文映射：aliyun→阿里云OSS、minio→MinIO、cloudflare→Cloudflare R2、local→本地存储

### 1.3 上传失败重传机制

**新增状态码：** `UPLOAD_FAILED = 3001`（转码成功但上传失败，区别于 `PROCESSING_FAIL = 3000`）

**逻辑：**
- 转码成功但上传失败 → 标记资源 `UPLOAD_FAILED`，不改变 video 状态（保留在"处理中"列表），不清除转码产物
- `markUploadFailed()` 代替 `completeTransaction(..., PROCESSING_FAIL)` — 只更新 resource 表，不过问 video
- `ReUploadVideo()` 服务函数：查找 `UPLOAD_FAILED` 资源 → 从 `VideoFile.DirName` 重建输出目录 → 重跑 `uploadToOSS` → 成功后调用 `completeTransaction` 流转状态
- API 端点: `POST /api/v1/video/reUploadVideo?vid=xxx`
- **前端：** 处理中 tab→上传状态列显示"上传失败" Tag + "重新上传"按钮；展开行同样显示按钮
- 批量"全部重新上传"按钮只对 `PROCESSING_FAIL` 有效（传统转码失败），不涵盖 `UPLOAD_FAILED`

### 1.4 文件变动记录

```
后端:
  server/internal/global/constant.go          +UPLOAD_FAILED = 3001
  server/internal/global/resource_status.go   +case UPLOAD_FAILED → "upload_failed"
  server/internal/service/video.go            +ReUploadVideo() 服务函数
  server/internal/service/transcoding.go      +MarkUploadFailed() 导出方法; uploadToOSS/completeTransaction 导出
  server/internal/api/v1/video.go             +ReUploadVideo() API handler
  server/internal/routes/video_router.go      +reUploadVideo 路由
  server/internal/initialize/data.go          +reUploadVideo 权限条目

前端 (E:\web\alnitak):
  web/admin-client/src/api/video.ts           +reUploadVideoAPI
  web/admin-client/src/views/content/video/index.vue  +reUploadVideo 函数 + 按钮
```

---

## 2. 系统架构

### 目录结构

```
server/
├── internal/
│   ├── api/v1/              # HTTP handlers
│   ├── routes/              # 路由注册
│   ├── config/              # 配置结构体
│   ├── global/              # 全局常量 + 工具
│   ├── service/             # 业务逻辑 (transcoding, video)
│   │   ├── transcoding.go           # 转码核心逻辑
│   │   ├── transcoding_constants.go  # 转码常量
│   │   ├── video.go                  # 视频服务函数
│   ├── initialize/          # 启动初始化 (含权限数据)
│   └── domain/
│       ├── dto/             # 数据传输对象
│       ├── vo/              # 视图对象
│       └── model/           # GORM 模型
├── pkg/oss/                 # OSS SDK 封装
│   ├── config.go
│   ├── index.go
│   ├── aliyun.go
│   ├── minio.go
│   ├── cloudflare.go
│   ├── local.go

web/admin-client (E:\web\alnitak\web\admin-client)
└── src/
    ├── api/video.ts          # 视频 API
    ├── typings/video.d.ts    # 类型定义
    └── views/content/video/
        ├── index.vue              # 视频管理主页面
        └── components/
            ├── table-action-drawer.vue
            └── video-modal.vue
```

### 关键配置

| 项 | 默认值 | 说明 |
|----|--------|------|
| `oss_upload_timeout` | 30 (秒) | OSS 上传超时 |
| `upload_mp4_file` | false | 是否将源文件一并上传到 OSS |

### 转码流程

```
用户上传 → 创建 Video + Resource(VIDEO_PROCESSING)
         → ProcessVideo (goroutine)
           → ffprobe 探测 → 转码 (GPU/NVIDIA)
           → 转码成功 → uploadToOSS (超时+重试+回滚)
             → 成功 → completeTransaction(WAITING_REVIEW)
             → 失败 → markUploadFailed(UPLOAD_FAILED)
                        ↓ 后续手动点击"重新上传"
                     → ReUploadVideo(重跑 uploadToOSS)
```

---

## 3. 状态码（视频相关）

| 值 | 常量 | 说明 |
|----|------|------|
| 0 | CREATED_VIDEO | 创建 |
| 1 | VIDEO_PROCESSING | 处理中 |
| 2 | SUBMIT_REVIEW | 提交审核 |
| 3 | WAITING_REVIEW | 等待审核 |
| 4 | AUDIT_APPROVED | 审核通过 |
| -1 | AUDIT_REJECTED | 审核拒绝 |
| 3000 | PROCESSING_FAIL | 处理失败（含转码失败、文件缺失等） |
| **3001** | **UPLOAD_FAILED** | **转码成功，上传 OSS 失败（2026-06-09 新增）** |

---

## 4. 常见问题/Q&A

### Q: 重新上传按钮在哪？
A: 在处理中（processing）tab 下，展开行 OSS 上传进度区域。只有当资源状态为 `UPLOAD_FAILED`（上传失败）时可见。操作列也会显示"重新上传"按钮。

### Q: 重传失败后，之前的转码产物会被清吗？
A: 不会。`markUploadFailed` 只更新数据库状态，不清除磁盘文件。只有显式调用 `StopTranscodingAndCleanup` 才会清理。

### Q: 重新上传会重新转码吗？
A: 不会。`ReUploadVideo` 只跑 `uploadToOSS`，跳过转码步骤。视频文件已存在于 `./upload/video/<DirName>/` 目录下。

### Q: 为什么不同时提供"全部重新上传"按钮？
A: `UPLOAD_FAILED` 保持 video 状态为 `VIDEO_PROCESSING`，现有"全部重新转码"逻辑只针对 `PROCESSING_FAIL` 视频。按需单独重传即可。

---

## 5. 待做/局限

- [x] OSS 超时配置
- [x] 上传指数退避重试 + 部分失败回滚
- [x] 上传进度追踪（后端 API + 前端展示）
- [x] UPLOAD_FAILED 状态 + 重传机制
- [ ] 管理员审核通过后，`upload_failed` 的资源如何处理（当前会卡住）
- [ ] 重传成功后是否刷新 OSS CDN 缓存

---

## 6. 开发环境

- **后端路径**: `E:\server\alnitak\server`
- **前端路径**: `E:\web\alnitak\web\admin-client`
- **后端构建**: `go build ./...`（无错误）
- **前端类型检查**: `vue-tsc --noEmit`（无错误）
- **Go 版本约束**: 无 gopls LSP，靠 `go build` 验证
- **注意**: 后端和服务端代码在两个不同仓库，前端在 `E:\web\alnitak`
