<template>
  <div class="player-container">
    <div class="player" id="wplayer"></div>
  </div>
</template>

<script setup lang="ts">
import Hls from "hls.js";
import * as dashjs from "dashjs";
import Wplayer from 'wplayer-next';
import { statusCode } from "@/utils/status-code";
import { ref, shallowRef, onBeforeUnmount } from 'vue';
import { getResourceQualityApi, getVideoFileUrl, getVideoFileUrlDash, getVideoFileAPI } from "@/api/video";
import { useMessage } from "naive-ui";
import { getResourceUrl } from "@/utils/resource";
import { storageData } from "@/utils/storage-data";

const message = useMessage();

let player: any = null;
const defaultQuality = ref('');
const hls = shallowRef<Hls | null>(null);
const dash = shallowRef<any>(null);
// DASH 清晰度切换状态（Wplayer 创建新 video 后依赖此状态恢复进度）
let lastPlaybackState: { time: number; playing: boolean } = { time: 0, playing: false };
const options: PlayerOptionsType = {
  container: null,
  video: {
    quality: [],
    defaultQuality: 0,
    type: 'customHls',
    customType: {
      // HLS 播放（兼容旧资源 + Safari/iOS 原生 HLS）
      customHls: (video: HTMLVideoElement) => {
        if (Hls.isSupported()) {
          getVideoFileAPI(video.src).then((res) => {
            if (!res.data) return;
            if (!hls.value) hls.value = new Hls();
            const indexFile = res.data.split('\n').map((line: string) => {
              if (line.includes(".ts")) {
                return getResourceUrl(line)
              } else {
                return line
              }
            })
            var blob = new Blob([indexFile.join('\n')], { type: 'text/plain' });
            var blobUrl = URL.createObjectURL(blob);
            hls.value.loadSource(blobUrl);
            hls.value.attachMedia(video);
            hls.value.on(Hls.Events.ERROR, () => {
              console.error("资源加载失败");
            });
          })
        } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
          // Safari/iOS 原生 HLS 播放
          video.src = video.src;
        }
      },
      // DASH 播放（新资源 SegmentBase 模式）
      // Wplayer 切换清晰度会创建全新 video 元素，无法复用旧 dashPlayer
      customDash: (video: HTMLVideoElement) => {
        const savedTime = lastPlaybackState.time;
        const wasPlaying = lastPlaybackState.playing;

        // 销毁旧实例
        if (dash.value) {
          dash.value.reset();
          dash.value = null;
        }

        dash.value = dashjs.MediaPlayer().create();
        dash.value.updateSettings({
          streaming: {
            buffer: {
              bufferTimeDefault: 12,
              bufferTimeAtTopQuality: 30,
              bufferTimeAtTopQualityLongForm: 60,
              bufferPruningInterval: 10,
              bufferToKeep: 20,
            },
          },
          debug: { logLevel: 3 },
        });
        // 后台管理接口需要认证，通过 RequestModifier 注入 token
        const token = storageData.get('token');
        if (token) {
          dash.value.extend('RequestModifier', function () {
            return {
              modifyRequestHeader: function (xhr: XMLHttpRequest) {
                xhr.setRequestHeader('Authorization', token);
                return xhr;
              },
              modifyRequestURL: function (url: string) {
                return url;
              },
            };
          }, true);
        }
        dash.value.initialize(video, video.src, false);

        // 流初始化完成后触发元数据事件
        dash.value.on('streamInitialized', () => {
          video.dispatchEvent(new Event('loadedmetadata'));
        });

        // 恢复播放位置
        if (savedTime > 0) {
          let restored = false;
          dash.value.on('playbackTimeUpdated', function onRestore() {
            if (restored) return;
            restored = true;
            dash.value.off('playbackTimeUpdated', onRestore);
            dash.value.seek(savedTime);
            if (wasPlaying) {
              video.play().catch((err: any) => {
                if (err.name !== 'AbortError') console.error('[DASH] 播放失败:', err);
              });
            }
          });
        }

        // playbackEnded 兜底触发 ended 事件
        let endedHandled = false;
        video.addEventListener('ended', () => { endedHandled = true; });
        dash.value.on('playbackEnded', () => {
          if (endedHandled) { endedHandled = false; return; }
          video.dispatchEvent(new Event('ended'));
        });

        dash.value.on('error', (e: any) => {
          console.error('[DASH] 播放错误:', e);
        });
      },
    },
  },
  danmaku: {}
}

const loadVideo = async (resourceId: number) => {
  const el = document.getElementById('wplayer');
  if (el) {
    await loadResource(resourceId);

    if (player) player.destroy();

    options.container = el;
    player = new Wplayer(options);
    // 保存清晰度偏好
    player.on('quality_start', (quality: PlayerQualityType) => {
      localStorage.setItem('default-video-quality', quality.name);
    })

    // 持续保存播放状态（Wplayer 切换清晰度会创建新 video，customDash 依赖此状态恢复进度）
    player.on('timeupdate', () => {
      if (player?.video && player.video.currentTime > 0) {
        lastPlaybackState = { time: player.video.currentTime, playing: !player.video.paused };
      }
    });
  }
}

const resourceNameMap: Record<string, string> = {
  "640x360_500k_30": "360p",
  "640x360_1000k_30": "360p",
  "854x480_900k_30": "480p",
  "854x480_1500k_30": "480p",
  "1080x720_2000k_30": "720p", // 兼容之前的错误
  "1280x720_2000k_30": "720p",
  "1280x720_3000k_30": "720p",
  "1920x1080_3000k_30": "1080p",
  "1920x1080_6000k_30": "1080p",
  "1920x1080_6000k_60": "1080p60",
  "1920x1080_8000k_60": "1080p60",
}

/**
 * 根据清晰度字符串动态生成显示名称
 */
const getQualityDisplayName = (qualityStr: string): string => {
  if (resourceNameMap[qualityStr]) {
    return resourceNameMap[qualityStr];
  }

  try {
    const parts = qualityStr.split('_');
    const resolution = parts[0];
    const fpsStr = parts[parts.length - 1];
    const fps = parseInt(fpsStr, 10);

    if (resolution.includes('x')) {
      const [, height] = resolution.split('x').map(Number);
      const fpsSuffix = fps > 30 ? fps.toString() : '';

      if (height <= 360) return fpsSuffix ? `360p${fpsSuffix}` : '360p';
      if (height <= 480) return fpsSuffix ? `480p${fpsSuffix}` : '480p';
      if (height <= 720) return fpsSuffix ? `720p${fpsSuffix}` : '720p';
      if (height <= 1080) return fpsSuffix ? `1080p${fpsSuffix}` : '1080p';
      if (height <= 1440) return fpsSuffix ? `1440p${fpsSuffix}` : '1440p';
      if (height <= 2160) return fpsSuffix ? `4K${fpsSuffix}` : '4K';
      return fpsSuffix ? `${height}p${fpsSuffix}` : `${height}p`;
    }
  } catch (error) {
    console.warn('Failed to parse quality string:', qualityStr);
  }

  return qualityStr.split('_')[0] || qualityStr;
}

// 检测是否为 Safari 或 iOS 设备（它们不完整支持 MSE / dashjs）
const isSafariOrIOS = (): boolean => {
  const ua = navigator.userAgent;
  if (/iPad|iPhone|iPod/.test(ua)) return true;
  if (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1) return true;
  if (/Safari/.test(ua) && !/Chrome|CriOS|FxiOS|Edg/.test(ua)) return true;
  return false;
};

// 检测是否支持 dash.js
const supportsDashJs = (): boolean => {
  if (isSafariOrIOS()) return false;
  return !!(
    (window as any).MediaSource ||
    (window as any).ManagedMediaSource
  );
}

const loadResource = async (resourceId: number) => {
  const res = await getResourceQualityApi(resourceId);
  if (res.data.code === statusCode.OK && res.data.data.quality?.length > 0) {
    // 排序：分辨率从高到低，帧率从高到低
    const qualities = [...res.data.data.quality].sort((a: string, b: string) => {
      const wa = parseInt(a.split('x')[0], 10);
      const wb = parseInt(b.split('x')[0], 10);
      if (wb !== wa) return wb - wa;
      const fpsA = parseInt(a.split('_').pop() || '0', 10);
      const fpsB = parseInt(b.split('_').pop() || '0', 10);
      return fpsB - fpsA;
    });

    // 必须浏览器支持且服务器资源支持才使用 DASH
    const serverSupportsDash = res.data.data.supportsDash === true;
    const useDash = supportsDashJs() && serverSupportsDash;

    options.video.quality = qualities.map((item: string, index: number) => {
      const name = getQualityDisplayName(item);
      if (name === defaultQuality.value) {
        options.video.defaultQuality = index;
      }
      return {
        name,
        url: useDash ? getVideoFileUrlDash(resourceId, item) : getVideoFileUrl(resourceId, item),
      };
    });

    // 设置视频类型（HLS 或 DASH）
    if (useDash) {
      options.video.type = 'customDash';
    } else {
      options.video.type = 'customHls';
    }
  }
}

defineExpose({
  loadVideo
})

onBeforeUnmount(() => {
  if (player) player.destroy();
  if (hls.value) {
    hls.value.destroy();
    hls.value = null;
  }
  if (dash.value) {
    dash.value.reset();
    dash.value = null;
  }
})
</script>
    
<style lang="scss" scoped>
.player-container {
  height: 0;
  width: 100%;
  padding-bottom: 56.25%;
  position: relative;
  margin-bottom: 40px;

  .player {
    width: 100%;
    height: 100%;
    position: absolute;
    background-color: black;
  }
}
</style>