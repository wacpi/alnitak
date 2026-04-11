interface VideoListParam {
  page: number;
  pageSize: number;
}

// 视频信息
interface VideoType {
  vid: number;
  title: string;
  cover: string;
  desc: string;
  createdAt: string;
  tags: string;
  clicks: number;
  copyright: boolean;
  partitionId: number;
  author: UserInfoType;
  resources: ResourceType[];
  transcodingProgress?: number;
  transcodingDetails?: TranscodingProgressItem[];
}

interface TranscodingProgressItem {
  resourceId: number;
  resourceTitle: string;
  quality: string;
  progress: number;
  status: 'waiting' | 'processing' | 'success' | 'fail' | string;
}