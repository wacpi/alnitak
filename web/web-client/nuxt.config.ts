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
    optimizeDeps: {
      include: [
        // Element Plus 组件预构建
        'element-plus/es/components/form/index',
        'element-plus/es/components/input/index',
        'element-plus/es/components/radio/index',
        'element-plus/es/components/radio-button/index',
        'element-plus/es/components/radio-group/index',
        'element-plus/es/components/date-picker/index',
        'element-plus/es/components/button/index',
        'element-plus/es/components/pagination/index',
        'element-plus/es/components/icon/index',
        'element-plus/es/components/scrollbar/index',
        'element-plus/es/components/dropdown/index',
        'element-plus/es/components/dropdown-item/index',
        'element-plus/es/components/dropdown-menu/index',
        'element-plus/es/components/switch/index',
        'element-plus/es/components/dialog/index',
        'element-plus/es/components/progress/index',
        'element-plus/es/components/upload/index',
        'element-plus/es/components/tag/index',
        'element-plus/es/components/tabs/index',
        'element-plus/es/components/tab-pane/index',
        'element-plus/es/components/popconfirm/index',
        'element-plus/es/components/checkbox/index',
        'element-plus/es/components/loading/index',
        'element-plus/es/components/message/index',
        'element-plus/es/components/message-box/index',
        // 其他常用依赖
        'moment',
        'hls.js',
        'wplayer-next',
        'vue-picture-cropper',
        'spark-md5',
        '@icon-park/vue-next',
        'axios',
        'js-cookie'
      ]
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
