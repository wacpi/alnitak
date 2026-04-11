## 全局登录态改造实施记录（web-client）

### 约定

- **目标**：最小侵入地收敛登录态与登录引导，保证既有功能不被破坏
- **阶段**：先做阶段 A（纯前端收敛），阶段 B（严格 SSR）依赖后端配合
- **术语**：
  - 登录态：`guest/auth/unknown`
  - 回跳：`redirectAfterLogin`
  - 弹窗：全局唯一登录弹窗宿主

---

### Step 1：补齐架构与实施文档

- 新增文档：`docs/auth/global-auth.md`
  - 说明现状、目标、架构图（mermaid）、分阶段策略、测试清单
- 新增实施日志：`docs/auth/implementation-log.md`

**为什么**：在落代码前明确边界与数据流，避免后续“组件各改各的”。

---

### Step 2：新增 AuthStore（Pinia）

计划新增（阶段 A）：

- `src/stores/auth-store.ts`（或 `src/composables/auth-store.ts`，视项目约定而定）
  - `status/user/loginModalOpen/redirectAfterLogin`
  - `fetchMe/openLoginModal/logout/initFromSSR`

**为什么**：让全站登录态有单一真相；组件不再自行请求 `getUserInfo()` 做判断。

---

### Step 3：根部挂载全局登录弹窗宿主

计划改造：

- 在 `src/app.vue`（或根布局）挂载 `LoginDialog`，由 `AuthStore.loginModalOpen` 控制。

**为什么**：任意页面/组件可打开登录弹窗，不需要到处 `v-if` 引入弹窗组件。

---

### Step 4：统一 requireLogin() 行为（从“跳转”到“弹窗+回跳”）

计划改造：

- `src/utils/require-login.ts`：\n
  - 不再直接 `navigateTo('/login')`\n
  - 统一 `authStore.openLoginModal({ redirect, reason })`

**为什么**：业务入口只关心“需要登录”，不关心登录方式与 UI 细节。

---

### Step 5：Header 统一接入 AuthStore

计划改造：

- `src/components/home-header/index.vue`\n
- `src/components/header-bar/index.vue`

目标：

- 登录按钮统一 `authStore.openLoginModal()`\n
- 头像/用户信息统一读取 `authStore.user`

---

### Step 6：请求层失效处理统一唤起登录弹窗

计划改造：

- `src/utils/request.ts`
  - 收到 `LOGIN_AGAIN`：清理 token/refreshToken/user_id，并唤起 `authStore.openLoginModal()`\n
  - 保留现有 refresh 队列逻辑（阶段 A）

---

### Step 7：路由守卫与 store 对齐

计划改造：

- `src/middleware/auth.ts`
  - CSR 侧：以 `AuthStore.status` 为准\n
  - SSR 侧（阶段 A）：保持现有逻辑，尽量减少闪烁\n
  - SSR 严格（阶段 B）：改为 SSR 初始化时校验 cookie 会话并注入 store

---

### Step 8：阶段 B（严格 SSR）后端配合清单

计划输出：

- Cookie 方案（HttpOnly、Secure、SameSite）\n
- `me/refresh/logout` 接口语义与错误码约定\n
- SSR 初始化的调用时机与缓存策略

