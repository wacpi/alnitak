<script setup lang="ts">
import * as echarts from 'echarts'
import { onBeforeUnmount, onMounted, ref } from 'vue'

const reportMonth = ref('2023-10')

const kpis = [
  { label: '总曝光量', value: '12.8M', delta: '+14%', up: true },
  { label: '观看时长', value: '45.2K', unit: '小时/月', delta: '+8.2%', up: true },
  { label: '平均留存率', value: '68.5%', delta: '-0.3%', up: false },
  { label: '预计收益', value: '¥84.2K', delta: '+21%', up: true },
]

const ageGroups = [
  { label: '18–24 岁', pct: 45 },
  { label: '25–34 岁', pct: 30 },
  { label: '35–44 岁', pct: 15 },
  { label: '其他', pct: 10 },
]

const topRegions = [
  { name: '中国大陆', pct: 54 },
  { name: '北美', pct: 22 },
  { name: '东南亚', pct: 12 },
]

const topContent = [
  {
    rank: 1,
    title: '2023 巅峰纪录片：深蓝之境',
    tags: 'DOCUMENTARY · 4K HEVC',
    rating: 98,
    views: '2.4M',
    gradient: 'from-cyan-800/90 to-indigo-950',
  },
  {
    rank: 2,
    title: '光影大师课 · 色彩与叙事',
    tags: 'SERIES · 1080p HDR',
    rating: 94,
    views: '1.1M',
    gradient: 'from-violet-800/80 to-studio-bg',
  },
  {
    rank: 3,
    title: 'Future City: 2077',
    tags: 'DOCUMENTARY · 4K',
    rating: 91,
    views: '890K',
    gradient: 'from-slate-700/90 to-slate-900',
  },
]

const retentionRef = ref<HTMLElement | null>(null)
let retentionChart: echarts.ECharts | null = null

function initRetention() {
  if (!retentionRef.value) return
  retentionChart = echarts.init(retentionRef.value, undefined, { renderer: 'canvas' })
  const labels = Array.from({ length: 21 }, (_, i) => {
    const m = i
    return `${m}:00`
  })
  const base = labels.map((_, i) =>
    Math.round(32 + Math.sin(i * 0.35) * 22 + (i > 12 ? (i - 12) * 2 : 0)),
  )
  retentionChart.setOption({
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(18,26,38,0.95)',
      borderColor: '#1e2a3d',
      textStyle: { color: '#e2e8f0' },
      formatter: (params: unknown) => {
        const p = (params as { seriesName?: string; data?: number; name?: string }[])[0]
        if (!p) return ''
        return `${p.name}<br/>${p.seriesName}: ${p.data}%`
      },
    },
    grid: { left: '2%', right: '2%', top: 16, bottom: 8, containLabel: true },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: labels,
      axisLine: { lineStyle: { color: '#2d3f59' } },
      axisLabel: { color: '#64748b', fontSize: 10 },
    },
    yAxis: {
      type: 'value',
      min: 0,
      max: 100,
      splitLine: { lineStyle: { color: '#1e2a3d', type: 'dashed' } },
      axisLabel: { color: '#64748b', formatter: '{value}%' },
    },
    series: [
      {
        name: '观众留存',
        type: 'line',
        smooth: true,
        symbol: 'circle',
        symbolSize: 4,
        lineStyle: { color: '#22d3ee', width: 2 },
        itemStyle: { color: '#22d3ee' },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(34,211,238,0.35)' },
            { offset: 1, color: 'rgba(34,211,238,0.02)' },
          ]),
        },
        data: base,
      },
    ],
  })
}

function onResize() {
  retentionChart?.resize()
}

onMounted(() => {
  initRetention()
  window.addEventListener('resize', onResize)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  retentionChart?.dispose()
  retentionChart = null
})

function exportReport() {
  alert('导出报告（演示）')
}
</script>

<template>
  <div class="mx-auto max-w-[1400px] space-y-6 pb-4">
    <!-- 页头 -->
    <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight text-studio-fg">数据中心</h1>
        <p class="mt-1 text-xs font-medium uppercase tracking-wider text-cyan-500/80">
          PGC Analytics Overview · Last 30 Days
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-3">
        <input
          v-model="reportMonth"
          type="month"
          class="analytics-month-input rounded-xl border border-studio-border bg-studio-card px-3 py-2 pr-2 text-sm text-studio-fg focus:border-cyan-500/50 focus:outline-none focus:ring-1 focus:ring-cyan-500/30"
        />
        <button
          type="button"
          class="inline-flex items-center gap-2 rounded-xl border border-studio-border bg-studio-card px-4 py-2 text-sm font-medium text-studio-fg transition hover:bg-studio-elevated"
          @click="exportReport"
        >
          <svg class="h-4 w-4 text-studio-muted" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-width="2" d="M4 16v2a2 2 0 002 2h12M8 12l4 4m0-4v12m0-4l4-4" />
          </svg>
          导出报告
        </button>
      </div>
    </div>

    <!-- KPI -->
    <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <div
        v-for="k in kpis"
        :key="k.label"
        class="rounded-2xl border border-studio-border bg-studio-card p-5"
      >
        <p class="text-xs font-medium text-studio-muted">{{ k.label }}</p>
        <div class="mt-2 flex flex-wrap items-baseline gap-2">
          <span class="text-2xl font-semibold text-studio-fg">{{ k.value }}</span>
          <span v-if="k.unit" class="text-sm text-studio-muted">{{ k.unit }}</span>
        </div>
        <p class="mt-2 text-xs">
          <span :class="k.up ? 'text-emerald-400' : 'text-amber-400'">{{ k.delta }}</span>
          <span class="text-studio-muted"> · 环比</span>
        </p>
      </div>
    </div>

    <!-- 留存 + 受众 -->
    <div class="grid gap-4 lg:grid-cols-5">
      <section class="rounded-2xl border border-studio-border bg-studio-card p-5 lg:col-span-3">
        <div class="mb-2 flex items-center justify-between">
          <h2 class="text-sm font-semibold text-studio-fg">观众留存率</h2>
          <span class="text-xs text-studio-muted">单支内容 · 0:00–20:00 进度</span>
        </div>
        <div ref="retentionRef" class="h-[280px] w-full" />
      </section>

      <section class="rounded-2xl border border-studio-border bg-studio-card p-5 lg:col-span-2">
        <h2 class="text-sm font-semibold text-studio-fg">受众画像</h2>

        <p class="mt-4 text-xs font-medium text-studio-muted">性别分布</p>
        <div class="mt-2 space-y-2">
          <div>
            <div class="mb-1 flex justify-between text-xs">
              <span class="text-studio-fg-muted">男性</span>
              <span class="text-cyan-400">62%</span>
            </div>
            <div class="h-2 overflow-hidden rounded-full bg-studio-elevated">
              <div class="h-full w-[62%] rounded-full bg-gradient-to-r from-cyan-500 to-cyan-600" />
            </div>
          </div>
          <div>
            <div class="mb-1 flex justify-between text-xs">
              <span class="text-studio-fg-muted">女性</span>
              <span class="text-violet-400">38%</span>
            </div>
            <div class="h-2 overflow-hidden rounded-full bg-studio-elevated">
              <div class="h-full w-[38%] rounded-full bg-gradient-to-r from-violet-500 to-violet-600" />
            </div>
          </div>
        </div>

        <p class="mt-6 text-xs font-medium text-studio-muted">年龄段</p>
        <ul class="mt-3 space-y-3">
          <li v-for="g in ageGroups" :key="g.label" class="text-xs">
            <div class="mb-1 flex justify-between text-studio-fg-subtle">
              <span>{{ g.label }}</span>
              <span class="text-studio-fg">{{ g.pct }}%</span>
            </div>
            <div class="h-1.5 overflow-hidden rounded-full bg-studio-elevated">
              <div
                class="h-full rounded-full bg-slate-500"
                :style="{ width: `${g.pct}%`, background: 'linear-gradient(90deg,#64748b,#94a3b8)' }"
              />
            </div>
          </li>
        </ul>
      </section>
    </div>

    <!-- 全球触达 + 表现优异 -->
    <div class="grid gap-4 lg:grid-cols-5">
      <section class="relative overflow-hidden rounded-2xl border border-studio-border bg-studio-card p-5 lg:col-span-2">
        <div class="flex items-start justify-between">
          <h2 class="text-sm font-semibold text-studio-fg">全球触达</h2>
          <span
            class="rounded-full border border-emerald-500/40 bg-emerald-500/10 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-emerald-400"
          >
            Heatmap active
          </span>
        </div>

        <div
          class="relative mt-4 flex h-[220px] items-center justify-center overflow-hidden rounded-xl bg-gradient-to-br from-[#0d1520] via-studio-bg to-cyan-950/30 ring-1 ring-studio-border"
        >
          <div
            class="pointer-events-none absolute inset-0 opacity-40"
            style="
              background-image: radial-gradient(circle at 30% 40%, rgba(6, 182, 212, 0.35) 0%, transparent 45%),
                radial-gradient(circle at 70% 55%, rgba(139, 92, 246, 0.2) 0%, transparent 50%);
            "
          />
          <span class="relative text-xs text-studio-muted">地图热力占位 · 接 GeoIP / 播放日志</span>

          <div
            class="absolute bottom-3 left-3 right-3 rounded-lg border border-studio-border/80 bg-studio-card/95 px-3 py-2 backdrop-blur-sm"
          >
            <p class="text-[10px] font-semibold uppercase tracking-wide text-studio-muted">Top regions</p>
            <ul class="mt-1 space-y-1">
              <li
                v-for="r in topRegions"
                :key="r.name"
                class="flex justify-between text-xs text-studio-fg-muted"
              >
                <span>{{ r.name }}</span>
                <span class="text-cyan-400/90">{{ r.pct }}%</span>
              </li>
            </ul>
          </div>
        </div>
      </section>

      <section class="rounded-2xl border border-studio-border bg-studio-card p-5 lg:col-span-3">
        <h2 class="text-sm font-semibold text-studio-fg">表现优异的内容</h2>
        <ul class="mt-4 space-y-4">
          <li
            v-for="item in topContent"
            :key="item.rank"
            class="flex gap-4 rounded-xl border border-studio-border/60 bg-studio-elevated/40 p-3"
          >
            <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-cyan-500/15 text-sm font-bold text-cyan-400">
              {{ item.rank }}
            </div>
            <div
              class="h-14 w-24 shrink-0 overflow-hidden rounded-lg ring-1 ring-studio-border bg-gradient-to-br"
              :class="item.gradient"
            />
            <div class="min-w-0 flex-1">
              <p class="font-medium text-studio-fg line-clamp-1">{{ item.title }}</p>
              <p class="mt-0.5 text-[11px] uppercase tracking-wide text-studio-muted">{{ item.tags }}</p>
              <div class="mt-2 flex flex-wrap gap-3 text-xs">
                <span class="text-emerald-400/90">{{ item.rating }}% 评分</span>
                <span class="text-studio-muted">{{ item.views }} 播放</span>
              </div>
            </div>
          </li>
        </ul>
      </section>
    </div>

    <!-- 实时流健康（演示条） -->
    <section
      class="flex flex-col gap-4 rounded-2xl border border-dashed border-cyan-500/25 bg-studio-card/50 px-5 py-4 sm:flex-row sm:items-center sm:justify-between"
    >
      <div class="flex items-center gap-3">
        <span class="text-xs font-semibold uppercase tracking-wider text-studio-muted">Real-time stream health</span>
        <span class="flex items-center gap-1.5 text-xs font-medium text-emerald-400">
          <span class="relative flex h-2 w-2">
            <span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-50" />
            <span class="relative h-2 w-2 rounded-full bg-emerald-400" />
          </span>
          LIVE NOW
        </span>
      </div>
      <div class="flex flex-1 items-center gap-4 max-w-xl">
        <div class="h-2 flex-1 overflow-hidden rounded-full bg-studio-elevated">
          <div class="h-full w-[92%] rounded-full bg-gradient-to-r from-emerald-500 to-cyan-500" />
        </div>
      </div>
      <div class="flex flex-wrap gap-6 font-mono text-[11px] text-studio-muted">
        <span><span class="text-studio-muted">FPS</span> 60.0</span>
        <span><span class="text-studio-muted">BITRATE</span> 12.4 Mbps</span>
        <span><span class="text-studio-muted">DROPPED</span> 0.0%</span>
      </div>
    </section>
  </div>
</template>

<style scoped>
/* 浅色：深灰图标；深色见下方非 scoped 规则 */
.analytics-month-input::-webkit-calendar-picker-indicator {
  cursor: pointer;
  width: 1.125rem;
  height: 1.125rem;
  margin-left: 0.25rem;
  opacity: 1;
  filter: none;
  background: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='18' height='18' viewBox='0 0 24 24' fill='none' stroke='%23475569' stroke-width='2' stroke-linecap='round'%3E%3Crect x='3' y='4' width='18' height='18' rx='2' ry='2'/%3E%3Cpath d='M16 2v4M8 2v4M3 10h18'/%3E%3C/svg%3E")
    center / contain no-repeat;
}
</style>

<style>
html.dark .analytics-month-input::-webkit-calendar-picker-indicator {
  background: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='18' height='18' viewBox='0 0 24 24' fill='none' stroke='%23e2e8f0' stroke-width='2' stroke-linecap='round'%3E%3Crect x='3' y='4' width='18' height='18' rx='2' ry='2'/%3E%3Cpath d='M16 2v4M8 2v4M3 10h18'/%3E%3C/svg%3E")
    center / contain no-repeat;
}
</style>
