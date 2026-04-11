<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import { createPgcApi, pickPgcIdFromCreateData, type CreatePgcReq, type PgcType } from '@/api/pgc'
import { uploadImageApi } from '@/api/upload'
import { useAuthStore } from '@/stores/auth'
import { toast } from '@/utils/toast'

type CategoryValue = 'cn' | 'jp' | 'documentary' | 'movie' | 'tv'

const categories: { value: CategoryValue; label: string; pgcType: PgcType }[] = [
  { value: 'cn', label: '国创(CN)', pgcType: 1 },
  { value: 'jp', label: '日创(JP)', pgcType: 2 },
  { value: 'documentary', label: '纪录片', pgcType: 3 },
  { value: 'movie', label: '电影', pgcType: 4 },
  { value: 'tv', label: '电视剧', pgcType: 5 },
]

const seriesCategoryValues: CategoryValue[] = ['cn', 'jp', 'tv']

const router = useRouter()
const auth = useAuthStore()

const form = ref({
  projectTitle: '',
  category: 'cn' as CategoryValue,
  coverUrl: '',
  releaseYear: String(new Date().getFullYear()),
  area: '',
  rating: 0,
  synopsis: '',
})

const isSeries = computed(() => seriesCategoryValues.includes(form.value.category))

/** 计划总集数（连载类选填，创建后也可在编辑页修改） */
const plannedTotalEpisodes = ref<number | null>(null)
/** 与「已更新至第几集」配合计算是否连载中（创建时尚无真实剧集时为预估） */
const latestPublishedEpisode = ref<number | null>(0)

watch(isSeries, (series) => {
  if (!series) {
    plannedTotalEpisodes.value = null
    latestPublishedEpisode.value = null
  } else {
    if (latestPublishedEpisode.value == null) latestPublishedEpisode.value = 0
  }
})

const latestEpisodeInvalid = computed(() => {
  if (!isSeries.value) return false
  const total = plannedTotalEpisodes.value ?? 0
  const latest = latestPublishedEpisode.value ?? 0
  return total > 0 && latest > total
})

function discardDraft() {
  if (!confirm('确定放弃当前草稿？未保存内容将丢失。')) return
  form.value = {
    projectTitle: '',
    category: 'cn',
    coverUrl: '',
    releaseYear: String(new Date().getFullYear()),
    area: '',
    rating: 0,
    synopsis: '',
  }
  plannedTotalEpisodes.value = null
  latestPublishedEpisode.value = null
}

const submitting = ref(false)
const submitError = ref('')

const coverUploading = ref(false)
const coverUploadError = ref('')
const coverFileInput = ref<HTMLInputElement | null>(null)

async function onPickCoverFile(e: Event) {
  coverUploadError.value = ''
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (!auth.token) {
    coverUploadError.value = '未登录，无法上传'
    return
  }
  coverUploading.value = true
  try {
    const res = await uploadImageApi(auth.token, file)
    if (res.code !== 200) {
      coverUploadError.value = res.msg || '上传失败'
      return
    }
    const url = (res.data as any)?.url as string | undefined
    if (!url) {
      coverUploadError.value = '上传成功但未返回 url'
      return
    }
    form.value.coverUrl = url
  } catch (err: any) {
    coverUploadError.value = err?.message || '上传失败'
  } finally {
    coverUploading.value = false
    if (coverFileInput.value) coverFileInput.value.value = ''
  }
}

function validateForm() {
  if (!form.value.projectTitle.trim()) return '请填写项目标题'
  if (!form.value.coverUrl.trim()) return '请上传封面或填写封面 URL（后端必填）'
  const year = Number(form.value.releaseYear)
  if (!Number.isFinite(year) || year < 1900 || year > 2100) return '上映/首播年份不合法'
  if (form.value.rating < 0 || form.value.rating > 10) return '评分必须在 0-10 之间'
  if (isSeries.value) {
    const t = plannedTotalEpisodes.value
    if (t != null && (t < 1 || t > 999)) return '计划总集数应在 1～999 之间'
    if (latestEpisodeInvalid.value) return '「已更新至第几集」不能大于计划总集数'
  }
  return ''
}

async function submitCreate() {
  submitError.value = ''
  const m = validateForm()
  if (m) {
    submitError.value = m
    toast('error', m)
    return
  }
  if (!auth.token) {
    submitError.value = '未登录'
    toast('error', '未登录')
    return
  }

  const cat = categories.find((c) => c.value === form.value.category) ?? categories[0]
  const body: CreatePgcReq = {
    pgc_type: cat.pgcType,
    title: form.value.projectTitle.trim(),
    cover: form.value.coverUrl.trim(),
    desc: form.value.synopsis.trim(),
    year: Number(form.value.releaseYear),
    area: form.value.area.trim(),
    rating: Number(form.value.rating) || 0,
    is_ongoing: Boolean(
      isSeries.value &&
        (plannedTotalEpisodes.value ?? 0) > 0 &&
        (latestPublishedEpisode.value ?? 0) < (plannedTotalEpisodes.value ?? 0),
    ),
    episodes: [],
  }
  if (isSeries.value && plannedTotalEpisodes.value != null && plannedTotalEpisodes.value > 0) {
    body.total_episodes = plannedTotalEpisodes.value
  }

  submitting.value = true
  try {
    const res = await createPgcApi(auth.token, body)
    if (res.code !== 200) {
      submitError.value = res.msg || '创建失败'
      toast('error', submitError.value)
      return
    }
    const newPgcId = pickPgcIdFromCreateData(res.data)
    if (!newPgcId) {
      submitError.value = '创建成功但未返回有效 PGC ID（请确认后端 create 接口将 pgc_id 以字符串返回）'
      toast('error', submitError.value)
      return
    }
    toast('success', '创建成功，正在打开编辑页以添加剧集')
    await router.push(`/content/${newPgcId}/edit`)
  } catch (e: any) {
    submitError.value = e?.message || '创建失败'
    toast('error', submitError.value)
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-[1280px]">
    <div class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h2 class="text-xl font-semibold text-studio-fg">创建新资产</h2>
        <p class="mt-1 text-sm text-studio-muted">
          先创建作品主体（元数据）；保存后在编辑页上传视频并逐集添加，流程类似哔哩哔哩「先建 Season、再挂 Ep」。
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-3">
        <button
          type="button"
          class="rounded-xl border border-studio-border bg-studio-card px-4 py-2 text-sm text-studio-fg-muted transition hover:bg-studio-elevated"
          @click="discardDraft"
        >
          放弃草稿
        </button>
        <button
          type="button"
          class="rounded-xl bg-gradient-to-r from-cyan-500 to-cyan-600 px-4 py-2 text-sm font-semibold text-white shadow-lg shadow-cyan-500/20 transition hover:from-cyan-400 hover:to-cyan-500 disabled:opacity-60"
          :disabled="submitting"
          @click="submitCreate"
        >
          {{ submitting ? '创建中…' : '创建内容' }}
        </button>
      </div>
    </div>

    <div class="grid gap-6 lg:grid-cols-[1fr_320px]">
      <div class="space-y-6">
        <section class="rounded-2xl border border-studio-border bg-studio-card p-6">
          <h3 class="text-sm font-semibold text-studio-fg">核心元数据</h3>
          <div v-if="submitError" class="mt-4 rounded-xl border border-rose-500/30 bg-rose-500/10 p-3 text-sm text-rose-400">
            {{ submitError }}
          </div>
          <div class="mt-4 grid gap-4 sm:grid-cols-2">
            <div class="sm:col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-studio-fg-muted">项目标题</label>
              <input
                v-model="form.projectTitle"
                type="text"
                placeholder="输入作品全称（例如：极光之门：觉醒）"
                class="w-full rounded-xl border border-studio-border bg-studio-input px-4 py-2.5 text-sm text-studio-fg placeholder:text-studio-muted focus:border-cyan-500/50 focus:outline-none focus:ring-1 focus:ring-cyan-500/30"
              />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-studio-fg-muted">分类</label>
              <select
                v-model="form.category"
                class="w-full rounded-xl border border-studio-border bg-studio-input px-4 py-2.5 text-sm text-studio-fg focus:border-cyan-500/50 focus:outline-none focus:ring-1 focus:ring-cyan-500/30"
              >
                <option v-for="c in categories" :key="c.value" :value="c.value">
                  {{ c.label }}
                </option>
              </select>
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-studio-fg-muted">上映 / 首播年份</label>
              <input
                v-model="form.releaseYear"
                type="text"
                class="w-full rounded-xl border border-studio-border bg-studio-input px-4 py-2.5 text-sm text-studio-fg focus:border-cyan-500/50 focus:outline-none focus:ring-1 focus:ring-cyan-500/30"
              />
            </div>
            <div class="sm:col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-studio-fg-muted">封面（上传或填写 URL，二选一）</label>
              <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
                <input
                  ref="coverFileInput"
                  type="file"
                  accept="image/*"
                  class="block w-full text-sm text-studio-muted file:mr-3 file:rounded-lg file:border-0 file:bg-studio-elevated file:px-3 file:py-2 file:text-sm file:font-medium file:text-studio-fg hover:file:text-cyan-300"
                  :disabled="coverUploading"
                  @change="onPickCoverFile"
                />
                <span class="text-xs whitespace-nowrap text-studio-muted">
                  {{ coverUploading ? '上传中…' : '或' }}
                </span>
                <input
                  v-model="form.coverUrl"
                  type="text"
                  placeholder="https://... 或 /api/image/..."
                  class="w-full rounded-xl border border-studio-border bg-studio-input px-4 py-2.5 text-sm text-studio-fg placeholder:text-studio-muted focus:border-cyan-500/50 focus:outline-none focus:ring-1 focus:ring-cyan-500/30"
                />
              </div>
              <p v-if="coverUploadError" class="mt-2 text-xs text-rose-400">{{ coverUploadError }}</p>
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-studio-fg-muted">地区（选填）</label>
              <input
                v-model="form.area"
                type="text"
                placeholder="CN / JP / US ..."
                class="w-full rounded-xl border border-studio-border bg-studio-input px-4 py-2.5 text-sm text-studio-fg placeholder:text-studio-muted focus:border-cyan-500/50 focus:outline-none focus:ring-1 focus:ring-cyan-500/30"
              />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-studio-fg-muted">评分（0-10）</label>
              <input
                v-model.number="form.rating"
                type="number"
                min="0"
                max="10"
                step="0.1"
                class="w-full rounded-xl border border-studio-border bg-studio-input px-4 py-2.5 text-sm text-studio-fg focus:border-cyan-500/50 focus:outline-none focus:ring-1 focus:ring-cyan-500/30"
              />
            </div>
            <div class="sm:col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-studio-fg-muted">剧情简介</label>
              <textarea
                v-model="form.synopsis"
                rows="4"
                placeholder="故事梗概、世界观与看点…"
                class="w-full resize-y rounded-xl border border-studio-border bg-studio-input px-4 py-3 text-sm text-studio-fg placeholder:text-studio-muted focus:border-cyan-500/50 focus:outline-none focus:ring-1 focus:ring-cyan-500/30"
              />
            </div>
          </div>
        </section>

        <section v-if="isSeries" class="rounded-2xl border border-cyan-500/20 bg-studio-card/80 p-6">
          <h3 class="text-sm font-semibold text-cyan-400">连载计划（选填）</h3>
          <p class="mt-1 text-xs text-studio-muted">
            仅写入计划总集数与是否连载；单集视频请在创建完成后的「编辑内容」页上传并添加。
          </p>
          <div class="mt-4 grid gap-4 sm:grid-cols-2">
            <div>
              <label class="mb-1.5 block text-xs font-medium text-studio-fg-muted">计划总集数</label>
              <input
                v-model.number="plannedTotalEpisodes"
                type="number"
                min="1"
                max="999"
                placeholder="留空则创建后再在编辑页填写"
                class="w-full rounded-xl border border-studio-border bg-studio-input px-4 py-2.5 text-sm text-studio-fg placeholder:text-studio-muted focus:border-cyan-500/50 focus:outline-none focus:ring-1 focus:ring-cyan-500/30"
              />
            </div>
            <div>
              <label class="mb-1.5 block text-xs font-medium text-studio-fg-muted">已更新至第几集（预估）</label>
              <input
                v-model.number="latestPublishedEpisode"
                type="number"
                min="0"
                :max="plannedTotalEpisodes ?? 999"
                class="w-full rounded-xl border border-studio-border bg-studio-input px-4 py-2.5 text-sm text-studio-fg focus:border-cyan-500/50 focus:outline-none focus:ring-1 focus:ring-cyan-500/30"
              />
              <p v-if="latestEpisodeInvalid" class="mt-1 text-xs text-rose-400">不能大于计划总集数</p>
            </div>
          </div>
        </section>

        <div class="flex justify-end lg:hidden">
          <button
            type="button"
            class="rounded-xl bg-gradient-to-r from-cyan-500 to-cyan-600 px-5 py-2.5 text-sm font-semibold text-white shadow-lg shadow-cyan-500/20 transition hover:from-cyan-400 hover:to-cyan-500 disabled:opacity-60"
            :disabled="submitting"
            @click="submitCreate"
          >
            {{ submitting ? '创建中…' : '创建内容' }}
          </button>
        </div>
      </div>

      <aside class="space-y-4">
        <div class="rounded-2xl border border-studio-border bg-studio-card p-5">
          <p class="text-xs font-medium uppercase tracking-wide text-studio-muted">Preview</p>
          <div class="mt-3 aspect-[2/3] rounded-lg bg-studio-elevated ring-1 ring-studio-border" />
          <p class="mt-3 text-xs text-studio-muted">预览模式</p>
          <p class="mt-1 text-sm font-medium text-studio-fg line-clamp-2">
            {{ form.projectTitle || '项目标题将显示于此' }}
          </p>
        </div>
        <div class="rounded-2xl border border-studio-border bg-studio-card p-5">
          <h4 class="text-sm font-semibold text-studio-fg">提示</h4>
          <p class="mt-3 text-xs leading-relaxed text-studio-muted">
            创建成功后自动进入编辑页，在「剧集管理」中上传视频并添加每一集；总集数也可在编辑页随时调整。
          </p>
        </div>
      </aside>
    </div>
  </div>
</template>
