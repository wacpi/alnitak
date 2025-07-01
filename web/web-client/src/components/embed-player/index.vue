<template>
  <div id="wplayer" ref="playerContainer"></div>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue';
import Hls from 'hls.js';
import Wplayer from 'wplayer-next';
import { getResourceQualityApi, getVideoFileUrl } from '@/api/video';
import { getDanmakuAPI } from '@/api/danmaku';

const props = defineProps<{
  videoInfo: VideoType;
  part: number;
  progress: number | null;
}>();

console.log('[embed-player] props:', props);

const playerContainer = ref<HTMLElement | null>(null);
let player: any = null;
let originalDanmaku: DanmakuType[] = [];

const setDanmaku = (data: DanmakuType[]) => {
  originalDanmaku = Array.isArray(data) ? data : [];
}

const injectDanmaku = () => {
  if (player && player.danmaku) {
    player.danmaku.update(Array.isArray(originalDanmaku) ? originalDanmaku : []);
    player.danmaku.show();
    console.log('[embed-player] danmaku injected:', originalDanmaku.length);
  }
}

const resourceNameMap: Record<string, string> = {
  "640x360_500k_30": "360p",
  "854x480_900k_30": "480p",
  "1080x720_2000k_30": "720p",// 兼容之前的错误Add commentMore actions
  "1280x720_2000k_30": "720p",
  "1920x1080_3000k_30": "1080p",
  "1920x1080_6000k_60": "1080p60",
};

const getQualities = (qualityList: string[], resourceId: number) => {
  console.log('[embed-player] getQualities', qualityList, resourceId);
  // 主站同款排序
  const sorted = [...qualityList].sort((a, b) => {
    const wa = parseInt(a.split('x')[0], 10);
    const wb = parseInt(b.split('x')[0], 10);
    if (wb !== wa) return wb - wa;
    const fpsA = parseInt(a.split('_').pop() || '0', 10);
    const fpsB = parseInt(b.split('_').pop() || '0', 10);
    return fpsB - fpsA;
  });
  const mapped = sorted.map((item) => ({
    name: resourceNameMap[item] || item,
    url: getVideoFileUrl(resourceId, item),
    type: 'hls',
  }));
  console.log('[embed-player] qualities mapped:', mapped);
  return mapped;
};

const loadDanmaku = async () => {
  const vid = props.videoInfo.vid;
  const part = props.part;
  console.log('[embed-player] loadDanmaku start', vid, part);
  const res = await getDanmakuAPI(vid, part);
  console.log('[embed-player] getDanmakuAPI result:', res);
  setDanmaku(res.data.code === 200 && Array.isArray(res.data.data.danmaku) ? res.data.data.danmaku : []);
  injectDanmaku();
}

const initPlayer = async () => {
  const container = playerContainer.value;
  console.log('[embed-player] initPlayer, container:', container);
  if (!container) return;
  if (player) return;

  // 确保 Hls.js 在全局可用
  if (!(window as any).Hls) {
    (window as any).Hls = Hls;
    console.log('[embed-player] Hls.js injected to window');
  }

  const resource = props.videoInfo.resources[props.part - 1];
  console.log('[embed-player] resource:', resource);
  const res = await getResourceQualityApi(resource.id);
  console.log('[embed-player] getResourceQualityApi result:', res);
  let qualities = [];
  if (res.data.code === 200 && res.data.data.quality.length > 0) {
    qualities = getQualities(res.data.data.quality, resource.id);
  } else {
    qualities = [{ name: '默认', url: resource.url, type: 'hls' }];
  }

  console.log('[embed-player] Wplayer qualities', qualities);
  /* === 播放器实例化片段 start === */
  player = new Wplayer({
    container,
    video: {
      quality: qualities,
      defaultQuality: 0,
      autoplay: false,
      controls: ["play", "progress", "volume", "quality", "fullscreen"],
      type: 'customHls',
      customType: {
        customHls: function (video: HTMLVideoElement) {
          console.log('[embed-player] customHls called', video.src);
          if ((window as any)._mainHls) {
            (window as any)._mainHls.destroy();
            (window as any)._mainHls = null;
            console.log('[embed-player] old _mainHls destroyed');
          }
          (window as any)._mainHls = new Hls();
          (window as any)._mainHls.loadSource(video.src);
          (window as any)._mainHls.attachMedia(video);
          (window as any)._mainHls.on(Hls.Events.ERROR, () => {
            console.error("[embed-player] 资源加载失败");
          });
        },
      },
    },
    danmaku: { show: true },
    //theme: "#ff5c5c",
    preload: "auto",
    volume: 0.8,
  });
  /* === 播放器实例化片段 end === */
  console.log('[embed-player] Wplayer instance', player);

  player.on('loadedmetadata', () => {
    console.log('[embed-player] player loadedmetadata');
    injectDanmaku();
  });

  // 加载弹幕
  await loadDanmaku();
  console.log('[embed-player] loadDanmaku finished');
};

onMounted(() => {
  console.log('[embed-player] onMounted');
  initPlayer();
});
</script>

<style scoped>
#wplayer {
  height: 100vh;
  width: 100vw;
  margin: 0;
  padding: 0;
}
</style> 
