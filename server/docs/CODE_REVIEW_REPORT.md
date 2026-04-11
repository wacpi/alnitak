# Alnitak 项目代码审查报告

> 审查范围：server 目录下主要业务代码  
> 审查时间：2025-03

---

## 一、Bug / 逻辑问题

### 1. OSS 上传错误未处理（upload.go）

**位置**：`internal/service/upload.go`

- **L87-88**：`UploadImg` 中 `PutObjectFromFile` 返回值未检查，上传失败时仍返回成功
- **L655-657**：`initVideo` 中封面 OSS 上传 `PutObjectFromFile` 返回值未检查，失败时无感知

**影响**：图片/封面上传到 OSS 失败时，接口仍返回成功，用户看到的是无效链接。

**建议**：检查 `PutObjectFromFile` 返回值，失败时记录日志并返回错误。

---

### 2. 封面生成失败未中断流程（upload.go）

**位置**：`internal/service/upload.go` L651-654

```go
if err := GenerateCover(videoPath, filePath); err != nil {
    utils.ErrorLog("生成封面失败", "upload", ...)
}
// 未 return，继续执行
```

**影响**：封面生成失败时仍继续创建视频记录，可能产生无封面视频。

**建议**：封面生成失败时直接 `return 0, err`，或至少标记为“封面缺失”状态。

---

### 3. 视频切片路径遍历风险（file.go）

**位置**：`internal/api/v1/file.go` L71-84, L93-129

`GetVideoSlice` / `GetVideoStream` 中 `file` 来自 `ctx.Param("file")`，未做路径遍历校验：

```go
filePath := "./upload/video/" + dir + "/" + file
ctx.File(filePath)  // 或 http.ServeFile
```

若 `file` 为 `../../../etc/passwd` 等，可能访问到预期外文件。`buildFromLegacyContent` 中 `line` 来自数据库 `Content`，若被污染也可能构造恶意 URL。

**建议**：对 `file` 做白名单校验（如仅允许 `[a-zA-Z0-9_.-]+`），或使用 `filepath.Clean` + 前缀检查，确保结果仍在 `./upload/video/<dir>/` 下。

---

### 4. VideoCodec 硬编码与 1080p60 不匹配

**位置**：
- `internal/service/transcoding.go` L646：`VideoCodec: "avc1.640028"`
- `internal/service/video.go` L664：`codec = "avc1.640028"`（buildMPDSegmentList 兜底）

`avc1.640028` 表示 H.264 High@L4.0，1080p60 通常需要 Level 4.2（如 `avc1.64002A`）。

**影响**：部分播放器可能误判 1080p60 流不支持，导致 ABR 切换异常。

**建议**：按实际分辨率/帧率选择 codec，或至少对 60fps 使用 `avc1.64002A`。

---

## 二、设计 / 架构问题

### 5. 删除视频后本地目录未立即清理

**位置**：`internal/service/video.go` `deleteVideoAndRelatedData`、`internal/service/upload.go` `decreaseVideoFileRefCount`

删除视频时：
- 会停止转码并清理转码产物
- `decreaseVideoFileRefCount` 在引用为 0 时只删除 `VideoFile` 记录
- 物理目录（含 `upload.*` 源文件）依赖定时任务 `CleanupOrphanedResources` 清理

**影响**：删除视频后，磁盘占用会延迟释放，高删除量时可能堆积大量孤立目录。

**建议**：在 `decreaseVideoFileRefCount` 中，当 `RefCount` 归零且删除 `VideoFile` 时，同步删除对应本地目录；或缩短清理任务周期。

---

### 6. MP4 box 解析逻辑重复

**位置**：`internal/service/transcoding.go` 与 `internal/service/video.go`

- `getMP4InitRange`：遍历 ftyp/moov/sidx，解析 box size
- `parseSidxBox`：同样遍历 box，解析 sidx

两处都有 box size 解析、扩展 size、`boxSize==0` 等逻辑，实现略有差异（如 `binary.BigEndian` vs 手动移位）。

**建议**：抽取公共的 MP4 box 遍历/解析函数，减少重复和潜在不一致。

---

### 7. baseURL 构建逻辑重复

**位置**：`internal/service/video.go`

以下多处重复相同逻辑：

```go
var baseURL string
if global.Config.Storage.OssType == "local" && global.Config.Storage.Domain != "" {
    baseURL = global.Config.Storage.Domain
}
```

出现于：L498-500、L621-623、L669-671 等。

**建议**：提取为 `getLocalBaseURL() string`，统一复用。

---

## 三、重复代码

### 8. MPD/M3U8 构建中 Representation 结构重复

**位置**：`internal/service/video.go`

- `buildMPDSegmentBase`：单 Representation
- `buildMPDSegmentBaseUnified`：多 Representation 循环
- `buildMPDSegmentList`：SegmentList 模式
- `buildM3U8MasterSegmentBase`、`buildM3U8VideoSegmentBase`、`buildM3U8AudioSegmentBase` 等

XML/M3U8 字符串拼接模式相似，可考虑模板或辅助函数减少重复。

---

### 9. “通过 DirName 查找 VideoFile” 逻辑多处重复

**位置**：`video.go`、`resource.go`、`upload.go` 等

多处出现：

```go
} else if indexFile.DirName != "" {
    var vf model.VideoFile
    if global.Mysql.Where("dir_name = ?", indexFile.DirName).First(&vf).Error == nil {
        decreaseVideoFileRefCount(vf.ID, ...)
    }
}
```

**建议**：封装为 `findVideoFileByDirName(dirName string) (*model.VideoFile, error)`，统一调用。

---

## 四、冗余 / 可优化代码

### 10. pgc.go 中 defer recover 与显式 Rollback 重复

**位置**：`internal/service/pgc.go` L78-83、L214-219

```go
defer func() {
    if r := recover(); r != nil {
        tx.Rollback()
        ...
    }
}()
// 后面还有显式 tx.Rollback()
```

`recover` 主要应对 panic，正常分支已有 `tx.Rollback()`。若 panic 后不再执行后续逻辑，defer 中的 Rollback 有意义；否则存在重复处理。

**建议**：明确 panic 场景下的处理策略，避免重复 Rollback 或多余 defer。

---

### 11. buildMPDSegmentList 中 if/else 分支重复

**位置**：`internal/service/video.go` L684-696

```go
if file.InitFile != "" {
    sb.WriteString(`        <SegmentList timescale="1000" duration="%d">`...)
    sb.WriteString(`          <Initialization sourceURL="%s"/>`...)
} else {
    sb.WriteString(`        <SegmentList timescale="1000" duration="%d">`...)
    // 仅少了 Initialization
}
```

两分支都写 `SegmentList`，仅 else 少写 `Initialization`。

**建议**：先写 `SegmentList`，再在 `InitFile != ""` 时写 `Initialization`，减少重复。

---

### 12. 魔数未集中管理

**位置**：多处

- 转码并发数：2 / 3
- GPU 失败阈值：3
- 等待音频超时：3000 次 × 100ms
- 分页上限：30、100
- 缓存时间：18000 秒

**建议**：在 `internal/config` 或常量文件中集中定义，便于调整和文档化。

---

## 五、其他建议

### 13. 缺少单元测试

项目内未发现 `*_test.go` 文件，转码、MP4 解析、权限等核心逻辑缺少自动化测试。

**建议**：至少为 `getMP4InitRange`、`parseSidxBox`、`formatDuration`、`dashRepresentationLess` 等纯函数补充测试。

---

### 14. 错误信息对用户暴露过多

部分 `errors.New` 直接返回底层错误（如数据库、文件系统），可能泄露内部实现。

**建议**：对用户返回统一、简化的错误文案，详细错误仅记录日志。

---

### 15. Redis Get 返回值未校验

**位置**：`internal/cache/slice.go`、`internal/cache/video.go` 等

`global.Redis.Get(key)` 可能返回 `redis.Nil`，部分调用未显式处理。

**建议**：统一处理 `redis.Nil`，避免将空字符串与“键不存在”混淆。

---

## 六、总结

| 类别       | 数量 | 优先级 |
|------------|------|--------|
| Bug/逻辑   | 4    | 高     |
| 设计问题   | 2    | 中     |
| 重复代码   | 2    | 低     |
| 冗余/优化  | 2    | 低     |
| 其他建议   | 3    | 中     |

**优先处理**：  
1. OSS 上传错误处理（#1）  
2. 视频切片路径遍历校验（#3）  
3. 封面生成失败流程控制（#2）  
4. VideoCodec 与 1080p60 的匹配（#4）
