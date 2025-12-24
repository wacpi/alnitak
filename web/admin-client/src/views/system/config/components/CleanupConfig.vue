<template>
  <div class="cleanup">
    <n-alert type="info" style="margin-bottom: 20px;">
      <template #header>资源清理说明</template>
      <ul style="margin: 0; padding-left: 20px;">
        <li>清理软删除超过30天的视频文件和图片文件</li>
        <li>清理磁盘上存在但数据库中无引用的孤立文件</li>
        <li>同时清理本地文件和OSS上的文件</li>
        <li>建议先预览再执行，避免误删</li>
      </ul>
    </n-alert>

    <n-space vertical>
      <n-space>
        <n-button type="info" @click="getPreview" :loading="previewLoading">
          预览待清理资源
        </n-button>
        <n-button type="error" @click="handleCleanup" :loading="cleanupLoading" :disabled="!hasPreview">
          执行清理
        </n-button>
      </n-space>

      <n-card v-if="hasPreview" title="清理预览" size="small" style="margin-top: 20px;">
        <n-descriptions :column="2" label-placement="left" bordered>
          <n-descriptions-item label="待清理视频目录">
            <n-tag :type="result.cleanedVideoDirs > 0 ? 'warning' : 'success'">
              {{ result.cleanedVideoDirs }} 个
            </n-tag>
          </n-descriptions-item>
          <n-descriptions-item label="待清理图片文件">
            <n-tag :type="result.cleanedImages > 0 ? 'warning' : 'success'">
              {{ result.cleanedImages }} 个
            </n-tag>
          </n-descriptions-item>
          <n-descriptions-item label="待清理视频文件记录">
            <n-tag :type="result.cleanedVideoFiles > 0 ? 'warning' : 'success'">
              {{ result.cleanedVideoFiles }} 条
            </n-tag>
          </n-descriptions-item>
          <n-descriptions-item label="待清理索引文件记录">
            <n-tag :type="result.cleanedIndexFiles > 0 ? 'warning' : 'success'">
              {{ result.cleanedIndexFiles }} 条
            </n-tag>
          </n-descriptions-item>
          <n-descriptions-item label="待清理图片文件记录">
            <n-tag :type="result.cleanedImageFiles > 0 ? 'warning' : 'success'">
              {{ result.cleanedImageFiles }} 条
            </n-tag>
          </n-descriptions-item>
          <n-descriptions-item label="待清理Resource记录">
            <n-tag :type="result.cleanedResources > 0 ? 'warning' : 'success'">
              {{ result.cleanedResources }} 条
            </n-tag>
          </n-descriptions-item>
          <n-descriptions-item label="错误数量">
            <n-tag :type="result.errors.length > 0 ? 'error' : 'success'">
              {{ result.errors.length }} 个
            </n-tag>
          </n-descriptions-item>
        </n-descriptions>

        <!-- 文件列表预览 -->
        <div v-if="result.items && result.items.length > 0" style="margin-top: 16px;">
          <n-divider>待清理文件列表</n-divider>
          <n-data-table
            :columns="itemColumns"
            :data="result.items"
            :max-height="300"
            size="small"
            :bordered="true"
          />
        </div>

        <n-alert v-if="result.errors.length > 0" type="error" title="错误信息" style="margin-top: 16px;">
          <ul style="margin: 0; padding-left: 20px;">
            <li v-for="(err, index) in result.errors" :key="index">{{ err }}</li>
          </ul>
        </n-alert>
      </n-card>

      <n-card v-if="cleanupResult" title="清理结果" size="small" style="margin-top: 20px;">
        <n-result :status="cleanupResult.errors.length > 0 ? 'warning' : 'success'"
                  :title="cleanupResult.errors.length > 0 ? '清理完成（有错误）' : '清理完成'">
          <template #footer>
            <n-descriptions :column="2" label-placement="left" bordered>
              <n-descriptions-item label="已清理视频目录">
                <n-tag type="info">{{ cleanupResult.cleanedVideoDirs }} 个</n-tag>
              </n-descriptions-item>
              <n-descriptions-item label="已清理图片文件">
                <n-tag type="info">{{ cleanupResult.cleanedImages }} 个</n-tag>
              </n-descriptions-item>
              <n-descriptions-item label="已清理视频文件记录">
                <n-tag type="info">{{ cleanupResult.cleanedVideoFiles }} 条</n-tag>
              </n-descriptions-item>
              <n-descriptions-item label="已清理索引文件记录">
                <n-tag type="info">{{ cleanupResult.cleanedIndexFiles }} 条</n-tag>
              </n-descriptions-item>
              <n-descriptions-item label="已清理图片文件记录">
                <n-tag type="info">{{ cleanupResult.cleanedImageFiles }} 条</n-tag>
              </n-descriptions-item>
              <n-descriptions-item label="已清理Resource记录">
                <n-tag type="info">{{ cleanupResult.cleanedResources }} 条</n-tag>
              </n-descriptions-item>
              <n-descriptions-item label="错误数量">
                <n-tag :type="cleanupResult.errors.length > 0 ? 'error' : 'success'">
                  {{ cleanupResult.errors.length }} 个
                </n-tag>
              </n-descriptions-item>
            </n-descriptions>
          </template>
        </n-result>

        <n-alert v-if="cleanupResult.errors.length > 0" type="error" title="错误信息" style="margin-top: 16px;">
          <ul style="margin: 0; padding-left: 20px;">
            <li v-for="(err, index) in cleanupResult.errors" :key="index">{{ err }}</li>
          </ul>
        </n-alert>
      </n-card>
    </n-space>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from "vue";
import { statusCode } from "@/utils/status-code";
import { getCleanupPreviewAPI, executeCleanupAPI } from '@/api/config';
import {
  NAlert, NButton, NSpace, NCard, NDescriptions,
  NDescriptionsItem, NTag, NResult, NDataTable, NDivider,
  useMessage, useDialog
} from "naive-ui";
import type { DataTableColumns } from "naive-ui";

interface CleanupItem {
  type: string;
  path: string;
  reason: string;
}

interface CleanupResultType {
  cleanedVideoDirs: number;
  cleanedImages: number;
  cleanedVideoFiles: number;
  cleanedIndexFiles: number;
  cleanedImageFiles: number;
  cleanedResources: number;
  errors: string[];
  dryRun: boolean;
  items: CleanupItem[];
}

const message = useMessage();
const dialog = useDialog();

const previewLoading = ref(false);
const cleanupLoading = ref(false);

const result = ref<CleanupResultType>({
  cleanedVideoDirs: 0,
  cleanedImages: 0,
  cleanedVideoFiles: 0,
  cleanedIndexFiles: 0,
  cleanedImageFiles: 0,
  cleanedResources: 0,
  errors: [],
  dryRun: true,
  items: []
});

const cleanupResult = ref<CleanupResultType | null>(null);

// 文件列表表格列定义
const itemColumns: DataTableColumns<CleanupItem> = [
  {
    title: '类型',
    key: 'type',
    width: 80,
    render: (row) => row.type === 'video' ? '视频' : '图片'
  },
  {
    title: '路径/文件名',
    key: 'path',
    ellipsis: { tooltip: true }
  },
  {
    title: '清理原因',
    key: 'reason',
    width: 180
  }
];

const hasPreview = computed(() => {
  return result.value.cleanedVideoDirs > 0 ||
         result.value.cleanedImages > 0 ||
         result.value.cleanedVideoFiles > 0 ||
         result.value.cleanedIndexFiles > 0 ||
         result.value.cleanedImageFiles > 0;
});

const getPreview = async () => {
  previewLoading.value = true;
  cleanupResult.value = null;
  try {
    const res = await getCleanupPreviewAPI();
    if (res.data.code === statusCode.OK) {
      result.value = res.data.data.result;
      if (!hasPreview.value) {
        message.success("没有需要清理的资源");
      }
    } else {
      message.error(res.data.msg || "获取预览失败");
    }
  } catch {
    message.error("获取预览失败");
  } finally {
    previewLoading.value = false;
  }
};

const handleCleanup = () => {
  dialog.warning({
    title: '确认清理',
    content: `确定要清理这些资源吗？此操作不可逆！\n\n将清理：\n- ${result.value.cleanedVideoDirs} 个视频目录\n- ${result.value.cleanedImages} 个图片文件`,
    positiveText: '确定清理',
    negativeText: '取消',
    onPositiveClick: executeCleanup
  });
};

const executeCleanup = async () => {
  cleanupLoading.value = true;
  try {
    const res = await executeCleanupAPI();
    if (res.data.code === statusCode.OK) {
      cleanupResult.value = res.data.data.result;
      // 重置预览
      result.value = {
        cleanedVideoDirs: 0,
        cleanedImages: 0,
        cleanedVideoFiles: 0,
        cleanedIndexFiles: 0,
        cleanedImageFiles: 0,
        cleanedResources: 0,
        errors: [],
        dryRun: true,
        items: []
      };
      message.success("清理完成");
    } else {
      message.error(res.data.msg || "清理失败");
    }
  } catch {
    message.error("清理失败");
  } finally {
    cleanupLoading.value = false;
  }
};
</script>

<style lang="scss" scoped>
.cleanup {
  padding: 20px;
}
</style>
