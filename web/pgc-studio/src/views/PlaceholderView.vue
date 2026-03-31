<script setup lang="ts">
import { useRoute } from 'vue-router'

const route = useRoute()
const title = route.meta.title as string
</script>

<template>
  <div class="flex flex-col items-center justify-center py-24 text-center">
    <div class="rounded-2xl border border-dashed border-studio-border bg-studio-card px-10 py-12">
      <h2 class="text-lg font-semibold text-studio-fg">{{ title }}</h2>
      <p class="mt-2 text-sm text-studio-muted">该模块正在接入，后续将对接 PGC 后端 API。</p>
    </div>
  </div>
</template>
