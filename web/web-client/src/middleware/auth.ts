import { storageData } from "@/utils/storage-data";
import { useAuthStore } from "@/stores/auth-store";

export default defineNuxtRouteMiddleware(async (to) => {
  if (process.server) {
    // 阶段 A：SSR 侧无法可靠读取 localStorage 凭证，只能用 cookie 做粗判。
    // 阶段 B：改为通过 HttpOnly Cookie + /auth/me 严格校验，并注入 AuthStore。
    const userId = useCookie('user_id');
    if (!userId.value) {
      return navigateTo(`/login?redirect=${to.path}`);
    }
    return;
  }

  const auth = useAuthStore();
  const nuxtApp = useNuxtApp();

  // 没有任何凭证线索时，直接当游客处理，避免无意义的接口探测。
  if (!storageData.get('refreshToken') && auth.status === 'unknown') {
    auth.markGuest();
  }

  if (auth.status === 'unknown') {
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

