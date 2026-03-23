# 播放页路由从 `/video/:id` 迁移到 `/watch` 方案说明

本文档面向所有前后端开发，说明：

- 当前播放页路由的设计（旧方案）；
- 新的统一 `/watch` 路由方案（方案 1）；
- 两者的优缺点对比；
- 为什么要做这次改造，以及后续开发应该遵守的约定。

---

## 1. 旧方案回顾：按类型分入口 + 路径参数

### 1.1 当前主要形态（示意）

- **视频详情**：
  - PC Web：`/video/:id`（例如 `/video/123`）
  - 移动 Web / Flutter：内部通常只关心 `vid`，各种页面/消息里也直接拼 `/video/${vid}`。
- **文章 / 专栏**：
  - 类似 `/article/:id`（若有）。
- **历史 / 消息 / 稿件管理等**：
  - 指向视频的链接也都是 `/video/${item.vid}`，附带 `?p=分P` 等查询参数。

特点：**路径本身表达了内容类型**（video、article），ID 一般是自增数字 `id`（后续逐步引入 shortId）。

### 1.2 旧方案的优点

- **直观**：
  - 开发者一眼看到 `/video/123` 就知道是「视频详情页」，`123` 是视频 ID。
- **符合传统 REST 直觉**：
  - `/video/:id` 看起来像资源详情，`/article/:id` 像文章详情。
- **短期上手成本低**：
  - 对小型项目和早期开发来说，很快就能跑起来。

### 1.3 旧方案的问题 / 隐性成本

随着功能增多，旧方案会暴露出一些问题（这次重构的主要原因）：

- **入口分散**：
  - 视频详情、文章详情、PGC 详情、合集详情等，都可能各自有一套 path（`/video`、`/article`、`/pgc`、`/playlist`…）。
  - 任何关于「观看行为」的统计 / A/B 实验 / 埋点，都需要在多个入口分别维护。
- **播放器壳子难以统一**：
  - 如果玩家（播放器 UI + 布局）需要对所有内容类型共享（视频 + PGC + 直播等），在多入口方案下代码很难完全复用。
- **扩展新类型时 path 不断膨胀**：
  - 后续引入更多内容形态（PGC、合集、直播间、短视频等），就要不断增加新的详情 path。
  - URL 空间逐渐碎片化，不利于长期演进。
- **与短 ID / 对外 ID 的演进脱节**：
  - 我们已经在引入 shortId（短 ID）方案，对外不再暴露简单数字 ID。
  - 继续大量使用 `/video/:id` 的写法，在心智上仍然默认「id 是一个简单数字」，不利于未来只依赖短 ID 的世界。

从「要做到接近 YouTube 规模」的目标看，**统一入口、统一播放器壳子、统一埋点**，会更利于长期演进。

---

## 2. 新方案：统一 `/watch` 入口 + 类型字母

### 2.1 目标与形态

新方案借鉴了 YouTube 的做法，但做了更通用的抽象：

- 所有「播放/详情」行为统一入口：`/watch`
- 通过 **Query 参数的字母** 表示内容类型和 ID：

| 内容类型         | Query 键 | 说明                 | 示例                         |
|------------------|----------|----------------------|------------------------------|
| 视频             | `v`      | video                | `/watch?v=AbC123xYz-_`       |
| 文章 / 专栏      | `a`      | article              | `/watch?a=DeF456pQk-_`       |
| PGC / 剧集类内容 | `s`      | series（season/番剧）| `/watch?s=SeR789LmN-_`       |
| 合集 / 播放列表  | `l`      | list / playlist      | `/watch?l=PlYabc123-_`       |
| 直播间（预留）   | `r`      | room / liveRoom      | `/watch?r=LiVexyz789-_`      |

> 约定：同一时刻只允许出现一个主类型键（v/a/s/l/r），否则视为非法或按优先级处理。

ID 值本身推荐使用 **短 ID（shortId）**，也兼容过渡期的数字 ID：

- 短 ID：`EncodeUint64ToShortID` 生成的 11 位字符串（`A-Z a-z 0-9 - _`）。
- 兼容期：如果收到的参数看起来是纯数字，则按旧的自增 ID 或 Snowflake 数字处理。

### 2.2 新方案的优点

1. **唯一的播放入口，利于长期演进**
   - 所有内容类型的「观看行为」都集中在 `/watch` 下。
   - 前端只需要一个顶层播放器壳子，根据 query 判断类型再装不同的子组件。
   - 后端、中台统计 / A/B 实验 / 埋点都可以对 `/watch` 做统一处理。

2. **类型扩展简单清晰**
   - 新增内容形态（例如 story、live、space 剧、交互内容等）时，只需增加一个字母键即可。
   - 不再需要为每个类型新增一条 path，避免 URL 空间膨胀。

3. **与短 ID 方案天然契合**
   - `?v=shortId` / `?a=shortId` / `?s=shortId`，天然就是「公开 ID」，不会泄露内部数值 ID。
   - 便于在前端和分享链接中统一使用短 ID 作为唯一标识。

4. **前后端职责清晰**
   - 前端：只管「拼对了 `/watch` + 正确的类型字母 + ID」，并在详情页解析 query 决定类型调用哪组 API。
   - 后端：仍按 `vid` / `aid` / `pgc_id` 等参数名收参，在入口做一次短 ID → 内部 ID 的解析。

5. **用户心智与主流产品一致**
   - `/watch` 本身已经被 YouTube 和其他视频平台教育为「播放页」。
   - 对用户来说，即使看到 `?v=xxx`、`?a=xxx`，也不会觉得奇怪。

### 2.3 新方案的缺点 / 需要适应的地方

- **路径不再直接体现类型**：
  - 看到 `/watch` 时，需要再看 query 才知道是视频还是文章。
  - 对开发者调试时，略微比 `/video/:id` 少一点直观，但通过浏览器地址栏仍然很容易判断（v/a/s/l/r）。
- **初次迁移的成本**：
  - 所有前端跳转、后端路由和文档都需要一次性对齐；
  - 老的 `/video/:id` 链接需要保留一段兼容期（可 301 或内部跳转到 `/watch`）。

---

## 3. 旧路由 vs 新路由：优劣对照

| 维度               | 旧方案：`/video/:id` 等              | 新方案：`/watch?x=shortId`                         |
|--------------------|--------------------------------------|---------------------------------------------------|
| 播放入口           | 多入口：`/video` `/article` `/pgc`… | 单入口：始终 `/watch`                             |
| URL 语义           | path 直观表达类型                    | path 表达「播放页」，类型在 query 中              |
| 扩展新类型         | 新增 path（/live、/story …）         | 新增一个字母（例如 r, t …）即可                   |
| 统计与埋点         | 多入口分别埋点                       | 对 `/watch` 统一埋点，根据 query 再细分           |
| 播放器复用         | 不同 path 可能引入多个壳子           | 单壳多模式：一个 `/watch` 内按类型切布局与组件    |
| 与短 ID 策略契合度 | 一般（仍偏向数字 ID 心智）           | 高度契合：`?x=<shortId>` 即公开 ID，自然可逆映射 |
| 长期维护成本       | 类型多时 path 越来越碎               | URL 模式稳定，可长期演进                          |

综合来看，新方案更适合「后期规模接近 YouTube」的目标。

---

## 4. 对开发者的具体约定（务必遵守）

### 4.1 所有新播放/详情页，统一使用 `/watch`

- **禁止** 再新增新的「详情」级 path（如 `/video/:id`、`/article/:id` 等用于播放）。
- 视频/文章/PGC 的列表页、管理页、频道页等仍可使用：
  - `/video` `/article` `/pgc` `/space` `/playlist` 等 path，用于导航和筛选；
  - 但点击进入播放时，**统一跳到 `/watch?...`**。

### 4.2 前端路由跳转规范

- 视频：

```ts
const idForRoute = video.shortId || String(video.vid);
router.push(`/watch?v=${idForRoute}`);
```

- 文章：

```ts
const idForRoute = article.shortId || String(article.aid);
router.push(`/watch?a=${idForRoute}`);
```

- PGC / 剧集（后续）：

```ts
const idForRoute = pgc.shortId || String(pgc.pgcId);
router.push(`/watch?s=${idForRoute}`);
```

### 4.3 前端详情页解析规范

在 `/watch` 页（Nuxt/Vue/Flutter 等）统一收口：

```ts
const route = useRoute();
const v = route.query.v as string | undefined;
const a = route.query.a as string | undefined;
const s = route.query.s as string | undefined;

if (v) {
  // 视频详情逻辑：调用 getVideoById(vid = v)
} else if (a) {
  // 文章详情逻辑：调用 getArticleById(aid = a)
} else if (s) {
  // PGC/剧集详情逻辑
} else {
  // 参数非法 → 跳转 404 或首页
}
```

Flutter/移动端同理，从 URL 或路由参数中读取 `v/a/s` 再决定类型。

### 4.4 后端参数解析规范（兼容短 ID 和数字 ID）

- 视频相关接口继续使用 `vid` 参数：
  - `/api/v1/video/getVideoById?vid=xxx`
  - `/api/v1/video/getVideoStatus?vid=xxx`
- 在入口统一做一次解析：

```go
// 伪代码：放在公共 util / service 包中
func ParseVideoID(raw string) (uint, error) {
    raw = strings.TrimSpace(raw)
    if raw == "" {
        return 0, errors.New("empty vid")
    }

    // 1. 短 ID 解析：11 位且字符集合法，按 short_id 查一次 Video.ID
    if len(raw) == 11 && isValidShortID(raw) {
        var video model.Video
        if err := global.Mysql.Where("short_id = ?", raw).First(&video).Error; err == nil {
            return video.ID, nil
        }
        // 找不到再回退到数字解析
    }

    // 2. 回退到数字 ID
    return utils.StringToUint(raw), nil
}
```

所有以 `vid` 收参的 API 应逐步改用 `ParseVideoID`，文章/PGC 同理（`ParseArticleID`、`ParsePGCID`）。

---

## 5. 迁移与兼容建议

- **阶段 1**：后端 VO 里补全 `shortId` 字段（已进行），前端开始读取但不立即切换所有链接。
- **阶段 2**：前端将所有新功能 & 新入口优先改用 `/watch` 方案，旧的 `/video/:id` 仍可访问。
- **阶段 3**：通过 301 或内部重定向，将 `/video/:id` 自动跳转到 `/watch?v=...`，保留兼容期。
- **阶段 4**：新版本客户端和页面完全不再生成 `/video/:id` 链接，只使用 `/watch`，旧链接保留长期或逐步废弃。

在迁移期内，后端务必保证：

- 短 ID 与内部 ID 的映射是稳定且可逆的；
- 所有用到稿件 ID 的接口都兼容「短 ID 或数字 ID」两种。

---

## 6. 总结

- 旧路由（`/video/:id` 等）在项目早期简单直观，但在多类型内容、统一播放器壳子、统计与实验、短 ID 方案等需求下，会逐渐暴露扩展性问题。
- 新路由（统一 `/watch` + 类型字母 `v/a/s/l/r`）是面向长期的结构化设计，更适合未来扩展到 PGC、合集、直播甚至更多形态，并与短 ID 方案天然对齐。
- 所有新开发应按本文档的路由规范执行，旧路由仅作为过渡兼容存在，逐步迁移到新方案。

