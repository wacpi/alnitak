<template>
  <div id="artplayer" ref="playerContainer"></div>
</template>

<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, watch, nextTick } from 'vue';
import Artplayer from 'artplayer';
import Hls from 'hls.js';
import * as dashjs from 'dashjs';
import artplayerPluginDanmuku from 'artplayer-plugin-danmuku';
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

const playerContainer = ref<HTMLElement | null>(null);
let player: any = null;
let dashPlayer: any = null;
let hlsPlayerState: HlsPlayerState = { instance: null, videoElement: null, playPromise: null };
let originalDanmaku: DanmakuType[] = [];

// ===== 前端日志回传 =====
const _logBuffer: string[] = [];
let _logTimer: ReturnType<typeof setTimeout> | null = null;

function clog(...args: any[]) {
  const msg = args.map(a => (typeof a === 'object' ? JSON.stringify(a) : String(a))).join(' ');
  console.log(msg);
  _logBuffer.push(`[${new Date().toISOString()}] ${msg}`);
  if (!_logTimer) {
    _logTimer = setTimeout(flushLogs, 2000);
  }
}

function flushLogs() {
  _logTimer = null;
  if (_logBuffer.length === 0) return;
  const logs = _logBuffer.splice(0);
  fetch('/api/v1/video/clientLog', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ logs }),
  }).catch(() => {});
}
// ===========================

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
  if (qualityItem?.type === 'dash') {
    return 'dash';
  }
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

const getQualities = (qualityList: string[], resourceId: number, serverSupportsDash: boolean) => {
  const sorted = [...qualityList].sort((a, b) => {
    const wa = parseInt(a.split('x')[0], 10);
    const wb = parseInt(b.split('x')[0], 10);
    if (wb !== wa) return wb - wa;
    const fpsA = parseInt(a.split('_').pop() || '0', 10);
    const fpsB = parseInt(b.split('_').pop() || '0', 10);
    return fpsB - fpsA;
  });

  // 必须浏览器支持且服务器资源支持才使用 DASH
  const useDash = supportsDashJs() && serverSupportsDash;
  // 读取保存的清晰度偏好
  const savedQuality = localStorage.getItem('artplayer-quality');

  return sorted.map((item, idx) => {
    const displayName = getQualityDisplayName(item);
    return {
      // 如果有保存的清晰度且匹配，则设为默认；否则使用第一个
      default: savedQuality ? displayName === savedQuality : idx === 0,
      html: displayName,
      url: useDash ? getVideoFileUrlDash(resourceId, item) : getVideoFileUrl(resourceId, item),
      type: useDash ? 'dash' : 'm3u8',
    };
  });
};

// 检测是否为 Safari 或 iOS 设备（它们不完整支持 MSE / dashjs）
const isSafariOrIOS = (): boolean => {
  const ua = navigator.userAgent;
  if (/iPad|iPhone|iPod/.test(ua)) return true;
  if (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1) return true;
  if (/Safari/.test(ua) && !/Chrome|CriOS|FxiOS|Edg/.test(ua)) return true;
  return false;
};

const supportsDashJs = (): boolean => {
  if (isSafariOrIOS()) return false;
  return !!(
    (window as any).MediaSource ||
    (window as any).ManagedMediaSource
  );
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
  clog('[art] initPlayer called, isSafariOrIOS=', isSafariOrIOS(), 'supportsDash=', supportsDashJs());
  const container = playerContainer.value;
  if (!container) {
    clog('[art] container is null, abort');
    return;
  }
  if (player) {
    clog('[art] player already exists, abort');
    return;
  }

  // 防御性检查：确保 videoInfo 和 resources 存在
  if (!props.videoInfo?.resources?.length) {
    console.warn('[art.vue] videoInfo.resources is empty or undefined');
    return;
  }

  const resource = props.videoInfo.resources[props.part - 1];
  console.log('[art.vue] resource:', resource);
  if (!resource?.id) {
    console.warn('[art.vue] resource not found for part:', props.part);
    return;
  }
  clog('[art] calling getResourceQualityApi, id=', resource.id);
  const res = await getResourceQualityApi(resource.id);
  clog('[art] quality result, code=', res.data.code, 'count=', res.data.data?.quality?.length);
  let qualities = [];
  if (res.data.code === 200 && res.data.data.quality.length > 0) {
    // 从服务器获取是否支持 DASH（新资源 SegmentBase 模式）
    const serverSupportsDash = res.data.data.supportsDash === true;
    qualities = getQualities(res.data.data.quality, resource.id, serverSupportsDash);
  } else {
    qualities = [{ default: true, html: '默认', url: resource.url, type: 'm3u8' }];
  }
  clog('[art] qualities:', qualities.map((q: any) => q.html + '(' + q.type + ')').join(', '));

  await loadDanmaku();

  const type = guessType(qualities[0].url, qualities[0]);
  const isDash = type === 'dash';

  // 读取本地循环播放初始状态
  const loopInit = localStorage.getItem('artplayer-loop') === '1';

  player = new Artplayer({
    container,
    url: qualities[0].url,
    quality: qualities,
    type,
    isLive: false,
    autoplay: true,
    muted: false,
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
    // DASH 模式下禁用 artplayer 的 loop（它会 seek+play 导致 AbortError），
    // 改用原生 video.loop，dashjs 能正确处理
    loop: isDash ? false : loopInit,
  settings: [
    {
      html: '循环播放',
      icon: '<svg viewBox="0 0 1024 1024" width="20" height="20"><path d="M512 64C264.6 64 64 264.6 64 512h64c0-211.7 172.3-384 384-384s384 172.3 384 384-172.3 384-384 384c-70.7 0-137.2-19.2-194.1-52.6l90.1-90.1c12.5-12.5 12.5-32.8 0-45.3s-32.8-12.5-45.3 0l-144 144c-12.5 12.5-12.5 32.8 0 45.3l144 144c12.5 12.5 32.8 12.5 45.3 0s12.5-32.8 0-45.3l-90.1-90.1C374.8 924.8 443.2 944 512 944c247.4 0 448-200.6 448-448S759.4 64 512 64z" fill="#2196f3"/></svg>',
      switch: loopInit,
      onSwitch(item: any): boolean {
        const newLoop = !item.switch;
        localStorage.setItem('artplayer-loop', newLoop ? '1' : '0');

        if (player) {
          if (isDash) {
            // DASH 模式：切换手动循环标记
            dashLoopEnabled = newLoop;
          } else {
            // HLS 模式：用 artplayer 自身的 loop
            player.option.loop = newLoop;
          }
        }

        return newLoop;
      },
      name: 'loop-setting',  // 这里可以保留 name 属性
    },
  ],
    layers: [
      // ...Layer 配置...
    ],
customType: {
      m3u8: function (video: HTMLVideoElement, url: string) {
        clog('[m3u8] called, url=', url);
        clog('[m3u8] Hls.isSupported=', Hls.isSupported(), 'canPlayType=', video.canPlayType('application/vnd.apple.mpegurl'));
        const savedVolumeState = getSavedVolumeState();
        const playbackState = getSavedPlaybackState(video);
        const volumeState = {
          volume: playbackState.currentTime > 0 ? playbackState.volume : savedVolumeState.volume,
          muted: playbackState.currentTime > 0 ? playbackState.muted : savedVolumeState.muted,
        };

        setupVolumePersistence(video);

        if (Hls.isSupported()) {
          clog('[m3u8] using hls.js');
          createHlsPlayer(
            video,
            url,
            hlsPlayerState,
            { ...playbackState, volume: volumeState.volume, muted: volumeState.muted },
            {
              maxBufferLength: 30,
              maxMaxBufferLength: 60,
            }
          );
        } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
          clog('[m3u8] using native HLS (Safari/iOS)');
          video.src = url;
          // 监听原生 HLS 事件
          video.addEventListener('loadedmetadata', () => clog('[native-hls] loadedmetadata, duration=', video.duration));
          video.addEventListener('canplay', () => clog('[native-hls] canplay'));
          video.addEventListener('playing', () => clog('[native-hls] playing'));
          video.addEventListener('waiting', () => clog('[native-hls] waiting'));
          video.addEventListener('stalled', () => clog('[native-hls] stalled'));
          video.addEventListener('error', () => {
            const e = video.error;
            clog('[native-hls] error:', e?.code, e?.message);
          });
          if (playbackState.currentTime > 0) {
            video.currentTime = playbackState.currentTime;
          }
        }
      },
      dash: function (video: HTMLVideoElement, url: string) {
        if (dashPlayer) {
          try {
            const prevUrl = dashPlayer.getManifestUrl();
            if (prevUrl === url) {
              // 同一URL（循环播放），不重建 dashPlayer，让视频元素自行处理
              return;
            }
            console.log('[art.vue] DASH 切换URL:', prevUrl, '->', url);
            dashPlayer.attachSource(url);
            return;
          } catch {
          }
          dashPlayer.reset();
          dashPlayer = null;
        }

        if (dashjs && dashjs.MediaPlayer) {
          dashPlayer = dashjs.MediaPlayer().create();
          dashPlayer.updateSettings({
            streaming: {
              buffer: {
                bufferTimeDefault: 12,
                bufferTimeAtTopQuality: 30,
                bufferTimeAtTopQualityLongForm: 60,
                bufferPruningInterval: 10,
                bufferToKeep: 20,
              },
            },
            debug: {
              logLevel: 3,
            },
          });
          dashPlayer.initialize(video, url, false);

          // playbackEnded 兜底触发 ended 事件（DASH SegmentBase 可能不触发原生 ended）
          // 用标志位防止与原生 ended 重复触发
          let endedHandled = false;
          video.addEventListener('ended', () => { endedHandled = true; });
          dashPlayer.on('playbackEnded', () => {
            if (endedHandled) { endedHandled = false; return; }
            video.dispatchEvent(new Event('ended'));
          });
        } else if (video.canPlayType('application/dash+xml')) {
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
        heatmap: false, // 禁用热力图，避免切换清晰度时 NaN 错误
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

  // DASH 模式下手动处理循环播放
  // artplayer 的 loop 会 seek+play 产生竞态，dashjs 的 SegmentBase 也不支持原生 video.loop
  // 所以直接监听 video 的 ended 事件，用 dashjs seek 后延时 play
  let dashLoopEnabled = isDash && loopInit;
  if (isDash) {
    player.on('video:ended', () => {
      if (!dashLoopEnabled || !dashPlayer) return;
      dashPlayer.seek(0);
      setTimeout(() => {
        player?.video?.play().catch(() => {});
      }, 150);
    });
  }

  // 监听 ready 事件，安全地开始播放
  player.on('ready', () => {
    const playPromise = player.play();
    if (playPromise !== undefined) {
      playPromise.catch((error: any) => {
        if (error.name !== 'AbortError' && error.name !== 'NotAllowedError') {
          console.error('[art.vue] 播放错误:', error);
        }
      });
    }
  });

  player.on('quality', (item: any) => {
    // 保存清晰度选择
    if (item.html) {
      localStorage.setItem('artplayer-quality', item.html);
    }
    const newType = guessType(item.url, item);
    if (player && player.switchUrl) {
      player.switchUrl(item.url, newType);
    }
  });
};

onMounted(() => {
  console.log('[art.vue] onMounted, videoInfo:', props.videoInfo);
  console.log('[art.vue] onMounted, container:', playerContainer.value);

  // 使用 nextTick 确保 DOM 完全渲染后再初始化
  nextTick(() => {
    console.log('[art.vue] nextTick, resources:', props.videoInfo?.resources);
    console.log('[art.vue] nextTick, container after tick:', playerContainer.value);
    if (props.videoInfo?.resources?.length) {
      initPlayer();
    }
  });
});

// 监听 videoInfo 变化，当数据加载完成后初始化播放器
watch(
  () => props.videoInfo,
  (newVal) => {
    console.log('[art.vue] watch videoInfo changed:', newVal);
    if (newVal?.resources?.length && !player && playerContainer.value) {
      nextTick(() => {
        initPlayer();
      });
    }
  },
  { immediate: true, deep: true }
);

// 组件卸载时清理资源
onBeforeUnmount(() => {
  flushLogs();
  if (player) {
    player.destroy();
    player = null;
  }
  if (dashPlayer) {
    dashPlayer.reset();
    dashPlayer = null;
  }
  destroyHlsPlayer(hlsPlayerState);
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
