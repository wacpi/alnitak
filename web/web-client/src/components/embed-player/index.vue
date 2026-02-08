<template>
  <div id="wplayer" ref="playerContainer"></div>
</template>

<script setup lang="ts">
import { onMounted, ref, watch, nextTick, onBeforeUnmount } from 'vue';
import Hls from 'hls.js';
import * as dashjs from 'dashjs';
import Wplayer from 'wplayer-next';
import { getResourceQualityApi, getVideoFileUrl, getVideoFileUrlDash } from '@/api/video';
import { getDanmakuAPI } from '@/api/danmaku';
import {
  createHlsPlayer,
  destroyHlsPlayer,
  getSavedPlaybackState,
  setupVolumePersistence,
  getSavedVolumeState,
  type HlsPlayerState,
} from '@/utils/hls-player';

const props = defineProps<{
  videoInfo: VideoType;
  part: number;
  progress: number | null;
}>();

console.log('[embed-player] props:', props);

const playerContainer = ref<HTMLElement | null>(null);
let player: any = null;
let dashPlayer: any = null;
let originalDanmaku: DanmakuType[] = [];
const hlsPlayerState: HlsPlayerState = { instance: null, videoElement: null, playPromise: null };

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
      // 标准帧率(30fps)不显示后缀，高帧率(>30)显示帧率后缀
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

  // 检测是否支持 DASH
  const supportDash = supportsDashJs()

  const mapped = sorted.map((item) => ({
    name: getQualityDisplayName(item),
    url: supportDash ? getVideoFileUrlDash(resourceId, item) : getVideoFileUrl(resourceId, item),
  }));
  console.log('[embed-player] qualities mapped:', mapped);
  return { qualities: mapped, supportDash };
};

// 检测是否支持 dash.js
const supportsDashJs = (): boolean => {
  const video = document.createElement('video')
  return !!(
    (window as any).MediaSource ||
    video.canPlayType('application/dash+xml') !== '' ||
    (window as any).dashjs !== undefined
  )
}

const loadDanmaku = async () => {
  const vid = props.videoInfo.vid;
  const part = props.part;
  console.log('[embed-player] loadDanmaku start', vid, part);
  const res = await getDanmakuAPI(vid, part);
  console.log('[embed-player] getDanmakuAPI result:', res);
  setDanmaku(res.data.code === 200 && Array.isArray(res.data.data.danmaku) ? res.data.data.danmaku : []);
  injectDanmaku();
}

// 新增：获取 URL 参数
function getQueryParam(name: string): string | null {
  const url = window.location.href;
  name = name.replace(/[\[\]]/g, '\\$&');
  const regex = new RegExp('[?&]' + name + '(=([^&#]*)|&|#|$)');
  const results = regex.exec(url);
  if (!results) return null;
  if (!results[2]) return '';
  return decodeURIComponent(results[2].replace(/\+/g, ' '));
}

const autoplayParam = getQueryParam('autoplay');
const mutedParam = getQueryParam('muted');
const shouldAutoplay = autoplayParam === '1' || autoplayParam === 'true';
const shouldMuted = mutedParam === '1' || mutedParam === 'true';

const initPlayer = async () => {
  const container = playerContainer.value;
  console.log('[embed-player] initPlayer, container:', container);
  if (!container) return;
  if (player) return;

  // 防御性检查：确保 videoInfo 和 resources 存在
  if (!props.videoInfo?.resources?.length) {
    console.warn('[embed-player] videoInfo.resources is empty or undefined');
    return;
  }

  const resource = props.videoInfo.resources[props.part - 1];
  if (!resource?.id) {
    console.warn('[embed-player] resource not found for part:', props.part);
    return;
  }

  // 确保 Hls.js 在全局可用
  if (!(window as any).Hls) {
    (window as any).Hls = Hls;
    console.log('[embed-player] Hls.js injected to window');
  }

  console.log('[embed-player] resource:', resource);
  const res = await getResourceQualityApi(resource.id);
  console.log('[embed-player] getResourceQualityApi result:', res);
  let qualities: any[] = [];
  let supportDash = false;
  if (res.data.code === 200 && res.data.data.quality.length > 0) {
    const result = getQualities(res.data.data.quality, resource.id);
    qualities = result.qualities;
    supportDash = result.supportDash;
  } else {
    qualities = [{ name: '默认', url: resource.url }];
    supportDash = false;
  }

  console.log('[embed-player] Wplayer qualities', qualities, 'supportDash:', supportDash);
  /* === 播放器实例化片段 start === */
  player = new Wplayer({
    container,
    video: {
      quality: qualities,
      defaultQuality: 0,
      autoplay: shouldAutoplay,
      controls: ["play", "progress", "volume", "quality", "fullscreen"],
      type: supportDash ? 'customDash' : 'customHls',
      customType: {
        customHls: function (video: HTMLVideoElement) {
          console.log('[embed-player] customHls called', video.src);
          const savedVolumeState = getSavedVolumeState();
          const playbackState = getSavedPlaybackState(video);
          const volumeState = {
            volume: playbackState.currentTime > 0 ? playbackState.volume : savedVolumeState.volume,
            muted: playbackState.currentTime > 0 ? playbackState.muted : savedVolumeState.muted,
          };

          setupVolumePersistence(video);

          createHlsPlayer(
            video,
            video.src,
            hlsPlayerState,
            { ...playbackState, volume: volumeState.volume, muted: volumeState.muted },
            {
              maxBufferLength: 30,
              maxMaxBufferLength: 60,
              onError: (event, data) => {
                console.error('[embed-player] HLS 错误:', event, data);
              },
            }
          );
        },
        customDash: function (video: HTMLVideoElement) {
          console.log('[embed-player] customDash called', video.src);
          if (dashPlayer) {
            dashPlayer.reset();
            dashPlayer = null;
          }
          dashPlayer = dashjs.MediaPlayer().create();
          dashPlayer.initialize(video, video.src, false);
          dashPlayer.on(dashjs.MediaPlayer.events.ERROR, (_e: any, err: any) => {
            console.error('[embed-player] DASH 播放错误:', err);
          });
        },
      },
    },
    danmaku: { show: true },
    preload: "auto",
    volume: shouldMuted ? 0 : 0.8,
    muted: shouldMuted,
  });
  /* === 播放器实例化片段 end === */
  console.log('[embed-player] Wplayer instance', player);

  // 新增：强制设置 video 元素属性，确保自动静音播放生效
  setTimeout(() => {
    const videoEl = player?.video;
    if (videoEl) {
      videoEl.muted = shouldMuted;
      videoEl.volume = shouldMuted ? 0 : 0.8;
      if (shouldAutoplay && typeof videoEl.play === 'function') {
        videoEl.play();
      }
    }
  }, 300);

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
  nextTick(() => {
    if (props.videoInfo?.resources?.length) {
      initPlayer();
    }
  });
});

// 组件卸载时清理
onBeforeUnmount(() => {
  if (player) {
    player.destroy();
    player = null;
  }
  destroyHlsPlayer(hlsPlayerState);
  if (dashPlayer) {
    dashPlayer.reset();
    dashPlayer = null;
  }
});

// 监听 videoInfo 变化，当数据加载完成后初始化播放器
watch(
  () => props.videoInfo,
  (newVal) => {
    console.log('[embed-player] watch videoInfo changed:', newVal);
    if (newVal?.resources?.length && !player && playerContainer.value) {
      nextTick(() => {
        initPlayer();
      });
    }
  },
  { immediate: true, deep: true }
);
</script>

<style scoped>
#wplayer {
  height: 100vh;
  width: 100vw;
  margin: 0;
  padding: 0;
}
</style> 
