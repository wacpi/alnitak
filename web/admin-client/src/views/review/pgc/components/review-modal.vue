<template>
  <n-modal v-model:show="modalVisible" style="width: 420px;" preset="card" title="审核不通过">
    <p style="margin: 0 0 16px;">确定驳回该条 PGC 内容？驳回后前台不可见。</p>
    <n-space justify="end">
      <n-button @click="modalVisible = false">取消</n-button>
      <n-button type="primary" :loading="submitting" @click="handleSubmit">确定</n-button>
    </n-space>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { NModal, NButton, NSpace } from "naive-ui";
import { reviewPGCFailedAPI } from "@/api/pgc";
import { statusCode } from "@/utils/status-code";

const emit = defineEmits(['update:visible', 'finish']);
const props = withDefaults(defineProps<{
  visible: boolean;
  pgcId: string;
}>(), {
  visible: false,
})

const submitting = ref(false);

const modalVisible = computed({
  get() {
    return props.visible;
  },
  set(visible) {
    emit('update:visible', visible);
  }
});

const handleSubmit = async () => {
  if (!props.pgcId) return;
  submitting.value = true;
  try {
    const res = await reviewPGCFailedAPI({ pgc_id: props.pgcId });
    if (res.data.code === statusCode.OK) {
      modalVisible.value = false;
      emit("finish");
    }
  } finally {
    submitting.value = false;
  }
}
</script>
