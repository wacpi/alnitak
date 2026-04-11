import { onMounted, ref, type Ref } from 'vue';

/**
 * 客户端挂载后再视为「已水合」，用于避免 SSR 游客态与 CSR 登录态首帧 DOM 不一致导致的 hydration mismatch。
 * 与 Pinia `isLoggedIn` 组合为 `computed(() => hydrated.value && auth.isLoggedIn)` 即可。
 */
export function useClientHydrated(): { hydrated: Ref<boolean> } {
  const hydrated = ref(false);
  onMounted(() => {
    hydrated.value = true;
  });
  return { hydrated };
}
