<template>
  <div class="alnitak-editor">
    <WangEditor v-model="valueHtml" :defaultConfig="editorConfig" :mode="mode" @onCreated="handleCreated" />
  </div>
</template>


<script  setup lang="ts">
import '@wangeditor/editor/dist/css/style.css' // 引入 css
import { onBeforeUnmount, ref, shallowRef, computed } from 'vue';
import { getResourceUrl } from '@/utils/resource';

const props = withDefaults(defineProps<{
  content: string; //弹窗可见性
}>(), {
  content: "",
})

const mode = 'default';
const editorRef = shallowRef();
const editorConfig = { placeholder: '', readOnly: true }

const handleCreated = (editor: any) => {
  editorRef.value = editor // 记录 editor 实例，重要！
};

// 处理内容中的图片URL
const processContentImages = (html: string): string => {
  if (!html) return '';
  // 匹配 img 标签的 src 属性
  return html.replace(/<img([^>]*?)src=["']([^"']+)["']/gi, (match, prefix, src) => {
    const processedSrc = getResourceUrl(src);
    return `<img${prefix}src="${processedSrc}"`;
  });
};

// 内容 HTML
const valueHtml = computed(() => processContentImages(props.content));

// 组件销毁时，也及时销毁编辑器
onBeforeUnmount(() => {
  if (editorRef.value) {
    editorRef.value.destroy()
    editorRef.value = null;
  }
})
</script> 

<style lang="scss" scoped>
.alnitak-editor {
  margin-top: 16px;
  border-radius: 3px;

  :deep(.w-e-text-container) {
    background-color: var(--bg-elev-1);
  }

  :deep(.w-e-text-container [data-slate-editor]) {
    color: var(--font-primary-1);
  }

  :deep(.w-e-text-container p),
  :deep(.w-e-text-container span),
  :deep(.w-e-text-container div),
  :deep(.w-e-text-container h1),
  :deep(.w-e-text-container h2),
  :deep(.w-e-text-container h3),
  :deep(.w-e-text-container h4),
  :deep(.w-e-text-container h5),
  :deep(.w-e-text-container li),
  :deep(.w-e-text-container td),
  :deep(.w-e-text-container th) {
    color: var(--font-primary-1);
  }

  :deep(.w-e-text-container a) {
    color: var(--primary-color);
  }
}
</style>
