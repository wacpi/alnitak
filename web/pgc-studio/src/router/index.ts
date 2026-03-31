import { createRouter, createWebHistory } from 'vue-router'
import StudioLayout from '@/layouts/StudioLayout.vue'
import { isAuthenticated, verifyAuthentication } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue'),
      meta: { title: '登录' },
    },
    {
      path: '/',
      component: StudioLayout,
      redirect: '/dashboard',
      children: [
        {
          path: 'dashboard',
          name: 'dashboard',
          component: () => import('@/views/DashboardView.vue'),
          meta: { title: '控制台' },
        },
        {
          path: 'content',
          name: 'content',
          component: () => import('@/views/ContentManagementView.vue'),
          meta: {
            title: '内容管理',
            headerSubtitle: '资深制片人',
            searchPlaceholder: '搜索档案...',
          },
        },
        {
          path: 'content/:pgcId/edit',
          name: 'content-edit',
          component: () => import('@/views/EditAssetView.vue'),
          meta: {
            title: '编辑内容',
          },
        },
        {
          path: 'submit',
          name: 'submit',
          component: () => import('@/views/CreateAssetView.vue'),
          meta: { title: '投稿发布' },
        },
        {
          path: 'analytics',
          name: 'analytics',
          component: () => import('@/views/AnalyticsView.vue'),
          meta: {
            title: '数据分析',
            headerSubtitle: '资深制片人',
            searchPlaceholder: '搜索分析报告...',
          },
        },
        {
          path: 'settings',
          name: 'settings',
          component: () => import('@/views/PlaceholderView.vue'),
          meta: { title: '设置' },
        },
        {
          path: 'help',
          name: 'help',
          component: () => import('@/views/PlaceholderView.vue'),
          meta: { title: '帮助支持' },
        },
      ],
    },
  ],
})

router.beforeEach(async (to) => {
  const authed = isAuthenticated()
  if (to.path === '/login') {
    return authed ? { path: '/dashboard' } : true
  }

  if (!authed) {
    return { path: '/login' }
  }

  const valid = await verifyAuthentication()
  if (!valid) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }

  return true
})

export default router
