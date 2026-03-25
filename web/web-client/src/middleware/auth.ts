import { useAuthStore } from "@/stores/auth-store";

export default defineNuxtRouteMiddleware(async (to) => {
  const auth = useAuthStore();

  if (process.server) {
    // 阶段 B：严格依赖服务端插件 auth-init.server 中 HttpOnly Cookie + /auth/me 的校验结果
    if (auth.status !== "auth") {
      return navigateTo(`/login?redirect=${encodeURIComponent(to.fullPath)}`);
    }
    return;
  }

  const nuxtApp = useNuxtApp();

  // 客户端进入受保护页：先拉齐会话（含 HttpOnly Cookie），避免仅凭默认 guest 误判
  if (auth.status !== "auth") {
    await auth.fetchMe();
  }

  if (auth.status !== 'auth') {
    // 首屏 hydration 期间如果直接 abortNavigation，可能导致页面一直处于“加载中”状态。
    // 因此：首屏阶段使用登录页重定向兜底；后续导航阶段再使用弹窗拦截。
    if (nuxtApp.isHydrating) {
      return navigateTo(`/login?redirect=${to.fullPath}`);
    }
    auth.openLoginModal({ redirect: to.fullPath, reason: '该页面需要登录' });
    return abortNavigation();
  }
});

