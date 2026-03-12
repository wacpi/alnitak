<template>
  <div class="user-manage">
    <n-card class="user-card" :bordered="false">
      <div class="user-card-content">
        <n-tabs type="line" v-model:value="activeTab" @update:value="handleTabChange">
          <n-tab name="published">已发布</n-tab>
          <n-tab name="failed">
            处理失败
            <n-badge v-if="failedCount > 0" :value="failedCount" :max="99"
              style="margin-left: 6px;" />
          </n-tab>
        </n-tabs>
        <n-space class="search-bar" justify="space-between">
          <n-space align="center" :size="18">
            <n-button :disabled="loading" size="small" type="primary" @click="getTableData">
              <n-icon>
                <refresh></refresh>
              </n-icon>
            </n-button>
            <n-button v-if="activeTab === 'failed'" :disabled="loading || tableData.length === 0"
              size="small" type="warning" @click="reTranscodeAll">
              全部重新转码
            </n-button>
          </n-space>
        </n-space>
        <n-data-table class="table" remote :columns="currentColumns" :data="tableData" :loading="loading"
          :pagination="pagination" flex-height />
        <table-action-drawer v-model:visible="visibleDrawer" :data="editData!"></table-action-drawer>
      </div>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { h, onBeforeMount, reactive, ref, computed } from 'vue';
import { Refresh } from "@vicons/ionicons5";
import useLoading from '@/hooks/loading-hooks';
import { statusCode } from '@/utils/status-code';
import { getVideoListAPI, getFailedVideoListAPI, deleteVideoAPI, reTranscodeVideoAPI, getReviewResourceListAPI } from '@/api/video';
import type { DataTableColumns } from 'naive-ui';
import { getResourceUrl } from '@/utils/resource';
import usePartition from '@/hooks/partition-hooks';
import TableActionDrawer from './components/table-action-drawer.vue';
import { NCard, NImage, NIcon, NButton, NDataTable, NPopconfirm, NSpace, NTabs, NTab, NBadge, useMessage, useDialog } from 'naive-ui';

const { loading, startLoading, endLoading } = useLoading(false);
const { getPartition, getPartitionName } = usePartition("video");

const message = useMessage();
const dialog = useDialog();

const activeTab = ref('published');
const failedCount = ref(0);

const visibleDrawer = ref(false);
const openDrawer = () => {
  visibleDrawer.value = true;
}

// 编辑视频
const editData = ref<VideoType>();
const editVideo = (row: VideoType) => {
  editData.value = row;
  openDrawer();
}

// 删除视频
const deleteVideo = async (row: VideoType) => {
  const res = await deleteVideoAPI(row.vid);
  if (res.data.code === statusCode.OK) {
    message.success('删除成功');
    await getTableData();
    if (activeTab.value !== 'failed') await fetchFailedCount();
  } else {
    message.error(res.data.msg);
  }
}

// 重新转码视频
const reTranscodeVideo = async (row: VideoType) => {
  // 多分P：尽量对每个资源都提交一次重转码（后端若支持 resourceId 参数即可全覆盖）
  try {
    const listRes = await getReviewResourceListAPI(row.vid);
    const resources = (listRes.data.code === statusCode.OK && listRes.data.data?.resources) ? listRes.data.data.resources : [];

    if (resources.length > 0) {
      let success = 0;
      let fail = 0;
      for (const r of resources) {
        const res = await reTranscodeVideoAPI(row.vid, r.id);
        if (res.data.code === statusCode.OK) success++;
        else fail++;
      }
      message.success(`重新转码任务已提交（${success}${fail ? ` 成功，${fail} 失败` : ' 成功'}）`);
    } else {
      const res = await reTranscodeVideoAPI(row.vid);
      if (res.data.code === statusCode.OK) {
        message.success('重新转码任务已提交');
      } else {
        message.error(res.data.msg);
      }
    }

    if (activeTab.value === 'failed') await getTableData();
  } catch (e: any) {
    // 兜底：资源列表拉取失败时仍尝试按 vid 提交
    const res = await reTranscodeVideoAPI(row.vid);
    if (res.data.code === statusCode.OK) {
      message.success('重新转码任务已提交');
      if (activeTab.value === 'failed') await getTableData();
    } else {
      message.error(res.data.msg);
    }
  }
}

// 全部重新转码
const reTranscodeAll = () => {
  dialog.warning({
    title: '确认',
    content: `确定要对全部 ${pagination.itemCount} 个失败视频重新转码吗？`,
    positiveText: '确定',
    negativeText: '取消',
    onPositiveClick: async () => {
      let successCount = 0;
      let failCount = 0;
      for (const row of tableData.value) {
        try {
          const listRes = await getReviewResourceListAPI(row.vid);
          const resources = (listRes.data.code === statusCode.OK && listRes.data.data?.resources) ? listRes.data.data.resources : [];
          if (resources.length > 0) {
            for (const r of resources) {
              const res = await reTranscodeVideoAPI(row.vid, r.id);
              if (res.data.code === statusCode.OK) successCount++;
              else failCount++;
            }
          } else {
            const res = await reTranscodeVideoAPI(row.vid);
            if (res.data.code === statusCode.OK) successCount++;
            else failCount++;
          }
        } catch {
          const res = await reTranscodeVideoAPI(row.vid);
          if (res.data.code === statusCode.OK) successCount++;
          else failCount++;
        }
      }
      message.success(`已提交 ${successCount} 个转码任务${failCount > 0 ? `，${failCount} 个失败` : ''}`);
      await getTableData();
    }
  });
}

// 已发布列
const publishedColumns: DataTableColumns<VideoType> = [
  {
    key: 'vid',
    title: 'ID',
    width: 90,
    align: 'center'
  },
  {
    key: 'avatar',
    title: '封面',
    align: 'center',
    width: 80,
    render: row => {
      return h(NImage, {
        src: getResourceUrl(row.cover),
        width: 60,
        height: 32,
      })
    }
  },
  {
    key: 'title',
    title: '标题',
    align: 'center'
  },
  {
    key: 'desc',
    title: '简介',
    align: 'center',
  },
  {
    key: 'partition',
    title: '分区',
    align: 'center',
    render: row => {
      return getPartitionName(row.partitionId)
    }
  },
  {
    key: 'clicks',
    title: '播放量',
    align: 'center'
  },
  {
    key: 'actions',
    title: '操作',
    align: 'center',
    width: 240,
    render: row => {
      return h(NSpace, { justify: 'center' }, {
        default: () => [
          h(NButton, {
            size: 'small',
            onClick: () => editVideo(row)
          }, { default: () => '详情' }),
          h(NButton, {
            size: 'small',
            type: 'warning',
            onClick: () => reTranscodeVideo(row)
          }, { default: () => '转码' }),
          h(NPopconfirm, {
            onPositiveClick: () => deleteVideo(row),
          }, {
            default: () => '是否删除视频?',
            trigger: () => h(NButton, {
              size: 'small',
              type: 'error',
            }, { default: () => '删除' })
          })
        ]
      })
    }
  }
]

// 失败列
const failedColumns: DataTableColumns<VideoType> = [
  {
    key: 'vid',
    title: 'ID',
    width: 70,
    align: 'center'
  },
  {
    key: 'avatar',
    title: '封面',
    align: 'center',
    width: 80,
    render: row => {
      return h(NImage, {
        src: getResourceUrl(row.cover),
        width: 60,
        height: 32,
      })
    }
  },
  {
    key: 'title',
    title: '标题',
    align: 'center',
    ellipsis: { tooltip: true }
  },
  {
    key: 'author',
    title: '上传者',
    align: 'center',
    width: 120,
    render: row => {
      return row.author?.name || '-';
    }
  },
  {
    key: 'createdAt',
    title: '创建时间',
    align: 'center',
    width: 170,
    render: row => {
      return new Date(row.createdAt).toLocaleString();
    }
  },
  {
    key: 'actions',
    title: '操作',
    align: 'center',
    width: 180,
    render: row => {
      return h(NSpace, { justify: 'center' }, {
        default: () => [
          h(NButton, {
            size: 'small',
            type: 'warning',
            onClick: () => reTranscodeVideo(row)
          }, { default: () => '重新转码' }),
          h(NPopconfirm, {
            onPositiveClick: () => deleteVideo(row),
          }, {
            default: () => '是否删除视频?',
            trigger: () => h(NButton, {
              size: 'small',
              type: 'error',
            }, { default: () => '删除' })
          })
        ]
      })
    }
  }
]

const currentColumns = computed(() => {
  return activeTab.value === 'published' ? publishedColumns : failedColumns;
});

const tableData = ref<VideoType[]>([]);
const getTableData = async () => {
  startLoading();
  const page = pagination.page || 1;
  const pageSize = pagination.pageSize || 1;

  const api = activeTab.value === 'published' ? getVideoListAPI : getFailedVideoListAPI;
  const res = await api({ page, pageSize });
  if (res.data.code === statusCode.OK) {
    tableData.value = res.data.data.list || [];
    pagination.itemCount = res.data.data.total;
    if (activeTab.value === 'failed') {
      failedCount.value = res.data.data.total;
    }
  }
  endLoading();
}

// 获取失败数量（用于badge显示）
const fetchFailedCount = async () => {
  const res = await getFailedVideoListAPI({ page: 1, pageSize: 1 });
  if (res.data.code === statusCode.OK) {
    failedCount.value = res.data.data.total;
  }
}

const handleTabChange = () => {
  pagination.page = 1;
  getTableData();
}

const pagination = reactive({
  page: 1,
  pageSize: 10,
  itemCount: 0,
  showSizePicker: true,
  pageSizes: [10, 15, 20, 25, 30],
  onChange: (page: number) => {
    pagination.page = page;
    getTableData();
  },
  onUpdatePageSize: (pageSize: number) => {
    pagination.pageSize = pageSize;
    pagination.page = 1;
    getTableData();
  }
});

onBeforeMount(async () => {
  await getPartition();
  await getTableData();
  await fetchFailedCount();
})
</script>

<style lang="scss" scoped>
.user-manage {
  height: 100%;

  .user-card {
    height: 100%;

    .user-card-content {
      height: 100%;
      display: flex;
      flex-direction: column;

      .search-bar {
        padding: 12px 0;
      }

      .table {
        flex: 1;
      }
    }
  }
}
</style>
