<template>
  <div class="player-container">
    <div class="player" id="wplayer"></div>
  </div>
</template>

<script setup lang="ts">
import Hls from "hls.js";
import Dash from "dashjs";
import Wplayer from 'wplayer-next';
import { statusCode } from "@/utils/status-code";
import { ref, shallowRef, onBeforeUnmount } from 'vue';
import { getResourceQualityApi, getVideoFileUrl, getVideoFileUrlDash, getVideoFileAPI } from "@/api/video";
import { useMessage } from "naive-ui";
import { getResourceUrl } from "@/utils/resource";

const message = useMessage();

let player: any = null;
const defaultQuality = ref('');
const hls = shallowRef<Hls | null>(null);
const dash = shallowRef<any>(null);
const options: PlayerOptionsType = {
  container: null,
  video: {
    quality: [],
    defaultQuality: 0,
    type: 'customHls',
    customType: {
      // HLS 播放（兼容旧资源）
      customHls: (video: HTMLVideoElement) => {
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
      },
      // DASH 播放（新资源 SegmentBase 模式）
      customDash: (video: HTMLVideoElement) => {
        console.log('[DASH] 初始化播放器, src:', video.src);

        // 保存当前播放位置（用于切换清晰度时恢复）
        const savedTime = video.currentTime > 0 ? video.currentTime : 0;
        const wasPlaying = !video.paused;
        console.log('[DASH] 保存播放位置:', savedTime, '是否正在播放:', wasPlaying);

        // 销毁旧的 DASH 实例
        if (dash.value) {
          dash.value.reset();
          dash.value = null;
        }

        // 创建新的 DASH 实例
        dash.value = Dash.MediaPlayer().create();
        // 优化缓冲配置
        dash.value.updateSettings({
          streaming: {
            buffer: {
              bufferTimeDefault: 12,
              bufferTimeAtTopQuality: 30,
              bufferTimeAtTopQualityLongForm: 60,
            },
          },
          debug: {
            logLevel: 3, // WARN level
          },
        });
        dash.value.initialize(video, video.src, false);

        // 恢复播放位置（使用字符串事件名，兼容 dashjs 5.x）
        dash.value.on('streamInitialized', () => {
          console.log('[DASH] 流初始化完成');

          // 恢复播放位置
          if (savedTime > 0) {
            console.log('[DASH] 恢复播放位置到:', savedTime);
            video.currentTime = savedTime;
          }

          // 如果之前正在播放，继续播放
          if (wasPlaying) {
            video.play().catch((err: any) => {
              if (err.name !== 'AbortError') {
                console.error('[DASH] 自动播放失败:', err);
              }
            });
          }

          // 手动触发事件，让播放器更新时长
          video.dispatchEvent(new Event('loadedmetadata'));
        });

        // 监听错误（使用字符串事件名）
        dash.value.on('error', (e: any) => {
          console.error('[DASH] 播放错误:', e);
        });

        console.log('[DASH] 播放器初始化完成');
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
    player.on('quality_start', (quality: PlayerQualityType) => {
      localStorage.setItem('default-video-quality', quality.name);
    })
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

// 检测是否支持 dash.js
const supportsDashJs = (): boolean => {
  const video = document.createElement('video');
  return !!(
    (window.MediaSource || (window as any).webkitMediaSource) ||
    video.canPlayType('application/dash+xml') !== '' ||
    (window as any).dashjs !== undefined
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