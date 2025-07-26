<template>
  <div class="video">
    <header-bar class="header"></header-bar>
    <div class="video-main">
      <div class="mian-content">
        <div class="left-column">
          <div class="video-player" ref="playerContainerRef">
            <client-only>
              <video-player v-if="videoInfo && playerReady" ref="playerRef" :video-info="videoInfo" :part="currentPart" :progress="pendingProgress" :key="videoInfo?.vid + '-' + currentPart"></video-player>
            </client-only>
            <div v-if="!showPlayer" class="skeleton"></div>
          </div>
          <!-- 标题和版权信息 -->
          <div class="video-title-box">
            <p class="video-title">{{ videoInfo?.title }}</p>
            <p v-show="videoInfo?.copyright" class="copyright">
              <el-icon class="icon" color='#fd6d6f'>
                <forbid-icon></forbid-icon>
              </el-icon>
              <span>未经作者授权，禁止转载</span>
            </p>
          </div>
          <!-- 点赞收藏等数据 -->
          <div class="video-toolbar">
            <div class="toolbar-left">
              <archive-info v-if="videoInfo" :vid="videoInfo.vid"></archive-info>
            </div>
            <div class="toolbar-right">
              <span>{{ onlineCount }} 人在看</span>
              <span>{{ videoInfo?.clicks }} 播放</span>
              <span>{{ videoInfo ? formatTime(videoInfo.createdAt) : '' }}</span>
            </div>
          </div>
          <!-- 简介部分 -->
          <div class="video-desc-container">
            <div ref="descRef" class="basic-desc-info" :style="`height: ${foldDesc ? foldDescHeight : 'auto'};`">
              <span class="desc-info-text">{{ videoInfo?.desc }}</span>
            </div>
            <div class="toggle-btn" v-show="showFoldBtn" @click="foldDesc = !foldDesc">
              <span class="toggle-btn-text">{{ foldDesc ? '展开更多' : '收起' }}</span>
            </div>
          </div>
          <!-- 标签部分 -->
          <div class="tags-box">
            <div class="tag" v-for="item in videoInfo?.tags.split(',')">{{ item }}</div>
          </div>
          <!-- 评论区 -->
          <comment-list v-if="videoInfo" :vid="videoInfo.vid"></comment-list>
        </div>
        <div class="right-column">
          <!-- 作者信息 -->
          <author-card v-if="videoInfo" :info="videoInfo.author"></author-card>
          <!-- 添加弹幕列表 -->
          <div class="danmaku-list-container">
            <danmaku-list ref="danmakuListRef" :height="danmakuListHeight"></danmaku-list>
          </div>
          <!-- 视频分集 -->
          <div v-if="videoInfo && videoInfo.resources.length > 1">
            <part-list 
              ref="partListRef"
              :resources="videoInfo.resources" 
              :active="currentPart" 
              @change="changePart"
            ></part-list>
          </div>
          <!-- 相关推荐 -->
          <recommend-list 
            ref="recommendListRef" 
            v-if="videoInfo" 
            :vid="videoInfo.vid"
            :show-autoplay-control="!videoInfo || videoInfo.resources.length <= 1"
          ></recommend-list>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch, type ComponentPublicInstance } from "vue";
import { ElIcon } from "element-plus";
import { Forbid as ForbidIcon } from "@icon-park/vue-next";
import { formatTime } from "@/utils/format";
import PartList from "./components/PartList.vue";
import AuthorCard from './components/AuthorCard.vue';
import ArchiveInfo from './components/ArchiveInfo.vue';
import CommentList from "./components/CommentList.vue";
import DanmakuList from "./components/DanmakuList.vue";
import HeaderBar from "@/components/header-bar/index.vue";
import VideoPlayer from "@/components/video-player/index.vue";
import RecommendList from "./components/RecommendList.vue";
import { asyncGetVideoInfoAPI } from "@/api/video";
import { createUUID } from "@/utils/uuid";
import { getDanmakuAPI } from "@/api/danmaku";
import { getHistoryProgressAPI, addHistoryAPI } from "@/api/history";
import { globalConfig } from '@/utils/global-config';

const route = useRoute();
const router = useRouter();

// 获取视频信息
const videoInfo = ref<VideoType>();
const videoId = route.params.id.toString();
const { data } = await asyncGetVideoInfoAPI(videoId);
if ((data.value as any).code === statusCode.OK) {
  videoInfo.value = (data.value as any).data.video as VideoType;
} else {
  // 处理视频信息不存在
  navigateTo('/404');
}


const playerContainerRef = ref<HTMLElement | null>(null)
const danmakuListHeight = ref(300);
const playerRef = ref<ComponentPublicInstance<{ 
  seek: (time: number) => void; 
  uploadHistory: () => void; 
  setDanmaku: (data: any[]) => void; 
  setOnReady: (cb: () => void) => void;
  setOnEnded: (cb: () => void) => void;
}> | null>(null);

const handelResize = () => {
  nextTick(() => {
    danmakuListHeight.value = ((playerContainerRef.value?.clientWidth || 730) * 0.5625) + 40 - 104;
  })
}

// 视频分集
// 校验 p 参数有效性，无效则重定向到 p1
if (route.query.p && Number(route.query.p) > videoInfo.value!.resources.length) {
  router.replace({ path: `/video/${videoId}`, query: { p: 1 } });
}
const currentPart = ref(Number(route.query.p) || 1);
const pendingProgress = ref<number | null>(null);

// 获取组件引用
const recommendListRef = ref<InstanceType<typeof RecommendList> | null>(null);
const partListRef = ref<InstanceType<typeof PartList> | null>(null);

// 视频播放结束时的自动连播逻辑
const onVideoEnded = () => {
  console.log('视频播放结束，检查自动连播状态');
  
  // 判断是多分集还是单集
  const hasMultipleParts = videoInfo.value && videoInfo.value.resources.length > 1;
  
  if (hasMultipleParts) {
    // 多分集：检查分集自动连播
    if (partListRef.value?.autonext) {
      const nextPart = partListRef.value.getNextPart?.();
      console.log('自动连播下一分集:', nextPart);
      
      if (nextPart) {
        setTimeout(() => {
          changePart(nextPart);
        }, 1000);// 这里设置延迟时间：3000毫秒 = 3秒
      } else {
        console.log('已是最后一集，检查推荐视频');
        // 最后一集播放完，检查推荐自动连播
        checkRecommendAutoplay();
      }
    }
  } else {
    // 单集：检查推荐自动连播
    checkRecommendAutoplay();
  }
};

// 检查推荐视频自动连播
const checkRecommendAutoplay = () => {
  if (recommendListRef.value?.autonext) {
    const nextVideo = recommendListRef.value.getNextVideo?.();
    console.log('自动连播下一个推荐视频:', nextVideo);
    
    if (nextVideo) {
      setTimeout(() => {
        navigateTo(`/video/${nextVideo.vid}`);
      }, 3000);
    } else {
      console.log('没有更多推荐视频了');
    }
  }
};

const onPlayerReady = () => {
  // 原有的进度恢复逻辑
  if (pendingProgress.value === -1 && playerRef.value && playerRef.value.seek) {
    playerRef.value.seek(0);
    pendingProgress.value = null;
    return;
  }
  if (pendingProgress.value !== null && playerRef.value && playerRef.value.seek) {
    playerRef.value.seek(pendingProgress.value);
    pendingProgress.value = null;
  }
  
  // 新增：绑定播放结束事件
  if (playerRef.value && playerRef.value.setOnEnded) {
    playerRef.value.setOnEnded(onVideoEnded);
    console.log('自动连播事件已绑定');
  }
};

watch(playerRef, (val) => {
  if (val && val.setOnReady) {
    val.setOnReady(onPlayerReady);
  }
});

const changePart = async (target: number) => {
  // 移除手动上报，让播放器组件自己处理
  // 只负责切换逻辑
  if (videoInfo.value?.resources[target - 1]) {
    currentPart.value = target;
  }
  router.replace({ query: { p: currentPart.value } });

  // 主动请求新分集进度
  if (videoInfo.value) {
    const res = await getHistoryProgressAPI(videoInfo.value.vid, currentPart.value);
    if (res.data.code === 200 && res.data.data && typeof res.data.data.progress === 'number' && res.data.data.progress > 0) {
      pendingProgress.value = res.data.data.progress;
    } else {
      pendingProgress.value = null;
    }
  }
}

// 监听路由参数 p，自动切换分P和弹幕
watch(() => route.query.p, async (newP) => {
  const partNum = Number(newP) || 1;
  // 如果分P有效，则切换；否则重定向到 p1
  if (videoInfo.value?.resources[partNum - 1]) {
    currentPart.value = partNum;
    // 获取历史进度
    const res = await getHistoryProgressAPI(videoInfo.value.vid, partNum);
    if (res.data.code === 200 && res.data.data && typeof res.data.data.progress === 'number') {
      // 如果有历史进度，恢复播放进度
      pendingProgress.value = res.data.data.progress;
    } else {
      // 如果没有历史进度，直接设置为 null 或初始值
      pendingProgress.value = null;
    }
    // 获取弹幕列表
    getDanmakuList(videoInfo.value.vid, partNum);
  } else {
    // 如果分P无效，重定向到 p1
    router.replace({ path: `/video/${videoId}`, query: { p: 1 } });
  }
});

// 获取弹幕列表
const danmakuListRef = ref<InstanceType<typeof DanmakuList> | null>(null);

const getDanmakuList = async (vid: number, part: number) => {
  const res = await getDanmakuAPI(vid, part);
  if (res.data.code === statusCode.OK) {
    const danmakus = res.data.data.danmaku || [];
    nextTick(() => {
      playerRef.value?.setDanmaku(danmakus)
      danmakuListRef.value?.setDanmaku(danmakus)
    })
  }
};

// 简介部分
const foldDesc = ref(true); // 是否折叠简介
const descRef = ref<HTMLElement>();
const showPlayer = ref(false);
const showFoldBtn = ref(false); // 是否显示展开和折叠按钮
const foldDescHeight = ref('auto'); // 折叠状态下简介的最大高度
const playerReady = ref(false);
onMounted(async () => {
  if (descRef.value!.clientHeight >= 80) {
    showFoldBtn.value = true;
    foldDescHeight.value = '80px';
  } else {
    showFoldBtn.value = false;
    foldDescHeight.value = 'auto';
  }

  if (videoInfo.value) {
    try {
      // 带 p 参数时也请求 getProgress?vid=xx&part=xx
      let res;
      if (!route.query.p) {
        res = await getHistoryProgressAPI(videoInfo.value.vid);
      } else {
        res = await getHistoryProgressAPI(videoInfo.value.vid, Number(route.query.p));
      }
      if (res && res.data.code === 200 && res.data.data) {
        const { part, progress } = res.data.data;
        if (part && part !== currentPart.value && videoInfo.value.resources[part - 1]) {
          currentPart.value = part;
          router.replace({ query: { p: part } });
          await nextTick();
        }
        getDanmakuList(videoInfo.value.vid, part || currentPart.value);
        if (typeof progress === 'number' && progress > 0) {
          console.log('[id.vue] pendingProgress赋值:', progress);
          pendingProgress.value = progress;
        } else {
          console.log('[id.vue] pendingProgress赋值: null');
          pendingProgress.value = null;
        }
      } else {
        getDanmakuList(videoInfo.value.vid, currentPart.value);
        console.log('[id.vue] pendingProgress赋值: null');
        pendingProgress.value = null;
      }
    } catch (e) {
      getDanmakuList(videoInfo.value.vid, currentPart.value);
    }
  }

  handelResize();
  window.addEventListener("resize", handelResize);

  nextTick(() => {
    showPlayer.value = true;
    playerReady.value = true;
  })
})

//websocket
const onlineCount = ref(1);//在线人数
let SocketURL = "";
let websocket: WebSocket | null = null;
//初始化weosocket
const initWebSocket = () => {
  let clientId = localStorage.getItem("ws-client-id");
  if (!clientId) {
    clientId = createUUID();
    localStorage.setItem("ws-client-id", clientId);
  }
  const wsProtocol = window.location.protocol === 'http:' ? 'ws://' : 'wss://';
  const domain = globalConfig.domain || window.location.host;
  SocketURL = wsProtocol + domain + `/api/v1/online/video?vid=${videoId}&clientId=${clientId}`;

  websocket = new WebSocket(SocketURL);
  websocket.onmessage = websocketOnmessage;
}

//数据接收
const websocketOnmessage = (e: any) => {
  const res = JSON.parse(e.data);
  onlineCount.value = res.number;
}

onBeforeMount(() => {
  initWebSocket();
})

onBeforeUnmount(() => {
  window.removeEventListener("resize", handelResize);
  if (websocket) {
    websocket.close();
    websocket = null;
  }
})

// 移除 needReportAfterSwitch 相关逻辑
// watch(pendingProgress, (val) => {
//   // 删除这个 watch，不再手动上报
// });

// 新增：监听 route.params.id 变化，重新拉取视频信息和重置状态
watch(() => route.params.id, async (newId, oldId) => {
  if (newId !== oldId) {
    // 重新拉取视频信息
    const { data } = await asyncGetVideoInfoAPI(newId.toString());
    if ((data.value as any).code === statusCode.OK) {
      videoInfo.value = (data.value as any).data.video as VideoType;
      // 重置分集
      currentPart.value = Number(route.query.p) || 1;
      // 重置弹幕和进度
      getDanmakuList(videoInfo.value.vid, currentPart.value);
      const res = await getHistoryProgressAPI(videoInfo.value.vid, currentPart.value);
      if (res.data.code === 200 && res.data.data && typeof res.data.data.progress === 'number') {
        pendingProgress.value = res.data.data.progress;
      } else {
        pendingProgress.value = null;
      }
    } else {
      navigateTo('/404');
    }
  }
});

useHead({
  title: () => videoInfo.value?.title ? `${videoInfo.value.title} - ${globalConfig.title}` : globalConfig.title
})
</script>

<style lang="scss" scoped>
.header {
  position: fixed;
}

.video-main {
  padding-top: 80px;
  margin: 0 auto;
  min-width: 1200px;
  /* 保持最小宽度为 1200px */
}

.mian-content {
  display: flex;
  width: 100%;
  max-width: 1700px; // 调整到1700px
  margin: 0 auto;
  padding: 0 45px; // 调整内边距到45px
  position: relative;
  box-sizing: border-box;
}

.left-column {
  flex: 1;

  .video-player {
    position: relative;
    margin: 0 auto;
    width: 100%;
    /*16:9*/
    min-width: 680px;
    min-height: 382px;

    .skeleton {
      width: 100%;
      padding-bottom: 56.25%;
      background-color: #f0f2f5;
    }
  }

  .video-title-box {
    width: 100%;
    height: 54px;
    display: flex;

    .video-title {
      width: calc(100% - 160px);
      font-weight: 500;
      line-height: 28px;
      margin: 13px 0;
      font-size: 20px;
      color: #18191C;
      overflow: hidden;
      white-space: nowrap;
      text-overflow: ellipsis;
    }

    .copyright {
      width: 180px;
      display: flex;
      align-items: center;
      justify-content: flex-end;
      font-size: 13px;
      color: #9499A0;

      .icon {
        padding: 0 6px;
      }
    }
  }

  .video-toolbar {
    color: #9499A0;
    font-size: 13px;
    padding-bottom: 12px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    border-bottom: 1px solid #E3E5E7;

    .toolbar-right {
      display: inline-block;

      span {
        margin-left: 20px;
      }
    }
  }


  // 简介部分
  .video-desc-container {
    margin: 16px 0;

    .basic-desc-info {
      white-space: pre-line;
      letter-spacing: 0;
      color: #18191C;
      font-size: 15px;
      line-height: 24px;
      overflow: hidden;

      .desc-info-text {
        white-space: pre-line;
      }
    }

    .toggle-btn {
      margin-top: 10px;
      font-size: 13px;
      line-height: 18px;

      .toggle-btn-text {
        cursor: pointer;
        color: #61666D;

        &:hover {
          color: var(--primary-hover-color);
        }
      }
    }
  }

  // 标签部分
  .tags-box {
    padding-bottom: 6px;
    margin: 16px 0 20px 0;
    border-bottom: 1px solid #E3E5E7;

    .tag {
      color: #61666d;
      background: #f1f2f3;
      height: 28px;
      line-height: 28px;
      border-radius: 14px;
      font-size: 13px;
      padding: 0 12px;
      box-sizing: border-box;
      transition: all .3s;
      display: inline-flex;
      align-items: center;
      cursor: pointer;
      margin: 0 12px 8px 0;
    }
  }
}

.right-column {
  width: 340px;
  margin-left: 30px;
  z-index: 1;

  // 新增：弹幕列表和分集板块间距
  .danmaku-list-container {
    margin-bottom: 18px;
  }
}

// 简化响应式设计，删除多余断点
@media (max-width: 1700px) {
  .mian-content {
    max-width: 1500px;
    padding: 0 35px;
  }
}

@media (max-width: 1500px) {
  .mian-content {
    max-width: 1300px;
    padding: 0 25px;
  }
}

@media (max-width: 1200px) {
  .video-main {
    min-width: auto;
  }
  
  .mian-content {
    max-width: 100%;
    padding: 0 15px;
  }
}
</style>
