<template>
  <div class="player-container">
    <div class="player" id="dplayer"></div>
    <div class="danmaku-send">
      <danmaku-send ref="danmakuSendRef" @send="sendDanmaku" @change-show="changeShow" @opacity-change="opacityChange"
        @set-filter="filterDanmaku"></danmaku-send>
    </div>
  </div>
</template>

<script setup lang="ts">
import Hls from "hls.js";
import * as dashjs from "dashjs";
import Wplayer from 'wplayer-next';
import { ref, shallowRef, watch, onMounted, onBeforeUnmount } from 'vue';
import { getDanmakuAPI, sendDanmakuAPI } from "@/api/danmaku";
import DanmakuSend from "./components/DanmakuSend.vue";
import { getResourceQualityApi, getVideoFileUrl, getVideoFileUrlDash, getVideoFileUrlDashUnified } from "@/api/video";
import { addHistoryAPI, getHistoryProgressAPI } from "@/api/history";
import { statusCode } from "@/utils/status-code";
import { useMessage } from "naive-ui";

const props = withDefaults(defineProps<{
  videoInfo: VideoType;
  part: number
}>(), {
  part: 1
})

const message = useMessage();

let player: any = null;
const defaultQuality = ref('');
const hls = shallowRef<Hls | null>(null);
const dash = shallowRef<any>(null);
const danmakuSendRef = ref<InstanceType<typeof DanmakuSend> | null>(null);

// DASH 统一 MPD 模式状态
let dashUnifiedMode = false;
let dashQualityMap: Map<string, number> = new Map();

// 检测是否支持 DASH 播放
const supportsDashJs = (): boolean => {
  const ua = navigator.userAgent;
  // Safari / iOS 不使用 DASH
  if (/iPad|iPhone|iPod/.test(ua)) return false;
  if (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1) return false;
  if (/Safari/.test(ua) && !/Chrome|CriOS|FxiOS|Edg/.test(ua)) return false;
  return !!((window as any).MediaSource || (window as any).ManagedMediaSource);
};
const options: PlayerOptionsType = {
  container: null,
  video: {
    quality: [],
    defaultQuality: 0,
    pic: '',
    type: 'customHls',
    customType: {
      // TODO: 处理IOS系统中的hls视频播放
      customHls: function (video: HTMLVideoElement) {
        if (!hls.value) hls.value = new Hls();
        hls.value.loadSource(video.src);
        hls.value.attachMedia(video);
        hls.value.on(Hls.Events.ERROR, () => {
          console.error("资源加载失败");
        });
      },
      // DASH 播放（统一 MPD 模式）
      customDash: function (video: HTMLVideoElement) {
        // 统一 MPD 模式下，如果 dashPlayer 已存在且 URL 相同，跳过重复初始化
        if (dash.value) {
          try {
            const prevUrl = dash.value.getManifestUrl();
            if (prevUrl === video.src) {
              return;
            }
          } catch {}
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
            abr: {
              autoSwitchBitrate: { video: false, audio: false },
            },
          },
          debug: { logLevel: 0 },
        });
        dash.value.initialize(video, video.src, false);

        // playbackEnded 兜底触发 ended 事件
        let endedHandled = false;
        video.addEventListener('ended', () => { endedHandled = true; });
        dash.value.on('playbackEnded', () => {
          if (endedHandled) { endedHandled = false; return; }
          video.dispatchEvent(new Event('ended'));
        });

        dash.value.on('error', (e: any) => {
          console.error('[mobile DASH] 播放错误:', e);
        });
      },
    },
  },
  danmaku: {}
}

let disableLeave = 0;
let disableType: number[] = [];
const initFilterConfig = () => {
  const disableTypeConfig = localStorage.getItem('danmaku-disable-type');
  if (disableTypeConfig) {
    disableType = JSON.parse(disableTypeConfig);
  }

  const disableLeaveConfig = localStorage.getItem('danmaku-disable-leave');
  if (disableLeaveConfig) {
    disableLeave = parseInt(disableLeaveConfig);
  }
}

const loadPart = async (part: number) => {
  const el = document.getElementById('dplayer');
  if (el) {
    await loadResource(part);
    await getDanmaku(part);

    if (player) player.destroy();

    options.container = el;
    player = new Wplayer(options);
    player.on('quality_start', (quality: PlayerQualityType) => {
      localStorage.setItem('default-video-quality', quality.name);
    })

    // 统一 DASH 模式：拦截 Wplayer 的清晰度切换，改用 dash.js API 无缝切换
    if (dashUnifiedMode) {
      player.switchQuality = function (index: number | string) {
        const idx = typeof index === 'string' ? parseInt(index) : index;
        if (idx === player.qualityIndex) return;

        const quality = player.options.video.quality[idx];
        if (!quality) return;

        player.qualityIndex = idx;
        player.quality = quality;
        const qualityText = player.template?.qualityButton?.querySelector('.wplayer-quality-text');
        if (qualityText) qualityText.textContent = quality.name;

        const dashIndex = dashQualityMap.get(quality.name);
        if (dashIndex !== undefined && dash.value) {
          dash.value.setRepresentationForTypeByIndex('video', dashIndex, true);
          player.notice(`切换至 ${quality.name}`, 1000, undefined, 'switch-quality');
        }

        localStorage.setItem('default-video-quality', quality.name);
        player.events.trigger('quality_start', quality);
      };
    }

    filterDanmaku({ disableLeave, disableType });
  }
}

const resourceNameMap: Record<string, string> = {
  "640x360_500k_30": "360p",
  "640x360_1000k_30": "360p",
  "854x480_900k_30": "480p",
  "854x480_1500k_30": "480p",
  "1080x720_2000k_30": "720p",// 兼容之前的错误
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
      const [width, height] = resolution.split('x').map(Number);
      // YouTube风格：取短边作为分辨率标签
      const shortSide = Math.min(width, height);
      const fpsSuffix = fps > 30 ? fps.toString() : '';

      if (shortSide <= 360) return fpsSuffix ? `360p${fpsSuffix}` : '360p';
      if (shortSide <= 480) return fpsSuffix ? `480p${fpsSuffix}` : '480p';
      if (shortSide <= 720) return fpsSuffix ? `720p${fpsSuffix}` : '720p';
      if (shortSide <= 1080) return fpsSuffix ? `1080p${fpsSuffix}` : '1080p';
      if (shortSide <= 1440) return fpsSuffix ? `1440p${fpsSuffix}` : '1440p';
      if (shortSide <= 2160) return fpsSuffix ? `4K${fpsSuffix}` : '4K';
      return fpsSuffix ? `${shortSide}p${fpsSuffix}` : `${shortSide}p`;
    }
  } catch (error) {
    console.warn('Failed to parse quality string:', qualityStr);
  }

  return qualityStr.split('_')[0] || qualityStr;
}

const loadResource = async (part: number) => {
  // 防御性检查
  if (!props.videoInfo?.resources?.length) {
    console.warn('[video-player] videoInfo.resources is empty or undefined');
    return;
  }

  const resource = props.videoInfo.resources[part - 1];
  if (!resource?.id) {
    console.warn('[video-player] resource not found for part:', part);
    return;
  }

  const res = await getResourceQualityApi(resource.id);
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

    const serverSupportsDash = res.data.data.supportsDash === true;
    const useDash = supportsDashJs() && serverSupportsDash;
    const qualityOrderFromServer = (res.data.data.qualityOrder as string[]) || [];

    if (useDash && qualityOrderFromServer.length > 0) {
      // 统一 DASH MPD 模式
      dashUnifiedMode = true;
      dashQualityMap = new Map();
      qualityOrderFromServer.forEach((q: string, index: number) => {
        dashQualityMap.set(getQualityDisplayName(q), index);
      });

      const unifiedMpdUrl = getVideoFileUrlDashUnified(resource.id);
      options.video.quality = qualities.map((item: string, index: number) => {
        const name = getQualityDisplayName(item);
        if (name === defaultQuality.value) {
          options.video.defaultQuality = index;
        }
        return { name, url: unifiedMpdUrl };
      });
      options.video.type = 'customDash';
    } else {
      dashUnifiedMode = false;
      options.video.quality = qualities.map((item: string, index: number) => {
        const name = getQualityDisplayName(item);
        if (name === defaultQuality.value) {
          options.video.defaultQuality = index;
        }
        return {
          name,
          url: useDash ? getVideoFileUrlDash(resource.id, item) : getVideoFileUrl(resource.id, item),
        };
      });
      options.video.type = useDash ? 'customDash' : 'customHls';
    }
  }
}

let originalDanmaku: DanmakuType[] = [];
const getDanmaku = async (part: number) => {
  const res = await getDanmakuAPI(props.videoInfo.vid, part);
  if (res.data.code === statusCode.OK) {
    if (res.data.data.danmaku) {
      originalDanmaku = res.data.data.danmaku;
    }
  }
}

// 弹幕显示改变
const changeShow = (val: boolean) => {
  if (val) {
    player.danmaku.show();
  } else {
    player.danmaku.hide();
  }
}

const opacityChange = (val: number) => {
  player.danmaku.opacity(val);
}

const sendDanmaku = (danmakuForm: DrawDanmakuType) => {
  if (danmakuForm.text == "") {
    player.notice("弹幕内容不能为空")
    return;
  }

  player.danmaku.send(danmakuForm, async (danmaku: AddDanmakuType) => {
    danmaku.vid = props.videoInfo.vid;
    danmaku.part = props.part;
    console.log('111',111)
    const res = await sendDanmakuAPI(danmaku);
    if (res.data.code !== statusCode.OK) {
      message.error(res.data.msg);
    }
  })
}

//过滤弹幕
const filterDanmaku = (filter: FilterDanmakuType) => {
  localStorage.setItem('danmaku-disable-type', filter.disableType.toString());
  localStorage.setItem('danmaku-disable-leave', filter.disableLeave.toString());

  const data = originalDanmaku.filter((item) => {
    return !isDisableType(item, filter.disableType) && (Math.floor(Math.random() * 10) + 1) > filter.disableLeave;
  });
  player.danmaku.update(data);

  player.on('danmaku_loaded', () => {
    console.log("danmaku_load_end")
  })

  // 更新弹幕数量
  danmakuSendRef.value?.updateDanmakuCount(data.length);
}

//是否为屏蔽类型
const isDisableType = (item: DanmakuType, disableType: Array<number>) => {
  if (disableType.includes(item.type))
    return true;
  if (disableType.includes(3) && (item.color !== '#fff' && item.color !== '#ffffff'))
    return true;

  return false;
}

// 上传历史记录
const uploadHistory = async () => {
  await addHistoryAPI({ vid: props.videoInfo.vid, part: props.part, time: player.video.currentTime });
}

// 获取历史记录
const getHistoryProgress = async () => {
  const res = await getHistoryProgressAPI(props.videoInfo.vid, props.part);
  if (res.data.code === statusCode.OK) {
    const progress = res.data.data.progress;
    if (typeof progress === 'number' && progress > 0) {
      player.seek(progress);
    }
  }
}

watch(() => props.part, (val) => {
  loadPart(val);
});

let timer: number | null = null;
onMounted(async () => {
  const quality = localStorage.getItem('default-video-quality');
  if (quality) {
    defaultQuality.value = quality;
  } else {
    defaultQuality.value = '720p';
    localStorage.setItem('default-video-quality', '720p');
  }

  initFilterConfig();
  await loadPart(props.part);

  // 获取当前播放进度（如果没有就保存历史记录）
  await getHistoryProgress();

  timer = window.setInterval(() => {
    if (player && player.video && !player.video.paused && !player.video.ended && player.video.currentTime > 0) {
      uploadHistory();
    }
  }, 10000)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer);
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


  .danmaku-send {
    position: absolute;
    width: 100%;
    bottom: -40px;
  }
}
</style>
