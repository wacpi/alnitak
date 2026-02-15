<template>
  <div class="video">
    <header-bar class="header"></header-bar>
    <div class="video-main">
      <div class="mian-content">
        <div class="left-column">
          <div class="video-player" ref="playerContainerRef">
            <client-only>
              <video-player v-if="videoInfo && playerReady" ref="playerRef" :video-info="videoInfo" :part="currentPart"
                :progress="pendingProgress" :key="videoInfo?.vid + '-' + currentPart"></video-player>
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
          <comment-list v-if="videoInfo" :vid="videoInfo.vid" @seek-time="handleSeekTime"></comment-list>
        </div>
        <div class="right-column">
          <!-- 作者信息 -->
          <author-card v-if="videoInfo" :info="videoInfo.author"></author-card>
          <!-- 添加弹幕列表 -->
          <div class="danmaku-list-container">
            <danmaku-list ref="danmakuListRef" :height="danmakuListHeight"></danmaku-list>
          </div>
          <!-- 合并的分P和合集列表 -->
          <video-collection 
            ref="collectionRef" 
            v-if="videoInfo" 
            :vid="videoInfo.vid"
            :resources="videoInfo.resources"
            :current-part="currentPart"
            @change-part="changePart"
          ></video-collection>
          <!-- 相关推荐 -->
          <recommend-list ref="recommendListRef" v-if="videoInfo" :vid="videoInfo.vid"
            :show-autoplay-control="!videoInfo || (videoInfo.resources.length <= 1 && !hasCollection)"></recommend-list>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick, type ComponentPublicInstance } from "vue";
import { ElIcon } from "element-plus";
import { Forbid as ForbidIcon } from "@icon-park/vue-next";
import { formatTime } from "@/utils/format";
import PartList from "./components/PartList.vue";
import AuthorCard from './components/AuthorCard.vue';
import ArchiveInfo from './components/ArchiveInfo.vue';
import VideoCollection from "./components/VideoCollection.vue";
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
let videoId = route.params.id.toString();
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
const collectionRef = ref<InstanceType<typeof VideoCollection> | null>(null);
const hasCollection = computed(() => !!collectionRef.value?.hasPlaylist);

// 视频播放结束时的自动连播逻辑
const onVideoEnded = () => {
  console.log('视频播放结束，检查自动连播状态');

  // 优先检查当前视频是否有分P
  const hasParts = (videoInfo.value?.resources?.length || 0) > 1
  const curPart = currentPart.value
  
  if (hasParts) {
    // 当前视频有分P，检查是否还有下一个分P
    if (curPart < (videoInfo.value?.resources?.length || 0)) {
      // 切换到下一个分P
      setTimeout(() => {
        changePart(curPart + 1)
      }, 1000)
      return
    }
  }
  
  // 没有分P或已是最后一个分P，检查合集自动连播
  const collectionRefVal = collectionRef.value
  console.log('合集自动连播状态:', collectionRefVal?.autonext)
  if (collectionRefVal?.autonext) {
    const nextVideo = collectionRefVal.getNextVideo?.()
    console.log('合集下一个视频:', nextVideo)
    if (nextVideo) {
      setTimeout(() => {
        if (nextVideo.resourceId) {
          navigateTo(`/video/${nextVideo.vid}?resourceId=${nextVideo.resourceId}`)
        } else {
          navigateTo(`/video/${nextVideo.vid}`)
        }
      }, 1000)
      return
    }
  }
  
  // 合集没有下一个或未开启，检查推荐
  checkRecommendAutoplay()
};

// 检查推荐视频自动连播
const checkRecommendAutoplay = () => {
  const recommendRef = recommendListRef.value
  console.log('检查推荐列表:', recommendRef?.videoList?.value?.length)
  
  // 直接检查推荐列表是否有视频，有的话就播放
  const videoList = recommendRef?.videoList?.value
  if (videoList && videoList.length > 0) {
    const nextVideo = videoList[0]
    console.log('播放推荐列表第一个视频:', nextVideo)
    setTimeout(() => {
      navigateTo(`/video/${nextVideo.vid}`)
    }, 3000)
    return
  }
  
  console.log('没有推荐视频了')
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

// 处理评论区时间跳转
const handleSeekTime = (seconds: number) => {
  if (playerRef.value && playerRef.value.seek) {
    playerRef.value.seek(seconds);
  }
};

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
  if (descRef.value && descRef.value.clientHeight >= 80) {
    showFoldBtn.value = true;
    foldDescHeight.value = '80px';
  } else {
    showFoldBtn.value = false;
    foldDescHeight.value = 'auto';
  }

  if (videoInfo.value) {
    try {
      // 带 p 参数时也请求 getProgress?vid=xx&part=xx
      const res = await getHistoryProgressAPI(videoInfo.value.vid, route.query.p ? Number(route.query.p) : undefined);
      if (res.data.code === 200 && res.data.data) {
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

  // 初始化 WebSocket 连接（统计在看人数）
  console.log('[video page] onMounted 执行，准备初始化 WebSocket, videoId:', videoId);
  initWebSocket();
  document.addEventListener('visibilitychange', handleVisibilityChange);
})

//websocket
const onlineCount = ref(0);//在线人数,初始值为0
let SocketURL = "";
let websocket: WebSocket | null = null;
let reconnectTimer: number | null = null;
let reconnectAttempts = 0;
const MAX_RECONNECT_ATTEMPTS = 10; // 增加重连次数
let isManualClose = false; // 标记是否为手动关闭
let heartbeatTimer: number | null = null; // 心跳定时器
let lastMessageTime = 0; // 最后一次收到消息的时间

// 关闭当前 WebSocket 连接
const closeWebSocket = () => {
  if (reconnectTimer) {
    clearTimeout(reconnectTimer);
    reconnectTimer = null;
  }
  if (heartbeatTimer) {
    clearInterval(heartbeatTimer);
    heartbeatTimer = null;
  }
  if (websocket) {
    isManualClose = true;
    websocket.close();
    websocket = null;
  }
  reconnectAttempts = 0;
  onlineCount.value = 0;
}

// 重新连接 WebSocket（切换视频时使用）
const reconnectWebSocket = () => {
  closeWebSocket();
  isManualClose = false;
  initWebSocket();
}

//初始化weosocket
const initWebSocket = () => {
  let clientId = localStorage.getItem("ws-client-id");
  if (!clientId) {
    clientId = createUUID();
    localStorage.setItem("ws-client-id", clientId);
  }

  // 开发环境直连后端，生产环境通过 Nginx 代理
  if (process.dev) {
    const wsProtocol = globalConfig.https ? 'wss://' : 'ws://';
    SocketURL = `${wsProtocol}${globalConfig.domain}/api/v1/online/video?vid=${videoId}&clientId=${clientId}`;
  } else {
    const wsProtocol = window.location.protocol === 'https:' ? 'wss://' : 'ws://';
    SocketURL = `${wsProtocol}${window.location.host}/api/v1/online/video?vid=${videoId}&clientId=${clientId}`;
  }

  // 清理旧的定时器
  if (heartbeatTimer) {
    clearInterval(heartbeatTimer);
    heartbeatTimer = null;
  }

  try {
    console.log('[WebSocket] 开始连接:', SocketURL);
    websocket = new WebSocket(SocketURL);

    // 连接打开事件
    websocket.onopen = () => {
      console.log('[WebSocket] 连接成功');
      reconnectAttempts = 0; // 重置重连次数
      lastMessageTime = Date.now();

      // 启动心跳检测
      startHeartbeat();
    };

    // 消息接收事件
    websocket.onmessage = websocketOnmessage;

    // 错误处理
    websocket.onerror = (error) => {
      console.error('[WebSocket] 连接错误:', error);
    };

    // 连接关闭事件
    websocket.onclose = (event) => {
      console.log('[WebSocket] 连接关闭 - Code:', event.code, 'Reason:', event.reason, 'wasClean:', event.wasClean);
      websocket = null;

      // 停止心跳检测
      if (heartbeatTimer) {
        clearInterval(heartbeatTimer);
        heartbeatTimer = null;
      }

      // 只有在非手动关闭时才尝试重连
      if (!isManualClose) {
        // 1000是正常关闭,但我们仍然重连以保持在线状态
        if (reconnectAttempts < MAX_RECONNECT_ATTEMPTS) {
          reconnectAttempts++;
          const delay = Math.min(1000 * Math.pow(2, reconnectAttempts - 1), 10000);
          console.log(`[WebSocket] ${delay}ms 后尝试第 ${reconnectAttempts} 次重连...`);
          reconnectTimer = window.setTimeout(() => {
            initWebSocket();
          }, delay);
        } else {
          console.error('[WebSocket] 已达到最大重连次数,停止重连');
        }
      }
    };
  } catch (error) {
    console.error('[WebSocket] 创建连接失败:', error);
  }
}

// 启动心跳检测
const startHeartbeat = () => {
  heartbeatTimer = window.setInterval(() => {
    if (websocket && websocket.readyState === WebSocket.OPEN) {
      // 发送心跳消息给后端，保持连接活跃并重置后端的读超时
      try {
        websocket.send('ping');
        lastMessageTime = Date.now();
      } catch {
        console.warn('[WebSocket] 发送心跳失败');
        websocket.close();
      }
    } else if (websocket && websocket.readyState !== WebSocket.CONNECTING) {
      console.warn('[WebSocket] 检测到连接异常,状态:', websocket.readyState);
      websocket.close();
    }
  }, 25000); // 每 25 秒发一次心跳（后端 60 秒读超时，30 秒 ping）
}

// 监听页面可见性变化
const handleVisibilityChange = () => {
  if (document.hidden) {
    console.log('[WebSocket] 页面进入后台');
  } else {
    console.log('[WebSocket] 页面回到前台');
    // 页面回到前台时检查连接状态
    if (!websocket || websocket.readyState !== WebSocket.OPEN) {
      console.log('[WebSocket] 页面恢复时检测到连接断开,尝试重连');
      reconnectAttempts = 0; // 重置重连次数
      initWebSocket();
    }
  }
}

//数据接收
const websocketOnmessage = (e: any) => {
  try {
    // 更新最后一次收到消息的时间
    lastMessageTime = Date.now();

    const res = JSON.parse(e.data);

    // 处理在线人数
    if (typeof res.number === 'number') {
      onlineCount.value = res.number;
      console.log('[WebSocket] 更新在线人数:', res.number);
    }
  } catch (error) {
    console.error('[WebSocket] 解析消息失败:', error);
  }
}


onBeforeUnmount(() => {
  window.removeEventListener("resize", handelResize);
  document.removeEventListener('visibilitychange', handleVisibilityChange);

  // 标记为手动关闭
  // 清理 WebSocket
  closeWebSocket();

  // 清理组件引用,避免内存泄漏
  playerRef.value = null;
  recommendListRef.value = null;
  partListRef.value = null;
  collectionRef.value = null;
  danmakuListRef.value = null;
  playerContainerRef.value = null;
  descRef.value = undefined;
})

// 移除 needReportAfterSwitch 相关逻辑
// watch(pendingProgress, (val) => {
//   // 删除这个 watch，不再手动上报
// });

// 新增：监听 route.params.id 变化，重新拉取视频信息和重置状态
watch(() => route.params.id, async (newId, oldId) => {
  if (newId !== oldId) {
    // 更新 videoId
    videoId = newId.toString();

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

      // 重新连接 WebSocket 以更新在线人数
      if (process.client) {
        reconnectWebSocket();
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

//视频主页面
.video-main {
  padding-top: 80px;
  margin: 0 auto;
  min-width: 1200px;
  /* 保持最小宽度为 1200px */
}

//主内容区域
.mian-content {
  display: flex;
  justify-content: center; // 让内容水平居中
  width: 100%;
  max-width: calc(100% - 100px);
  margin: 0 auto;
  position: relative;
}

//左侧内容区域
.left-column {
  flex: 1;
  max-width: 1200px; // 最大宽度900px
  margin-top: 20px; // 向下移动21像素

  //视频播放器
  .video-player {
    position: relative;
    margin: 0 auto;
    width: 100%;
    /*16:9*/
    min-width: 680px;
    min-height: 382px;
    // background-color: var(--bg-elev-1);

    .skeleton {
      width: 100%;
      padding-bottom: 56.25%;
      background-color: var(--bg-elev-1);
      border: 1px solid var(--border-color);
      position: relative;
      overflow: hidden;
    }

    .skeleton::after {
      content: '';
      position: absolute;
      inset: 0;
      background: linear-gradient(90deg,
          transparent 0%,
          rgba(255, 255, 255, 0.06) 50%,
          transparent 100%);
      animation: skeleton-shimmer 1.2s infinite;
    }
  }

  //标题和版权信息
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
      color: var(--font-primary-1);
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
      color: var(--font-primary-3);

      .icon {
        padding: 0 6px;
      }
    }
  }

  .video-toolbar {
    color: var(--font-primary-3);
    font-size: 13px;
    padding-bottom: 12px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    border-bottom: 1px solid var(--border-color);

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
      color: var(--font-primary-1);
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
        color: var(--font-primary-2);

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
    border-bottom: 1px solid var(--border-color);

    .tag {
      color: var(--font-primary-2);
      background: var(--border-color);
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

      &:hover {
        background: var(--hover-bg);
        color: var(--font-primary-1);
      }
    }
  }
}

@keyframes skeleton-shimmer {
  0% {
    transform: translateX(-100%);
  }

  100% {
    transform: translateX(100%);
  }
}

//右侧内容区域
.right-column {
  width: 340px;
  margin-left: 30px;
  z-index: 1;

  // 新增：弹幕列表和分集板块间距
  .danmaku-list-container {
    margin-bottom: 18px;
  }
}
</style>
