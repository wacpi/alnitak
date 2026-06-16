# 转码分离服务部署文档

## 概述

转码模块支持两种执行模式：

| 模式 | 说明 | 适用场景 |
|------|------|----------|
| `local`（默认） | 主进程内直接调用 ffmpeg 转码 | 单机部署、开发环境 |
| `remote` | 主进程推送任务到 Redis Stream，`transcoder-worker` 消费执行 | 多机集群、GPU 分离、扩缩容 |

两种模式对调用方透明——切换 mode 后 `VideoTransCoding()`、`GetVideoTranscodingProgress()`、`StopTranscodingAndCleanup()` 等接口行为一致。

## 架构

### Local 模式

```
请求 → API → TranscoderService → ffmpeg（本地子进程）
                                 → OSS（上传产物）
```

### Remote 模式

```
                         ┌──────────────────┐
请求 → API → RemoteTranscoder → Redis Stream
                         │                  │
                         │     ┌────────────┘
                         │     ↓
                         │  transcoder-worker (N 个副本)
                         │     │
                         │     ├── 从 OSS 下载源文件
                         │     ├── ffmpeg 编码
                         │     ├── 上传产物到 OSS
                         │     ├── 写状态到 Redis Hash
                         │     └── 发布完成通知
                         │
                   pollProgress（轮询 Redis Hash）
                         │
                    completeTransaction（落库）
```

## 前置条件

### Local 模式
- Go 1.22+
- ffmpeg + ffprobe 在 PATH 中
- MySQL
- OSS（可选，`oss_type=local` 时存本地）

### Remote 模式（额外）
- Redis 6.2+（Stream + Pub/Sub）
- Worker 节点可以访问 OSS
- Worker 节点可以访问 Redis

## 配置

### 配置项

```yaml
# config/application.prod.yaml

transcoding:
  # 执行模式：local | remote
  mode: "local"

  # 编码器
  use_gpu: false
  use_h265: false
  use_av1: false

  # 是否生成 1080p60 档
  generate_1080p60: false

  # 并发数（CPU/GPU 分别配置）
  max_cpu_concurrency: 2
  max_gpu_concurrency: 2

redis:
  host: "127.0.0.1"
  port: 6379
  password: ""

storage:
  oss_type: "aliyun"        # aliyun / minio / cloudflare / local
  endpoint: "https://..."
  bucket: "my-bucket"
  key_id: ""
  key_secret: ""
  region: ""
  domain: "https://..."
  upload_mp4_file: false     # 是否保留原文件
  # 可选：备用 OSS（多源容灾）
  backup:
    oss_type: "minio"
    endpoint: "http://..."
    bucket: "backup-bucket"
    key_id: ""
    key_secret: ""
    app_id: ""
    region: ""
    domain: ""
```

### 配置说明

| 字段 | 默认值 | 说明 |
|------|--------|------|
| `mode` | `local` | 切换 remote 前必须部署 worker |
| `use_gpu` | `false` | GPU 编码，启用后自动选 nvenc |
| `max_cpu_concurrency` | 2 | CPU 模式下编多少个 quality |
| `oss_type` | — | `local` 跳过后处理逻辑 |
| `backup` | nil | 配置后自动异步上传到备用 OSS |

## Local 模式部署

```powershell
# 编译
cd server
go build -o alnitak.exe .

# 确认 ffmpeg 可用
ffmpeg -version
ffprobe -version

# 运行
./alnitak.exe
```

Local 模式不需要额外运维——ffmpeg 在主进程内启动为子进程，转码进度通过共享内存维护，API 直接返回。

## Remote 模式部署

### Step 1：编译

```powershell
# 主服务
cd server
go build -o alnitak.exe .

# Worker
go build -o transcoder-worker.exe ./cmd/transcoder-worker
```

### Step 2：主服务配置

```yaml
transcoding:
  mode: "remote"

redis:
  host: "10.0.0.1"   # 指向共享 Redis
  port: 6379
```

主服务启动后只在必要配置下开启 Redis 连接——client 是 lazy 创建。

### Step 3：Worker 配置

Worker 复用主服务的 `conf/application.prod.yaml`。Worker 启动时从该配置读 `transcoding.*`、`redis.*`、`storage.*` 三个段。

```powershell
# 把编译产物和配置部署到 Worker 节点
transcoder-worker.exe
  -env prod                  # 加载 conf/application.prod.yaml
  -concurrency 2             # 最大并发转码数
  -id "worker-01"            # 标识（默认取 hostname）
  -health-port 9100          # 健康检查端口
```

Worker 支持多副本部署，通过 Redis Stream consumer group 自动负载。

### Step 4：验证连通

```powershell
# Worker 就绪检查
curl http://worker-01:9100/ready
# {"status":"ready","health":{"healthy":true,...}}

# Worker 统计
curl http://worker-01:9100/stats
```

### Step 5：Redis Stream 监控

```bash
# 查看队列积压
redis-cli XLEN transcoding:queue

# 查看消费者组状态
redis-cli XINFO GROUP transcoding:queue transcoder

# 查看死信队列
redis-cli XLEN transcoding:deadletter

# 查看指定资源转码状态
redis-cli HGETALL transcoding:status:{resourceID}
```

## Worker 健康检查

| 端点 | 说明 |
|------|------|
| `GET /health` | 存活检查，总是返回 `{"status":"ok"}` |
| `GET /ready` | 就绪检查，Redis 连通则返回 `ready` 否则 `503` |
| `GET /stats` | 运行时状态快照 |

```json
// GET /stats
{
  "healthy": true,
  "startedAt": "2026-06-17T10:00:00+08:00",
  "uptime": "3h12m5s",
  "concurrency": 2,
  "jobsActive": 1,
  "jobsTotal": 42,
  "jobsFailed": 0,
  "groupID": "transcoder-worker-worker-01"
}
```

## Docker 部署（参考）

```dockerfile
FROM golang:1.22 AS builder
WORKDIR /app
COPY . .
RUN go build -o alnitak ./server
RUN go build -o transcoder-worker ./server/cmd/transcoder-worker

FROM ubuntu:22.04
RUN apt-get update && apt-get install -y ffmpeg
COPY --from=builder /app/alnitak /
COPY --from=builder /app/transcoder-worker /
COPY --from=builder /app/server/conf /conf
```

## 扩缩容指南

### 水平扩容（Remote 模式）

启动更多 Worker 副本即可。多个 Worker 共享同一个 Redis consumer group：

```powershell
# 节点 A
transcoder-worker.exe -id "worker-a" -concurrency 2

# 节点 B
transcoder-worker.exe -id "worker-b" -concurrency 2
```

Redis 自动在消费者间分发消息。节点宕机后，pending 消息由存活的 Worker 通过 `recoverPending` 自动重新认领，无需人工干预。

### 垂直扩容（GPU 升级）

Worker 节点配置 `use_gpu: true`，主服务不直接感知 Worker 的 GPU 配置，Worker 按自身配置执行编码降级策略：

```
GPU AV1 → GPU H.265 → GPU H.264 → CPU H.264
```

## 故障恢复

### 消息投递保障

- 消费使用 Redis Stream consumer group，`XReadGroup` 读取
- Worker 处理完消息后 `XAck` 确认
- Worker 崩溃后，未确认消息在 5 分钟后被其他 Worker 认领
- 超过 3 次重试的消息移入 `transcoding:deadletter` 死信队列

### 死信处理

```bash
# 查看死信
redis-cli XLEN transcoding:deadletter
redis-cli XRANGE transcoding:deadletter - +

# 死信格式
# 1) originalMsgID - 原始消息 ID
# 2) reason - 失败原因
# 3) movedAt - 移入时间戳
```

### 幂等保证

- `saveIndexRecord` 使用 GORM `FirstOrCreate`，不会重复插入
- 多音轨记录同样使用 `FirstOrCreate`（`resource_id + language` 唯一约束）
- 回调处理 `handleRemoteCompletion` 会检查 DB 状态，不会重复处理

## 版本记录

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0 | 2026-06-17 | 初始版，支持 local/remote 模式 |
