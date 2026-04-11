# 后端配置修改说明

## 修改内容

本次修改将后端返回的视频 URL 从**相对路径**改为**直链**，使播放器可以直接加载媒体文件，支持 `Media(start:)` 精确 seek。

## 需要修改的配置

在 `config.yaml` 中添加 `domain` 配置：

```yaml
storage:
  # 存储类型: local, aliyun, minio, tencent, cloudflare
  oss_type: "local"
  
  # 【关键】添加域名配置，用于生成本地存储的直链
  # 格式: http://域名:端口 或 https://域名
  # 例如: http://anime.ayypd.cn:9000
  domain: "http://your-domain:port"
  
  # 其他配置保持不变...
  key_id: ""
  key_secret: ""
  bucket: ""
  endpoint: ""
```

## 配置示例

### 本地存储（开发环境）
```yaml
storage:
  oss_type: "local"
  domain: "http://localhost:9000"
```

### 本地存储（生产环境）
```yaml
storage:
  oss_type: "local"
  domain: "http://anime.ayypd.cn:9000"
```

### OSS 存储（阿里云）
```yaml
storage:
  oss_type: "aliyun"
  # OSS 不需要配置 domain，会自动生成签名直链
  key_id: "your-key-id"
  key_secret: "your-key-secret"
  bucket: "your-bucket"
  endpoint: "oss-cn-hangzhou.aliyuncs.com"
```

## 修改的文件

- `internal/service/video.go`
  - 新增 `getMediaFileURL()` 函数
  - 修改 `buildPlayURLJSON()` 使用直链
  - 修改 `buildMPDSegmentBase()` 使用直链
  - 修改 `buildM3U8MasterSegmentBase()` 使用直链
  - 修改 `buildM3U8VideoSegmentBase()` 使用直链
  - 修改 `buildM3U8AudioSegmentBase()` 使用直链
  - 修改 `buildM3U8SegmentList()` 使用直链
  - 修改 `buildMPDSegmentList()` 使用直链

## 验证方法

1. 重启后端服务
2. 请求视频播放地址：
   ```
   GET /api/v1/video/getVideoFile?resourceId=88&quality=720p&format=json
   ```
3. 检查返回的 JSON 中 `baseUrl` 是否为完整 URL：
   ```json
   {
     "code": 0,
     "data": {
       "dash": {
         "video": [{
           "baseUrl": "http://anime.ayypd.cn:9000/api/v1/video/stream/xxx.m4s?key=xxx"
         }]
       }
     }
   }
   ```

## 注意事项

1. **必须配置 domain**：如果不配置，本地存储会退回到相对路径，seek 功能可能失效
2. **CORS 配置**：确保域名支持跨域访问（本地存储已配置 CORS）
3. **防火墙**：确保端口对外开放
4. **HTTPS**：生产环境建议使用 HTTPS

## 回滚方法

如果出现问题，只需将 `domain` 留空或删除：
```yaml
storage:
  oss_type: "local"
  domain: ""  # 留空会使用相对路径（向后兼容）
```
