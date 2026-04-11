interface BaseResourceType {
  id: number;
  shortId?: string;
  title: string;
}

// 分P
interface ResourceType extends BaseResourceType {
  url: string;
  duration: number;
  status: number;
  quality: number;
  uploading?: boolean;
  percent?: number;
}

// 分P
interface UploadResourceType extends BaseResourceType {
  status: number;
  uploading: boolean;
  percent: number;
}
