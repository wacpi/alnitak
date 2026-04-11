<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'

import SliderCaptcha from '@/components/SliderCaptcha.vue'
import { useAuthStore } from '@/stores/auth'

type LoginFormState = {
  email: string
  password: string
}

const router = useRouter()
const auth = useAuthStore()

const form = ref<LoginFormState>({
  email: '',
  password: '',
})

const errorMsg = ref('')

const needCaptcha = ref(false)
const captchaId = ref('')

const submitting = ref(false)

async function submitOnce(nextCaptchaId?: string) {
  errorMsg.value = ''
  submitting.value = true
  try {
    const res = await auth.login(form.value.email, form.value.password, nextCaptchaId)
    if (res.ok) {
      router.push('/dashboard')
      return
    }

    if (res.code === -1 && res.captchaId) {
      needCaptcha.value = true
      captchaId.value = res.captchaId
      errorMsg.value = res.msg || '需要人机验证'
      return
    }

    needCaptcha.value = false
    captchaId.value = ''
    errorMsg.value = res.msg || '登录失败'
  } finally {
    submitting.value = false
  }
}

async function onCaptchaValidated() {
  // 滑块通过后，使用后端给出的 captchaId 再次提交登录
  await submitOnce(captchaId.value)
}

async function submit() {
  // 第一次登录：不带 captchaId
  await submitOnce('')
}
</script>

<template>
  <div class="flex h-full items-center justify-center px-4 py-12">
    <div class="w-full max-w-md rounded-2xl border border-studio-border bg-studio-card p-6 shadow-sm">
      <div class="text-center">
        <h1 class="text-2xl font-semibold tracking-tight text-studio-fg">PGC 管理系统</h1>
        <p class="mt-1 text-sm text-studio-muted">请使用邮箱 + 密码登录</p>
      </div>

      <form class="mt-7 space-y-4" @submit.prevent="submit">
        <div>
          <label class="mb-1 block text-sm font-medium text-studio-muted">邮箱</label>
          <input
            v-model="form.email"
            type="email"
            class="w-full rounded-xl border border-studio-border bg-studio-input px-3 py-2.5 text-sm text-studio-fg placeholder:text-studio-muted outline-none focus:border-cyan-400 focus:ring-2 focus:ring-cyan-400/20"
            placeholder="name@example.com"
            autocomplete="username"
            required
          />
        </div>

        <div>
          <label class="mb-1 block text-sm font-medium text-studio-muted">密码</label>
          <input
            v-model="form.password"
            type="password"
            class="w-full rounded-xl border border-studio-border bg-studio-input px-3 py-2.5 text-sm text-studio-fg placeholder:text-studio-muted outline-none focus:border-cyan-400 focus:ring-2 focus:ring-cyan-400/20"
            placeholder="至少 8 位"
            autocomplete="current-password"
            required
            minlength="8"
          />
        </div>

        <div v-if="errorMsg" class="rounded-xl border border-rose-500/30 bg-rose-500/10 p-3 text-sm text-rose-400">
          {{ errorMsg }}
        </div>

        <button
          type="submit"
          class="mt-2 inline-flex w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-cyan-500 to-cyan-600 px-4 py-2.5 text-sm font-semibold text-white shadow-lg shadow-cyan-500/25 transition hover:from-cyan-400 hover:to-cyan-500 disabled:opacity-60"
          :disabled="auth.loading || submitting"
        >
          登录
        </button>
      </form>

      <div v-if="needCaptcha" class="mt-6">
        <SliderCaptcha :captcha-id="captchaId" @validated="onCaptchaValidated" />
      </div>
    </div>
  </div>
</template>

