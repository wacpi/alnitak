import { ref } from 'vue';
import { getPartitionAPI } from '@/api/partition';
import { statusCode } from '@/utils/status-code';

export default function usePartition(partitionType: "video" | "article") {
  // 获取分区列表
  const partitionList = ref<Array<PartitionType>>([]);//所有分区
  const getPartition = async () => {
    const res = await getPartitionAPI(partitionType);
    if (res.data.code === statusCode.OK) {
      partitionList.value = res.data.data.partitions;
    }
  }

  // 获取分区名（id 为子分区 ID；未加载分区表、id 为 0 或数据不匹配时避免显示 undefined/undefined）
  const getPartitionName = (id: number) => {
    if (id == null || id === 0) {
      return '-';
    }
    const subpartition = partitionList.value.find((item) => {
      return item.id === id;
    });
    if (!subpartition) {
      return '-';
    }
    const partition = partitionList.value.find((item) => {
      return item.id === subpartition.parentId;
    });
    if (!partition) {
      return subpartition.name ?? '-';
    }
    return `${partition.name}/${subpartition.name}`;
  }


  return {
    partitionList,
    getPartition,
    getPartitionName
  };
}
