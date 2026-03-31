<template>
  <div class="user-manage">
    <n-card class="user-card" :bordered="false">
      <div class="user-card-content">
        <n-space class="search-bar" justify="space-between">
          <n-space align="center" :size="18">
            <n-button :disabled="loading" size="small" type="primary" @click="getTableData">
              <n-icon>
                <refresh></refresh>
              </n-icon>
            </n-button>
          </n-space>
        </n-space>
        <n-data-table class="table" remote :columns="columns" :data="tableData" :loading="loading"
          :row-key="rowKey"
          :max-height="tableMaxHeight"
          scroll-x="960"
          :pagination="pagination" />
        <table-action-drawer v-model:visible="visibleDrawer" :data="detailsData!"
          @finish="reviewFinish"></table-action-drawer>
      </div>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { computed, h, onBeforeMount, reactive, ref } from 'vue';
import useSystemStore from '@/stores/modules/system-store';
import { Refresh } from "@vicons/ionicons5";
import useLoading from '@/hooks/loading-hooks';
import { statusCode } from '@/utils/status-code';
import { getPGCReviewListAPI } from '@/api/pgc';
import type { DataTableColumns } from 'naive-ui';
import { getResourceUrl } from '@/utils/resource';
import TableActionDrawer from './components/table-action-drawer.vue';
import { NCard, NImage, NIcon, NButton, NDataTable, NSpace, useMessage } from 'naive-ui';

const { loading, startLoading, endLoading } = useLoading(false);
const message = useMessage();
const systemStore = useSystemStore();

/** flex-height 在 layout 嵌套 flex 下易出现表体高度为 0；改用 max-height + 横向滚动 */
const tableMaxHeight = computed(() => {
  const h = systemStore.contentBorderBoxHeight;
  if (typeof h === 'number' && h > 160) {
    return Math.max(h - 130, 240);
  }
  return Math.min(window.innerHeight - 200, 720);
});

const rowKey = (row: PGCReviewRow) =>
  row.pgc_id != null && row.pgc_id !== '' ? String(row.pgc_id) : `id:${row.id ?? ''}`;

const visibleDrawer = ref(false);
const openDrawer = () => {
  visibleDrawer.value = true;
}

const detailsData = ref<PGCReviewRow>();
const viewDetails = (row: PGCReviewRow) => {
  detailsData.value = row;
  openDrawer();
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

const columns: DataTableColumns<PGCReviewRow> = [
  {
    key: 'pgc_id',
    title: 'PGC ID',
    width: 160,
    align: 'center'
  },
  {
    key: 'cover',
    title: '封面',
    align: 'center',
    width: 80,
    render: row => {
      if (!row.cover) return '-';
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
    key: 'pgc_type',
    title: '类型',
    width: 100,
    align: 'center',
    render: row => pgcTypeLabel(row.pgc_type),
  },
  {
    key: 'desc',
    title: '简介',
    align: 'center',
  },
  {
    key: 'actions',
    title: '操作',
    align: 'center',
    width: 120,
    render: row => {
      return h(NSpace, { justify: 'center' }, {
        default: () => [
          h(NButton, {
            size: 'small',
            type: 'primary',
            onClick: () => viewDetails(row)
          }, { default: () => '详情' }),
        ]
      })
    }
  }
]

const tableData = ref<PGCReviewRow[]>([]);
const getTableData = async () => {
  startLoading();
  try {
    const page = pagination.page || 1;
    const pageSize = pagination.pageSize || 10;
    const res = await getPGCReviewListAPI({ page, pageSize });
    const code = Number(res.data?.code);
    const payload = res.data?.data as { list?: PGCReviewRow[]; total?: number } | undefined;
    if (code === statusCode.OK && payload) {
      tableData.value = Array.isArray(payload.list) ? payload.list : [];
      pagination.itemCount = payload.total ?? 0;
      return;
    }
    tableData.value = [];
    pagination.itemCount = 0;
    message.error((res.data as { msg?: string })?.msg || '获取 PGC 待审列表失败');
  } catch {
    tableData.value = [];
    pagination.itemCount = 0;
    message.error('获取 PGC 待审列表失败');
  } finally {
    endLoading();
  }
}

const reviewFinish = () => {
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
  await getTableData();
})
</script>

<style lang="scss" scoped>
.user-manage {
  height: 100%;

  .user-card {
    border-radius: 0;

    :deep(.n-card__content) {
      height: calc(100vh - 130px);
      padding-right: 6px;

      .user-card-content {
        height: 100%;
        display: flex;
        flex-direction: column;

        .search-bar {
          margin-bottom: 12px;
        }

        .table {
          flex: 1;
          min-height: 0;
        }
      }
    }
  }
}
</style>
