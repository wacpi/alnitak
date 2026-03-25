import { defineStore } from 'pinia';
import Cookies from 'js-cookie';
import { getUserInfoAPI } from '@/api/user';
import { statusCode } from '@/utils/status-code';
import { storageData } from '@/utils/storage-data';
import { logoutAPI } from '@/api/auth';
import { authDebug } from '@/utils/auth-debug';

export type AuthStatus = 'unknown' | 'guest' | 'auth';

const getRedirectUrl = () => {
  if (typeof window === 'undefined') return '/';
  return `${window.location.pathname}${window.location.search}${window.location.hash}`;
};

export const useAuthStore = defineStore('auth', {
  state: () => ({
    // 阶段 A：默认以游客态渲染，确保 SSR 与首屏 hydration DOM 结构一致，避免 hydration mismatch。
    // 阶段 B：若 SSR 已严格校验登录态，将通过 initFromSSR() 覆盖该默认值。
    status: 'guest' as AuthStatus,
    user: null as UserInfoType | null,
    loginModalOpen: false,
    redirectAfterLogin: '' as string,
    lastAuthError: '' as string,
    _fetchingMe: false,
  }),
  getters: {
    isLoggedIn: (s) => s.status === 'auth',
  },
  actions: {
    initFromSSR(payload?: { status: AuthStatus; user?: UserInfoType | null }) {
      if (!payload) return;
      this.status = payload.status;
      this.user = payload.user ?? null;
    },

    async fetchMe() {
      if (this._fetchingMe) return;
      authDebug('[auth] fetchMe start', {
        prevStatus: this.status,
        hasToken: Boolean(storageData.get('token')),
        hasRefreshToken: Boolean(storageData.get('refreshToken')),
        hasUserIdCookie: Boolean(Cookies.get('user_id')),
      });

      // 无任何本地凭证时，直接判定为游客态，避免无意义的 /getUserInfo 探测。
      // 否则在“跨标签页退出登录”场景下，可能触发接口返回 LOGIN_AGAIN，进而误弹登录弹窗。
      if (!storageData.get('token') && !storageData.get('refreshToken') && !Cookies.get('user_id')) {
        this.user = null;
        this.status = 'guest';
        this.lastAuthError = '';
        authDebug('[auth] fetchMe skip (no credentials)', { status: this.status });
        return;
      }

      this._fetchingMe = true;
      this.lastAuthError = '';
      try {
        const res = await getUserInfoAPI();
        if (res.data.code === statusCode.OK) {
          this.user = res.data.data.userInfo;
          this.status = 'auth';
          authDebug('[auth] fetchMe ok', { status: this.status, uid: this.user?.uid });
          return;
        }
        this.user = null;
        this.status = 'guest';
        this.lastAuthError = res.data.msg || '获取用户信息失败';
        authDebug('[auth] fetchMe guest', { code: res.data.code, msg: res.data.msg });
      } catch (e: any) {
        this.user = null;
        this.status = 'guest';
        this.lastAuthError = e?.message || '网络错误';
        authDebug('[auth] fetchMe error', { message: this.lastAuthError });
      } finally {
        this._fetchingMe = false;
        authDebug('[auth] fetchMe end', { status: this.status });
      }
    },

    openLoginModal(opts?: { redirect?: string; reason?: string }) {
      const redirect = opts?.redirect || getRedirectUrl();
      this.redirectAfterLogin = redirect;
      this.loginModalOpen = true;
      if (opts?.reason) this.lastAuthError = opts.reason;
      authDebug('[auth] openLoginModal', { redirect, reason: opts?.reason });
    },

    closeLoginModal() {
      this.loginModalOpen = false;
    },

    markGuest() {
      this.status = 'guest';
      this.user = null;
    },

    async logout() {
      authDebug('[auth] logout start');
      try {
        const rt = storageData.get('refreshToken');
        if (rt) {
          await logoutAPI(rt);
        }
      } catch {
        // 退出登录失败不阻止本地清理
      }

      storageData.remove('token');
      storageData.remove('refreshToken');
      Cookies.remove('user_id');

      this.markGuest();
      authDebug('[auth] logout end', { status: this.status });
    },
  },
});

