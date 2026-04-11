# PGC (专业生成内容) 系统

## 简介

PGC (Professional Generated Content) 系统是一个专业视频内容管理模块，类似于B站的番剧、纪录片、电影等内容管理功能。该系统提供了完整的内容创建、管理、审核、查询等功能，支持分集管理、用户权限控制等高级特性。

---

## 功能特性

### 核心功能
- ✅ **PGC内容管理**: 创建、更新、删除、查询PGC内容
- ✅ **剧集管理**: 添加、删除、查询PGC剧集
- ✅ **用户组权限**: 基于用户组的权限控制系统
- ✅ **内容搜索**: 支持关键词搜索和多种筛选条件
- ✅ **连载管理**: 支持连载状态管理
- ✅ **推荐系统**: 基于创建时间的推荐功能

### 高级特性
- ✅ **事务支持**: 所有数据库操作使用事务保证数据一致性
- ✅ **缓存机制**: 用户组信息5分钟缓存，提升性能
- ✅ **权限控制**: 细粒度的权限控制（PGC权限、管理员权限）
- ✅ **数据校验**: 完整的参数校验和错误处理
- ✅ **雪花ID**: 使用雪花算法生成唯一ID

---

## 技术架构

### 分层结构

```
┌─────────────────────────────────────┐
│         API Layer (HTTP)            │  internal/api/v1/
├─────────────────────────────────────┤
│      Service Layer (Business)       │  internal/service/
├─────────────────────────────────────┤
│       Domain Layer (Model/DTO)      │  internal/domain/
├─────────────────────────────────────┤
│      Database Layer (MySQL/GORM)    │  global.Mysql
└─────────────────────────────────────┘
```

### 技术栈
- **Web框架**: Gin
- **ORM**: GORM
- **数据库**: MySQL
- **缓存**: 内存缓存（用户组）
- **ID生成**: Snowflake算法
- **认证**: JWT + Casbin

---

## 数据库设计

### 表结构

#### 1. pgc_content (PGC内容表)
存储PGC内容的基本信息

**主要字段**:
- `pgc_id` - PGC内容ID (雪花ID，唯一)
- `pgc_type` - PGC类型 (国创/日创/纪录片/电影/电视剧)
- `title` - 标题
- `cover` - 封面URL
- `desc` - 简介
- `year` - 年份
- `area` - 地区
- `rating` - 评分 (0-10)
- `is_ongoing` - 是否连载中
- `total_episodes` - 总集数
- `current_episodes` - 已更新集数
- `status` - 审核状态
- `operator_id` - 运营人员ID

**索引**: pgc_id (唯一), type_status, operator_id, deleted_at

---

#### 2. pgc_episode (PGC剧集表)
存储PGC内容的剧集信息

**主要字段**:
- `pgc_id` - PGC内容ID
- `episode_number` - 剧集序号
- `title` - 剧集标题
- `vid` - 关联视频ID
- `duration` - 时长（秒）
- `status` - 状态 (正常/下架)
- `publish_time` - 发布时间

**索引**: pgc_id, vid, deleted_at

---

#### 3. user_group (用户组表)
存储用户组信息

**主要字段**:
- `uid` - 用户ID
#### PGC内容管理
| 接口 | 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|------|
| 创建PGC | POST | `/create` | 是 | 创建新的PGC内容 |
| 更新PGC | PUT | `/update` | 是 | 更新PGC内容 |
| 删除PGC | DELETE | `/:pgc_id` | 是 | 删除PGC内容 |
| 获取列表 | GET | `/list` | 否 | 分页获取PGC列表 |
| 获取详情 | GET | `/detail` | 否 | 获取PGC详情 |
| 搜索PGC | GET | `/search` | 否 | 搜索PGC内容 |
| 按类型获取 | GET | `/type/:type` | 否 | 按类型获取PGC |
| 获取连载 | GET | `/ongoing` | 否 | 获取连载中PGC |
| 获取推荐 | GET | `/recommended` | 否 | 获取推荐PGC |
| 获取详情+剧集 | GET | `/detail-with-episodes` | 否 | 获取PGC详情及剧集 |

#### 剧集管理
| 接口 | 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|------|
| 获取剧集列表 | GET | `/:pgc_id/episodes` | 否 | 获取PGC剧集列表 |
| 添加剧集 | POST | `/:pgc_id/episodes/add` | 是 | 添加新剧集 |
| 删除剧集 | DELETE | `/:pgc_id/episodes/:id` | 是 | 删除剧集 |

| 白名单用户 | 1 | 特殊权限用户 | 查看PGC内容 + 特殊功能 |
| 黑名单用户 | 2 | 受限用户 | 仅能查看公开内容 |
| PGC用户 | 3 | PGC创作者 | 创建、编辑、删除PGC |
| 高危用户 | 4 | 高风险用户 | 功能受限 |

### 权限说明

- **查看权限**: 所有用户都可以查看已审核通过的PGC内容
- **PGC权限**: 用户需要属于PGC用户组才能创建、编辑、删除PGC内容
- **管理员权限**: 用户需要具有管理员角色才能管理用户组

---

## 常量定义

### PGC类型 (PGCType)
```go
const (
    PGCTypeNone        = 0  // 非PGC
    PGCTypeCN          = 1  // 国创（番剧 CN）
    PGCTypeJP          = 2  // 日创（番剧 JP）
    PGCTypeDocumentary = 3 // 纪录片
    PGCTypeMovie       = 4 // 电影
    PGCTypeTVSeries    = 5 // 电视剧
)
```

### PGC审核状态 (PGCAuditStatus)
```go
const (
    PGCAuditDraft     = 0   // 草稿
    PGCAuditSubmitted = 100 // 已提交
    PGCAuditProcessing = 200 // 审核中
    PGCAuditApproved  = 300 // 审核通过
    PGCAuditRejected  = 400 // 审核驳回
)
```

### 剧集状态 (PGCEpisodeStatus)
```go
const (
    PGCEpisodeNormal  = 0  // 正常
    PGCEpisodeOffline = -1 // 下架
)
```

---

## 权限系统

PGC系统使用项目现有的 **Role + Casbin** 权限控制系统。

### Role系统

项目使用Role角色管理系统：
- **001**: 普通用户（默认角色）
- **002**: 管理员
- **003**: PGC创作者（需自行创建）

### 权限说明

需要PGC权限的接口：
- 创建PGC内容
- 更新PGC内容
- 删除PGC内容
- 添加剧集
- 删除剧集

无需权限的接口（所有用户可访问）：
- 获取PGC列表
- 获取PGC详情
- 获取剧集列表
- 搜索PGC
- 按类型获取PGC
- 获取连载中PGC
- 获取推荐PGC

### 权限配置

详细的权限配置说明请参考：[PGC权限配置指南](./PGC_AUTH.md)

**快速配置**：
```sql
-- 1. 创建PGC角色
INSERT INTO role (name, code, desc, home_page, created_at, updated_at)
VALUES ('PGC创作者', '003', 'PGC内容创作者角色', '/pgc', NOW(), NOW());

-- 2. 添加PGC权限规则
INSERT INTO casbin_rule (ptype, v0, v1, v2) VALUES
('p', '003', '/api/v1/pgc/create', 'POST'),
('p', '003', '/api/v1/pgc/update', 'PUT'),
('p', '003', '/api/v1/pgc/:pgc_id', 'DELETE'),
('p', '003', '/api/v1/pgc/:pgc_id/episodes/add', 'POST'),
('p', '003', '/api/v1/pgc/:pgc_id/episodes/:id', 'DELETE');

-- 3. 给用户分配PGC角色
UPDATE user SET role = '003' WHERE id = <user_id>;
```

---

## 使用示例

### 1. 创建PGC内容

```javascript
const createPGC = async () => {
  const response = await fetch('/api/v1/pgc/create', {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      pgc_type: 1,
      title: '进击的巨人',
      cover: 'https://example.com/cover.jpg',
      desc: '人类与巨人之间的战争',
      year: 2013,
      area: '日本',
      rating: 9.5,
      is_ongoing: true,
      episodes: [
        {
          episode_number: 1,
          title: '致两千年后的你',
          vid: 123456,
          duration: 1440.5,
          publish_time: '2024-01-01 10:00:00'
        }
      ]
    })
  });
  
  const result = await response.json();
  console.log(result);
};
```

### 2. 获取PGC列表

```javascript
const getPGCList = async (page = 1, pageSize = 20) => {
  const response = await fetch(
    `/api/v1/pgc/list?page=${page}&page_size=${pageSize}&pgc_type=1&is_ongoing=true`
  );
  
  const result = await response.json();
  console.log(result);
};
```

### 3. 添加剧集

```javascript
const addEpisode = async (pgcId) => {
  const response = await fetch(`/api/v1/pgc/${pgcId}/episodes/add`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      episode_number: 13,
      title: '新的开始',
      vid: 123458,
      duration: 1440.0,
      publish_time: '2024-01-15 10:00:00'
    })
  });
  
  const result = await response.json();
  console.log(result);
};
```

---

## 项目结构

```
internal/
├── api/v1/              # API层
│   ├── pgc.go          # PGC相关API
│   └── user_group.go   # 用户组相关API
├── service/            # 服务层
│   ├── pgc.go          # PGC业务逻辑
│   └── user_group.go   # 用户组业务逻辑
├── domain/             # 领域层
│   ├── model/          # 数据模型
│   │   ├── pgc_content.go
│   │   ├── pgc_episode.go
│   │   └── user_group.go
│   └── dto/            # 数据传输对象
│       ├── pgc.go
│       └── user_group.go
├── middleware/         # 中间件
│   └── pgc_auth.go     # PGC权限中间件
├── routes/             # 路由
│   └── pgc_router.go   # PGC路由
└── global/             # 全局变量
    └── constant.go     # 常量定义

docs/                   # 文档
├── PGC_API.md          # 完整API文档
├── PGC_API_QUICK_REF.md # API快速参考
└── PGC_README.md       # 本文档
```

---

## 部署说明

### 1. 表结构创建

项目使用GORM的AutoMigrate自动创建表结构，启动服务时会自动创建以下表：
- `user_group` - 用户组表
- `pgc_content` - PGC内容表
- `pgc_episode` - PGC剧集表

**注意**: 表结构在 `internal/initialize/tables.go` 中配置，服务启动时会自动创建。

### 2. 配置修改

无需额外配置，PGC系统使用项目现有的数据库和Redis配置。

### 3. 启动服务

```bash
# 开发环境
go run cmd/main.go -env=dev

# 生产环境
go run cmd/main.go -env=prod

# 编译后运行
go build -o cmd.exe cmd/main.go
./cmd.exe -env=prod
```

### 4. 验证服务

```bash
# 测试PGC列表接口
curl http://localhost:9000/api/v1/pgc/list?page=1&page_size=20

# 测试PGC详情接口
curl "http://localhost:9000/api/v1/pgc/detail?pgc_id=1234567890123456789"
```

---

## 注意事项

### 开发注意事项
1. **事务处理**: 所有涉及多表的操作必须使用事务
2. **错误处理**: 使用统一的错误处理机制
3. **日志记录**: 关键操作需要记录日志
4. **参数校验**: 所有接口参数必须进行校验

### 使用注意事项
1. **权限控制**: 确保用户具有相应的权限
2. **剧集序号**: 剧集序号不能重复，添加时请注意检查
3. **视频关联**: 添加剧集时，关联的视频ID必须真实存在
4. **物理删除**: 删除操作为物理删除，数据将无法恢复
5. **缓存机制**: 用户组信息有5分钟的缓存，权限变更可能需要等待

---

## 性能优化

### 已实现的优化
1. **用户组缓存**: 用户组信息缓存5分钟，减少数据库查询
2. **索引优化**: 为常用查询字段添加了索引
3. **分页查询**: 所有列表接口都支持分页，避免返回过多数据

### 建议的优化
1. **Redis缓存**: 可以将热门PGC内容缓存到Redis
2. **CDN加速**: 封面和视频资源建议使用CDN
3. **读写分离**: 高并发场景可以考虑数据库读写分离
4. **搜索引擎**: 复杂搜索场景可以考虑使用Elasticsearch

---

## 测试建议

### 功能测试
1. 测试PGC内容的创建、更新、删除
2. 测试剧集的添加、删除
3. 测试搜索和筛选功能
4. 测试权限控制

### 性能测试
1. 测试大列表查询性能
2. 测试并发创建PGC
3. 测试缓存效果

### 安全测试
1. 测试权限绕过
2. 测试SQL注入
3. 测试XSS攻击

---

## 更新日志

### v1.0.0 (2024-01-01)
- 初始版本发布
- 实现PGC内容管理
- 实现剧集管理
- 实现用户组管理
- 实现搜索和筛选功能
- 实现连载功能
- 完成API文档

---

## 常见问题 (FAQ)

### Q1: 如何给用户添加PGC权限？
A: 使用管理员账号调用 `/api/v1/pgc/admin/user-group/add` 接口，将用户添加到PGC用户组（group_type=3）。

### Q2: PGC内容创建后需要审核吗？
A: 目前PGC内容创建后自动设置为审核通过状态（status=300），无需人工审核。

### Q3: 剧集序号可以重复吗？
A: 不可以，同一PGC内容的剧集序号必须唯一。

### Q4: 删除PGC会删除关联的剧集吗？
A: 是的，删除PGC时会物理删除所有关联的剧集。

### Q5: 用户组权限变更多久生效？
A: 用户组信息有5分钟的缓存，权限变更后最多需要5分钟生效。

---

## 技术支持

如有问题或建议，请联系开发团队。

---

**文档版本**: v1.0.0  
**最后更新**: 2024-01-01
