<template>
  <div class="embed-video-container">
    <div class="player-box" @mouseenter="showOverlay" @mouseleave="hideOverlay">
      <div class="player-info-overlay" v-if="videoInfo" v-show="infoOverlayVisible" @mouseenter="showOverlay" @mouseleave="hideOverlay">
        <a class="title-link" :href="`/video/${videoInfo.vid}${currentPart > 1 ? `?p=${currentPart}` : ''}`" target="_blank">{{ videoInfo.title }}</a>
        <div class="up-info">
          <img class="avatar" :src="getResourceUrl(videoInfo.author.avatar)" :alt="videoInfo.author.name" />
          <a class="up-name" :href="`/user/${videoInfo.author.uid}`" target="_blank">{{ videoInfo.author.name }}</a>
        </div>
      </div>
      <client-only>
        <embed-player v-if="videoInfo" :video-info="videoInfo" :part="currentPart" :progress="pendingProgress" />
      </client-only>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, nextTick, computed, watchEffect } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import EmbedPlayer from '@/components/embed-player/index.vue';
import { asyncGetVideoInfoAPI } from '@/api/video';
import { getHistoryProgressAPI } from '@/api/history';
import { getDanmakuAPI } from '@/api/danmaku';
import { getResourceUrl } from '@/utils/resource';

const route = useRoute();
const router = useRouter();
const videoInfo = ref<VideoType>();
const videoId = route.params.id.toString();
const currentPart = ref(Number(route.query.p) || 1);
const pendingProgress = ref<number | null>(null);
const playerRef = ref();
const playerReady = ref(false);
const infoOverlayVisible = ref(false);
let hoverCount = 0;

// 获取视频信息
const { data } = await asyncGetVideoInfoAPI(videoId);
if ((data.value as any).code === 200) {
  videoInfo.value = (data.value as any).data.video as VideoType;
} else {
  router.replace('/404');
}

// 校验分集参数
if (route.query.p && Number(route.query.p) > (videoInfo.value?.resources.length || 1)) {
  router.replace({ path: `/embed/video/${videoId}`, query: { p: 1 } });
}

// 加载弹幕
const loadDanmaku = async () => {
  if (!videoInfo.value) return;
  const res = await getDanmakuAPI(videoInfo.value.vid, currentPart.value);
  if (res.data.code === 200) {
    playerRef.value?.setDanmaku(res.data.data);
  }
};

// 播放器 ready 回调
const onPlayerReady = () => {
  if (pendingProgress.value !== null && playerRef.value?.seek) {
    playerRef.value.seek(pendingProgress.value);
    pendingProgress.value = null;
  }
  playerReady.value = true;
  loadDanmaku();
};

// 路由监听
watch(() => route.query.p, (newP) => {
  const partNum = Number(newP) || 1;
  if (videoInfo.value?.resources[partNum - 1]) {
    currentPart.value = partNum;
  } else {
    router.replace({ path: `/embed/video/${videoId}`, query: { p: 1 } });
  }
});

// 播放器 ref 变更
watch(playerRef, (val) => {
  if (val?.setOnReady) {
    val.setOnReady(onPlayerReady);
  }
});

// 弹幕显隐绑定 infoOverlay（可选）
watchEffect(() => {
  infoOverlayVisible.value = playerRef.value?.controlsVisible !== false;
});

// 鼠标控制
const showOverlay = () => {
  hoverCount++;
  infoOverlayVisible.value = true;
};
const hideOverlay = () => {
  hoverCount--;
  if (hoverCount <= 0) {
    infoOverlayVisible.value = false;
    hoverCount = 0;
  }
};
</script>


<style scoped>
html, body, #__nuxt, .embed-video-container, .player-box, .player-container, .player, #dplayer {
  width: 100vw;
  height: 100vh;
  min-width: 0;
  min-height: 0;
  margin: 0;
  padding: 0;
  background: #000;
  overflow: hidden;
  position: relative;
}
.embed-video-container {
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
}
.player-box {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100vw;
  height: 100vh;
}
.player-container, .player, #dplayer {
  width: 100%;
  height: 100%;
  min-width: 0;
  min-height: 0;
}
video {
  width: 100% !important;
  height: 100% !important;
  object-fit: contain;
  background: #000;
  display: block;
}
.player-info-overlay {
  position: absolute;
  top: 18px;
  left: 18px;
  background: transparent !important;
  border-radius: 8px;
  padding: 8px 16px 8px 12px;
  display: flex;
  align-items: center;
  gap: 16px;
  z-index: 2;
  transition: opacity 0.3s;
  box-shadow: none !important;
}
.player-info-overlay .title-link {
  color: #fff;
  font-size: 15px;
  font-weight: 700;
  max-width: 320px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-decoration: none;
  margin-right: 16px;
  transition: color 0.2s, opacity 0.2s;
  text-shadow: none !important;
  opacity: 0.85;
}
.player-info-overlay .title-link:hover {
  color: #fff;
  opacity: 1;
  text-shadow: none;
  text-decoration: none;
}
.player-info-overlay .up-info {
  display: flex;
  align-items: center;
  gap: 8px;
}
.player-info-overlay .avatar {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  object-fit: cover;
  background: #eee;
}
.player-info-overlay .up-name {
  color: #fff;
  font-size: 13px;
  font-weight: 400;
  text-decoration: none;
  opacity: 0.85;
}
.player-info-overlay .up-name:hover {
  text-decoration: underline;
  opacity: 1;
}
</style> 