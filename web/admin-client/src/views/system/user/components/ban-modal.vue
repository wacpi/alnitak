<template>
  <n-modal v-model:show="modalVisible" style="width: 700px;" preset="card" title="封禁用户">
    <n-form ref="formRef" label-placement="left" :label-width="96" :model="formModel" :rules="rules">
      <n-grid :cols="24" :x-gap="18">
        <n-form-item-grid-item :span="24" label="封禁时间" path="timestamp">
          <n-date-picker v-model:value="formModel.timestamp" type="date" />
        </n-form-item-grid-item>
        <n-form-item-grid-item :span="24" label="封禁原因" path="reason">
          <n-input v-model:value="formModel.reason" />
        </n-form-item-grid-item>
      </n-grid>
    </n-form>
    <n-space :size="24" justify="end">
      <n-button class="form-btn" @click="handleClose">取消</n-button>
      <n-button class="form-btn" type="primary" @click="banUser">确定</n-button>
    </n-space>
  </n-modal>
</template>

<script setup lang="ts">
import { computed, ref, watch, nextTick, reactive } from "vue";
import type { FormInst, FormRules } from "naive-ui";
import { NModal, NSpace, NButton, NForm, NInput, NDatePicker, NGrid, NFormItemGridItem, useMessage } from "naive-ui";
import moment from "moment";
import { banUserAPI } from "@/api/user";
import { statusCode } from "@/utils/status-code";

const emit = defineEmits(['update:visible']);
const props = withDefaults(defineProps<{
  visible: boolean; //弹窗可见性
  uid: number
}>(), {
  visible: false,
})

const message = useMessage();

const modalVisible = computed({
  get() {
    return props.visible;
  },
  set(visible) {
    emit('update:visible', visible);
  }
});

const handleClose = async () => {
  modalVisible.value = false;
}

const formRef = ref<HTMLElement & FormInst>();

const formModel = reactive<BanUserType & { timestamp: number }>({
  uid: 0,
  reason: "",
  endTime: "",
  timestamp: Date.now() + 86400000,
});

const rules: FormRules = {
  reason: { required: true, message: '请输入封禁原因', trigger: ['blur', 'input'] },
}

const banUser = async () => {
  await formRef.value?.validate();
  if (formModel.timestamp - Date.now() < 0) {
    message.error('封禁时间不能低于1天');
    return;
  }

  formModel.uid = props.uid;
  formModel.endTime = moment(formModel.timestamp).format('YYYY-MM-DD');

  const res = await banUserAPI(formModel);
  if (res.data.code === statusCode.OK) {
    message.success('封禁成功');
    handleClose();
  } else {
    message.error(res.data.msg);
  }
}


watch(() => props.visible, (newVal) => {
  if (newVal) {

  }
})
</script>

<style lang="scss" scoped>
.form-btn {
  width: 72px;
}
</style>
