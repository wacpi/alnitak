# PGC API 同步指南

## 概述

新增的PGC API已添加到 `authApiDesc` 表中，需要同步到数据库的 `api` 表。

---

## 已添加的PGC API

### 需要鉴权的API（已添加到 authApiDesc）

| 方法 | 路径 | 说明 | 分类 |
|------|------|------|------|
| POST | /api/v1/pgc/create | 创建PGC内容 | PGC |
| PUT | /api/v1/pgc/update | 更新PGC内容 | PGC |
| DELETE | /api/v1/pgc/:pgc_id | 删除PGC内容 | PGC |
| POST | /api/v1/pgc/:pgc_id/episodes/add | 添加PGC剧集 | PGC |
| DELETE | /api/v1/pgc/:pgc_id/episodes/:id | 删除PGC剧集 | PGC |

### 公开的API（不需要鉴权，不写入api表）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/pgc/list | 获取PGC列表 |
| GET | /api/v1/pgc/detail | 获取PGC详情 |
| GET | /api/v1/pgc/:pgc_id/episodes | 获取剧集列表 |
| GET | /api/v1/pgc/search | 搜索PGC内容 |
| GET | /api/v1/pgc/type/:type | 按类型获取PGC |
| GET | /api/v1/pgc/ongoing | 获取连载中PGC |
| GET | /api/v1/pgc/recommended | 获取推荐PGC |
| GET | /api/v1/pgc/detail-with-episodes | 获取PGC详情及剧集 |

---

## 同步API到数据库

### 方法1：启动时自动同步（推荐）

在启动服务时添加 `-api` 参数：

```bash
# 开发环境
go run cmd/main.go -env=dev -api

# 生产环境
go run cmd/main.go -env=prod -api

# 编译后运行
./cmd.exe -env=prod -api
```

**说明**：
- `-api` 参数会触发 `SyncApiData()` 函数
- 该函数会自动将 `authApiDesc` 中定义的API同步到数据库
- 只会同步数据库中不存在的API，不会重复添加

### 方法2：手动SQL插入

如果不想重启服务，可以手动插入SQL：

```sql
-- PGC管理API
INSERT INTO api (method, path, category, desc, created_at, updated_at)
VALUES
('POST', '/api/v1/pgc/create', 'PGC', '创建PGC内容', NOW(), NOW()),
('PUT', '/api/v1/pgc/update', 'PGC', '更新PGC内容', NOW(), NOW()),
('DELETE', '/api/v1/pgc/:pgc_id', 'PGC', '删除PGC内容', NOW(), NOW()),
('POST', '/api/v1/pgc/:pgc_id/episodes/add', 'PGC', '添加PGC剧集', NOW(), NOW()),
('DELETE', '/api/v1/pgc/:pgc_id/episodes/:id', 'PGC', '删除PGC剧集', NOW(), NOW());
```

---

## 验证同步结果

### 检查API表

```sql
SELECT * FROM api WHERE category = 'PGC';
```

应该能看到5条记录。

### 检查日志

同步成功后会看到日志输出：

```
INFO  自动同步API数据完成  newCount=5  module=initialize
INFO  新增API  method=POST  path=/api/v1/pgc/create  category=PGC  desc=创建PGC内容
INFO  新增API  method=PUT  path=/api/v1/pgc/update  category=PGC  desc=更新PGC内容
INFO  新增API  method=DELETE  path=/api/v1/pgc/:pgc_id  category=PGC  desc=删除PGC内容
INFO  新增API  method=POST  path=/api/v1/pgc/:pgc_id/episodes/add  category=PGC  desc=添加PGC剧集
INFO  新增API  method=DELETE  path=/api/v1/pgc/:pgc_id/episodes/:id  category=PGC  desc=删除PGC剧集
```

如果API已存在，会看到：

```
INFO  API数据已是最新，无需同步  module=initialize
```

---

## 后续添加新API

下次添加新的需要鉴权的API时，只需：

1. 在 `internal/initialize/data.go` 的 `authApiDesc` map 中添加API
2. 如果是新分类，在 `inferCategory` 函数的 `categoryMap` 中添加映射
3. 启动服务时加上 `-api` 参数即可自动同步

### 示例

假设要添加一个新的PGC审核API：

```go
// 在 authApiDesc 中添加
"POST|/api/v1/pgc/reviewApproved": "PGC审核通过（后台管理）",
```

然后运行：
```bash
go run cmd/main.go -env=dev -api
```

---

## 注意事项

1. **只同步需要鉴权的API**：公开的API不需要写入api表
2. **自动去重**：重复运行不会重复添加
3. **分类自动推断**：系统会根据路径自动推断分类
4. **只需同步一次**：首次同步后，后续更新数据库不需要再次运行

---

**文档版本**: v1.0.0
**最后更新**: 2024-01-01
