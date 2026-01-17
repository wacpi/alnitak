<template>
  <div id="artplayer" ref="playerContainer"></div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import Artplayer from 'artplayer';
import Hls from 'hls.js';
import artplayerPluginDanmuku from 'artplayer-plugin-danmuku';
import { getResourceQualityApi, getVideoFileUrl } from '@/api/video';
import { getDanmakuAPI } from '@/api/danmaku';

const props = defineProps<{
  videoInfo: VideoType;
  part: number;
  progress: number | null;
}>();

const playerContainer = ref<HTMLElement | null>(null);
let player: any = null;
let originalDanmaku: DanmakuType[] = [];

const resourceNameMap: Record<string, string> = {
  "640x360_1000k_30": "360p",
  "854x480_1500k_30": "480p",
  "1280x720_3000k_30": "720p",
  "1920x1080_6000k_30": "1080p",
  "1920x1080_8000k_60": "1080p60",
};

/**
 * 根据清晰度字符串动态生成显示名称
 * 格式: "宽x高_码率k_帧率" 例如 "854x480_900k_30" 或 "1920x1080_6000k_60"
 */
const getQualityDisplayName = (qualityStr: string): string => {
  // 如果映射表中存在，直接返回
  if (resourceNameMap[qualityStr]) {
    return resourceNameMap[qualityStr];
  }

  try {
    // 解析格式: "宽x高_码率k_帧率"
    const parts = qualityStr.split('_');
    const resolution = parts[0]; // "854x480"
    const fpsStr = parts[parts.length - 1]; // "30"、"60"、"24"、"50" 等任意帧率值
    const fps = parseInt(fpsStr, 10); // 转换为数字

    if (resolution.includes('x')) {
      const [width, height] = resolution.split('x').map(Number);
      
      // 根据高度判断清晰度，并根据实际帧率动态生成后缀
      const fpsSuffix = fps > 30 ? fps.toString() : '';
      
      if (height <= 360) {
        return fpsSuffix ? `360p${fpsSuffix}` : '360p';
      } else if (height <= 480) {
        return fpsSuffix ? `480p${fpsSuffix}` : '480p';
      } else if (height <= 720) {
        return fpsSuffix ? `720p${fpsSuffix}` : '720p';
      } else if (height <= 1080) {
        return fpsSuffix ? `1080p${fpsSuffix}` : '1080p';
      } else if (height <= 1440) {
        return fpsSuffix ? `1440p${fpsSuffix}` : '1440p';
      } else if (height <= 2160) {
        return fpsSuffix ? `4K${fpsSuffix}` : '4K';
      } else {
        // 其他分辨率，显示实际分辨率或高度
        return fpsSuffix ? `${height}p${fpsSuffix}` : `${height}p`;
      }
    }
  } catch (error) {
    console.warn('Failed to parse quality string:', qualityStr, error);
  }

  // 解析失败，返回原始字符串（去掉部分后缀使其更简洁）
  return qualityStr.split('_')[0] || qualityStr;
};

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
    html: getQualityDisplayName(item),  // 使用自适配函数
    url: getVideoFileUrl(resourceId, item),
  }));
};

const loadDanmaku = async () => {
  const vid = props.videoInfo.vid;
  const part = props.part;
  const res = await getDanmakuAPI(vid, part);
  originalDanmaku = res.data.code === 200 && Array.isArray(res.data.data.danmaku)
    ? res.data.data.danmaku.map((d: any) => ({
        ...d,
        mode: d.type,
      }))
    : [];
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

  // 读取本地循环播放初始状态
  const loopInit = localStorage.getItem('artplayer-loop') === '1';

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
    autoPlayback: true,
    screenshot: true,
    hotkey: true,
    pip: true,
    theme: '#2196f3',
    loop: loopInit,
  settings: [
    {
      html: '循环播放',
      icon: '<svg viewBox="0 0 1024 1024" width="20" height="20"><path d="M512 64C264.6 64 64 264.6 64 512h64c0-211.7 172.3-384 384-384s384 172.3 384 384-172.3 384-384 384c-70.7 0-137.2-19.2-194.1-52.6l90.1-90.1c12.5-12.5 12.5-32.8 0-45.3s-32.8-12.5-45.3 0l-144 144c-12.5 12.5-12.5 32.8 0 45.3l144 144c12.5 12.5 32.8 12.5 45.3 0s12.5-32.8 0-45.3l-90.1-90.1C374.8 924.8 443.2 944 512 944c247.4 0 448-200.6 448-448S759.4 64 512 64z" fill="#2196f3"/></svg>',
      switch: loopInit,
      onSwitch(item: any, $item: any, event: any): boolean {
        const newLoop = !item.switch;
        localStorage.setItem('artplayer-loop', newLoop ? '1' : '0');

        // 直接操作 player 对象
        if (player && player.video) {
          player.video.loop = newLoop;  // 设置视频循环状态
        }

        return newLoop;  // 返回新状态
      },
      name: 'loop-setting',  // 这里可以保留 name 属性
    },
  ],
    layers: [
      // ...Layer 配置...
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
		//弹幕输入框
        emitter: false,
		
        maxLength: 200,
        lockTime: 5,
        theme: 'dark',
        beforeEmit(danmu: any) {
          return new Promise((resolve) => {
            console.log(danmu);
            setTimeout(() => {
              resolve(true);
            }, 300);
          });
        },
      }),
    ],
  });
//弹幕输入框
//  player.on('ready', () => {
//    const danmakuInput = player.container.querySelector('.apd-input');
//    if (danmakuInput) {
//      danmakuInput.setAttribute('id', 'artplayer-danmaku-input');
//      danmakuInput.setAttribute('name', 'artplayerDanmaku');
//    }
//  });

  player.on('quality', (item: any) => {
    const newType = guessType(item.url, item);
    if (player && player.switchUrl) {
      player.switchUrl(item.url, newType);
    }
  });
};

onMounted(() => {
  initPlayer();
});
</script>

<style scoped>
#artplayer {
  height: 100vh;
  width: 100vw;
  margin: 0;
  padding: 0;
}
</style>
