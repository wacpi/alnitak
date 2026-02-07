<template>
  <!-- 播放器容器和弹幕发送区 -->
  <div class="player-container">
    <div class="player" id="dplayer"></div>
    <div class="danmaku-send">
      <danmaku-send ref="danmakuSendRef" @send="sendDanmaku" @change-show="changeShow" @opacity-change="opacityChange"
        @set-filter="filterDanmaku"></danmaku-send>
    </div>
  </div>
</template>

<script setup lang="ts">
// ===== 依赖与类型定义 =====
import Hls from "hls.js";
import * as dashjs from "dashjs";
import Wplayer from 'wplayer-next';
import { ref, shallowRef, onBeforeMount, watch, onMounted, onBeforeUnmount } from 'vue';
import { getDanmakuAPI, sendDanmakuAPI } from "@/api/danmaku";
import DanmakuSend from "./components/DanmakuSend.vue";
import { getResourceQualityApi, getVideoFileUrl, getVideoFileUrlDash } from "@/api/video";
import { addHistoryAPI } from "@/api/history";
import {
  createHlsPlayer,
  destroyHlsPlayer,
  getSavedPlaybackState,
  restorePlaybackState,
  setupVolumePersistence,
  getSavedVolumeState,
  type HlsPlayerState,
  type PlaybackState,
} from "@/utils/hls-player";

// ===== 组件属性定义 =====
const props = withDefaults(defineProps<{
  videoInfo: VideoType;
  part: number;
  progress: number | null;
}>(), {
  part: 1,
  progress: null
})

// ===== 播放器与弹幕相关变量 =====
let player: any = null;
let dashPlayer: any = null;
const defaultQuality = ref('');
const hlsPlayerState: HlsPlayerState = { instance: null, videoElement: null, playPromise: null };
const dash = shallowRef<any>(null);
const hasEnded = ref(false);

// ===== 清晰度切换时保存播放状态 =====
// 持续保存最新的播放状态，用于清晰度切换时恢复
let lastPlaybackState: { time: number; playing: boolean } = { time: 0, playing: false };
let qualitySwitchState: { time: number; playing: boolean } | null = null;
const danmakuSendRef = ref<InstanceType<typeof DanmakuSend> | null>(null);
const options: PlayerOptionsType = {
  container: null,
  video: {
    quality: [],
    defaultQuality: 0,
    pic: '',
    type: 'customHls',
    customType: {
      customHls: function (video: HTMLVideoElement) {
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
          }
        );
      },
      // DASH 播放（支持 B站风格 mpd SegmentBase）
      customDash: function (video: HTMLVideoElement) {
        console.log('[DASH] 初始化播放器, src:', video.src);

        const savedVolume = localStorage.getItem('wplayer-volume');
        const savedMuted = localStorage.getItem('wplayer-muted');
        const prevVolume = savedVolume !== null ? parseFloat(savedVolume) : 1;
        const prevMuted = savedMuted === '1';

        // 优先级：1. qualitySwitchState（quality_start保存的）2. lastPlaybackState（timeupdate保存的）3. video元素当前状态
        let savedTime = 0;
        let wasPlaying = false;
        let source = 'video元素';

        if (qualitySwitchState && qualitySwitchState.time > 0) {
          savedTime = qualitySwitchState.time;
          wasPlaying = qualitySwitchState.playing;
          source = 'qualitySwitchState';
        } else if (lastPlaybackState.time > 0) {
          savedTime = lastPlaybackState.time;
          wasPlaying = lastPlaybackState.playing;
          source = 'lastPlaybackState';
        } else if (video.currentTime > 0) {
          savedTime = video.currentTime;
          wasPlaying = !video.paused;
          source = 'video元素';
        }
        console.log('[DASH] 恢复播放位置:', savedTime, '是否正在播放:', wasPlaying, '来源:', source);

        // 使用后清除状态
        qualitySwitchState = null;

        // 销毁旧的 DASH 实例
        if (dash.value) {
          dash.value.reset();
          dash.value = null;
        }

        // 创建新的 DASH 实例
        dash.value = dashjs.MediaPlayer().create();
        // 优化缓冲配置，减少掉帧
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
          debug: {
            logLevel: 3, // WARN level, 减少控制台日志
          },
        });
        dash.value.initialize(video, video.src, false);

        // 保存恢复状态的引用
        const restoreState = { time: savedTime, playing: wasPlaying, restored: false };

        // 恢复音量状态（使用字符串事件名）
        dash.value.on('streamInitialized', () => {
          video.volume = prevVolume;
          video.muted = prevMuted;
          console.log('[DASH] 流初始化完成，音量:', video.volume);

          // 手动触发 loadedmetadata 事件，让播放器更新时长
          video.dispatchEvent(new Event('loadedmetadata'));
        });

        // 使用 playbackTimeUpdated 事件恢复播放位置（dash.js 推荐的时机）
        const onPlaybackTimeUpdated = () => {
          if (restoreState.restored) return;
          restoreState.restored = true;

          // 移除事件监听器（先移除，避免重复触发）
          if (dash.value) {
            dash.value.off('playbackTimeUpdated', onPlaybackTimeUpdated);
          }

          if (restoreState.time > 0) {
            console.log('[DASH] playbackTimeUpdated - 准备恢复播放位置到:', restoreState.time);
            // 使用 setTimeout 延迟执行，确保 dashjs 完全就绪
            setTimeout(() => {
              if (dash.value) {
                // 使用 dashjs 的 seek 方法
                dash.value.seek(restoreState.time);
                console.log('[DASH] 已调用 dash.seek(), 当前 video.currentTime =', video.currentTime);
              }

              // 如果之前正在播放，继续播放
              if (restoreState.playing) {
                video.play().catch((err) => {
                  if (err.name !== 'AbortError') {
                    console.error('[DASH] 自动播放失败:', err);
                  }
                });
              }
            }, 100);
          } else if (restoreState.playing) {
            // 没有需要恢复的位置，但需要继续播放
            video.play().catch((err) => {
              if (err.name !== 'AbortError') {
                console.error('[DASH] 自动播放失败:', err);
              }
            });
          }
        };
        dash.value.on('playbackTimeUpdated', onPlaybackTimeUpdated);

        // 监听 playbackMetaDataLoaded 确保时长可用
        dash.value.on('playbackMetaDataLoaded', () => {
          console.log('[DASH] 元数据加载完成，时长:', video.duration);
          video.dispatchEvent(new Event('durationchange'));
        });

        // 监听错误
        dash.value.on('error', (e: any) => {
          console.error('[DASH] 播放错误:', e);
        });

        console.log('[DASH] 播放器初始化完成');
      },
    },
  },
  danmaku: {}
}

// ===== 弹幕过滤配置 =====
let disableLeave = 0;
let disableType: number[] = [];
const initFilterConfig = () => {
  const disableTypeConfig = localStorage.getItem('danmaku-disable-type');
  if (disableTypeConfig) {
    disableType = disableTypeConfig.split(',').map((item) => parseInt(item));
  }

  const disableLeaveConfig = localStorage.getItem('danmaku-disable-leave');
  if (disableLeaveConfig) {
    disableLeave = parseInt(disableLeaveConfig);
  }
}

// ===== 进度续播相关 =====
let pendingSeek: number | null = null;

// ===== 监听 progress 属性变化，自动 seek =====
watch(
  () => props.progress,
  (val) => {
    if (val != null && player) {
      player.seek(val);
      pendingSeek = null;
    } else if (val != null) {
      pendingSeek = val;
    }
  },
  { immediate: true }
);
// ===== 工具函数 =====
// 将进度转换为整数秒
const toBizSecond = (v: number | null | undefined): number => {
  if (typeof v !== 'number' || !isFinite(v) || v <= 0) return 0;
  return Math.floor(v);
};


// ===== 播放器 ready 回调与定时上报历史 =====
let timer: number | null = null;
let hasReportedWatched = false; // 是否已上报过“已看完”
const onReadyCallbacks: Array<() => void> = [];
const setOnReady = (cb: () => void) => {
  onReadyCallbacks.push(cb);
};

// ===== 本地已看完标记工具函数 =====
const getWatchedKey = () => `video-watched-${props.videoInfo.vid}-${props.part}`;
const isWatched = () => localStorage.getItem(getWatchedKey()) === '1';
const setWatched = () => localStorage.setItem(getWatchedKey(), '1');
const clearWatched = () => localStorage.removeItem(getWatchedKey());

// ===== 分集切换与播放器实例化 =====
// 添加播放结束回调
const onEndedCallback = ref<(() => void) | null>(null);

const setOnEnded = (callback: () => void) => {
  onEndedCallback.value = callback;
};

const loadPart = async (part: number) => {
  // 重置播放结束标记
  hasEnded.value = false;

  const el = document.getElementById('dplayer');
  if (el) {
    await loadResource(part);
    /* === 播放器销毁与重建实例化片段 start === */
    if (player) player.destroy();
    options.container = el;
    player = new Wplayer(options);
    /* === 播放器销毁与重建实例化片段 end === */
    hasReportedWatched = false;
    clearWatched();

    // 监听清晰度切换开始，在切换前保存当前状态
    player.on('quality_start', (quality: PlayerQualityType) => {
      // 优先直接从 video 元素读取（如果还有效的话），否则使用 lastPlaybackState
      if (player?.video && player.video.currentTime > 0) {
        qualitySwitchState = {
          time: player.video.currentTime,
          playing: !player.video.paused,
        };
        console.log('[quality_start] 从video元素保存播放状态:', qualitySwitchState);
      } else {
        qualitySwitchState = { ...lastPlaybackState };
        console.log('[quality_start] 从lastPlaybackState保存播放状态:', qualitySwitchState);
      }
      localStorage.setItem('default-video-quality', quality.name);
    });

    // 持续保存播放状态，用于清晰度切换时恢复
    player.on('timeupdate', () => {
      if (player?.video && player.video.currentTime > 0) {
        lastPlaybackState = {
          time: player.video.currentTime,
          playing: !player.video.paused,
        };
      }
    });

    // 也在暂停时保存状态
    player.on('pause', () => {
      if (player?.video && player.video.currentTime > 0) {
        lastPlaybackState = {
          time: player.video.currentTime,
          playing: false,
        };
      }
    });
    filterDanmaku({ disableLeave, disableType });

    if (player && typeof player.play === 'function') {
      player.play();
    }

    // 监听播放完成事件，上报已看完并终止定时上报
    player.on('ended', async () => {
      hasEnded.value = true; // 标记为已结束

      try {
        // ✅ 业务层统一使用“整数秒”
        const duration = Math.floor(player.video.duration || 0);

        await addHistoryAPI({
          vid: props.videoInfo.vid,
          part: props.part,
          time: -1,        // 已看完统一用 -1
          duration,        // 整数秒
        });
      } catch (error) {
        console.error('上报播放完成失败:', error);
      }

      hasReportedWatched = true;
      setWatched();

      if (onEndedCallback.value) {
        onEndedCallback.value();
      }
    });


    // 监听进度条大跨度跳转
    let lastSeekTime = 0;
    player.on('seeked', () => {
      const current = player.video.currentTime;
      if (Math.abs(current - lastSeekTime) > 10 && !isWatched() && !hasEnded.value) {
        const current = Math.floor(player.video.currentTime || 0);
        const duration = Math.floor(player.video.duration || 0);
        addHistoryAPI({ vid: props.videoInfo.vid, part: props.part, time: current, duration });
      }
      lastSeekTime = current;
    });
  }
}

// ===== 清晰度映射表与资源加载 =====
const resourceNameMap: Record<string, string> = {
  "640x360_1000k_30": "360p",
  "854x480_1500k_30": "480p",
  "1280x720_3000k_30": "720p",
  "1920x1080_6000k_30": "1080p",
  "1920x1080_8000k_60": "1080p60",
}

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
}

// 视频播放信息缓存
const videoPlayInfoCache = new Map<string, any>();

const loadResource = async (part: number) => {
  // 防御性检查
  if (!props.videoInfo?.resources?.length) {
    console.warn('[video-player] videoInfo.resources is empty or undefined');
    return;
  }

  const resource = props.videoInfo.resources[part - 1]
  if (!resource?.id) {
    console.warn('[video-player] resource not found for part:', part);
    return;
  }

  const res = await getResourceQualityApi(resource.id)
  if (res.data.code === statusCode.OK && res.data.data.quality?.length > 0) {
    // 复制并根据分辨率宽度 & 帧率从高到低排序
    const qualities = [...res.data.data.quality] as string[]
    qualities.sort((a, b) => {
      // 解析宽度
      const wa = parseInt(a.split('x')[0], 10)
      const wb = parseInt(b.split('x')[0], 10)
      if (wb !== wa) {
        return wb - wa
      }
      // 宽度相同时，解析帧率
      const fpsA = parseInt(a.split('_').pop() || '0', 10)
      const fpsB = parseInt(b.split('_').pop() || '0', 10)
      return fpsB - fpsA
    })

    // 必须浏览器支持且服务器资源支持才使用 DASH
    const serverSupportsDash = res.data.data.supportsDash === true
    const useDash = supportsDashJs() && serverSupportsDash

    // 映射并设置默认质量索引
    options.video.quality = qualities.map((item, index) => {
      const name = getQualityDisplayName(item)
      if (name === defaultQuality.value) {
        options.video.defaultQuality = index
      }
      return {
        name,
        url: useDash ? getVideoFileUrlDash(resource.id, item) : getVideoFileUrl(resource.id, item),
      }
    })

    // 设置视频类型（HLS 或 DASH）
    if (useDash) {
      options.video.type = 'customDash'
    } else {
      options.video.type = 'customHls'
    }
  }
}

// 检测 MediaSource API 是否可用
const supportMediaSource = (): boolean => {
  return !!(window.MediaSource || (window as any).webkitMediaSource)
}

// 检测是否支持 dash.js
const supportsDashJs = (): boolean => {
  const video = document.createElement('video')
  return !!(
    supportMediaSource() ||
    video.canPlayType('application/dash+xml') !== '' ||
    (window as any).dashjs !== undefined
  )
}

// ===== 弹幕相关方法 =====
const originalDanmaku = shallowRef<DanmakuType[]>([]);
const setDanmaku = (data: DanmakuType[]) => {
  originalDanmaku.value = data;
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
    const res = await sendDanmakuAPI(danmaku);
    if (res.data.code !== statusCode.OK) {
      ElMessage.error(res.data.msg);
    }
  })
}

//过滤弹幕
const filterDanmaku = (filter: FilterDanmakuType) => {
  localStorage.setItem('danmaku-disable-type', filter.disableType.toString());
  localStorage.setItem('danmaku-disable-leave', filter.disableLeave.toString());

  const data = originalDanmaku.value.filter((item: DanmakuType) => {
    return !isDisableType(item, filter.disableType) && (Math.floor(Math.random() * 10) + 1) > filter.disableLeave;
  }).map((d: DanmakuType) => { return { ...d } });

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

// ===== 历史记录上报 =====
const uploadHistory = async () => {
  // 如果视频已播放结束，不再上报进度
  if (hasEnded.value) {
    console.log('视频已播放结束，跳过进度上报');
    return;
  }

  const duration = Math.floor(player.video.duration || 0); // 总时长取整
  const currentTime = Math.floor(player.video.currentTime); // 当前进度取整

  await addHistoryAPI({
    vid: props.videoInfo.vid,
    part: props.part,
    time: currentTime >= duration ? -1 : currentTime, // 播放完了就上报 -1
    duration,
  });
}


// ===== 分集切换监听 =====
watch(() => props.part, (newPart, oldPart) => {
  if (newPart !== oldPart) {
    // 切换前上报当前进度（如果未播放完）
    if (!hasEnded.value && !isWatched()) {
      uploadHistory();
    }
    // 加载新分集
    loadPart(newPart);
  }
});

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

  if (player) {
    player.on('loadedmetadata', () => {
      onReadyCallbacks.forEach(cb => cb());
      onReadyCallbacks.length = 0;
      // loadedmetadata 兜底 seek
      if (pendingSeek != null) {
        player.seek(pendingSeek);
        pendingSeek = null;
      }
    });
  }

  // 定时上报历史进度，若已看完则停止上报
  timer = window.setInterval(() => {
    if (!hasReportedWatched && !isWatched()) {
      // 检查视频是否正在播放，如果暂停或未播放则跳过上报
      if (player && player.video) {
        const isPlaying = !player.video.paused && !player.video.ended && player.video.currentTime > 0;
        if (isPlaying) {
          uploadHistory(); // 只有在播放时才上报
        } else {
          console.log('视频暂停或未播放，跳过定时上报');
        }
      } else {
        console.log('播放器未初始化，跳过定时上报');
      }
    }
  }, 10000)
})

// ===== 页面关闭/离开时上报进度，已看完则不再上报 =====
const reportOnLeave = () => {
  if (player && player.video && typeof player.video.currentTime === 'number' && !isWatched()) {
    const duration = Math.floor(player.video.duration || 0); // 总时长取整
    const currentTime = Math.floor(player.video.currentTime); // 当前进度取整
    addHistoryAPI({ vid: props.videoInfo.vid, part: props.part, time: currentTime >= duration ? -1 : currentTime, duration });
  }
};
if (typeof window !== 'undefined') {
  window.addEventListener('beforeunload', reportOnLeave);
}
onBeforeUnmount(() => {
  if (timer) clearInterval(timer);
  reportOnLeave();
  if (typeof window !== 'undefined') {
    window.removeEventListener('beforeunload', reportOnLeave);
  }
  if (player) {
    player.destroy();
    player = null;
  }
  destroyHlsPlayer(hlsPlayerState);
  if (dash.value) {
    dash.value.reset();
    dash.value = null;
  }
});

// ===== 对外暴露方法 =====
const seek = (time: number) => {
  if (player) {
    player.seek(time);
  }
};

defineExpose({
  seek,
  setOnReady,
  uploadHistory,
  setDanmaku,
  setOnEnded
})
</script>

<style lang="scss" scoped>
// ===== 播放器与弹幕样式 =====
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

    &.wplayer-fulled {
      position: fixed;
      top: 0;
      left: 0;
      width: 100vw;
      height: 100vh;
      z-index: 9999;
    }
  }


  .danmaku-send {
    position: absolute;
    width: 100%;
    bottom: -40px;

    .player-container.wplayer-fulled & {
      display: none;
    }
  }
}
</style>
