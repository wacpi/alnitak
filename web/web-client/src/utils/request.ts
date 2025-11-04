import axios from "axios";
import Cookies from "js-cookie";
import type { AxiosInstance, InternalAxiosRequestConfig } from "axios";
import { updateTokenAPI } from "@/api/auth";
import { statusCode } from "./status-code";
import { globalConfig as config, } from "./global-config";
import { storageData as storage } from "./storage-data";

// Token 刷新队列,使用类型定义
type TokenCallback = (token: string) => void;
let requests: TokenCallback[] = [];
let isRefreshing = false;
let refreshPromise: Promise<string> | null = null;
// 开发环境仅在“浏览器端”通过前端代理，SSR/生产环境直连后端
const isBrowser = typeof window !== 'undefined';
export const baseURL = (process.dev && isBrowser)
  ? ''
  : (config.domain ? `http${config.https ? 's' : ''}://${config.domain}` : '');

const service: AxiosInstance = axios.create({
  baseURL: `${baseURL}/api/`,
  withCredentials: true, // 跨域请求时发送Cookie
  timeout: 5000,
  headers: {},
});

// 刷新 token 的统一函数
const refreshToken = async (): Promise<string> => {
  if (refreshPromise) {
    return refreshPromise;
  }

  refreshPromise = (async () => {
    try {
      const localRefreshToken = storage.get('refreshToken');
      if (!localRefreshToken) {
        throw new Error('No refresh token available');
      }

      const tokenRes = await updateTokenAPI(localRefreshToken);
      if (tokenRes.data.code === statusCode.OK) {
        const token = tokenRes.data.data.token;
        const refreshToken = tokenRes.data.data.refreshToken;

        storage.set("token", token, 60);
        Cookies.set('user_id', tokenRes.data.data.userId);
        if (refreshToken && refreshToken !== localRefreshToken) {
          storage.set("refreshToken", refreshToken, 7 * 24 * 60);
        }

        // 执行队列中的所有回调
        requests.forEach(cb => cb(token));
        requests = [];

        return token;
      }
      throw new Error('Token refresh failed');
    } finally {
      isRefreshing = false;
      refreshPromise = null;
    }
  })();

  return refreshPromise;
};

//请求拦截器
service.interceptors.request.use(async (config) => {
  //如果为刷新token的请求则不拦截
  if (config.url === "v1/auth/updateToken") return config;
  if (localStorage) {
    if (storage.get('token')) {
      config.headers.Authorization = storage.get('token');
    } else {
      //如果没有accessToken且有refreshToken
      const localRefreshToken = storage.get('refreshToken');
      if (localRefreshToken) {
        if (!isRefreshing) {
          isRefreshing = true;
          try {
            const token = await refreshToken();
            config.headers.Authorization = token;
            return config;
          } catch (error) {
            console.error('Token refresh failed:', error);
            // 刷新失败,继续原请求
          }
        } else {
          // 正在刷新中,等待刷新完成
          return new Promise((resolve) => {
            requests.push((token: string) => {
              config.headers.Authorization = token;
              resolve(config);
            });
          });
        }
      }
    }
  }
  return config;
}, (error: any) => {
  return Promise.reject(error);
});

//响应拦截器
service.interceptors.response.use(async (res) => {
  // token 过期
  if (localStorage) {
    switch (res.data.code) {
      case statusCode.TOKEN_EXPRIED:
        if (storage.get('refreshToken')) {
          if (!isRefreshing) {
            // 首次收到需要刷新token的响应
            isRefreshing = true;
            try {
              const token = await refreshToken();
              res.config.headers.Authorization = token;
              return service.request(res.config);
            } catch (error) {
              console.error('Token refresh in response interceptor failed:', error);
              // 刷新失败,返回原响应
              return res;
            }
          } else {
            // 正在刷新中,等待刷新完成后重试
            return new Promise((resolve) => {
              requests.push((token: string) => {
                res.config.headers.Authorization = token;
                resolve(service(res.config));
              });
            });
          }
        }
        break;
      case statusCode.LOGIN_AGAIN:
        // 清理缓存信息
        storage.remove("token");
        storage.remove('refreshToken');
        Cookies.remove('user_id');
        navigateTo({ name: 'login' });
        break;
    }
  }
  return res;
}, (error) => {
  return Promise.reject(error);
});

export default service;