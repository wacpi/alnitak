<template>
  <div class="home">
    <home-header class="home-header" @change-fold="changeMenuFold"></home-header>
    <div class="home-content">
      <div class="home-left" :class="menuFold ? 'home-left-fold' : ''">
        <home-sidebar class="home-sidebar" :fold="menuFold"></home-sidebar>
      </div>
      <div class="home-right" :style="`margin-left: ${menuFold ? '50px' : '220px'};`">
        <div class="home-recommended" :class="menuFold ? 'recommended-fold' : ''">
          <div class="recommended-top">
            <div class="recommended-carousel">
              <div class="recommended-carousel-inner">
                <client-only>
                  <HomeCarousel></HomeCarousel>
                </client-only>
              </div>
            </div>
            <div class="recommended-side" :class="menuFold ? 'side-fold' : ''">
              <video-item v-for="item in videoList.slice(0, menuFold ? 6 : 4)" :info="item"></video-item>
            </div>
          </div>
          <div class="recommended-grid" :class="menuFold ? 'grid-fold' : ''">
            <video-item v-for="item in videoList.slice(menuFold ? 6 : 4)" :info="item"></video-item>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref } from 'vue';
import VideoItem from '@/components/home-video-item/index.vue';
import HomeSidebar from '@/components/home-sidebar/index.vue';
import HomeHeader from "@/components/home-header/index.vue";
import HomeCarousel from '@/components/alnitak-carousel/index.vue';
import { asyncGetHotVideoAPI, getHotVideoAPI } from "@/api/video";

useHead({
  title: globalConfig.title
});

const menuFold = ref(false);
const changeMenuFold = (val: boolean) => {
  menuFold.value = val;
}

// 客户端挂载后同步折叠状态
onMounted(() => {
  try {
    const saved = localStorage.getItem('menu-fold-state');
    if (saved === 'true') {
      menuFold.value = true;
    }
  } catch {}
});

// 获取分区
const page = ref(1);
const pageSize = 16;
const videoList = ref<VideoType[]>([])
const { data } = await asyncGetHotVideoAPI(page.value, pageSize);
if ((data.value as any).code === statusCode.OK) {
  videoList.value = (data.value as any).data.videos;
}

const noMore = ref(false);
const loading = ref(false);
const getViedeoList = async () => {
  loading.value = true;
  const res = await getHotVideoAPI(page.value, pageSize);
  if (res.data.code === statusCode.OK) {
    if (res.data.data.videos) {
      videoList.value = videoList.value.concat(res.data.data.videos);
    } else {
      noMore.value = true;
    }
  }
  loading.value = false;
}

const lazyLoading = (e: Event) => {
  const scrollTop = document.documentElement.scrollTop || document.body.scrollTop;
  if (scrollTop === 0) return;

  const clientHeight = document.documentElement.clientHeight;
  const scrollHeight = document.documentElement.scrollHeight;
  if (scrollTop + clientHeight >= scrollHeight) {
    if (!noMore.value && !loading.value) {
      page.value++;
      getViedeoList();
    }
  }
}

onMounted(() => {
  window.addEventListener('scroll', lazyLoading, true);
})

onBeforeUnmount(() => {
  window.removeEventListener('scroll', lazyLoading, true);
})
</script>

<style lang="scss" scoped>
.home {
  width: 100%;
  min-width: 1200px;
  overflow: hidden;

  .home-header {
    position: fixed;
    top: 0;
    width: 100%;
    z-index: 999;
    background-color: var(--bg-elev-1);
  }
}

.home-content {
  display: flex;
  margin-top: 60px;

  .home-left {
    position: fixed;
    height: 100%;
    width: 220px;
    z-index: 1;
    background-color: var(--bg-elev-1);
    transition: width .25s;
  }

  .home-left-fold {
    width: 50px;
  }

  .home-right {
    flex: 1;
    margin-top: 6px;
    color: var(--font-primary-1);
  }
}

.home-recommended {
  margin-left: 20px;
  width: calc(100% - 50px);
  overflow: hidden;
}

.recommended-top {
  display: flex;
  align-items: stretch;
  gap: 16px;

  /* 与右侧两列视频总高度对齐，避免轮播底部留空；不设死高，避免整体过高 */
  .recommended-carousel {
    flex: 2;
    min-width: 0;
    /* 行业常用：响应式高度 + 上限，避免轮播过高 */
    min-height: 0;
    display: flex;
    flex-direction: column;
  }

  .recommended-carousel-inner {
    height: clamp(220px, 26vw, 890px);
    position: relative;
    border-radius: 9px;
    overflow: hidden;
  }

  .recommended-carousel-inner :deep(.carousel-area) {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
  }

  .recommended-side {
    flex: 2;
    min-width: 0;
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 16px;
    align-content: start;
  }

  .side-fold {
    flex: 3;
    grid-template-columns: repeat(3, 1fr);
  }
}

.recommended-grid {
  display: grid;
  gap: 16px;
  margin-top: 16px;
  grid-template-columns: repeat(4, 1fr);
}

.grid-fold {
  grid-template-columns: repeat(5, 1fr);
}
</style>