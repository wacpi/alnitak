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
          :pagination="pagination" flex-height />
        <table-action-drawer v-model:visible="visibleDrawer" :data="editData!"></table-action-drawer>
      </div>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { h, onBeforeMount, reactive, ref } from 'vue';
import { Refresh } from "@vicons/ionicons5";
import useLoading from '@/hooks/loading-hooks';
import { statusCode } from '@/utils/status-code';
import { getPlaylistListManageAPI, deletePlaylistManageAPI } from '@/api/playlist';
import type { DataTableColumns } from 'naive-ui';
import { getResourceUrl } from '@/utils/resource';
import TableActionDrawer from './components/table-action-drawer.vue';
import { NCard, NImage, NIcon, NButton, NDataTable, NPopconfirm, NSpace, NTag, useMessage } from 'naive-ui';

const { loading, startLoading, endLoading } = useLoading(false);

const message = useMessage();

const visibleDrawer = ref(false);
const openDrawer = () => {
  visibleDrawer.value = true;
}

const editData = ref<PlaylistType>();
const viewDetails = (row: PlaylistType) => {
  editData.value = row;
  openDrawer();
}

const deletePlaylist = async (row: PlaylistType) => {
  const res = await deletePlaylistManageAPI(row.id);
  if (res.data.code === statusCode.OK) {
    message.success('删除成功');
    await getTableData();
  } else {
    message.error(res.data.msg);
  }
}

const statusMap: Record<number, { label: string; type: 'success' | 'warning' | 'error' | 'default' }> = {
  0: { label: '已通过', type: 'success' },
  500: { label: '待审核', type: 'warning' },
  2000: { label: '未通过', type: 'error' },
};

const columns: DataTableColumns<PlaylistType> = [
  {
    key: 'id',
    title: 'ID',
    width: 80,
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
    key: 'author',
    title: '作者',
    align: 'center',
    width: 120,
    render: row => {
      return row.author?.name || '-'
    }
  },
  {
    key: 'videoCount',
    title: '视频数',
    align: 'center',
    width: 80,
  },
  {
    key: 'views',
    title: '浏览量',
    align: 'center',
    width: 80,
  },
  {
    key: 'status',
    title: '状态',
    align: 'center',
    width: 90,
    render: row => {
      const s = statusMap[row.status] || { label: `未知(${row.status})`, type: 'default' as const };
      return h(NTag, { size: 'small', type: s.type }, { default: () => s.label });
    }
  },
  {
    key: 'isOpen',
    title: '公开',
    align: 'center',
    width: 70,
    render: row => {
      return row.isOpen ? '是' : '否'
    }
  },
  {
    key: 'actions',
    title: '操作',
    align: 'center',
    width: 160,
    render: row => {
      return h(NSpace, { justify: 'center' }, {
        default: () => [
          h(NButton, {
            size: 'small',
            onClick: () => viewDetails(row)
          }, { default: () => '详情' }),
          h(NPopconfirm, {
            onPositiveClick: () => deletePlaylist(row),
          }, {
            default: () => '是否删除该合集?',
            trigger: () => h(NButton, {
              size: 'small',
            }, { default: () => '删除' })
          })
        ]
      })
    }
  }
]

const tableData = ref<PlaylistType[]>([]);
const getTableData = async () => {
  startLoading();
  const page = pagination.page || 1;
  const pageSize = pagination.pageSize || 1;
  const res = await getPlaylistListManageAPI({ page, pageSize });
  if (res.data.code === statusCode.OK) {
    if (res.data.data.list) {
      tableData.value = res.data.data.list;
    } else {
      tableData.value = [];
    }
    pagination.itemCount = res.data.data.total;
    endLoading();
  }
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
    height: 100%;

    .user-card-content {
      height: 100%;
      display: flex;
      flex-direction: column;

      .search-bar {
        padding-bottom: 12px;
      }

      .table {
        flex: 1;
      }
    }
  }
}
</style>
