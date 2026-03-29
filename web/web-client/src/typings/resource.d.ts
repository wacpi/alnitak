interface BaseResourceType {
  id: number;
  title: string;
}

// 分P
interface ResourceType extends BaseResourceType {
  shortId?: string;
  fileId?: number;
  sortOrder?: number;
  playUrl?: string;
  statusName?: string;
  createdAt?: string;
  uid?: number;
  /** 时长（秒，整数） */
  duration: number;
  status: number;
  /** 上传流程本地态 */
  url?: string;
  quality?: number;
  uploading?: boolean;
  percent?: number;
}

// 分P
interface UploadResourceType extends BaseResourceType {
  status: number;
  uploading: boolean;
  percent: number;
}
