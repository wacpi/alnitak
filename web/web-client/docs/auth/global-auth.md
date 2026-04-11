## 全局登录状态管理（PC 端 web-client）

### 背景与目标

当前 `web/web-client` 登录态判断分散在多个组件与工具函数中（`useCookie('user_id')`、`storageData`、各自调用 `getUserInfo()`），导致：

- **一致性差**：同一页面不同模块对“是否登录”的判断可能不一致
- **体验不稳定**：首屏/切页可能出现“已登录 UI 闪烁”
- **扩展困难**：后续加权限、强制下线、风控、跨端会话时改动面过大

目标是建立行业级的全局登录态架构：

- **单一真相（SSOT）**：全站只从 `AuthStore` 获取登录态
- **统一登录引导**：未登录的交互动作统一“弹全局登录弹窗 + 回跳 redirect”
- **严格 SSR 判定（阶段 B）**：服务端渲染阶段也能严格验证会话有效（依赖后端 HttpOnly Cookie）

---

### 现状基线（关键文件）

- **路由守卫**：`src/middleware/auth.ts`
- **请求封装/刷新队列**：`src/utils/request.ts`
- **登录弹窗组件**：`src/components/login-dialog/index.vue`
- **交互拦截工具**：`src/utils/require-login.ts`（会在阶段 A 改为对接 `AuthStore`）

---

### 目标架构总览

核心组件：

- **AuthStore（Pinia）**：全局登录态唯一来源
  - `status`: `'unknown' | 'guest' | 'auth'`
  - `user`: `UserInfoType | null`
  - `loginModalOpen`: `boolean`
  - `redirectAfterLogin`: `string`
  - actions：`initFromSSR()` / `fetchMe()` / `openLoginModal()` / `logout()` 等
- **GlobalLoginModalHost**：根部只渲染 1 次的登录弹窗宿主（复用现有 `LoginDialog`）
- **requireLogin()**：统一的登录拦截入口（阶段 A：打开弹窗；阶段 B：SSR 严格后仍保持一致）

```mermaid
flowchart TD
BrowserReq[BrowserRequest] --> NuxtSSR[NuxtSSR]
NuxtSSR --> ReadCookie[ReadHttpOnlyCookies]
ReadCookie --> RefreshOrMe[CallRefreshOrMe]
RefreshOrMe -->|valid| SSRAuth[InitAuthStore(auth,user)]
RefreshOrMe -->|invalid| SSRGuest[InitAuthStore(guest,null)]
SSRAuth --> Render[RenderHTMLWithState]
SSRGuest --> Render

Render --> Hydrate[ClientHydration]
Hydrate --> ClientFetchMe[ClientFetchMe(guard)]
ClientFetchMe -->|ok| AuthStateAuth[AuthStore=auth]
ClientFetchMe -->|fail| AuthStateGuest[AuthStore=guest]

UserAction[LikeCollectCommentDanmaku] --> RequireLogin[requireLogin()]
RequireLogin -->|guest| OpenModal[AuthStore.openLoginModal(redirect)]
OpenModal --> LoginModal[GlobalLoginModal]
LoginModal -->|success| FetchMe[AuthStore.fetchMe()]
FetchMe --> NavigateBack[NavigateTo(redirect)]
```

---

### 阶段 A（纯前端收敛，立即可落地）

重点：在不依赖后端改造的前提下，先实现“全局单一真相 + 弹窗引导 + redirect”。

- 新增 `AuthStore`
- 根部挂载全局登录弹窗宿主（受 `AuthStore.loginModalOpen` 控制）
- `requireLogin()` 改为调用 `AuthStore.openLoginModal({ redirect, reason })`
- Header/关键业务组件从 store 读取登录态与用户信息
- 请求层保持现有 refresh 队列，但登录失效不强制跳转页面，改为打开弹窗

**注意**：阶段 A 仍无法做到“严格 SSR 判定”，因为当前凭证主要在 `localStorage`，SSR 读取不到。

---

### 阶段 B（严格 SSR 判定，需要后端配合）

要做到 SSR 严格判定，必须让 SSR 能读取到可验证的凭证。行业标准做法：

- 登录成功由后端 `Set-Cookie` 下发 **HttpOnly Cookie**（refresh/session）
- SSR 阶段调用 `GET /auth/me` 或 `POST /auth/refresh` 验证会话并返回用户信息
- 前端 SSR 初始化时把结果注入 `AuthStore.initFromSSR()`

#### 后端配合清单（建议实现）

- **Cookie 设计**
  - **refresh_token**：HttpOnly；建议 `SameSite=Lax`；生产必须 `Secure`
  - **access_token（可选）**：HttpOnly 短有效期；也可完全不下发，由 SSR/CSR 通过 refresh 获取
  - Path 建议 `/`；Domain 视部署情况决定（同域优先）
- **接口语义**
  - `POST /api/v1/auth/login`：成功后 `Set-Cookie(refresh_token=...)`，响应体返回 `user`（可选）
  - `POST /api/v1/auth/updateToken`：从 cookie 读取 refresh，刷新会话/令牌并返回必要信息；可同时滚动更新 refresh cookie
  - `GET /api/v1/user/getUserInfo`（或 `GET /api/v1/auth/me`）：基于 cookie 鉴权返回当前用户信息；未登录返回统一错误码（如 `LOGIN_AGAIN`）
  - `POST /api/v1/auth/logout`：清理服务端会话并 `Set-Cookie(refresh_token=; Max-Age=0)` 使 cookie 失效
- **错误码约定**
  - 未登录/登录失效：统一返回 `LOGIN_AGAIN`（前端将清理状态并唤起登录弹窗）
  - refresh 失效：同样返回 `LOGIN_AGAIN`，避免“无限刷新”
- **安全建议**
  - refresh cookie 建议绑定 UA/设备指纹（可选）并支持服务端吊销
  - 所有鉴权接口建议加 CSRF 防护（同站点可用 SameSite+自定义 header；跨站需 token）

安全与工程收益：

- 避免把敏感 token 放在 `localStorage`（降低 XSS 风险）
- SSR/CSR 登录态一致，避免首屏闪烁与误判

**Flutter 客户端（如独立工程 `alnitak_flutter`）**：不参与浏览器 Cookie/SSR 流程，继续使用 **JSON 中的 `token` / `refreshToken` + `Authorization`**，与后端双栈兼容；`updateToken` 响应若含轮换后的 `refreshToken` 须在客户端持久化，`logout` 可仅靠 body 中的 refresh。详细说明见该工程根目录下的 `AUTH_IMPLEMENTATION.md`。

---

### 测试清单（建议）

- 未登录：点赞/收藏/评论/弹幕 → 统一弹窗 → 登录成功后回到原页面继续操作
- 登录失效：接口返回“需要重新登录” → 清理状态 → 弹窗出现
- 切换账号：退出登录 → UI 全局刷新为游客态
- SSR 首屏：刷新页面不出现“先显示已登录再变游客/反之”的闪烁

