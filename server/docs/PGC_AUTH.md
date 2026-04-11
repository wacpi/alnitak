# PGC权限配置指南

## 概述

PGC系统使用项目现有的**Role + Casbin**权限控制系统。要使用PGC功能，需要创建并配置PGC角色。

---

## Role权限系统说明

项目使用Casbin进行细粒度的API权限控制，基于Role角色进行管理。

### 角色类型

系统默认有以下角色：

| Role Code | 角色名称 | 说明 |
|-----------|---------|------|
| 001 | 普通用户 | 默认角色，可以使用所有用户功能 |
| 002 | 管理员 | 系统管理员，拥有所有权限 |

---

## PGC角色配置步骤

### 1. 创建PGC角色

在数据库中插入PGC角色：

```sql
INSERT INTO role (name, code, desc, home_page, created_at, updated_at)
VALUES ('PGC创作者', '003', 'PGC内容创作者角色', '/pgc', NOW(), NOW());
```

### 2. 添加PGC权限规则

在Casbin规则表中添加PGC相关的API权限：

```sql
-- PGC内容管理
INSERT INTO casbin_rule (ptype, v0, v1, v2) VALUES ('p', '003', '/api/v1/pgc/create', 'POST');
INSERT INTO casbin_rule (ptype, v0, v1, v2) VALUES ('p', '003', '/api/v1/pgc/update', 'PUT');
INSERT INTO casbin_rule (ptype, v0, v1, v2) VALUES ('p', '003', '/api/v1/pgc/:pgc_id', 'DELETE');

-- PGC剧集管理
INSERT INTO casbin_rule (ptype, v0, v1, v2) VALUES ('p', '003', '/api/v1/pgc/:pgc_id/episodes/add', 'POST');
INSERT INTO casbin_rule (ptype, v0, v1, v2) VALUES ('p', '003', '/api/v1/pgc/:pgc_id/episodes/:id', 'DELETE');
```

### 3. 分配PGC角色给用户

将PGC角色分配给指定的用户：

```sql
UPDATE user SET role = '003' WHERE id = <user_id>;
```

或者在后台管理界面中修改用户的角色。

---

## 权限说明

### 需要PGC权限的接口

以下接口需要用户具有PGC角色（role_code = '003'）才能访问：

| 接口 | 方法 | 说明 |
|------|------|------|
| /api/v1/pgc/create | POST | 创建PGC内容 |
| /api/v1/pgc/update | PUT | 更新PGC内容 |
| /api/v1/pgc/:pgc_id | DELETE | 删除PGC内容 |
| /api/v1/pgc/:pgc_id/episodes/add | POST | 添加剧集 |
| /api/v1/pgc/:pgc_id/episodes/:id | DELETE | 删除剧集 |

### 无需权限的接口

以下接口所有登录用户都可以访问：

| 接口 | 方法 | 说明 |
|------|------|------|
| /api/v1/pgc/list | GET | 获取PGC列表 |
| /api/v1/pgc/detail | GET | 获取PGC详情 |
| /api/v1/pgc/:pgc_id/episodes | GET | 获取剧集列表 |
| /api/v1/pgc/search | GET | 搜索PGC |
| /api/v1/pgc/type/:type | GET | 按类型获取PGC |
| /api/v1/pgc/ongoing | GET | 获取连载中PGC |
| /api/v1/pgc/recommended | GET | 获取推荐PGC |
| /api/v1/pgc/detail-with-episodes | GET | 获取PGC详情及剧集 |

---

## 中间件说明

PGC系统使用以下中间件进行权限控制：

### RequirePGC()
检查用户是否具有PGC权限（角色代码为'001'、'002'或'003'）。

```go
// 使用示例
pgcAuth := pgcGroup.Group("")
pgcAuth.Use(middleware.Auth(), middleware.RequirePGC())
{
    pgcAuth.POST("create", api.CreatePGC)
    pgcAuth.PUT("update", api.UpdatePGC)
    // ...
}
```

### RequireAdmin()
检查用户是否为管理员（角色代码为'001'或'002'）。

---

## 常见问题

### Q1: 如何给用户添加PGC权限？
A: 两种方式：
1. 直接修改数据库：`UPDATE user SET role = '003' WHERE id = <user_id>;`
2. 通过后台管理界面修改用户角色（如果有相关功能）

### Q2: PGC权限和管理员权限有什么区别？
A:
- **管理员权限**（role_code = '001'或'002'）：拥有所有权限，包括系统管理
- **PGC权限**（role_code = '003'）：只能管理PGC内容和剧集，不能进行系统管理

### Q3: 用户可以同时有多个角色吗？
A: 不可以，每个用户只能有一个角色。User表的role字段存储的是角色的code。

### Q4: 如何创建新的PGC角色？
A:
1. 在role表中插入新角色
2. 在casbin_rule表中添加该角色的权限规则
3. 在RequirePGC()中间件中添加该角色的code判断

### Q5: 权限变更多久生效？
A: 权限变更后，用户需要重新登录才能生效，因为role信息存储在JWT token中。

---

## 数据库表说明

### role表
```sql
CREATE TABLE `role` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `name` varchar(20) NOT NULL COMMENT '角色名',
  `code` varchar(20) NOT NULL COMMENT '角色代码',
  `desc` varchar(100) DEFAULT NULL COMMENT '介绍',
  `home_page` varchar(20) DEFAULT NULL COMMENT '角色首页',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uix_role_code` (`code`),
  KEY `idx_role_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### casbin_rule表
```sql
CREATE TABLE `casbin_rule` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `ptype` varchar(100) DEFAULT NULL,
  `v0` varchar(100) DEFAULT NULL,
  `v1` varchar(100) DEFAULT NULL,
  `v2` varchar(100) DEFAULT NULL,
  `v3` varchar(100) DEFAULT NULL,
  `v4` varchar(100) DEFAULT NULL,
  `v5` varchar(100) DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### user表
```sql
CREATE TABLE `user` (
  ...
  `role` varchar(20) DEFAULT '001' COMMENT '角色ID',
  ...
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

---

## 完整配置示例

### 方式1：使用SQL脚本

创建一个完整的PGC角色配置脚本：

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

### 方式2：通过API（如果有）

通过系统的角色管理API创建角色和分配权限（参考 `internal/api/v1/role.go`）。

---

**文档版本**: v2.0.0  
**最后更新**: 2024-01-01
