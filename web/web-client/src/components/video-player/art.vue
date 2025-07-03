<template>
  <div class="player-container">
    <div id="artplayer" ref="playerContainer"></div>
    <div class="danmaku-send">
      <DanmakuSend
        ref="danmakuSendRef"
        @send="sendDanmaku"
        @change-show="changeShow"
        @opacity-change="opacityChange"
        @set-filter="filterDanmaku"
      />
    </div>
    <div class="danmaku-list">
      <DanmakuList ref="danmakuListRef" :height="danmakuListHeight" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, watch, onBeforeUnmount } from 'vue';
import Artplayer from 'artplayer';
import Hls from 'hls.js';
import artplayerPluginDanmuku from 'artplayer-plugin-danmuku';
import { getResourceQualityApi, getVideoFileUrl } from '@/api/video';
import { getDanmakuAPI, sendDanmakuAPI } from '@/api/danmaku';
import { addHistoryAPI, getHistoryProgressAPI } from '@/api/history';
import DanmakuSend from './components/DanmakuSend.vue';
import DanmakuList from '@/pages/video/components/DanmakuList.vue';
import { nextTick } from 'vue';

const props = defineProps<{
  videoInfo: VideoType;
  part: number;
  progress: number | null;
}>();

const playerContainer = ref<HTMLElement | null>(null);
let player: any = null;
let originalDanmaku: DanmakuType[] = [];
const danmakuSendRef = ref<InstanceType<typeof DanmakuSend> | null>(null);
let timer: number | null = null;
let pendingSeek: number | null = null;
// 进度恢复只用远程
let remoteResumeTime = 0;

const videoId = props.videoInfo.vid;
const part = props.part;
const artId = `${videoId}_${part}`; // 唯一id

const resourceNameMap: Record<string, string> = {
  "640x360_1000k_30": "360p",
  "854x480_1500k_30": "480p",
  "1280x720_3000k_30": "720p",
  "1920x1080_6000k_30": "1080p",
  "1920x1080_8000k_60": "1080p60",
};

const danmakuListRef = ref<InstanceType<typeof DanmakuList> | null>(null);
const danmakuListHeight = 300;

function guessType(url: string, qualityItem?: any) {
  if (
    url.includes('/api/v1/video/getVideoFile') ||
    (qualityItem && qualityItem.type === 'hls')
  ) {
    return 'm3u8';
  }
  if (url.endsWith('.m3u8')) return 'm3u8';
  if (url.endsWith('.mp4')) return 'mp4';
  if (url.endsWith('.flv')) return 'flv';
  return 'mp4';
}

const getQualities = (qualityList: string[], resourceId: number) => {
  const sorted = [...qualityList].sort((a, b) => {
    const wa = parseInt(a.split('x')[0], 10);
    const wb = parseInt(b.split('x')[0], 10);
    if (wb !== wa) return wb - wa;
    const fpsA = parseInt(a.split('_').pop() || '0', 10);
    const fpsB = parseInt(b.split('_').pop() || '0', 10);
    return fpsB - fpsA;
  });
  return sorted.map((item, idx) => ({
    default: idx === 0,
    html: resourceNameMap[item] || item,
    url: getVideoFileUrl(resourceId, item),
  }));
};

const setDanmaku = (data: DanmakuType[]) => {
  originalDanmaku = data;
  danmakuSendRef.value?.updateDanmakuCount(data.length);
  danmakuListRef.value?.setDanmaku(data);
};

const loadDanmaku = async () => {
  const vid = props.videoInfo.vid;
  const part = props.part;
  const res = await getDanmakuAPI(vid, part);
  originalDanmaku = res.data.code === 200 && Array.isArray(res.data.data.danmaku)
    ? res.data.data.danmaku.map((d: any) => ({
        ...d,
        mode: d.type, // 兼容 ArtPlayer 弹幕插件的 mode 字段
      }))
    : [];
  nextTick(() => {
    danmakuSendRef.value?.updateDanmakuCount(originalDanmaku.length);
    danmakuListRef.value?.setDanmaku(originalDanmaku);
  });
};

// 发送弹幕
const sendDanmaku = (danmakuForm: DrawDanmakuType) => {
  if (!danmakuForm.text) {
    player.notice?.("弹幕内容不能为空");
    return;
  }
  // 兼容 color 格式
  if (!/^#([0-9a-fA-F]{6})$/.test(danmakuForm.color)) {
    danmakuForm.color = `#${danmakuForm.color.replace(/^#?/, '')}`;
  }
  // 兼容 mode/type 字段
  const danmu = {
    ...danmakuForm,
    mode: danmakuForm.type,
    time: player.currentTime || 0,
  };
  // 发送到 ArtPlayer 弹幕插件
  player.plugins.artplayerPluginDanmuku?.send?.(danmu);
  // 发送到后端
  sendDanmakuAPI({
    ...danmu,
    vid: props.videoInfo.vid,
    part: props.part as any,
  } as any);
};

// 显隐弹幕
const changeShow = (val: boolean) => {
  player.plugins.artplayerPluginDanmuku?.show?.(val);
};

// 弹幕透明度
const opacityChange = (val: number) => {
  player.plugins.artplayerPluginDanmuku?.opacity?.(val);
};

// 弹幕过滤
const filterDanmaku = (filter: FilterDanmakuType) => {
  // 可根据 filter 过滤 originalDanmaku 并更新
  // player.plugins.artplayerPluginDanmuku?.update?.(filteredDanmaku);
};

const getRemoteProgress = async () => {
  const res = await getHistoryProgressAPI(videoId, part);
  if (res.data && typeof res.data.data?.progress === 'number') {
    return res.data.data.progress;
  }
  return 0;
};

const initPlayer = async () => {
  const container = playerContainer.value;
  if (!container) return;
  if (player) return;

  const resource = props.videoInfo.resources[props.part - 1];
  const res = await getResourceQualityApi(resource.id);
  let qualities = [];
  if (res.data.code === 200 && res.data.data.quality.length > 0) {
    qualities = getQualities(res.data.data.quality, resource.id);
  } else {
    qualities = [{ default: true, html: '默认', url: resource.url }];
  }

  await loadDanmaku();

  const type = guessType(qualities[0].url, qualities[0]);
  const loopInit = localStorage.getItem('artplayer-loop') === '1';

  // 只查远程进度
  remoteResumeTime = await getRemoteProgress();
  if (remoteResumeTime > 0) {
    pendingSeek = remoteResumeTime;
  }

  player = new Artplayer({
    container,
    url: qualities[0].url,
    quality: qualities,
    type,
    isLive: false,
    autoplay: true,
    volume: 0.8,
    fullscreen: true,
    setting: true,
    playbackRate: true,
    aspectRatio: true,
    screenshot: true,
    hotkey: true,
    pip: true,
    theme: '#2196f3',
    loop: loopInit,
    id: artId,
    //autoPlayback: true, // 自动本地恢复（已不用）
    settings: [
      {
        html: '循环播放',
        icon: '<svg viewBox="0 0 1024 1024" width="20" height="20"><path d="M512 64C264.6 64 64 264.6 64 512h64c0-211.7 172.3-384 384-384s384 172.3 384 384-172.3 384-384 384c-70.7 0-137.2-19.2-194.1-52.6l90.1-90.1c12.5-12.5 12.5-32.8 0-45.3s-32.8-12.5-45.3 0l-144 144c-12.5 12.5-12.5 32.8 0 45.3l144 144c12.5 12.5 32.8 12.5 45.3 0s12.5-32.8 0-45.3l-90.1-90.1C374.8 924.8 443.2 944 512 944c247.4 0 448-200.6 448-448S759.4 64 512 64z" fill="#2196f3"/></svg>',
        switch: loopInit,
        onSwitch(item: any, $item: any, event: any): boolean {
          const newLoop = !item.switch;
          localStorage.setItem('artplayer-loop', newLoop ? '1' : '0');
          // @ts-expect-error
          this.option.loop = newLoop;
          // @ts-expect-error
          if (this.video) this.video.loop = newLoop;
          // @ts-expect-error
          return this.option.loop;
        },
        name: 'loop-setting',
      },
    ],
    layers: [
      // 可在此添加自定义 Layer，如标题、UP主信息等
    ],
    customType: {
      m3u8: function (video: HTMLVideoElement, url: string) {
        if (Hls.isSupported()) {
          const hls = new Hls();
          hls.loadSource(url);
          hls.attachMedia(video);
        } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
          video.src = url;
        }
      },
    },
    plugins: [
      artplayerPluginDanmuku({
        danmuku: originalDanmaku,
        speed: 5,
        margin: [10, '25%'],
        opacity: 1,
        mode: 0,
        modes: [0, 1, 2],
        fontSize: 25,
        antiOverlap: true,
        synchronousPlayback: false,
        heatmap: true,
        width: 512,
        points: [],
        filter: (danmu: any) => (danmu.text || danmu.content || '').length <= 100,
        beforeVisible: () => true,
        visible: true,
        emitter: false, // 禁用弹幕发送
        maxLength: 200,
        lockTime: 5,
        theme: 'dark',
        beforeEmit(danmu: any) {
          return new Promise((resolve) => {
            setTimeout(() => {
              resolve(true);
            }, 300);
          });
        },
      }),
    ],
  });

  player.on('ready', () => {
    if (pendingSeek != null) {
      player.seek(pendingSeek);
      pendingSeek = null;
    }
  });

  player.on('quality', (item: any) => {
    const newType = guessType(item.url, item);
    if (player && player.switchUrl) {
      player.switchUrl(item.url, newType);
    }
  });

  player.on('loadedmetadata', () => {
    if (pendingSeek != null) {
      player.seek(pendingSeek);
      pendingSeek = null;
    }
  });

  // 定时/暂停/结束时同步远程
  const syncRemote = () => {
    addHistoryAPI({ vid: videoId, part, time: player.currentTime });
  };
  player.on('video:pause', syncRemote);
  player.on('video:ended', syncRemote);
  timer = window.setInterval(syncRemote, 10000);
};

onMounted(() => {
  initPlayer();
});

onBeforeUnmount(() => {
  if (timer) clearInterval(timer);
});

defineExpose({
  setDanmaku
});
</script>

<style scoped>
.player-container {
  height: 0;
  width: 100%;
  padding-bottom: 56.25%;
  position: relative;
  margin-bottom: 40px;

  #artplayer {
    width: 100%;
    height: 100%;
    position: absolute;
    background-color: black;
  }

  .danmaku-send {
    position: absolute;
    width: 100%;
    bottom: -40px;
  }
  .danmaku-list {
    margin-top: 20px;
  }
}
</style> 