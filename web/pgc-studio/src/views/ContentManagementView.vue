<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, nextTick } from 'vue'
import { RouterLink } from 'vue-router'

import { getPgcListApi, updatePgcStatusApi, deletePgcApi, type PgcContentLoose } from '@/api/pgc'
import { useAuthStore } from '@/stores/auth'
import { toast } from '@/utils/toast'

const PAGE_SIZE = 10

const loading = ref(false)
const errorMsg = ref('')

const page = ref(1)
const total = ref(0)
const list = ref<PgcContentLoose[]>([])

const filterType = ref<number | ''>('')
const filterKeyword = ref('')
const auth = useAuthStore()

const actionMenuOpenId = ref<string>('')
const actionMenuAnchor = ref<HTMLElement | null>(null)
const actionMenuPos = ref<{ top: number; left: number }>({ top: 0, left: 0 })

async function toggleActionMenu(id: string, anchor?: HTMLElement | null) {
  if (actionMenuOpenId.value === id) {
    closeActionMenu()
    return
  }
  actionMenuOpenId.value = id
  actionMenuAnchor.value = anchor ?? null
  await nextTick()
  syncActionMenuPos()
}

function closeActionMenu() {
  actionMenuOpenId.value = ''
  actionMenuAnchor.value = null
}

function syncActionMenuPos() {
  const el = actionMenuAnchor.value
  if (!el) return
  const r = el.getBoundingClientRect()
  // 菜单右对齐按钮，显示在按钮下方
  actionMenuPos.value = {
    top: r.bottom + 8,
    left: Math.max(8, r.right - 160), // 160 = 菜单宽度 w-40
  }
}

function onDocClick(e: MouseEvent) {
  const target = e.target as HTMLElement | null
  if (!target) return
  // 点击菜单/按钮内部不关闭；其他区域点击关闭
  if (target.closest('[data-action-menu-root]')) return
  closeActionMenu()
}

onMounted(() => {
  document.addEventListener('click', onDocClick)
  window.addEventListener('scroll', syncActionMenuPos, true)
  window.addEventListener('resize', syncActionMenuPos)
})
onUnmounted(() => {
  document.removeEventListener('click', onDocClick)
  window.removeEventListener('scroll', syncActionMenuPos, true)
  window.removeEventListener('resize', syncActionMenuPos)
})

function pick<T = any>(obj: any, keys: string[], fallback?: T): T {
  for (const k of keys) {
    if (obj && obj[k] !== undefined && obj[k] !== null) return obj[k] as T
  }
  return fallback as T
}

function pgcTypeLabel(t: number) {
  const map: Record<number, string> = {
    1: '国创(CN)',
    2: '日创(JP)',
    3: '纪录片',
    4: '电影',
    5: '电视剧',
  }
  return map[t] ?? String(t)
}

/** 与 server/internal/global/constant.go PGC 审核状态一致 */
function pgcAuditStatusMeta(s: number): { label: string; pillClass: string } {
  const map: Record<number, { label: string; pillClass: string }> = {
    [-1]: { label: '已下架', pillClass: 'border-zinc-500/40 bg-zinc-500/15 text-zinc-300' },
    0: { label: '草稿', pillClass: 'border-slate-500/40 bg-slate-500/10 text-slate-300' },
    100: { label: '已提交', pillClass: 'border-sky-500/35 bg-sky-500/10 text-sky-300' },
    200: { label: '审核中', pillClass: 'border-amber-500/40 bg-amber-500/10 text-amber-300' },
    300: { label: '已通过', pillClass: 'border-emerald-500/40 bg-emerald-500/10 text-emerald-300' },
    400: { label: '已驳回', pillClass: 'border-rose-500/40 bg-rose-500/10 text-rose-300' },
  }
  return (
    map[s] ?? {
      label: `状态(${s})`,
      pillClass: 'border-studio-border bg-studio-elevated text-studio-muted',
    }
  )
}

function rowId(row: any) {
  // PGCID 是 Snowflake（uint64），可能超过 JS Number 安全整数范围，必须用 string 保真
  const id = pick(row, ['pgc_id', 'PGCID'], '') as string
  if (id !== '' && id != null) return String(id)
  return `pk:${pick(row, ['id', 'ID'], '')}`
}

function auditStatusForRow(row: any) {
  return pgcAuditStatusMeta(Number(pick(row, ['status', 'Status'], 0)))
}

function isOffline(row: any) {
  const s = Number(pick(row, ['status', 'Status'], 0))
  return s === -1
}

async function toggleOffline(row?: any) {
  if (!row) {
    toast('error', '内容不存在或已刷新')
    closeActionMenu()
    return
  }
  if (!auth.token) {
    toast('error', '未登录')
    return
  }
  const id = rowId(row)
  const next = isOffline(row) ? 300 : -1 // 上架回到 Approved(300)，下架设为 -1
  if (!confirm(next === -1 ? '确定下架该内容？' : '确定上架该内容？')) return
  const res = await updatePgcStatusApi(auth.token, id, next)
  if (res.code !== 200) {
    toast('error', res.msg || '操作失败')
    return
  }
  closeActionMenu()
  toast('success', next === -1 ? '已下架' : '已上架')
  await load()
}

async function deleteRow(row?: any) {
  if (!row) {
    toast('error', '内容不存在或已刷新')
    closeActionMenu()
    return
  }
  if (!auth.token) {
    toast('error', '未登录')
    return
  }
  const id = rowId(row)
  if (!confirm('确定删除该内容？此操作不可恢复。')) return
  const res = await deletePgcApi(auth.token, id)
  if (res.code !== 200) {
    toast('error', res.msg || '删除失败')
    return
  }
  closeActionMenu()
  toast('success', '已删除')
  await load()
}

type StatIcon = 'stack' | 'pulse' | 'clipboard' | 'bars'

const stats = computed((): { label: string; value: string; icon: StatIcon }[] => {
  const ongoingCount = list.value.filter((r) => Boolean(pick(r, ['is_ongoing', 'IsOngoing'], false))).length
  const episodesSum = list.value.reduce((acc, r) => acc + Number(pick(r, ['current_episodes', 'CurrentEpisodes'], 0) || 0), 0)
  return [
    { label: '内容总数', value: String(total.value), icon: 'stack' },
    { label: '连载中(本页)', value: String(ongoingCount), icon: 'pulse' },
    { label: '已更新集数(本页)', value: String(episodesSum), icon: 'clipboard' },
    { label: '当前页', value: String(page.value), icon: 'bars' },
  ]
})

async function load() {
  loading.value = true
  errorMsg.value = ''
  try {
    const res = await getPgcListApi({
      page: page.value,
      pageSize: PAGE_SIZE,
      pgcType: filterType.value === '' ? undefined : filterType.value,
      keyword: filterKeyword.value.trim() || undefined,
    })
    if (res.code !== 200) {
      errorMsg.value = res.msg || '请求失败'
      toast('error', errorMsg.value)
      list.value = []
      total.value = 0
      return
    }
    total.value = Number((res.data as any)?.total ?? 0)
    list.value = ((res.data as any)?.list ?? []) as PgcContentLoose[]
  } catch (e: any) {
    errorMsg.value = e?.message || '请求异常'
    toast('error', errorMsg.value)
  } finally {
    loading.value = false
  }
}

function nextPage() {
  const maxPage = Math.max(1, Math.ceil(total.value / PAGE_SIZE))
  if (page.value >= maxPage) return
  page.value += 1
  load()
}

function prevPage() {
  if (page.value <= 1) return
  page.value -= 1
  load()
}

function applyFilters() {
  page.value = 1
  load()
}

onMounted(() => {
  load()
})
</script>

<template>
  <div class="mx-auto max-w-[1400px] space-y-6">
    <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight text-studio-fg">内容管理</h1>
        <p class="mt-1 max-w-2xl text-sm text-studio-muted">
          管理、规划并监控您的专业影音资产，掌握发布节奏与审核状态。
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-3">
        <div class="flex flex-wrap items-center gap-3 rounded-xl border border-studio-border bg-studio-card px-3 py-2">
          <select
            v-model="filterType"
            class="rounded-lg border border-studio-border bg-studio-input px-3 py-2 text-sm text-studio-fg"
          >
            <option value="">全部类型</option>
            <option :value="1">国创(CN)</option>
            <option :value="2">日创(JP)</option>
            <option :value="3">纪录片</option>
            <option :value="4">电影</option>
            <option :value="5">电视剧</option>
          </select>
          <input
            v-model="filterKeyword"
            type="text"
            placeholder="关键词（标题/简介）"
            class="w-56 rounded-lg border border-studio-border bg-studio-input px-3 py-2 text-sm text-studio-fg placeholder:text-studio-muted"
          />
          <button
            type="button"
            class="rounded-lg bg-studio-elevated px-3 py-2 text-sm text-studio-fg transition hover:text-cyan-400"
            @click="applyFilters"
          >
            查询
          </button>
        </div>
        <RouterLink
          to="/submit"
          class="inline-flex items-center gap-2 rounded-xl bg-gradient-to-r from-cyan-500 to-cyan-600 px-4 py-2.5 text-sm font-semibold text-slate-950 shadow-lg shadow-cyan-500/25 transition hover:from-cyan-400 hover:to-cyan-500"
        >
          <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-width="2" d="M12 5v14M5 12h14" />
          </svg>
          上传新内容
        </RouterLink>
      </div>
    </div>

    <!-- 统计卡片 -->
    <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <div
        v-for="s in stats"
        :key="s.label"
        class="rounded-2xl border border-studio-border bg-studio-card p-5"
      >
        <div class="flex items-start justify-between">
          <div>
            <p class="text-xs font-medium uppercase tracking-wide text-studio-muted">{{ s.label }}</p>
            <p class="mt-2 text-2xl font-semibold text-studio-fg">{{ s.value }}</p>
          </div>
          <div
            class="flex h-10 w-10 items-center justify-center rounded-xl bg-studio-elevated text-studio-muted"
          >
            <svg v-if="s.icon === 'stack'" class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-width="1.75" d="M4 8l8 4 8-4M4 8v8l8 4 8-4V8M4 16l8 4 8-4" stroke-linejoin="round" />
            </svg>
            <svg v-else-if="s.icon === 'pulse'" class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-width="1.75" d="M9 12h6m-6 4h3M8 4h8l2 10H6L8 4z" />
            </svg>
            <svg v-else-if="s.icon === 'clipboard'" class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-width="1.75"
                d="M9 5h6l1 2h3v12H5V7h3l1-2zM9 5v2h6V5"
                stroke-linejoin="round"
              />
            </svg>
            <svg v-else class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-width="1.75" d="M4 19V5M8 17V9m4 10V7m4 12v-6m4 6V11" />
            </svg>
          </div>
        </div>
      </div>
    </div>

    <!-- 资产表格 -->
    <section class="rounded-2xl border border-studio-border bg-studio-card p-5">
      <h2 class="text-sm font-semibold text-studio-fg">资产详情</h2>
      <p v-if="errorMsg" class="mt-3 rounded-xl border border-rose-500/30 bg-rose-500/10 px-4 py-3 text-sm text-rose-400">
        {{ errorMsg }}
      </p>
      <div class="mt-4 overflow-x-auto">
        <table class="w-full min-w-[1020px] border-collapse text-left text-sm">
          <thead>
            <tr class="text-xs text-studio-muted">
              <th class="pb-3 pl-2 pr-4 font-medium">内容</th>
              <th class="pb-3 px-3 font-medium">类型</th>
              <th class="pb-3 px-3 font-medium">连载</th>
              <th class="pb-3 px-3 font-medium">集数</th>
              <th class="pb-3 px-3 font-medium whitespace-nowrap">审核状态</th>
              <th class="pb-3 pr-2 pl-3 font-medium w-24 text-right">操作</th>
            </tr>
          </thead>
          <tbody class="text-studio-fg-muted">
            <tr
              v-for="row in list"
              :key="rowId(row)"
              class=""
            >
              <td class="py-4 pl-2 pr-4">
                <div class="flex gap-3">
                  <div
                    class="relative h-[72px] w-[120px] shrink-0 overflow-hidden rounded-lg bg-studio-elevated ring-1 ring-studio-border"
                  >
                    <img
                      :src="pick(row, ['cover', 'Cover'], '')"
                      alt=""
                      class="absolute inset-0 h-full w-full object-cover"
                    />
                  </div>
                  <div class="min-w-0 py-0.5">
                    <p class="font-medium text-studio-fg line-clamp-2">{{ pick(row, ['title', 'Title'], '') }}</p>
                    <p class="mt-1 text-xs text-studio-muted">
                      ID: {{ pick(row, ['pgc_id', 'PGCID'], '-') }}
                    </p>
                  </div>
                </div>
              </td>
              <td class="px-3 py-4 align-middle">
                <span
                  class="inline-flex rounded-full border border-cyan-500/30 bg-cyan-500/10 px-2.5 py-0.5 text-xs font-medium text-cyan-300"
                >
                  {{ pgcTypeLabel(Number(pick(row, ['pgc_type', 'PGCType'], 0))) }}
                </span>
              </td>
              <td class="px-3 py-4 align-middle">
                {{ pick(row, ['is_ongoing', 'IsOngoing'], false) ? '是' : '否' }}
              </td>
              <td class="px-3 py-4 align-middle">
                {{ pick(row, ['current_episodes', 'CurrentEpisodes'], 0) }}/{{ pick(row, ['total_episodes', 'TotalEpisodes'], 0) }}
              </td>
              <td class="px-3 py-4 align-middle">
                <span
                  class="inline-flex rounded-full border px-2.5 py-0.5 text-xs font-medium"
                  :class="auditStatusForRow(row).pillClass"
                >
                  {{ auditStatusForRow(row).label }}
                </span>
              </td>
              <td class="py-4 pr-2 pl-3 text-right align-middle">
                <div class="inline-flex items-center justify-end gap-1 align-middle">
                  <RouterLink
                    :to="`/content/${rowId(row)}/edit`"
                    class="inline-flex h-8 w-8 items-center justify-center rounded-lg text-studio-muted transition hover:bg-studio-elevated/70 hover:text-cyan-300"
                    title="编辑"
                  >
                    <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path
                        stroke-linecap="round"
                        stroke-width="1.75"
                        d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L7.5 21H3v-4.5L16.732 3.732z"
                      />
                    </svg>
                  </RouterLink>

                  <div class="relative inline-flex" data-action-menu-root>
                    <button
                      type="button"
                      class="inline-flex h-8 w-8 items-center justify-center rounded-lg text-studio-muted transition hover:bg-studio-elevated/70 hover:text-studio-fg-muted"
                      title="更多操作"
                      @click.stop="toggleActionMenu(rowId(row), $event.currentTarget as HTMLElement)"
                    >
                      <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-width="2" d="M12 5v.01M12 12v.01M12 19v.01" stroke-linecap="round" />
                      </svg>
                    </button>
                  </div>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="mt-4 flex flex-col items-center justify-between gap-3 pt-4 sm:flex-row">
        <p class="text-xs text-studio-muted">
          {{ loading ? '加载中…' : `共 ${total} 条，第 ${page} 页` }}
        </p>
        <div class="flex items-center gap-2">
          <button
            type="button"
            class="rounded-lg border border-studio-border bg-studio-card px-3 py-1 text-xs text-studio-fg transition hover:bg-studio-elevated disabled:opacity-50"
            :disabled="loading || page <= 1"
            @click="prevPage"
          >
            上一页
          </button>
          <button
            type="button"
            class="rounded-lg border border-studio-border bg-studio-card px-3 py-1 text-xs text-studio-fg transition hover:bg-studio-elevated disabled:opacity-50"
            :disabled="loading || page >= Math.ceil(total / PAGE_SIZE)"
            @click="nextPage"
          >
            下一页
          </button>
        </div>
      </div>
    </section>
  </div>

  <teleport to="body">
    <div
      v-if="actionMenuOpenId"
      data-action-menu-root
      class="fixed z-[9999] w-40 overflow-hidden rounded-xl border border-studio-border/80 bg-[#0a1426]/95 shadow-[0_8px_24px_rgba(0,0,0,0.45)] backdrop-blur-sm"
      :style="{ top: `${actionMenuPos.top}px`, left: `${actionMenuPos.left}px` }"
    >
      <div class="px-3 py-1.5 text-[11px] text-studio-muted/80">操作</div>
      <button
        type="button"
        class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-studio-fg-subtle transition hover:bg-studio-elevated/60 hover:text-amber-200"
        @click="toggleOffline(list.find((r) => rowId(r) === actionMenuOpenId))"
      >
        <span
          class="inline-flex h-2 w-2 rounded-full"
          :class="
            isOffline(list.find((r) => rowId(r) === actionMenuOpenId))
              ? 'bg-emerald-300'
              : 'bg-amber-300'
          "
        />
        {{
          isOffline(list.find((r) => rowId(r) === actionMenuOpenId))
            ? '上架'
            : '下架'
        }}
      </button>
      <button
        type="button"
        class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-rose-300 transition hover:bg-studio-elevated/60 hover:text-rose-200"
        @click="deleteRow(list.find((r) => rowId(r) === actionMenuOpenId))"
      >
        删除
      </button>
    </div>
  </teleport>
</template>
