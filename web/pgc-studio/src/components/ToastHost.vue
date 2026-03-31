<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'

type ToastType = 'success' | 'error' | 'info'

type ToastItem = {
  id: string
  type: ToastType
  message: string
}

const items = ref<ToastItem[]>([])

function addToast(type: ToastType, message: string) {
  const id = `${Date.now()}_${Math.random().toString(16).slice(2)}`
  items.value = [...items.value, { id, type, message }]
  window.setTimeout(() => {
    items.value = items.value.filter((t) => t.id !== id)
  }, 2600)
}

function onEvent(e: Event) {
  const ce = e as CustomEvent<{ type: ToastType; message: string }>
  if (!ce.detail?.message) return
  addToast(ce.detail.type ?? 'info', ce.detail.message)
}

onMounted(() => {
  window.addEventListener('pgc-studio-toast', onEvent as any)
})

onUnmounted(() => {
  window.removeEventListener('pgc-studio-toast', onEvent as any)
})
</script>

<template>
  <teleport to="body">
    <div class="fixed bottom-6 right-6 z-[9999] flex w-[360px] max-w-[calc(100vw-3rem)] flex-col gap-2">
      <div
        v-for="t in items"
        :key="t.id"
        class="rounded-xl border px-4 py-3 text-sm shadow-xl backdrop-blur"
        :class="
          t.type === 'success'
            ? 'border-emerald-500/25 bg-emerald-500/10 text-emerald-300'
            : t.type === 'error'
              ? 'border-rose-500/25 bg-rose-500/10 text-rose-300'
              : 'border-studio-border bg-studio-card/90 text-studio-fg-muted'
        "
      >
        {{ t.message }}
      </div>
    </div>
  </teleport>
</template>

