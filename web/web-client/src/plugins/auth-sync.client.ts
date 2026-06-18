import { useAuthStore } from '@/stores/auth-store';
import { authDebug } from '@/utils/auth-debug';

export default defineNuxtPlugin(() => {
  if (process.server) return;

  const auth = useAuthStore();

  const nuxtApp = useNuxtApp();
  nuxtApp.hook('app:mounted', () => {
    // 页面加载时通过 HttpOnly Cookie 尝试续签登录态
    auth.fetchMe();
  });
});
