<template>
  <n-drawer v-model:show="drawerVisible" :width="520">
    <n-drawer-content title="PGC 详情">
      <n-form v-if="data" label-placement="top">
        <n-grid :cols="24" :x-gap="18">
          <n-form-item-grid-item :span="12" label="PGC ID">{{ data.pgc_id }}</n-form-item-grid-item>
          <n-form-item-grid-item :span="12" label="类型">{{ pgcTypeLabel(data.pgc_type) }}</n-form-item-grid-item>
          <n-form-item-grid-item :span="12" label="状态">{{ statusLabel(data.status) }}</n-form-item-grid-item>
          <n-form-item-grid-item v-if="data.created_at" :span="12" label="创建时间">{{ data.created_at }}</n-form-item-grid-item>
          <n-form-item-grid-item :span="24" label="标题">{{ data.title }}</n-form-item-grid-item>
          <n-form-item-grid-item :span="24" label="简介">{{ data.desc || '暂无' }}</n-form-item-grid-item>
          <n-form-item-grid-item v-if="data.cover" :span="24" label="封面">
            <n-image :src="getResourceUrl(data.cover)" :width="200" />
          </n-form-item-grid-item>
        </n-grid>
      </n-form>
      <template #footer>
        <n-button class="btn" @click="openModal">不通过</n-button>
        <n-button class="btn" type="primary" @click="reviewApproved">通过</n-button>
      </template>
    </n-drawer-content>
    <review-modal v-if="props.data" v-model:visible="visibleModal" :pgc-id="props.data.pgc_id"
      @finish="reviewFinish"></review-modal>
  </n-drawer>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { statusCode } from '@/utils/status-code';
import { reviewPGCApprovedAPI } from "@/api/pgc";
import ReviewModal from './review-modal.vue';
import { getResourceUrl } from '@/utils/resource';
import { NButton, NDrawer, NDrawerContent, NForm, NGrid, NFormItemGridItem, NImage } from "naive-ui";

const emit = defineEmits(['update:visible', 'finish']);
const props = withDefaults(defineProps<{
  visible: boolean;
  data: PGCReviewRow;
}>(), {
  visible: false,
})

const drawerVisible = computed({
  get() {
    return props.visible;
  },
  set(visible) {
    emit('update:visible', visible);
  }
});

const visibleModal = ref(false);
const openModal = () => {
  visibleModal.value = true;
}

const pgcTypeLabel = (t: number) => {
  const m: Record<number, string> = {
    1: '国创',
    2: '日漫',
    3: '纪录片',
    4: '电影',
    5: '电视剧',
  };
  return m[t] ?? `类型${t}`;
};

const statusLabel = (s: number) => {
  if (s === -1) return '已下架';
  const m: Record<number, string> = {
    0: '草稿',
    100: '已提交',
    200: '审核中',
    300: '已通过',
    400: '已驳回',
  };
  return m[s] ?? String(s);
};

const reviewApproved = async () => {
  if (!props.data) return;
  const res = await reviewPGCApprovedAPI({ pgc_id: props.data.pgc_id });
  if (res.data.code === statusCode.OK) {
    reviewFinish();
  }
}

const reviewFinish = () => {
  drawerVisible.value = false;
  emit("finish");
}
</script>

<style lang="scss" scoped>
.btn {
  margin-left: 8px;
}
</style>
