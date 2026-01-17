<template>
  <n-modal v-model:show="modalVisible" style="width: 700px;" preset="card" title="审核不通过">
    <n-form ref="formRef" :style="{ maxWidth: '640px' }">
      <n-grid :cols="24" :x-gap="18" v-for="(item, index) in dynamicForm">
        <n-form-item-gi :span="12" label="位置">
          <n-select v-model:value="item.position" :options="positionOptions" />
        </n-form-item-gi>
        <n-form-item-gi :span="12" label="问题">
          <n-select v-model:value="item.question" :options="questionOptions" />
        </n-form-item-gi>
      </n-grid>

      <!-- 冲突稿件关联（可选） -->
      <n-divider style="margin: 12px 0;">内容冲突关联（可选）</n-divider>
      <n-grid :cols="24" :x-gap="18">
        <n-form-item-gi :span="12" label="冲突资源ID">
          <n-input-number v-model:value="conflictResourceId" placeholder="原稿件的资源ID" :min="0" style="width: 100%;" clearable />
        </n-form-item-gi>
        <n-form-item-gi :span="12" label="冲突原因">
          <n-select v-model:value="conflictReason" :options="conflictReasonOptions" clearable />
        </n-form-item-gi>
      </n-grid>
    </n-form>
    <n-space :size="24" justify="space-between">
      <n-button class="form-btn" @click="addItem">追加</n-button>
      <n-button class="form-btn" type="primary" @click="handleSubmit">确定</n-button>
    </n-space>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { NModal, NForm, NGrid, NFormItemGi, NSelect, NSpace, NButton, NDivider, NInputNumber } from "naive-ui";
import { reviewVideoFailedAPI } from "@/api/review";
import { reviewCode } from "@/utils/review-code";
import { statusCode } from "@/utils/status-code";

const emit = defineEmits(['update:visible', 'finish']);
const props = withDefaults(defineProps<{
  visible: boolean; //弹窗可见性
  vid: number;
  videoCount: number
}>(), {
  visible: false,
})

const positionOptions = ref([
  { label: "封面", value: 0, },
  { label: "标题", value: 1, },
])
const questionOptions = [
  { label: "其他", value: 0, },
  { label: "色情低俗", value: 1, },
  { label: "违法违禁", value: 2, },
  { label: "危险行为", value: 3, },
  { label: "血腥暴力", value: 4, },
  { label: "侵权著作权", value: 5, },
  { label: "传播不良价值观", value: 6, },
]

// 冲突原因选项
const conflictReasonOptions = [
  { label: "搬运他人作品", value: "搬运他人作品" },
  { label: "重复投稿", value: "重复投稿" },
  { label: "抄袭", value: "抄袭" },
  { label: "盗用素材", value: "盗用素材" },
  { label: "其他侵权", value: "其他侵权" },
]

// 冲突关联字段
const conflictResourceId = ref<number | null>(null);
const conflictReason = ref<string | null>(null);

const modalVisible = computed({
  get() {
    return props.visible;
  },
  set(visible) {
    emit('update:visible', visible);
  }
});

const dynamicForm = ref([
  { position: 0, question: 0 },
])

const addItem = () => {
  dynamicForm.value.push({ position: 0, question: 0 });
}

const getPositionText = (position: number) => {
  return positionOptions.value.find(item => item.value === position)?.label;
}

const getQuestionText = (position: number) => {
  return questionOptions.find(item => item.value === position)?.label;
}

const handleSubmit = async () => {
  let remark = '';
  for (const item of dynamicForm.value) {
    remark += `${getPositionText(item.position)}存在${getQuestionText(item.question)}问题;`
  }

  const params: ReviewVideoParam = {
    vid: props.vid,
    status: reviewCode.REVIEW_FAILED,
    remark: remark
  };

  // 如果填写了冲突关联，添加到参数中
  if (conflictResourceId.value) {
    params.conflictResourceId = conflictResourceId.value;
    params.conflictReason = conflictReason.value || '';
  }

  const res = await reviewVideoFailedAPI(params);
  if(res.data.code === statusCode.OK) {
    emit("finish");
  }
}

watch(() => props.visible, (newVal) => {
  if (newVal) {
    dynamicForm.value = [{ position: 0, question: 0 }];
    // 重置冲突关联字段
    conflictResourceId.value = null;
    conflictReason.value = null;

    positionOptions.value = Array.from(new Array(props.videoCount), (_, i) => {
      return { label: `P${i + 1}`, value: i + 2 }
    })
    positionOptions.value.unshift(...[
      { label: "封面", value: 0, },
      { label: "标题", value: 1, },
    ])
  }
})
</script>

<style lang="scss" scoped>
.form-btn {
  width: 72px;
}
</style>

