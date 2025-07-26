<template>
  <div class="part-list">
    <!-- 修改分集头部，将自动连播开关放在标题后面 -->
    <div class="part-head">
      <span class="title">分段列表</span>
      <span class="part">({{ props.active }}/{{ resources?.length }})</span>
      <!-- 自动连播开关直接放在这里 -->
      <span class="autoplay-switch" @click="autonext = !autonext">
        <span class="autoplay-text">自动连播</span>
        <span class="switch-button" :class="{ 'on': autonext }"></span>
      </span>
    </div>
    
    <el-scrollbar max-height="340px">
      <ul class="list-box">
        <li :class="['list-item', props.active - 1 === index ? 'active-part' : '']" v-for="(item, index) in resources"
          @click="changePart(index)">
          <div class="item-content">
            <span class="part-num">P{{ index + 1 }}</span>
            <span class="part-title">{{ item.title }}</span>
          </div>
          <div class="duration">{{ toDuration(item.duration) }}</div>
        </li>
      </ul>
    </el-scrollbar>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch, defineExpose, readonly } from "vue";
import { toDuration } from "@/utils/format";

const emits = defineEmits(['change']);
const props = withDefaults(defineProps<{
  active?: number
  resources: Array<ResourceType>
}>(), {
  active: 1
})

const changePart = (part: number) => {
  emits('change', part + 1)
}

// 添加自动连播状态
const autonext = ref(false);

// 从本地存储恢复状态
onMounted(() => {
  const saved = localStorage.getItem('video-autonext-parts');
  autonext.value = saved === 'true';
});

// 监听状态变化并保存
watch(autonext, (val) => {
  localStorage.setItem('video-autonext-parts', val.toString());
  console.log('分集自动连播状态变更:', val);
});

// 获取下一个分集
const getNextPart = () => {
  const currentIndex = props.resources.findIndex((_, index) => index + 1 === props.active);
  if (currentIndex >= 0 && currentIndex < props.resources.length - 1) {
    return currentIndex + 2; // 返回下一个分集号（从1开始）
  }
  return null;
};

// 暴露给父组件
defineExpose({
  autonext: readonly(autonext),
  getNextPart
});
</script>

<style lang="scss" scoped>
.part-list {
  position: relative;
  border-radius: 6px;
  background-color: #F1F2F3;
  margin-bottom: 18px;

  .part-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 16px 8px;
    border-bottom: 1px solid #e3e5e7;

    .title {
      font-size: 16px;
      color: #18191c;
      font-weight: 500;
    }

    .part {
      color: #9499a0;
      font-size: 14px;
      margin-left: 4px;
    }

    .autoplay-switch {
      display: flex;
      align-items: center;
      cursor: pointer;
      user-select: none;
      margin-left: auto;

      .autoplay-text {
        font-size: 13px;
        color: #61666d;
        margin-right: 8px;
      }

      .switch-button {
        width: 32px;
        height: 18px;
        background: #e3e5e7;
        border-radius: 9px;
        position: relative;
        transition: background-color 0.3s;

        &::after {
          content: '';
          position: absolute;
          top: 2px;
          left: 2px;
          width: 14px;
          height: 14px;
          background: #fff;
          border-radius: 50%;
          transition: transform 0.3s;
        }

        &.on {
          background: #00aeec;

          &::after {
            transform: translateX(14px);
          }
        }
      }
    }
  }

  .part-head {
    display: flex;
    padding: 14px 16px 0;
    align-items: center;
    justify-content: space-between;

    .part {
      color: #9499A0;
      font-size: 13px;
    }
  }
}

.list-box {
  padding: 0 6px;
  list-style: none;
  color: #18191c;
  margin: 0;
  padding: 0 10px 0 6px;

  .list-item {
    box-sizing: border-box;
    display: flex;
    justify-content: space-between;
    overflow: hidden;
    width: 100%;
    padding: 0 10px;
    height: 30px;
    line-height: 30px;
    color: #18191C;
    margin: 5px 0;
    transition: all 0.3s;
    cursor: pointer;
    white-space: nowrap;
    overflow: hidden;
    font-size: 13px;

    .item-content {
      display: flex;
      align-items: center;
      flex-shrink: 1;
      overflow: hidden;

      .part-num {
        margin-right: 10px;
      }

      .part-title {
        display: block;
        overflow: hidden;
        text-overflow: ellipsis;
        flex-shrink: 1;
      }
    }

    .duration {
      color: #9499A0;
    }
  }
}

.active-part {
  color: var(--primary-hover-color) !important;
  background-color: #fff;
  border-radius: 2px;
}
</style>
