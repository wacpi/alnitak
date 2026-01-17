<template>
  <n-drawer v-model:show="drawerVisible" :width="500">
    <n-drawer-content title="视频详情">
      <n-form v-if="data" label-placement="top">
        <n-grid :cols="24" :x-gap="18">
          <n-form-item-grid-item :span="12" label="作者ID">{{ data.author.uid }}</n-form-item-grid-item>
          <n-form-item-grid-item :span="12" label="用户名">{{ data.author.name }}</n-form-item-grid-item>
          <n-form-item-grid-item :span="12" label="上传时间">{{ formatTime(data.createdAt) }}</n-form-item-grid-item>
          <n-form-item-grid-item :span="24" label="视频标签">
            <n-tag class="tag" v-for="item in  data.tags.split(',')">{{ item }}</n-tag>
          </n-form-item-grid-item>
        </n-grid>
      </n-form>

      <!-- 关联稿件提示（全局去重功能） -->
      <div v-if="relatedResourcesList.length > 0" class="related-box">
        <n-alert type="warning" title="发现相同文件稿件">
          <div>该视频文件已被其他用户上传过，请审核是否为搬运/重复投稿：</div>
          <div class="related-list">
            <div v-for="related in relatedResourcesList" :key="related.resourceId" class="related-item">
              <div v-for="item in related.relatedResources" :key="item.resourceId" class="related-detail">
                <span class="related-info">
                  VID:{{ item.vid }} | 作者:{{ item.authorName }}(UID:{{ item.uid }}) |
                  状态:{{ getStatusText(item.status) }} | {{ formatTime(item.createdAt) }}
                </span>
              </div>
            </div>
          </div>
        </n-alert>
      </div>

      <div class="video-box">
        <span>视频列表</span>
        <n-scrollbar style="max-height: 300px;">
          <div class="video-item" v-for="(item, index) in resourceList">
            <div class="item-left">
              <span>P{{ index + 1 }} {{ item.title }}</span>
              <n-tag v-if="item.fileId && hasRelated(item.fileId)" size="small" type="warning" style="margin-left: 8px;">有关联</n-tag>
            </div>
            <div class="item-right">
              <n-button text @click="playVideo(item)">查看</n-button>
            </div>
          </div>
        </n-scrollbar>
      </div>
      <template #footer>
        <n-button class="btn" @click="openModal">不通过</n-button>
        <n-button class="btn" type="primary" @click="reviewVideoApproved">通过</n-button>
      </template>
    </n-drawer-content>
    <review-modal v-model:visible="visibleModal" :vid="props.data.vid" :video-count="resourceList.length"
      @finish="reviewFinish"></review-modal>
    <video-modal v-model:visible="visibleVideoModal" :resource-id="currentResourceId"></video-modal>
  </n-drawer>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { formatTime } from '@/utils/format';
import { getReviewResourceListAPI } from "@/api/video";
import { statusCode } from '@/utils/status-code';
import { reviewVideoApprovedAPI } from "@/api/review";
import ReviewModal from './review-modal.vue';
import VideoModal from './video-modal.vue';
import { NButton, NTag, NDrawer, NDrawerContent, NScrollbar, NForm, NGrid, NFormItemGridItem, NAlert } from "naive-ui";

// 状态码映射
const statusMap: Record<number, string> = {
  0: '已通过',
  200: '转码中',
  500: '待审核',
  2000: '未通过',
  3000: '处理失败',
};
const getStatusText = (status: number) => statusMap[status] || `未知(${status})`;

const emit = defineEmits(['update:visible', 'finish']);
const props = withDefaults(defineProps<{
  visible: boolean; //弹窗可见性
  data: VideoType;
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

const resourceList = ref<ResourceType[]>([]);
const relatedResourcesList = ref<any[]>([]);

const getReviewResourceList = async (vid: number) => {
  const res = await getReviewResourceListAPI(vid);
  if (res.data.code === statusCode.OK) {
    if (res.data.data.resources) {
      resourceList.value = res.data.data.resources;
    } else {
      resourceList.value = [];
    }
    // 获取关联稿件信息
    if (res.data.data.relatedResources) {
      relatedResourcesList.value = res.data.data.relatedResources;
    } else {
      relatedResourcesList.value = [];
    }
  }
}

// 检查某个fileId是否有关联稿件
const hasRelated = (fileId: number) => {
  return relatedResourcesList.value.some(r => r.fileId === fileId);
}

const currentResourceId = ref(0);
const visibleVideoModal = ref(false);
const playVideo = (r: ResourceType) => {
  currentResourceId.value = r.id;
  visibleVideoModal.value = true;
}

const reviewVideoApproved = async () => {
  const res = await reviewVideoApprovedAPI({ vid: props.data.vid });
  if (res.data.code === statusCode.OK) {
    reviewFinish();
  }
}

const reviewFinish = () => {
  drawerVisible.value = false;
  emit("finish");
}

watch(() => props.visible, (newVal) => {
  if (newVal) {
    if (props.data) {
      getReviewResourceList(props.data.vid);
    }
  }
})
</script>

<style lang="scss" scoped>
.tag {
  margin-right: 10px;
}

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

.related-box {
  margin-bottom: 16px;

  .related-list {
    margin-top: 8px;
  }

  .related-detail {
    padding: 4px 0;
    font-size: 12px;
    border-bottom: 1px dashed #e0e0e0;

    &:last-child {
      border-bottom: none;
    }
  }

  .related-info {
    color: #666;
  }
}
</style>
