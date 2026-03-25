import { useAuthStore } from '@/stores/auth-store';
import { authDebug } from '@/utils/auth-debug';

export default defineNuxtPlugin(() => {
  if (process.server) return;

  const auth = useAuthStore();

  // 关键：延后到 app:mounted 之后再同步登录态，避免 hydration mismatch。
  // SSR 渲染期 store.status 为 unknown；如果在 hydration 前改成 guest/auth，会导致 SSR/CSR DOM 不一致。
  const nuxtApp = useNuxtApp();
  nuxtApp.hook('app:mounted', () => {
    // 阶段 A：客户端启动后拉取一次用户信息，作为全局登录态基线。
    // 阶段 B：SSR 严格校验后，将由 initFromSSR() 注入，客户端只需兜底刷新。
    auth.fetchMe();
  });

  const onStorage = (e: StorageEvent) => {
    if (e.storageArea !== localStorage) return;
    if (e.key !== 'token' && e.key !== 'refreshToken') return;

    authDebug('[auth] storage event', {
      key: e.key,
      oldValue: Boolean(e.oldValue),
      newValue: Boolean(e.newValue),
    });

    // 退出登录（newValue 为空）时，其它标签页只需要切到游客态，不应主动弹出登录提示。
    if (!e.newValue) {
      auth.closeLoginModal();
      auth.markGuest();
      return;
    }

    // 登录/刷新凭证变化时再同步拉取用户信息
    auth.fetchMe();
  };

  nuxtApp.hook('app:mounted', () => {
    window.addEventListener('storage', onStorage);
  });
});

