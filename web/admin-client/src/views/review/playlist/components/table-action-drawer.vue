<template>
  <n-drawer v-model:show="drawerVisible" :width="500">
    <n-drawer-content title="合集详情">
      <n-form v-if="data" label-placement="top">
        <n-grid :cols="24" :x-gap="18">
          <n-form-item-grid-item :span="12" label="作者ID">{{ data.author?.uid }}</n-form-item-grid-item>
          <n-form-item-grid-item :span="12" label="用户名">{{ data.author?.name }}</n-form-item-grid-item>
          <n-form-item-grid-item :span="12" label="创建时间">{{ formatTime(data.createdAt) }}</n-form-item-grid-item>
          <n-form-item-grid-item :span="12" label="视频数量">{{ data.videoCount }}</n-form-item-grid-item>
          <n-form-item-grid-item :span="24" label="合集标题">{{ data.title }}</n-form-item-grid-item>
          <n-form-item-grid-item :span="24" label="合集简介">{{ data.desc || '暂无' }}</n-form-item-grid-item>
          <n-form-item-grid-item v-if="data.cover" :span="24" label="封面">
            <n-image :src="getResourceUrl(data.cover)" :width="200" />
          </n-form-item-grid-item>
        </n-grid>
      </n-form>

      <div class="video-box">
        <span>合集视频列表</span>
        <n-scrollbar style="max-height: 300px;">
          <div class="video-item" v-for="(item, index) in videoList">
            <div class="item-left">
              <span>P{{ index + 1 }} {{ item.title }}</span>
            </div>
          </div>
          <div v-if="videoList.length === 0" style="padding: 10px; color: #999;">暂无视频</div>
        </n-scrollbar>
      </div>
      <template #footer>
        <n-button class="btn" @click="openModal">不通过</n-button>
        <n-button class="btn" type="primary" @click="reviewApproved">通过</n-button>
      </template>
    </n-drawer-content>
    <review-modal v-model:visible="visibleModal" :playlist-id="props.data?.id"
      @finish="reviewFinish"></review-modal>
  </n-drawer>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { formatTime } from '@/utils/format';
import { statusCode } from '@/utils/status-code';
import { reviewPlaylistApprovedAPI, getPlaylistVideoListAPI } from "@/api/playlist";
import ReviewModal from './review-modal.vue';
import { getResourceUrl } from '@/utils/resource';
import { NButton, NDrawer, NDrawerContent, NScrollbar, NForm, NGrid, NFormItemGridItem, NImage } from "naive-ui";

const emit = defineEmits(['update:visible', 'finish']);
const props = withDefaults(defineProps<{
  visible: boolean;
  data: PlaylistType;
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

const videoList = ref<PlaylistVideoType[]>([]);

const getVideoList = async (id: number) => {
  const res = await getPlaylistVideoListAPI(id, 1, 200);
  if (res.data.code === statusCode.OK) {
    videoList.value = res.data.data.videos || [];
  } else {
    videoList.value = [];
  }
}

const reviewApproved = async () => {
  const res = await reviewPlaylistApprovedAPI({ id: props.data.id });
  if (res.data.code === statusCode.OK) {
    reviewFinish();
  }
}

const reviewFinish = () => {
  drawerVisible.value = false;
  emit("finish");
}

watch(() => props.visible, (newVal) => {
  if (newVal && props.data) {
    getVideoList(props.data.id);
  }
})
</script>

<style lang="scss" scoped>
.video-box {
  width: 100%;
  padding-bottom: 30px;

  .video-item {
    height: 36px;
    font-size: 14px;
    padding: 0 10px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    border-radius: 3px;
    border: 1px solid #efeff5;
    margin: 10px 0;

    .item-left {
      cursor: pointer;
    }
  }
}

.btn {
  width: 100px;
  margin-left: 10px;
}
</style>
