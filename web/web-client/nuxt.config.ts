import { globalConfig } from "./src/utils/global-config";

export default defineNuxtConfig({
  modules: [
    '@element-plus/nuxt',
    '@pinia/nuxt',
  ],
  app: {
    head: {
      title: globalConfig.title,
      meta: [
        {
          "name": "viewport",
          "content": "width=device-width, initial-scale=1"
        },
        {
          "charset": "utf-8"
        },
        {
          "name": "keywords",
          "content": globalConfig.keywords
        },
        {
          "name": "description",
          "content": globalConfig.description
        },
      ],
      link: [
        { rel: "icon", type: "image/x-icon", href: "/favicon.ico" }
      ],
      script: [
        {
          // 在首屏渲染前应用主题，避免刷新首页仍为浅色
          children: `(() => { try { const k='ui-theme-mode'; const m=(localStorage.getItem(k)||'light'); const r=document.documentElement; r.setAttribute('data-theme', m); if (m==='dark') r.classList.add('dark'); else r.classList.remove('dark'); } catch(e){} })();`,
        }
      ]
    }
  },
  plugins: [
    {
      src: '@/plugins/wang-editor',
      mode: 'client',
    },
    {
      src: '@/plugins/theme-init.client',
      mode: 'client',
    },
  ],
  css: [
    'element-plus/dist/index.css',
    'element-plus/theme-chalk/dark/css-vars.css',
    '~/assets/styles/element.scss'
  ],
  devtools: { enabled: true },
  srcDir: 'src/',
  vite: {
    define: {
      __VUE_OPTIONS_API__: true,
      __VUE_PROD_DEVTOOLS__: false,
      __VUE_PROD_HYDRATION_MISMATCH_DETAILS__: false,
    },
    server: {
      hmr: {
        overlay: false
      },
      proxy: {
        '/api': {
          target: process.env.API_PROXY_TARGET || `http${globalConfig.https ? 's' : ''}://${globalConfig.domain}`,
          changeOrigin: true,
          ws: true,
          // 可选：需要去掉 /api 前缀时，设置 API_PROXY_REWRITE=remove 并解开下一行
          // rewrite: process.env.API_PROXY_REWRITE === 'remove' ? (path) => path.replace(/^\/api/, '') : undefined,
          // 网络不稳定时适当拉长代理超时
          timeout: 30000,
          proxyTimeout: 30000,
        }
      }
    }
  }
})
