<template>
  <div class="archive-data">
    <!--点赞收藏-->
    <div class="archive-item">
      <el-icon :class="[likeAnimation, archive.hasLike ? 'active' : 'icon']" @click="likeClick">
        <like-icon></like-icon>
      </el-icon>
      <p>{{ stat.like }}</p>
    </div>
    <div class="archive-item">
      <el-icon :class="archive.hasCollect ? 'active' : 'icon'" @click="showCollect = true">
        <collect-icon></collect-icon>
      </el-icon>
      <p>{{ stat.collect }}</p>
    </div>
    <!-- 分享按钮 -->
    <div class="archive-item share-item" @mouseenter="onShareEnter" @mouseleave="onShareLeave">
      <el-icon class="icon">
        <ShareIcon />
      </el-icon>
      <div v-if="showShare" class="share-popover" @mouseenter="onShareEnter" @mouseleave="onShareLeave">
      <el-tabs v-model="shareTab">
        <el-tab-pane label="分享链接" name="link">
          <div class="embed-box">
            <el-input v-model="shareUrl" readonly></el-input>
            <el-button type="primary" @click="copyUrl">复制链接</el-button>
          </div>
        </el-tab-pane>
        <el-tab-pane label="嵌入代码" name="embed">
          <div class="embed-box">
            <el-input v-model="embedCode" readonly></el-input>
            <el-button type="primary" @click="copyEmbed">复制嵌入代码</el-button>
          </div>
        </el-tab-pane>
      </el-tabs>
      </div>
    </div>
    <collection-list v-if="showCollect" :vid="vid" @close="closeCollectionCard"></collection-list>
  </div>
</template>

<script setup lang="ts">
import { ref, onBeforeMount, computed } from 'vue';
import { ElIcon, ElMessage } from 'element-plus';
import LikeIcon from "@/components/icons/LikeIcon.vue";
import CollectIcon from "@/components/icons/CollectIcon.vue";
import { getVideoArchiveStatAPI } from "@/api/archive";
import { getLikeVideoStatusAPI, likeVideoAPI, cancelLikeVideoAPI } from "@/api/like";
import { getCollectVideoStatusAPI } from '@/api/collect';
import CollectionList from './CollectionList.vue';
import { useRoute } from 'vue-router';
import ShareIcon from '@/components/icons/ShareIcon.vue';

const props = defineProps<{
  vid: number;
}>();

// 点赞收藏数据
const stat = ref<{ like: number, collect: number }>({
  like: 0,
  collect: 0
});

const loading = ref(true);
const archive = reactive({ // 是否点赞收藏
  hasCollect: false,
  hasLike: false
})

// 分享相关
const showShare = ref(false);
const shareTab = ref('link');
const shareUrl = computed(() => window.location.href);
const route = useRoute();
const embedCode = computed(() => {
  const part = Number(route.query.p) || 1;
  const url = window.location.origin + `/embed/video/${props.vid}` + (part > 1 ? `?p=${part}` : '');
  return `<iframe src='${url}' width='800' height='450' frameborder='0' allowfullscreen></iframe>`;
});
const copyText = async (text: string, msg: string) => {
  try {
    await navigator.clipboard.writeText(text);
    ElMessage.success(msg);
  } catch (e) {
    // 降级处理
    const textarea = document.createElement('textarea');
    textarea.value = text;
    textarea.style.position = 'fixed';
    textarea.style.opacity = '0';
    document.body.appendChild(textarea);
    textarea.focus();
    textarea.select();
    try {
      document.execCommand('copy');
      ElMessage.success(msg);
    } catch (err) {
      ElMessage.error('复制失败，请手动复制');
    }
    document.body.removeChild(textarea);
  }
};

const copyUrl = () => copyText(shareUrl.value, '播放地址已复制');
const copyEmbed = () => copyText(embedCode.value, '嵌入代码已复制');

//获取点赞收藏关注信息
const getArchiveStat = async () => {
  const res = await getVideoArchiveStatAPI(props.vid);
  if (res.data.code === statusCode.OK) {
    stat.value = res.data.data.stat;
  }
}

// 获取是否点赞
const getLikeStatus = async () => {
  const res = await getLikeVideoStatusAPI(props.vid);
  if (res.data.code === statusCode.OK) {
    archive.hasLike = res.data.data.like;
  }
}

// 获取是否收藏
const getCollectStatus = async () => {
  const res = await getCollectVideoStatusAPI(props.vid);
  if (res.data.code === statusCode.OK) {
    archive.hasCollect = res.data.data.collect;
  }
}

const likeAnimation = ref('');
const likeClick = async () => { // 点赞点赞按钮
  if (loading.value) return;
  if (!archive.hasLike) {
    //调用点赞接口
    await likeVideoAPI(props.vid);
    likeAnimation.value = 'like-active';
    stat.value.like++;
  } else {
    await cancelLikeVideoAPI(props.vid);
    stat.value.like--;
  }

  archive.hasLike = !archive.hasLike;
}


const showCollect = ref(false);
// 关闭收藏弹窗
const closeCollectionCard = (val: number) => {
  if (val === 1) {
    stat.value.collect++;
    archive.hasCollect = true;
  } else if (val === -1) {
    stat.value.collect--;
    archive.hasCollect = false;
  }
  
  showCollect.value = false;
}

// 分享板块延迟隐藏逻辑
let sharePopoverTimer: number | null = null;

const onShareEnter = () => {
  if (sharePopoverTimer) {
    clearTimeout(sharePopoverTimer);
    sharePopoverTimer = null;
  }
  showShare.value = true;
};

const onShareLeave = () => {
  sharePopoverTimer = window.setTimeout(() => {
    showShare.value = false;
  }, 120); // 120ms 容错，保证鼠标能顺利移入板块
};

onBeforeMount(async () => {
  await getArchiveStat();
  await getLikeStatus();
  await getCollectStatus();

  loading.value = false;
})
</script>

<style lang="scss" scoped>
.archive-data {
  height: 30px;

  .archive-item {
    float: left;
    user-select: none;
    margin-right: 20px;

    i, .icon {
      font-size: 26px;
      width: 26px;
      height: 26px;
      line-height: 30px;
      cursor: pointer;
      vertical-align: middle;
    }

    p {
      font-size: 16px;
      float: right;
      margin: 0 6px;
      line-height: 30px;
    }

    .icon:hover {
      color: var(--primary-color);
    }

    .active {
      color: var(--primary-color);
    }

    .like-active {
      animation: scaleDraw .3s ease-in-out;
    }
  }
  .share-item {
    position: relative;
    .share-popover {
      position: absolute;
      bottom: 36px;
      left: 0;
      z-index: 100;
      background: #fff;
      border: 1px solid #eee;
      border-radius: 8px;
      box-shadow: 0 2px 8px rgba(0,0,0,0.08);
      padding: 16px 20px 8px 20px;
      min-width: 320px;
      min-height: 120px;
      .embed-box {
        display: flex;
        align-items: center;
        gap: 8px;
        margin-bottom: 8px;
      }
    }
  }
}


@keyframes scaleDraw {
  0% {
    transform: scale(1);
    /*开始为原始大小*/
  }

  25% {
    transform: scale(1.2);
    /*放大1.1倍*/
  }

  100% {
    transform: scale(1);
  }
}
</style>