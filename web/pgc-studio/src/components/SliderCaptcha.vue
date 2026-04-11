<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'

import { getSliderCaptchaApi, validateSliderApi } from '@/api/verify'
import type { SliderCaptchaData } from '@/api/verify'

const props = defineProps<{
  captchaId: string
}>()

const emit = defineEmits<{
  (e: 'validated'): void
}>()

const bgImgSrc = ref('')
const sliderImgSrc = ref('')
const y = ref(0)

const errorMsg = ref('')
const loadingCaptcha = ref(false)
const validating = ref(false)

const captchaDataReady = computed(() => Boolean(bgImgSrc.value && sliderImgSrc.value))

// 用于像素换算：把拖拽出来的“显示像素 X”转换成后端缓存的“原图像素 X”
const bgNaturalW = ref(0)
const bgNaturalH = ref(0)
const bgDisplayW = ref(0)
const bgDisplayH = ref(0)

const scaleX = computed(() => {
  if (!bgNaturalW.value || !bgDisplayW.value) return 1
  return bgNaturalW.value / bgDisplayW.value
})

const scaleY = computed(() => {
  if (!bgNaturalH.value || !bgDisplayH.value) return 1
  return bgNaturalH.value / bgDisplayH.value
})

const bgImgEl = ref<HTMLImageElement | null>(null)
const sliderImgEl = ref<HTMLImageElement | null>(null)

// 为了保证 slider 与背景缩放一致：让 slider 的显示高度等于背景显示高度
const sliderHeightPx = ref<number | null>(null)

const sliderLeftPx = ref(0)
const maxOffsetPx = computed(() => {
  if (!bgDisplayW.value || !sliderImgEl.value) return 9999
  return Math.max(0, bgDisplayW.value - sliderImgEl.value.clientWidth)
})

const sliderTopPx = computed(() => Math.round(y.value * scaleY.value))

async function fetchCaptcha() {
  if (!props.captchaId) return
  errorMsg.value = ''
  loadingCaptcha.value = true
  sliderLeftPx.value = 0
  sliderHeightPx.value = null

  try {
    const res = await getSliderCaptchaApi(props.captchaId)
    if (res.code !== 200) {
      errorMsg.value = res.msg || '获取验证码失败'
      return
    }

    const next = (res.data as any)?.slider_captcha as SliderCaptchaData | undefined
    if (!next) {
      errorMsg.value = '验证码数据缺失'
      return
    }

    bgImgSrc.value = next.bg_img
    sliderImgSrc.value = next.slider_img
    y.value = next.y

    await nextTick()
  } catch (e: any) {
    errorMsg.value = e?.message || '获取验证码失败'
  } finally {
    loadingCaptcha.value = false
  }
}

async function configureScalesIfReady() {
  const bgEl = bgImgEl.value
  const sliderEl = sliderImgEl.value
  if (!bgEl || !sliderEl) return

  // naturalWidth/Height 只有图片加载后才可靠
  if (!bgEl.naturalWidth || !bgEl.naturalHeight) return
  const displayW = bgEl.clientWidth
  const displayH = bgEl.clientHeight
  if (!displayW || !displayH) return

  bgNaturalW.value = bgEl.naturalWidth
  bgNaturalH.value = bgEl.naturalHeight
  bgDisplayW.value = displayW
  bgDisplayH.value = displayH

  // 保证 slider 与背景相同缩放
  sliderHeightPx.value = displayH
}

watch(
  () => props.captchaId,
  () => {
    fetchCaptcha()
  },
  { immediate: true },
)

function clamp(n: number, min: number, max: number) {
  return Math.min(max, Math.max(min, n))
}

let dragging = false
let startX = 0
let startLeft = 0

function onPointerDown(e: PointerEvent) {
  if (!captchaDataReady.value) return
  if (validating.value) return

  e.preventDefault()
  dragging = true
  startX = e.clientX
  startLeft = sliderLeftPx.value

  window.addEventListener('pointermove', onPointerMove)
  window.addEventListener('pointerup', onPointerUp)
}

async function onPointerUp() {
  if (!dragging) return
  dragging = false
  window.removeEventListener('pointermove', onPointerMove)
  window.removeEventListener('pointerup', onPointerUp)

  // 拖拽结束，提交验证
  if (!props.captchaId) return
  if (!bgNaturalW.value || !bgDisplayW.value) return

  validating.value = true
  errorMsg.value = ''
  try {
    const xBackend = Math.round(sliderLeftPx.value * scaleX.value)
    const res = await validateSliderApi({ captchaId: props.captchaId, x: xBackend })
    if (res.code !== 200) {
      errorMsg.value = res.msg || '滑块验证失败'
      sliderLeftPx.value = 0
      return
    }

    emit('validated')
  } catch (e: any) {
    errorMsg.value = e?.message || '滑块验证失败'
    sliderLeftPx.value = 0
  } finally {
    validating.value = false
  }
}

function onPointerMove(e: PointerEvent) {
  if (!dragging) return
  const delta = e.clientX - startX
  const nextLeft = clamp(startLeft + delta, 0, maxOffsetPx.value)
  sliderLeftPx.value = nextLeft
}

onBeforeUnmount(() => {
  dragging = false
  window.removeEventListener('pointermove', onPointerMove)
  window.removeEventListener('pointerup', onPointerUp)
})
</script>

<template>
  <div class="space-y-3">
    <div class="flex items-start justify-between gap-3">
      <div>
        <p class="text-sm font-medium text-studio-fg">滑块人机验证</p>
        <p class="mt-1 text-xs text-studio-muted">拖动滑块完成验证</p>
      </div>
      <button
        type="button"
        class="rounded-lg border border-studio-border bg-studio-card px-3 py-1.5 text-xs text-studio-fg transition hover:bg-studio-elevated"
        :disabled="loadingCaptcha || validating"
        @click.prevent="fetchCaptcha"
      >
        重新获取
      </button>
    </div>

    <div v-if="errorMsg" class="rounded-lg border border-rose-500/30 bg-rose-500/10 p-3 text-xs text-rose-400">
      {{ errorMsg }}
    </div>

    <div v-if="loadingCaptcha" class="text-xs text-studio-muted">加载验证码中...</div>

    <div
      v-else
      class="relative inline-block select-none"
      :class="captchaDataReady ? '' : 'opacity-70'"
      :style="{ width: bgDisplayW ? `${bgDisplayW}px` : 'auto' }"
    >
      <img
        ref="bgImgEl"
        :src="bgImgSrc"
        alt=""
        class="block max-w-full select-none"
        @load="configureScalesIfReady"
      />
      <img
        ref="sliderImgEl"
        :src="sliderImgSrc"
        alt=""
        class="absolute block cursor-grab select-none"
        :style="{
          left: `${sliderLeftPx}px`,
          top: `${sliderTopPx}px`,
          height: sliderHeightPx ? `${sliderHeightPx}px` : undefined,
        }"
        @pointerdown="onPointerDown"
        @load="configureScalesIfReady"
      />
    </div>
  </div>
</template>

