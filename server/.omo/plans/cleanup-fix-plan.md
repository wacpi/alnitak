# 后端清理系统修复计划

> Priority order: P0 > P1 > P2（按严重度和影响面排序）

---

## P0 — 可靠性修复（可能引发崩溃/数据丢失）

### 1. 清理并发安全 — sync.Mutex

**文件**: `internal/service/cleanup.go`

**问题**：Cron 每日 3:00 + 手动 API 可同时触发清理，多实例部署更会全部同时跑。

**方案**：在 `CleanupOrphanedResources` 入口加包级别 `sync.Mutex` + `sync/atomic` 标志位：

```go
var (
    cleanupMu     sync.Mutex
    cleanupRunning int32
)

func CleanupOrphanedResources(dryRun bool) CleanupResult {
    if !dryRun {
        if !atomic.CompareAndSwapInt32(&cleanupRunning, 0, 1) {
            // 已有清理任务运行中，直接返回空结果
            return CleanupResult{Errors: []string{"清理任务正在进行中"}}
        }
        defer atomic.StoreInt32(&cleanupRunning, 0)
    }
    cleanupMu.Lock()
    defer cleanupMu.Unlock()
    // ...
}
```

- dryRun 不阻塞（预览可以并行）
- 非 dryRun 互斥，重复调用直接返回

### 2. 清理事务保护 — GORM Transaction

**文件**: `internal/service/cleanup.go` — `cleanVideoDirDbRecords`

**问题**：连续多条 DELETE 不在事务内，崩溃导致 DB 和文件系统不一致。

**方案**：将 `cleanVideoDirDbRecords` 的 4 步 DELETE 包在 `global.Mysql.Transaction` 中：
- 收集 ResourceID（现有逻辑在事务外执行即可，不修改数据）
- 事务内：DELETE VideoIndexFile + VideoFileRef + VideoFile + Resource
- 事务成功后：删除本地目录 + OSS 文件（IO 操作放事务外，避免长事务）

### 3. Auth 中间件 nil claims — 防御性 nil 检查

**文件**: `internal/middleware/auth.go` — `Auth()` 函数

**问题**：`ParseToken` 返回 error 时 checked `claims.TokenType`，但 `claims` 可能为 nil。

**方案**：将过期检查移到 claims 非 nil 的路径下：

```go
_, claims, err := jwt_parse.ParseToken(tokenString)
if err != nil {
    // 先判断 claims 是否有效
    if claims != nil && errors.Is(err, jwt.ErrTokenExpired) && claims.TokenType == 0 {
        // 过期但 token 结构正确 → 提示刷新
        resp.Result(ctx, 3000, nil, "TOKEN过期")
        ctx.Abort()
        return
    }
    resp.Result(ctx, 2000, nil, "token验证失败")
    ctx.Abort()
    return
}
```

**关联文件**：`pkg/jwt/jwt.go` — `ParseToken` 需确保签名无效等场景返回 `(nil, nil, err)` 而非 `(nil, &Claims{...}, err)`，保持契约一致。

---

## P1 — 健壮性修复（特定场景出问题）

### 4. Cron panic recover

**文件**: `internal/cron/cron.go`

**问题**：所有 `gocron` 回调函数无 `recover`，任一任务 panic 会杀死整个 cron goroutine。

**方案**：每个 Do 调用包一层 `safeDo`：

```go
func safeDo(fn func()) func() {
    return func() {
        defer func() {
            if r := recover(); r != nil {
                zap.L().Error("cron任务panic恢复",
                    zap.Any("recover", r),
                    zap.Stack("stack"))
            }
        }()
        fn()
    }
}
```

所有 `c.Every(...).Do(...)` 改为 `c.Every(...).Do(safeDo(fn))`

### 5. 图片引用检查补全（已完成 ✓）

**状态**: 已在本次提交中完成。新增 PGCMedia、PGCContent、Collection 三张表的 cover 字段检查。

### 6. 字幕清理（已完成 ✓）

**状态**: 已在本次提交中完成。新增 `cleanOrphanedSubtitles` + `deleteOrphanedSubtitleFile`。

---

## P2 — 代码风格统一

### 7. 错误日志统一 — zap.L() 替代 utils.ErrorLog/InfoLog

**文件**: 全仓库

**问题**：三种日志风格混用：
- `zap.L().Error(...)` 
- `utils.ErrorLog(...)` — 内部封装了 zap
- `utils.InfoLog(...)`

**方案**：全仓库统一使用 `zap.L().Error(...)` / `zap.L().Info(...)`，废弃 `utils/log.go` 中的封装。

**注意**：涉及文件约 20+，采用 `subagent-driven-development` 并行替换。`utils/log.go` 保留函数签名加 `// Deprecated` 标记，不删除（防止第三方引用）。

### 8. API 错误响应统一

**文件**: `internal/resp/response.go`, `internal/api/v1/*.go`

**问题**：混合使用 `resp.FailWithMessage`、`resp.Result(ctx, code, nil, msg)` 等。

**方案**：
- 统一走 `resp.FailWithMessage(ctx, msg)` 或 `resp.Result(ctx, code, data, msg)`（带数据时）
- 删除无效的 code 分支，标准化 HTTP 语义（200=成功，400=参数错，401=未登录，403=权限不足，500=服务端错）

### 9. 清理 OSS 硬编码路径抽象

**文件**: `internal/service/cleanup.go` — `deleteVideoFromOSS`

**问题**：OSS objectKey 格式 `"video/" + dirName + "/" + fileName` 硬编码，和 `deleteOrphanedSubtitleFile` 中 `"subtitle/" + fileName` 不统一。

**方案**：提取常量：

```go
const (
    ossPrefixVideo    = "video"
    ossPrefixImage    = "image"
    ossPrefixSubtitle = "subtitle"
)
```

---

## 执行顺序建议

```
Round 1 (P0，必须):
  ├── 1. 并发安全锁     → cleanup.go
  ├── 2. 事务保护       → cleanup.go
  └── 3. Auth nil claims → auth.go + jwt.go

Round 2 (P1，推荐):
  └── 4. Cron recover   → cron.go

Round 3 (P2，代码整洁):
  ├── 7. 日志统一       → 全仓库并行替换
  ├── 8. API 响应统一   → resp + api handlers
  └── 9. OSS 前缀常量   → cleanup.go
```

每轮完成后 `go build` 验证 + 运行预览 `getCleanupPreview` 确认不影响现有功能。
