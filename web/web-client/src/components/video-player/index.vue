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
import Wplayer from 'wplayer-next';
import { ref, onBeforeMount, watch, onMounted, onBeforeUnmount } from 'vue';
import { getDanmakuAPI, sendDanmakuAPI } from "@/api/danmaku";
import DanmakuSend from "./components/DanmakuSend.vue";
import { getResourceQualityApi, getVideoFileUrl } from "@/api/video";
import { addHistoryAPI } from "@/api/history";

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
const defaultQuality = ref('');
const hls = shallowRef<Hls | null>(null);
let hasEnded = false; // 新增：标记视频是否已播放结束
const danmakuSendRef = ref<InstanceType<typeof DanmakuSend> | null>(null);
const options: PlayerOptionsType = {
  container: null,
  video: {
    quality: [],
    defaultQuality: 0,
    pic: '',
    type: 'customHls',
    customType: {
      // TODO: 处理IOS系统中的hls视频播放
      //这段代码先前造成清晰度切换进度丢失问题！
      customHls: function (video: HTMLVideoElement) {
        if (!hls.value) {  // 如果 Hls 实例不存在，才创建一个新的 Hls 实例
          hls.value = new Hls();
        } else {
          hls.value.destroy();  // 销毁旧的实例，防止内存泄漏
          hls.value = new Hls();  // 重新实例化 Hls（仅在必要时）
        }
        hls.value.loadSource(video.src);  // 加载新的 HLS 视频源
        hls.value.attachMedia(video);  // 将 HLS 实例附加到视频元素上

        hls.value.on(Hls.Events.ERROR, () => {
          console.error("资源加载失败");
        });
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
  hasEnded = false;

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

    player.on('quality_start', (quality: PlayerQualityType) => {
      localStorage.setItem('default-video-quality', quality.name);
    })
    filterDanmaku({ disableLeave, disableType });

    if (player && typeof player.play === 'function') {
      player.play();
    }

    // 监听播放完成事件，上报已看完并终止定时上报
    player.on('ended', async () => {
      hasEnded = true; // 标记为已结束

      try {
        await addHistoryAPI({ vid: props.videoInfo.vid, part: props.part, time: -1 });
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
      if (Math.abs(current - lastSeekTime) > 10 && !isWatched() && !hasEnded) {
        addHistoryAPI({ vid: props.videoInfo.vid, part: props.part, time: current });
      }
      lastSeekTime = current;
    });
  }
}

// ===== 清晰度映射表与资源加载 =====
const resourceNameMap = {
  "640x360_1000k_30": "360p",
  "854x480_1500k_30": "480p",
  "1280x720_3000k_30": "720p",
  "1920x1080_6000k_30": "1080p",
  "1920x1080_8000k_60": "1080p60",
}

const loadResource = async (part: number) => {
  const resource = props.videoInfo.resources[part - 1]
  const res = await getResourceQualityApi(resource.id)
  if (res.data.code === statusCode.OK) {
    // 复制并根据分辨率宽度 & 帧率从高到低排序
    const qualities = [...res.data.data.quality] as (keyof typeof resourceNameMap)[]
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

    // 映射并设置默认质量索引
    options.video.quality = qualities.map((item, index) => {
      const name = resourceNameMap[item]
      if (name === defaultQuality.value) {
        options.video.defaultQuality = index
      }
      return {
        name,
        url: getVideoFileUrl(resource.id, item),
      }
    })
  }
}

// ===== 弹幕相关方法 =====
let originalDanmaku: DanmakuType[] = [];
const setDanmaku = (data: DanmakuType[]) => {
  originalDanmaku = data;
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

  const data = originalDanmaku.filter((item) => {
    return !isDisableType(item, filter.disableType) && (Math.floor(Math.random() * 10) + 1) > filter.disableLeave;
  }).map((d) => { return { ...d } });

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
  if (hasEnded) {
    console.log('视频已播放结束，跳过进度上报');
    return;
  }
  await addHistoryAPI({ vid: props.videoInfo.vid, part: props.part, time: player.video.currentTime });
}

// ===== 分集切换监听 =====
watch(() => props.part, (newPart, oldPart) => {
  if (newPart !== oldPart) {
    // 切换前上报当前进度（如果未播放完）
    if (!hasEnded && !isWatched()) {
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
    addHistoryAPI({ vid: props.videoInfo.vid, part: props.part, time: player.video.currentTime });
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
});

// ===== 对外暴露方法 =====
defineExpose({
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
