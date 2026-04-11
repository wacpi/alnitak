import { useAuthStore } from '@/stores/auth-store';

export const requireLogin = async (actionName?: string): Promise<boolean> => {
  if (typeof window === 'undefined') return false;
  const auth = useAuthStore();
  if (auth.isLoggedIn) return true;
  auth.openLoginModal({ reason: actionName ? `登录后才能${actionName}` : '需要登录' });
  return false;
};

