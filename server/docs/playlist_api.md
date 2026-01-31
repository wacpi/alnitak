# 合集功能 API 文档

## 概述

合集（Playlist）是 UP主创建的公开视频合集，与"收藏夹"的区别：
- **收藏夹**：用户私人管理，收藏别人的视频
- **合集**：UP主公开管理，将自己的视频组织成系列

---

## 数据库表

### playlist 合集表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | 主键 |
| uid | uint | 创建者用户ID |
| title | varchar(100) | 合集标题 |
| cover | varchar(255) | 封面图URL |
| desc | varchar(500) | 简介 |
| is_open | bool | 是否公开（默认true） |
| video_count | int | 视频数量（冗余字段） |
| views | int64 | 浏览量 |
| favorites | int64 | 收藏数 |
| created_at | datetime | 创建时间 |
| updated_at | datetime | 更新时间 |
| deleted_at | datetime | 软删除时间 |

### playlist_video 合集视频关联表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | 主键 |
| playlist_id | uint | 合集ID |
| vid | uint | 视频ID |
| sort | int64 | 排序值（步长1000） |
| created_at | datetime | 创建时间 |
| deleted_at | datetime | 软删除时间 |

联合唯一索引：`(playlist_id, vid)`

---

## API 接口

### 一、合集管理（需登录）

#### 1. 创建合集
- **POST** `/api/v1/playlist/add`
- **请求体**：
```json
{
  "title": "合集标题",
  "cover": "封面图URL（可选）",
  "desc": "简介（可选）"
}
```
- **限制**：每用户最多50个合集，标题不超过100字符

#### 2. 编辑合集
- **PUT** `/api/v1/playlist/edit`
- **请求体**：
```json
{
  "id": 1,
  "title": "新标题",
  "cover": "新封面URL",
  "desc": "新简介",
  "isOpen": true
}
```
- **限制**：只有创建者可编辑

#### 3. 删除合集
- **DELETE** `/api/v1/playlist/del/:id`
- **限制**：只有创建者可删除，级联删除关联视频记录

#### 4. 获取自己的合集列表
- **GET** `/api/v1/playlist/myList`
- **响应**：
```json
{
  "code": 200,
  "data": {
    "total": 5,
    "playlists": [
      {
        "id": 1,
        "title": "合集标题",
        "cover": "封面URL",
        "desc": "简介",
        "isOpen": true,
        "videoCount": 10,
        "views": 1234,
        "favorites": 56,
        "createdAt": "2025-01-01T00:00:00Z"
      }
    ]
  }
}
```

---

### 二、合集视频管理（需登录）

#### 5. 添加视频到合集
- **POST** `/api/v1/playlist/video/add`
- **请求体**：
```json
{
  "playlistId": 1,
  "vids": [101, 102, 103]
}
```
- **限制**：
  - 只有合集创建者可操作
  - 只能添加自己的已审核通过的视频
  - 已存在的视频自动跳过
  - 合集最多200个视频

#### 6. 从合集移除视频
- **POST** `/api/v1/playlist/video/del`
- **请求体**：
```json
{
  "playlistId": 1,
  "vids": [101, 102]
}
```

#### 7. 调整合集视频排序
- **POST** `/api/v1/playlist/video/sort`
- **请求体**（传入完整的视频ID顺序数组）：
```json
{
  "playlistId": 1,
  "vids": [103, 101, 102]
}
```
- 按传入的数组顺序重新分配排序值

---

### 三、公开查询（无需登录）

#### 8. 获取合集详情
- **GET** `/api/v1/playlist/info?id=1`
- **响应**：
```json
{
  "code": 200,
  "data": {
    "playlist": {
      "id": 1,
      "uid": 100,
      "title": "合集标题",
      "cover": "封面URL",
      "desc": "简介",
      "isOpen": true,
      "videoCount": 10,
      "views": 1235,
      "favorites": 56,
      "createdAt": "2025-01-01T00:00:00Z",
      "updatedAt": "2025-01-15T00:00:00Z",
      "author": {
        "uid": 100,
        "name": "用户名",
        "avatar": "头像URL",
        ...
      }
    }
  }
}
```
- 每次访问自动增加浏览量
- 非公开合集只有创建者可见

#### 9. 获取合集视频列表
- **GET** `/api/v1/playlist/video/list?playlistId=1&page=1&pageSize=20`
- **响应**：
```json
{
  "code": 200,
  "data": {
    "total": 10,
    "videos": [
      {
        "vid": 101,
        "title": "视频标题",
        "cover": "封面URL",
        "duration": 360.5,
        "clicks": 1000,
        "desc": "视频简介",
        "createdAt": "2025-01-01T00:00:00Z"
      }
    ]
  }
}
```
- 按排序值升序返回

#### 10. 获取用户的公开合集列表
- **GET** `/api/v1/playlist/userList?uid=100&page=1&pageSize=20`
- **响应**：同"获取自己的合集列表"，但只返回公开合集

---

## 手动注册 API 权限

由于 API 和 Casbin 规则只在首次初始化时写入，已有数据的数据库需要手动插入。
在后台管理面板的"API管理"中添加以下记录，或直接执行下面的 SQL：

### 插入 API 记录

```sql
INSERT INTO api (method, path, category, `desc`, created_at, updated_at) VALUES
('POST',   '/api/v1/playlist/add',        '合集', '创建合集',           NOW(), NOW()),
('PUT',    '/api/v1/playlist/edit',       '合集', '编辑合集',           NOW(), NOW()),
('DELETE', '/api/v1/playlist/del/:id',    '合集', '删除合集',           NOW(), NOW()),
('GET',    '/api/v1/playlist/myList',     '合集', '获取自己的合集列表',  NOW(), NOW()),
('POST',   '/api/v1/playlist/video/add',  '合集', '添加视频到合集',     NOW(), NOW()),
('POST',   '/api/v1/playlist/video/del',  '合集', '从合集移除视频',     NOW(), NOW()),
('POST',   '/api/v1/playlist/video/sort', '合集', '调整合集视频排序',   NOW(), NOW());
```

### 插入 Casbin 规则（普通用户角色 001）

```sql
INSERT INTO casbin_rule (ptype, v0, v1, v2) VALUES
('p', '001', '/api/v1/playlist/add',        'POST'),
('p', '001', '/api/v1/playlist/edit',       'PUT'),
('p', '001', '/api/v1/playlist/del/:id',    'DELETE'),
('p', '001', '/api/v1/playlist/myList',     'GET'),
('p', '001', '/api/v1/playlist/video/add',  'POST'),
('p', '001', '/api/v1/playlist/video/del',  'POST'),
('p', '001', '/api/v1/playlist/video/sort', 'POST');
```

### 插入 Casbin 规则（管理员角色 002）

```sql
INSERT INTO casbin_rule (ptype, v0, v1, v2) VALUES
('p', '002', '/api/v1/playlist/add',        'POST'),
('p', '002', '/api/v1/playlist/edit',       'PUT'),
('p', '002', '/api/v1/playlist/del/:id',    'DELETE'),
('p', '002', '/api/v1/playlist/myList',     'GET'),
('p', '002', '/api/v1/playlist/video/add',  'POST'),
('p', '002', '/api/v1/playlist/video/del',  'POST'),
('p', '002', '/api/v1/playlist/video/sort', 'POST');
```

> **注意**：公开查询接口（`info`、`video/list`、`userList`）不需要登录，走的是无认证路由，不需要添加 API 和 Casbin 规则。
>
> 执行 SQL 后需要**重启服务端**让 Casbin 重新加载规则。
