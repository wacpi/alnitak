<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import { useThemeStore } from '@/stores/theme'
import { useAuthStore } from '@/stores/auth'
import SliderCaptcha from '@/components/SliderCaptcha.vue'
import ToastHost from '@/components/ToastHost.vue'
import { sendEmailCodeApi } from '@/api/verify'
import { modifyPwdApi } from '@/api/auth'

const route = useRoute()
const router = useRouter()
const theme = useThemeStore()
const { mode } = storeToRefs(theme)
const auth = useAuthStore()
const { userInfo, token } = storeToRefs(auth)

const navMain = [
  { name: 'dashboard', to: '/dashboard', label: '控制台', icon: 'grid' },
  { name: 'content', to: '/content', label: '内容管理', icon: 'folder' },
  { name: 'submit', to: '/submit', label: '投稿发布', icon: 'upload' },
  { name: 'analytics', to: '/analytics', label: '数据分析', icon: 'chart' },
] as const

const navBottom = [
  { name: 'settings', to: '/settings', label: '设置', icon: 'gear' },
  { name: 'help', to: '/help', label: '帮助支持', icon: 'help' },
] as const

const isActive = (name: string) => {
  if (name === 'dashboard') return route.name === 'dashboard'
  return route.name === name
}

const searchPlaceholder = computed(
  () =>
    (route.meta.searchPlaceholder as string | undefined) ?? '搜索资源、投稿或报告...',
)

const avatarSrc = computed(() => {
  const avatar = userInfo.value?.avatar
  if (avatar) return avatar
  const seed = encodeURIComponent(userInfo.value?.name ?? 'PGC')
  return `https://api.dicebear.com/7.x/avataaars/svg?seed=${seed}`
})

/** 按钮图标表示「当前」主题：深色 → 月亮，浅色 → 太阳 */
const isDark = computed(() => mode.value === 'dark')

// 头像菜单（修改密码 / 退出登录）
const userMenuOpen = ref(false)
const userMenuWrapRef = ref<HTMLElement | null>(null)

// 修改密码弹窗
const showModifyPwd = ref(false)
const modifyPwdEmail = ref('')
const modifyPwdNewPassword = ref('')
const modifyPwdEmailCode = ref('')
const modifyPwdErrorMsg = ref('')

const sliderCaptchaId = ref('') // 给后端校验用的验证码ID
const sendingCode = ref(false)
const sendCountdown = ref(0)
let sendCountdownTimer: number | undefined

// 退出登录确认弹窗
const showLogoutConfirm = ref(false)
const logoutConfirmMsg = ref('确定要退出登录吗？')

function stopCountdown() {
  if (sendCountdownTimer) window.clearInterval(sendCountdownTimer)
  sendCountdownTimer = undefined
}

function openModifyPwd() {
  userMenuOpen.value = false
  showLogoutConfirm.value = false
  showModifyPwd.value = true

  modifyPwdErrorMsg.value = ''
  modifyPwdNewPassword.value = ''
  modifyPwdEmailCode.value = ''
  sliderCaptchaId.value = ''
  sendCountdown.value = 0
  modifyPwdEmail.value = userInfo.value?.email ?? ''
  stopCountdown()
}

function closeModifyPwd() {
  showModifyPwd.value = false
  modifyPwdErrorMsg.value = ''
  sendCountdown.value = 0
  stopCountdown()
  sliderCaptchaId.value = ''
}

function openLogoutConfirm() {
  userMenuOpen.value = false
  showModifyPwd.value = false
  showLogoutConfirm.value = true
}

function closeLogoutConfirm() {
  showLogoutConfirm.value = false
}

function toggleUserMenu() {
  if (!token.value) return
  userMenuOpen.value = !userMenuOpen.value
}

function onDocClick(e: MouseEvent) {
  const t = e.target as Node
  if (!userMenuWrapRef.value) return
  if (userMenuWrapRef.value.contains(t)) return
  userMenuOpen.value = false
}

async function requestSendEmailCode() {
  const email = modifyPwdEmail.value.trim()
  if (!email) {
    modifyPwdErrorMsg.value = '请输入邮箱'
    return
  }

  modifyPwdErrorMsg.value = ''
  sendingCode.value = true
  try {
    const res = await sendEmailCodeApi({ email, captchaId: sliderCaptchaId.value })

    if (res.code === 200) {
      const next = (res.data as any)?.countdown as number | undefined
      sendCountdown.value = next ?? 60
      // 成功后清空 slider，避免用户误操作
      sliderCaptchaId.value = ''

      stopCountdown()
      sendCountdownTimer = window.setInterval(() => {
        sendCountdown.value = Math.max(0, sendCountdown.value - 1)
      }, 1000)

      return
    }

    if (res.code === -1) {
      const nextCaptchaId = (res.data as any)?.captchaId as string | undefined
      if (nextCaptchaId) {
        sliderCaptchaId.value = nextCaptchaId
        modifyPwdErrorMsg.value = '需要人机验证，完成后将自动发送验证码'
        return
      }
    }

    modifyPwdErrorMsg.value = res.msg || '发送失败'
  } finally {
    sendingCode.value = false
  }
}

async function onSliderValidated() {
  // slider 通过后再尝试发送邮箱验证码
  await requestSendEmailCode()
}

async function submitModifyPwd() {
  modifyPwdErrorMsg.value = ''
  const email = modifyPwdEmail.value.trim()
  if (!email) {
    modifyPwdErrorMsg.value = '请输入邮箱'
    return
  }
  if (modifyPwdNewPassword.value.length < 8) {
    modifyPwdErrorMsg.value = '密码长度不能小于 8 位'
    return
  }
  if (modifyPwdEmailCode.value.trim().length !== 6) {
    modifyPwdErrorMsg.value = '验证码必须为 6 位'
    return
  }

  const res = await modifyPwdApi({
    email,
    password: modifyPwdNewPassword.value,
    code: modifyPwdEmailCode.value.trim(),
    captchaId: '', // 后端 ModifyPwd 不依赖 captchaId 字段
  })

  if (res.code === 200) {
    closeModifyPwd()
    userMenuOpen.value = false
    return
  }

  modifyPwdErrorMsg.value = res.msg || '修改失败'
}

async function confirmLogout() {
  closeLogoutConfirm()
  await auth.logout()
  userMenuOpen.value = false
  router.push('/login')
}

onMounted(() => {
  if (!token.value) return
  if (!userInfo.value) auth.fetchMe()
  document.addEventListener('click', onDocClick)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', onDocClick)
  stopCountdown()
})
</script>

<template>
  <div class="flex h-full min-h-0 bg-studio-bg">
    <!-- Sidebar -->
    <aside
      class="flex w-60 shrink-0 flex-col border-r border-studio-border bg-studio-surface/80 backdrop-blur-sm"
    >
      <div class="px-5 pt-6 pb-4">
        <h1 class="text-lg font-semibold tracking-tight text-studio-fg">影音档案</h1>
        <p class="mt-0.5 text-xs text-studio-muted">PGC 管理系统</p>
      </div>

      <div class="px-4">
        <RouterLink
          to="/submit"
          class="flex w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-cyan-500 to-cyan-600 px-4 py-3 text-sm font-semibold text-slate-950 shadow-lg shadow-cyan-500/20 transition hover:from-cyan-400 hover:to-cyan-500"
        >
          <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" d="M12 5v14M5 12h14" />
          </svg>
          新建作品
        </RouterLink>
      </div>

      <nav class="mt-8 flex flex-1 flex-col gap-1 px-3">
        <RouterLink
          v-for="item in navMain"
          :key="item.name"
          :to="item.to"
          class="group flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm transition"
          :class="
            isActive(item.name)
              ? 'bg-studio-elevated text-cyan-400 shadow-[inset_3px_0_0_#06b6d4]'
              : 'text-studio-fg-subtle hover:bg-studio-card hover:text-studio-fg'
          "
        >
          <span
            class="flex h-8 w-8 items-center justify-center rounded-lg transition"
            :class="isActive(item.name) ? 'bg-cyan-500/15 text-cyan-400' : 'bg-studio-card text-studio-fg-subtle'"
          >
            <!-- icons -->
            <svg v-if="item.icon === 'grid'" class="h-[18px] w-[18px]" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
              <path d="M4 4h7v7H4V4zM13 4h7v7h-7V4zM4 13h7v7H4v-7zM13 13h7v7h-7v-7z" stroke-linejoin="round" />
            </svg>
            <svg v-else-if="item.icon === 'folder'" class="h-[18px] w-[18px]" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
              <path d="M3 7a2 2 0 012-2h4l2 2h7a2 2 0 012 2v10a2 2 0 01-2 2H5a2 2 0 01-2-2V7z" stroke-linejoin="round" />
            </svg>
            <svg v-else-if="item.icon === 'upload'" class="h-[18px] w-[18px]" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
              <path d="M7 16V6m0 0L3 10m4-4l4 4m6 4v6H5v-6" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
            <svg v-else class="h-[18px] w-[18px]" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
              <path d="M4 19V5M4 14l4-4 4 4 4-9 4 4" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
          </span>
          {{ item.label }}
        </RouterLink>
      </nav>

      <div class="mt-auto border-t border-studio-border px-3 py-4">
        <RouterLink
          v-for="item in navBottom"
          :key="item.name"
          :to="item.to"
          class="flex items-center gap-3 rounded-lg px-3 py-2 text-sm text-studio-fg-subtle transition hover:bg-studio-card hover:text-studio-fg-muted"
        >
          <svg v-if="item.icon === 'gear'" class="h-[18px] w-[18px]" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
            <path d="M12 15a3 3 0 100-6 3 3 0 000 6z M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 010 2.83 2 2 0 01-2.83 0l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83 0 2 2 0 010-2.83l.06-.06a1.65 1.65 0 00.33-1.82 1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 112.83-2.83l.06.06a1.65 1.65 0 001.82.33H9a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 2.83l-.06.06a1.65 1.65 0 00-.33 1.82V9c.26.604.852.997 1.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z" stroke-linejoin="round" />
          </svg>
          <svg v-else class="h-[18px] w-[18px]" fill="none" stroke="currentColor" stroke-width="1.75" viewBox="0 0 24 24">
            <path d="M9 9h6M9 13h4M12 17v4M5 5h14v14H5V5z" stroke-linejoin="round" />
          </svg>
          {{ item.label }}
        </RouterLink>
      </div>
    </aside>

    <!-- Main -->
    <div class="flex min-w-0 flex-1 flex-col">
      <header
        class="flex h-16 shrink-0 items-center justify-between gap-4 border-b border-studio-border bg-studio-bg/90 px-6 backdrop-blur-sm"
      >
        <div class="flex flex-1 justify-center px-8">
          <div class="relative w-full max-w-xl">
            <span class="pointer-events-none absolute inset-y-0 left-3 flex items-center text-studio-muted">
              <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <circle cx="11" cy="11" r="7" stroke-width="2" />
                <path d="M21 21l-4.3-4.3" stroke-linecap="round" stroke-width="2" />
              </svg>
            </span>
            <input
              type="search"
              :placeholder="searchPlaceholder"
              class="w-full rounded-full border border-studio-border bg-studio-card py-2 pl-10 pr-4 text-sm text-studio-fg placeholder:text-studio-muted/70 disabled:opacity-50"
            />
          </div>
        </div>

        <div class="flex shrink-0 items-center gap-3">
          <button
            type="button"
            class="flex items-center gap-2 rounded-lg px-2 py-1.5 text-studio-fg-subtle transition hover:bg-studio-card hover:text-studio-fg"
            :title="isDark ? '当前：深色 · 点击切换浅色' : '当前：浅色 · 点击切换深色'"
            :aria-label="isDark ? '切换为浅色模式' : '切换为深色模式'"
            @click.prevent="theme.toggle"
          >
            <svg
              v-if="isDark"
              class="h-5 w-5 shrink-0"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-width="2"
                d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"
              />
            </svg>
            <svg v-else class="h-5 w-5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-width="2"
                d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"
              />
            </svg>
            <span class="hidden text-xs font-medium sm:inline">{{ isDark ? '深色' : '浅色' }}</span>
          </button>
          <button
            type="button"
            class="rounded-lg p-2 text-studio-fg-subtle transition hover:bg-studio-card hover:text-studio-fg"
            aria-label="通知"
          >
            <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-width="2" d="M15 17h5l-1.4-1.4A2 2 0 0118 14.2V11a6 6 0 10-12 0v3.2c0 .5-.2 1-.6 1.4L4 17h5m6 0a3 3 0 11-6 0h6z" />
            </svg>
          </button>
          <button
            type="button"
            class="rounded-lg p-2 text-studio-fg-subtle transition hover:bg-studio-card hover:text-studio-fg"
            aria-label="应用"
          >
            <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-width="2" d="M4 5h7v7H4V5zM13 5h7v7h-7V5zM4 14h7v7H4v-7zM13 14h7v7h-7v-7z" />
            </svg>
          </button>
          <div
            ref="userMenuWrapRef"
            class="relative ml-2"
          >
            <button
              type="button"
              class="flex items-center gap-3 rounded-xl border border-studio-border bg-studio-card px-3 py-1.5 transition hover:bg-studio-elevated"
              aria-label="用户菜单"
              @click.stop.prevent="toggleUserMenu"
            >
              <img
                :src="avatarSrc"
                alt=""
                class="h-9 w-9 rounded-full bg-studio-elevated"
                width="36"
                height="36"
              />
            </button>

            <div
              v-if="userMenuOpen"
              class="absolute right-0 top-10 z-50 w-40 overflow-hidden rounded-xl border border-studio-border bg-studio-card shadow-lg"
            >
              <button
                type="button"
                class="block w-full px-4 py-3 text-left text-sm text-studio-fg-subtle transition hover:bg-studio-elevated hover:text-cyan-400"
                @click.stop.prevent="openModifyPwd"
              >
                修改密码
              </button>
              <button
                type="button"
                class="block w-full px-4 py-3 text-left text-sm text-studio-fg-subtle transition hover:bg-rose-500/10 hover:text-rose-400"
                @click.stop.prevent="openLogoutConfirm"
              >
                退出登录
              </button>
            </div>
          </div>
        </div>
      </header>

      <main class="min-h-0 flex-1 overflow-auto p-6">
        <RouterView />
      </main>
    </div>
  </div>

  <!-- 修改密码弹窗 -->
  <div v-if="showModifyPwd" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
    <div class="w-full max-w-xl rounded-2xl border border-studio-border bg-studio-card p-6 shadow-lg">
      <div class="flex items-center justify-between gap-4">
        <div>
          <h2 class="text-lg font-semibold text-studio-fg">修改密码</h2>
          <p class="mt-1 text-xs text-studio-muted">发送邮箱验证码并完成修改</p>
        </div>
        <button
          type="button"
          class="rounded-lg p-2 text-studio-fg-subtle transition hover:bg-studio-elevated hover:text-studio-fg"
          aria-label="关闭"
          @click.prevent="closeModifyPwd"
        >
          <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor">
            <path stroke-linecap="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      <div class="mt-6 space-y-4">
        <div>
          <label class="mb-1 block text-sm font-medium text-studio-muted">邮箱</label>
          <input
            v-model="modifyPwdEmail"
            type="email"
            class="w-full rounded-xl border border-studio-border bg-studio-card px-3 py-2.5 text-sm outline-none focus:border-cyan-400 focus:ring-2 focus:ring-cyan-400/20"
            placeholder="name@example.com"
            autocomplete="email"
          />
        </div>

        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <label class="mb-1 block text-sm font-medium text-studio-muted">新密码</label>
            <input
              v-model="modifyPwdNewPassword"
              type="password"
              class="w-full rounded-xl border border-studio-border bg-studio-card px-3 py-2.5 text-sm outline-none focus:border-cyan-400 focus:ring-2 focus:ring-cyan-400/20"
              placeholder="至少 8 位"
              autocomplete="new-password"
              minlength="8"
            />
          </div>

          <div>
            <label class="mb-1 block text-sm font-medium text-studio-muted">验证码</label>
            <div class="flex items-center gap-2">
              <input
                v-model="modifyPwdEmailCode"
                inputmode="numeric"
                type="text"
                class="w-full rounded-xl border border-studio-border bg-studio-card px-3 py-2.5 text-sm outline-none focus:border-cyan-400 focus:ring-2 focus:ring-cyan-400/20"
                placeholder="6 位验证码"
                maxlength="6"
              />
              <button
                type="button"
                class="rounded-xl px-3 py-2 text-sm font-semibold transition"
                :class="
                  sendingCode || sendCountdown > 0
                    ? 'cursor-not-allowed bg-studio-elevated text-studio-fg-subtle'
                    : 'bg-gradient-to-r from-cyan-500 to-cyan-600 text-slate-950 hover:from-cyan-400 hover:to-cyan-500'
                "
                :disabled="sendingCode || sendCountdown > 0"
                @click.prevent="requestSendEmailCode"
              >
                {{ sendCountdown > 0 ? `已发送(${sendCountdown}s)` : '发送验证码' }}
              </button>
            </div>
          </div>
        </div>

        <div v-if="modifyPwdErrorMsg" class="rounded-lg border border-rose-500/30 bg-rose-500/10 p-3 text-sm text-rose-400">
          {{ modifyPwdErrorMsg }}
        </div>

        <div v-if="sliderCaptchaId" class="pt-2">
          <SliderCaptcha :captcha-id="sliderCaptchaId" @validated="onSliderValidated" />
        </div>

        <div class="flex items-center justify-end gap-3 pt-2">
          <button
            type="button"
            class="rounded-xl border border-studio-border bg-studio-card px-4 py-2.5 text-sm font-semibold text-studio-fg-subtle transition hover:bg-studio-elevated hover:text-studio-fg"
            @click.prevent="closeModifyPwd"
          >
            取消
          </button>
          <button
            type="button"
            class="rounded-xl bg-gradient-to-r from-cyan-500 to-cyan-600 px-5 py-2.5 text-sm font-semibold text-slate-950 shadow-lg shadow-cyan-500/25 transition hover:from-cyan-400 hover:to-cyan-500 disabled:opacity-60"
            :disabled="sendingCode"
            @click.prevent="submitModifyPwd"
          >
            确认修改
          </button>
        </div>
      </div>
    </div>
  </div>

  <!-- 退出登录确认弹窗 -->
  <div v-if="showLogoutConfirm" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
    <div class="w-full max-w-md rounded-2xl border border-studio-border bg-studio-card p-6 shadow-lg">
      <h2 class="text-lg font-semibold text-studio-fg">退出登录</h2>
      <p class="mt-2 text-sm text-studio-muted">{{ logoutConfirmMsg }}</p>
      <div class="mt-6 flex items-center justify-end gap-3">
        <button
          type="button"
          class="rounded-xl border border-studio-border bg-studio-card px-4 py-2.5 text-sm font-semibold text-studio-fg-subtle transition hover:bg-studio-elevated hover:text-studio-fg"
          @click.prevent="closeLogoutConfirm"
        >
          取消
        </button>
        <button
          type="button"
          class="rounded-xl bg-rose-500/90 px-4 py-2.5 text-sm font-semibold text-white transition hover:bg-rose-500 disabled:opacity-60"
          @click.prevent="confirmLogout"
        >
          确认退出
        </button>
      </div>
    </div>
  </div>

  <ToastHost />
</template>
