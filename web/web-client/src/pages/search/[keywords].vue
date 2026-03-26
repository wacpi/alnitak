<template>
  <div class="search-page">
    <header-bar></header-bar>
    <div class="search">
      <div class="search-form">
        <input
          class="input"
          v-model="searchKeywords"
          @keydown.enter="submitSearch"
        >
        <button class="btn" type="button" @click="submitSearch">
          <search-icon class="icon" size="16" />
        </button>
      </div>
    </div>

    <el-tabs v-model="activeTab" class="search-tabs" @tab-change="onTabChange">
      <el-tab-pane label="视频" name="video">
        <div class="card-list">
          <video-item
            v-for="item in videoList"
            :key="item.vid"
            :info="item"
            :keywords="routeKeywords"
          />
        </div>
        <p v-if="!videoLoading && videoList.length === 0 && didSearch" class="empty-hint">暂无相关视频</p>
      </el-tab-pane>
      <el-tab-pane label="专栏" name="article">
        <ul class="article-search-list">
          <li v-for="item in articleList" :key="item.aid" class="article-search-item">
            <nuxt-link class="content-wrapper" :to="`/article/${item.aid}`" target="_blank">
              <div class="content-main">
                <div class="title-row">{{ item.title }}</div>
                <div class="abstract">{{ removeHtml(item.content) }}</div>
                <div class="entry-footer">
                  <span class="clicks">{{ item.clicks }} 阅读</span>
                  <span v-if="item.tags" class="tags">{{ item.tags }}</span>
                </div>
              </div>
              <img v-if="item.cover" class="cover" :src="getResourceUrl(item.cover)" alt="封面">
            </nuxt-link>
          </li>
        </ul>
        <p v-if="!articleLoading && articleList.length === 0 && didSearch" class="empty-hint">暂无相关专栏</p>
      </el-tab-pane>
      <el-tab-pane label="UP主" name="user">
        <ul class="user-search-list">
          <li v-for="u in userList" :key="u.uid" class="user-search-item">
            <nuxt-link class="user-link" :to="`/user/${u.uid}`" target="_blank">
              <common-avatar class="avatar" :size="48" :url="u.avatar" />
              <div class="meta">
                <div class="name">{{ u.name }}</div>
                <div class="sign">{{ u.sign || '暂无签名' }}</div>
                <div class="fans" v-if="u.fans != null">粉丝 {{ u.fans }}</div>
              </div>
            </nuxt-link>
          </li>
        </ul>
        <p v-if="!userLoading && userList.length === 0 && didSearch" class="empty-hint">暂无相关用户</p>
      </el-tab-pane>
    </el-tabs>

    <p v-if="footerLoading" class="loading-more">加载中...</p>
    <p v-else-if="currentNoMore && currentListNonEmpty" class="loading-more muted">没有更多了</p>
  </div>
</template>

<script setup lang="ts">
import { onBeforeMount, onBeforeUnmount, ref, watch, computed } from 'vue';
import { searchVideoAPI } from '@/api/video';
import { searchArticleAPI } from '@/api/article';
import { searchUserAPI } from '@/api/user';
import HeaderBar from '@/components/header-bar/index.vue';
import VideoItem from './components/VideoItem.vue';
import CommonAvatar from '@/components/common-avatar/index.vue';
import { Search as SearchIcon } from '@icon-park/vue-next';
import { removeHtml } from '@/utils/format';
import { getResourceUrl } from '@/utils/resource';

const route = useRoute();
const router = useRouter();

const pageSize = 15;
const searchKeywords = ref('');
const routeKeywords = ref('');
const activeTab = ref<'video' | 'article' | 'user'>('video');
const didSearch = ref(false);

const videoList = ref<VideoType[]>([]);
const videoPage = ref(1);
const videoNoMore = ref(false);
const videoLoading = ref(false);

const articleList = ref<ArticleType[]>([]);
const articlePage = ref(1);
const articleNoMore = ref(false);
const articleLoading = ref(false);

const userList = ref<UserInfoType[]>([]);
const userPage = ref(1);
const userNoMore = ref(false);
const userLoading = ref(false);

const syncKeywordsFromRoute = () => {
  const raw = route.params.keywords?.toString() ?? '';
  try {
    routeKeywords.value = decodeURIComponent(raw);
  } catch {
    routeKeywords.value = raw;
  }
  searchKeywords.value = routeKeywords.value;
};

const resetAllLists = () => {
  videoList.value = [];
  videoPage.value = 1;
  videoNoMore.value = false;
  articleList.value = [];
  articlePage.value = 1;
  articleNoMore.value = false;
  userList.value = [];
  userPage.value = 1;
  userNoMore.value = false;
  didSearch.value = false;
};

const loadVideos = async (init: boolean) => {
  if (videoLoading.value) return;
  videoLoading.value = true;
  if (init) {
    videoPage.value = 1;
    videoList.value = [];
    videoNoMore.value = false;
  }
  const res = await searchVideoAPI({
    page: videoPage.value,
    pageSize,
    keywords: routeKeywords.value,
  });
  if (res.data.code === statusCode.OK && res.data.data.videos) {
    const chunk = res.data.data.videos;
    videoList.value.push(...chunk);
    if (chunk.length < pageSize) videoNoMore.value = true;
  } else {
    videoNoMore.value = true;
    if (init) ElMessage.error(res.data.msg || '获取失败');
  }
  videoLoading.value = false;
  didSearch.value = true;
};

const loadArticles = async (init: boolean) => {
  if (articleLoading.value) return;
  articleLoading.value = true;
  if (init) {
    articlePage.value = 1;
    articleList.value = [];
    articleNoMore.value = false;
  }
  const res = await searchArticleAPI({
    page: articlePage.value,
    pageSize,
    keywords: routeKeywords.value,
  });
  if (res.data.code === statusCode.OK && res.data.data.articles) {
    const chunk = res.data.data.articles;
    articleList.value.push(...chunk);
    if (chunk.length < pageSize) articleNoMore.value = true;
  } else {
    articleNoMore.value = true;
    if (init) ElMessage.error(res.data.msg || '获取失败');
  }
  articleLoading.value = false;
  didSearch.value = true;
};

const loadUsers = async (init: boolean) => {
  if (userLoading.value) return;
  userLoading.value = true;
  if (init) {
    userPage.value = 1;
    userList.value = [];
    userNoMore.value = false;
  }
  const res = await searchUserAPI({
    page: userPage.value,
    pageSize,
    keywords: routeKeywords.value,
  });
  if (res.data.code === statusCode.OK && res.data.data.users) {
    const chunk = res.data.data.users;
    userList.value.push(...chunk);
    if (chunk.length < pageSize) userNoMore.value = true;
  } else {
    userNoMore.value = true;
    if (init) ElMessage.error(res.data.msg || '获取失败');
  }
  userLoading.value = false;
  didSearch.value = true;
};

const loadCurrentTab = (init: boolean) => {
  if (activeTab.value === 'video') return loadVideos(init);
  if (activeTab.value === 'article') return loadArticles(init);
  return loadUsers(init);
};

const submitSearch = () => {
  const k = searchKeywords.value.trim();
  if (!k) {
    ElMessage.warning('请输入关键词');
    return;
  }
  router.push(`/search/${encodeURIComponent(k)}`);
};

const onTabChange = (name: string) => {
  if (!routeKeywords.value.trim()) return;
  if (name === 'article' && articleList.value.length === 0) loadArticles(true);
  if (name === 'user' && userList.value.length === 0) loadUsers(true);
};

const footerLoading = computed(() => {
  if (activeTab.value === 'video') return videoLoading.value;
  if (activeTab.value === 'article') return articleLoading.value;
  return userLoading.value;
});

const currentNoMore = computed(() => {
  if (activeTab.value === 'video') return videoNoMore.value;
  if (activeTab.value === 'article') return articleNoMore.value;
  return userNoMore.value;
});

const currentListNonEmpty = computed(() => {
  if (activeTab.value === 'video') return videoList.value.length > 0;
  if (activeTab.value === 'article') return articleList.value.length > 0;
  return userList.value.length > 0;
});

const lazyLoading = () => {
  if (currentNoMore.value || footerLoading.value) return;
  const scrollTop = document.documentElement.scrollTop || document.body.scrollTop;
  const clientHeight = document.documentElement.clientHeight;
  const scrollHeight = document.documentElement.scrollHeight;
  if (scrollTop + clientHeight < scrollHeight - 10) return;

  if (activeTab.value === 'video') {
    videoPage.value++;
    loadVideos(false);
  } else if (activeTab.value === 'article') {
    articlePage.value++;
    loadArticles(false);
  } else {
    userPage.value++;
    loadUsers(false);
  }
};

watch(
  () => route.params.keywords,
  async () => {
    syncKeywordsFromRoute();
    resetAllLists();
    activeTab.value = 'video';
    await loadVideos(true);
  },
);

onBeforeMount(async () => {
  syncKeywordsFromRoute();
  await loadVideos(true);
  window.addEventListener('scroll', lazyLoading, true);
});

onBeforeUnmount(() => {
  window.removeEventListener('scroll', lazyLoading, true);
});
</script>

<style lang="scss" scoped>
.search-page {
  min-height: 100vh;
  padding-bottom: 40px;
  background: var(--bg-page);
}

.search {
  width: 100%;
  margin: 30px auto;
  max-width: 500px;

  .search-form {
    position: relative;

    .input {
      border: 1px solid #e5e5e5;
      outline: none;
      padding: 8px 30px 8px 11px;
      height: 36px;
      font-size: 12px;
      line-height: 14px;
      border-radius: 18px;
      width: 100%;
      max-width: 500px;
      vertical-align: top;
      color: var(--font-primary-1);
      box-sizing: border-box;
    }

    .btn {
      position: absolute;
      top: 0;
      right: 10px;
      border: none;
      width: 20px;
      height: 36px;
      line-height: 36px;
      font-size: 14px;
      vertical-align: top;
      background: transparent;
      padding: 0;
      cursor: pointer;

      .icon {
        display: block;
        margin-top: 3px;
        width: 20px;
        height: 36px;
        color: var(--font-primary-5);
      }
    }
  }
}

.search-tabs {
  width: 90%;
  margin: 0 auto;
}

.card-list {
  display: grid;
  column-gap: 16px;
  grid-template-columns: repeat(5, 1fr);
  margin-top: 16px;
}

.article-search-list {
  list-style: none;
  padding: 0;
  margin: 16px 0 0;
}

.article-search-item {
  margin-bottom: 16px;

  .content-wrapper {
    display: flex;
    gap: 16px;
    padding: 16px;
    background: var(--bg-elev-1);
    border-radius: 8px;
    text-decoration: none;
    color: inherit;
  }

  .content-main {
    flex: 1;
    min-width: 0;
  }

  .title-row {
    font-size: 16px;
    font-weight: 500;
    margin-bottom: 8px;
  }

  .abstract {
    font-size: 13px;
    color: var(--font-primary-3);
    line-height: 1.5;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  .entry-footer {
    margin-top: 8px;
    font-size: 12px;
    color: var(--font-primary-5);
    display: flex;
    gap: 12px;
  }

  .cover {
    width: 160px;
    height: 100px;
    object-fit: cover;
    border-radius: 6px;
    flex-shrink: 0;
  }
}

.user-search-list {
  list-style: none;
  padding: 0;
  margin: 16px 0 0;
}

.user-search-item {
  margin-bottom: 12px;

  .user-link {
    display: flex;
    align-items: center;
    gap: 16px;
    padding: 12px 16px;
    background: var(--bg-elev-1);
    border-radius: 8px;
    text-decoration: none;
    color: inherit;
  }

  .meta {
    flex: 1;
    min-width: 0;
  }

  .name {
    font-weight: 500;
    font-size: 15px;
  }

  .sign {
    font-size: 13px;
    color: var(--font-primary-3);
    margin-top: 4px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .fans {
    font-size: 12px;
    color: var(--font-primary-5);
    margin-top: 4px;
  }
}

.empty-hint {
  text-align: center;
  color: var(--font-primary-5);
  padding: 24px;
}

.loading-more {
  text-align: center;
  padding: 16px;
  &.muted {
    color: var(--font-primary-5);
    font-size: 13px;
  }
}
</style>
