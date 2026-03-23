# B 帧=3 时期重新转码资源列表说明

## 1. 「B 帧改为 3」的时间

在 **当前仓库的 git 历史**中，**从未出现** `-bf 3` 的提交；所有涉及 `-bf` 的提交均为 **`-bf 0`**（例如 8253d60、31a98fb、2b09534 等）。

因此：
- 若你曾**本地**把 `-bf` 改为 `3` 且未提交，无法从 git 得到「改为 3」的精确时间。
- 若「改为 3」发生在其他分支或已丢失的提交，需要你在该分支/历史里查对应提交时间。

**可用的替代时间点（用于“从某时刻之后执行过转码”的筛选）：**
- **2026-02-28 12:09:08**：提交 8253d60（修复转码参数 bug，当时为 -bf 0）
- **2026-02-07 22:06:37**：提交 842b06d（添加重新转码与 DASH 协议）

若你确认某段时间曾用 `-bf 3` 部署，请以**该段时间的起始时间**作为下面查询的 `since` 时间。

---

## 2. 列出「某时间之后有过转码/更新的资源」（稿件 ID + 标题）

转码完成后会更新 `video_index_file`（或新建记录），故用 **`video_index_file.updated_at`** 作为「该资源最后一次转码/更新」的时间。  
关联 `resource` 和 `video` 即可得到**稿件 ID（vid）**和**标题（video.title）**。

### 2.1 直接执行 SQL（在 MySQL 客户端或管理工具中）

将 `'2026-02-28 12:09:00'` 换成你认定的「B 帧=3 开始」或「从该时刻之后执行过转码」的起始时间：

```sql
-- 某时间之后 video_index_file 有更新/创建的资源，列出稿件 ID 与标题（去重，以稿件维度）
SELECT DISTINCT
  v.id AS vid,
  v.title AS title
FROM video v
INNER JOIN resource r ON r.vid = v.id
INNER JOIN video_index_file vif ON vif.resource_id = r.id
WHERE vif.updated_at >= '2026-02-28 12:09:00'
  AND vif.deleted_at IS NULL
  AND r.deleted_at IS NULL
  AND v.deleted_at IS NULL
ORDER BY v.id;
```

若需要「该资源（分 P）最后转码时间」和「资源 ID」：

```sql
SELECT
  v.id AS vid,
  v.title AS title,
  r.id AS resource_id,
  vif.dir_name,
  vif.updated_at AS last_transcode_at
FROM video v
INNER JOIN resource r ON r.vid = v.id
INNER JOIN video_index_file vif ON vif.resource_id = r.id
WHERE vif.updated_at >= '2026-02-28 12:09:00'
  AND vif.deleted_at IS NULL
  AND r.deleted_at IS NULL
  AND v.deleted_at IS NULL
ORDER BY vif.updated_at DESC;
```

### 2.2 仅「重新转码」的界定

当前表结构没有单独的「是否重新转码」标记。  
- **重新转码**会更新或重建该 resource 的 `video_index_file`，从而更新 `video_index_file.updated_at`。  
- **首次转码**也会插入 `video_index_file`，所以 `updated_at` 在某个时间之后，既包含首次转码也包含重新转码。

若要**只**列重新转码，需要其一：
- 在业务上为「重新转码」打标记并落库，再在 SQL 中加条件；或  
- 用日志（例如带「重新转码」的日志）在对应时间范围内反查 resource_id/video_id，再与上面 SQL 结果做交集。

上面 SQL 先给出「自某时间以来有过转码记录（含首次与重新）的稿件 ID 与标题」；若你提供「重新转码」的判定方式，可以再收窄条件。

---

## 3. 小结

| 项目 | 说明 |
|------|------|
| B 帧=3 的时间 | Git 中无 `-bf 3` 提交，需用本地/其他分支记忆或替代时间（如 2026-02-28 12:09） |
| 列出稿件 ID + 标题 | 用 `video_index_file.updated_at >= since` + 与 `video`/`resource` 关联，见上面 SQL |
| 仅重新转码 | 需业务标记或日志反查，当前 SQL 为「某时间之后有转码记录」的列表 |
