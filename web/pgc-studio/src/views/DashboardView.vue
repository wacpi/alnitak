<script setup lang="ts">
import * as echarts from 'echarts'
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

const barRef = ref<HTMLElement | null>(null)
const radarRef = ref<HTMLElement | null>(null)
let barChart: echarts.ECharts | null = null
let radarChart: echarts.ECharts | null = null

const router = useRouter()

const stats = [
  {
    label: '总播放量',
    value: '1.24M',
    delta: '+12.4%',
    deltaLabel: '较上月',
    up: true,
  },
  {
    label: '关注人数',
    value: '84,200',
    delta: '+3,120',
    deltaLabel: '较上月',
    up: true,
  },
  {
    label: '预计收益',
    value: '¥42,850',
    delta: '-2.1%',
    deltaLabel: '较上月',
    up: false,
  },
  {
    label: '平均播放时长',
    value: '18m 42s',
    delta: '+0:45',
    deltaLabel: '较上月',
    up: true,
  },
]

const submissions = [
  {
    title: '深海光年 · 终章',
    category: '电影',
    resolution: '4K UHD',
    date: '2026-03-28',
    status: '审核中',
    statusType: 'pending' as const,
  },
  {
    title: '城市夜行记 S02E06',
    category: '电视剧',
    resolution: '1080p',
    date: '2026-03-26',
    status: '已发布',
    statusType: 'live' as const,
  },
  {
    title: '季风',
    category: '纪录片',
    resolution: '1080p',
    date: '2026-03-24',
    status: '待修改',
    statusType: 'edit' as const,
  },
  {
    title: '星尘物语 OVA',
    category: '动漫',
    resolution: '1440p',
    date: '2026-03-21',
    status: '已发布',
    statusType: 'live' as const,
  },
]

function initBar() {
  if (!barRef.value) return
  barChart = echarts.init(barRef.value, undefined, { renderer: 'canvas' })
  const days = Array.from({ length: 30 }, (_, i) => `${i + 1}日`)
  const natural = days.map(() => Math.round(30 + Math.random() * 70))
  const recommended = days.map(() => Math.round(15 + Math.random() * 45))
  barChart.setOption({
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(18,26,38,0.95)',
      borderColor: '#1e2a3d',
      textStyle: { color: '#e2e8f0' },
    },
    legend: {
      data: ['自然流量', '推荐流量'],
      right: 0,
      top: 0,
      textStyle: { color: '#94a3b8', fontSize: 12 },
    },
    grid: { left: '3%', right: '3%', bottom: '8%', top: 48, containLabel: true },
    xAxis: {
      type: 'category',
      data: days,
      axisLine: { lineStyle: { color: '#2d3f59' } },
      axisLabel: { color: '#64748b', fontSize: 10 },
    },
    yAxis: {
      type: 'value',
      splitLine: { lineStyle: { color: '#1e2a3d', type: 'dashed' } },
      axisLabel: { color: '#64748b' },
    },
    series: [
      {
        name: '自然流量',
        type: 'bar',
        barMaxWidth: 8,
        itemStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: '#22d3ee' },
            { offset: 1, color: '#0891b2' },
          ]),
          borderRadius: [4, 4, 0, 0],
        },
        data: natural,
      },
      {
        name: '推荐流量',
        type: 'bar',
        barMaxWidth: 8,
        itemStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: '#a78bfa' },
            { offset: 1, color: '#5b21b6' },
          ]),
          borderRadius: [4, 4, 0, 0],
        },
        data: recommended,
      },
    ],
  })
}

function initRadar() {
  if (!radarRef.value) return
  radarChart = echarts.init(radarRef.value, undefined, { renderer: 'canvas' })
  radarChart.setOption({
    backgroundColor: 'transparent',
    radar: {
      radius: '65%',
      center: ['50%', '52%'],
      indicator: [
        { name: '电影', max: 100 },
        { name: '电视剧', max: 100 },
        { name: '动漫', max: 100 },
        { name: '纪录片', max: 100 },
      ],
      splitNumber: 4,
      axisName: { color: '#94a3b8', fontSize: 11 },
      splitLine: { lineStyle: { color: '#1e2a3d' } },
      splitArea: {
        show: true,
        areaStyle: {
          color: ['rgba(6,182,212,0.04)', 'rgba(6,182,212,0.02)', 'rgba(6,182,212,0.04)', 'rgba(6,182,212,0.02)'],
        },
      },
      axisLine: { lineStyle: { color: '#2d3f59' } },
    },
    series: [
      {
        type: 'radar',
        symbol: 'circle',
        symbolSize: 5,
        lineStyle: { color: '#06b6d4', width: 2 },
        areaStyle: { color: 'rgba(6,182,212,0.25)' },
        data: [{ value: [42, 28, 15, 15], name: '分布领域' }],
      },
    ],
  })
}

function onResize() {
  barChart?.resize()
  radarChart?.resize()
}

onMounted(() => {
  initBar()
  initRadar()
  window.addEventListener('resize', onResize)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  barChart?.dispose()
  radarChart?.dispose()
  barChart = null
  radarChart = null
})

const statusClass = (t: (typeof submissions)[0]['statusType']) => {
  if (t === 'live') return 'border-emerald-500/40 bg-emerald-500/10 text-emerald-400'
  if (t === 'pending') return 'border-cyan-500/40 bg-cyan-500/10 text-cyan-400'
  return 'border-amber-500/40 bg-amber-500/10 text-amber-400'
}
</script>

<template>
  <div class="mx-auto max-w-[1400px] space-y-6">
    <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
      <div>
        <h2 class="text-2xl font-semibold tracking-tight text-studio-fg">创作表现</h2>
        <p class="mt-1 max-w-2xl text-sm text-studio-muted">
          实时监控您的专业影音档案在全球分发渠道的表现数据
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-3">
        <button
          type="button"
          class="inline-flex items-center gap-2 rounded-xl border border-studio-border bg-studio-card px-4 py-2.5 text-sm font-medium text-studio-fg transition hover:border-studio-muted hover:bg-studio-elevated"
        >
          <svg class="h-4 w-4 text-studio-muted" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-width="2" d="M4 16v2a2 2 0 002 2h12M7 10l5 5m0 0l5-5m-5 5V4" />
          </svg>
          导出报告
        </button>
        <button
          type="button"
          class="inline-flex items-center gap-2 rounded-xl border-2 border-dashed border-cyan-500/60 bg-cyan-500/5 px-4 py-2.5 text-sm font-semibold text-cyan-400 transition hover:border-cyan-400 hover:bg-cyan-500/10"
          @click="router.push('/submit')"
        >
          新建投稿
        </button>
      </div>
    </div>

    <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <div
        v-for="s in stats"
        :key="s.label"
        class="rounded-2xl border border-studio-border bg-studio-card p-5 shadow-sm"
      >
        <p class="text-xs font-medium uppercase tracking-wide text-studio-muted">{{ s.label }}</p>
        <p class="mt-2 text-2xl font-semibold text-studio-fg">{{ s.value }}</p>
        <p class="mt-2 flex items-center gap-2 text-xs">
          <span :class="s.up ? 'text-emerald-400' : 'text-rose-400'">{{ s.delta }}</span>
          <span class="text-studio-muted">{{ s.deltaLabel }}</span>
        </p>
      </div>
    </div>

    <div class="grid gap-4 lg:grid-cols-5">
      <section class="rounded-2xl border border-studio-border bg-studio-card p-5 lg:col-span-3">
        <div class="mb-4 flex items-center justify-between">
          <h3 class="text-sm font-semibold text-studio-fg">播放趋势</h3>
          <span class="text-xs text-studio-muted">近 30 天</span>
        </div>
        <div ref="barRef" class="h-[280px] w-full" />
      </section>

      <section class="relative rounded-2xl border border-studio-border bg-studio-card p-5 lg:col-span-2">
        <div class="mb-2 flex items-center justify-between">
          <h3 class="text-sm font-semibold text-studio-fg">分布领域</h3>
        </div>
        <ul class="mb-2 space-y-1 text-xs text-studio-muted">
          <li><span class="text-studio-fg-muted">电影</span> · 42%</li>
          <li><span class="text-studio-fg-muted">电视剧</span> · 28%</li>
          <li><span class="text-studio-fg-muted">动漫</span> · 15%</li>
          <li><span class="text-studio-fg-muted">纪录片</span> · 15%</li>
        </ul>
        <div class="relative h-[240px] w-full">
          <div ref="radarRef" class="absolute inset-0 h-full w-full" />
          <div
            class="pointer-events-none absolute inset-0 flex items-center justify-center pb-6 text-center text-sm font-medium text-studio-fg-subtle"
          >
            <span class="rounded-full bg-studio-card/90 px-3 py-1 text-xs text-studio-fg-subtle backdrop-blur">4 主要类别</span>
          </div>
        </div>
      </section>
    </div>

    <section class="rounded-2xl border border-studio-border bg-studio-card p-5">
      <div class="mb-4 flex items-center justify-between">
        <h3 class="text-sm font-semibold text-studio-fg">最近投稿</h3>
        <button
          type="button"
          class="text-xs font-medium text-cyan-400 hover:text-cyan-300"
          @click="router.push('/content')"
        >
          查看全部
        </button>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full min-w-[640px] border-collapse text-left text-sm">
          <thead>
            <tr class="text-xs text-studio-muted">
              <th class="pb-3 font-medium">资产名称</th>
              <th class="pb-3 font-medium">类别</th>
              <th class="pb-3 font-medium">分辨率</th>
              <th class="pb-3 font-medium">投稿日期</th>
              <th class="pb-3 font-medium">状态</th>
            </tr>
          </thead>
          <tbody class="text-studio-fg-muted">
            <tr
              v-for="row in submissions"
              :key="row.title"
              class=""
            >
              <td class="py-3.5 font-medium text-studio-fg">{{ row.title }}</td>
              <td class="py-3.5">{{ row.category }}</td>
              <td class="py-3.5">{{ row.resolution }}</td>
              <td class="py-3.5 text-studio-muted">{{ row.date }}</td>
              <td class="py-3.5">
                <span
                  class="inline-flex rounded-full border px-2.5 py-0.5 text-xs font-medium"
                  :class="statusClass(row.statusType)"
                >
                  {{ row.status }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>
