interface PlaylistType {
  id: number;
  uid: number;
  title: string;
  cover: string;
  desc: string;
  isOpen: boolean;
  status: number;
  videoCount: number;
  views: number;
  favorites: number;
  createdAt: string;
  author: UserInfoType;
}

interface ReviewPlaylistListParam {
  page: number;
  pageSize: number;
}

interface ReviewPlaylistParam {
  id: number;
  status?: number;
  remark?: string;
}

interface PlaylistVideoType {
  vid: number;
  title: string;
  cover: string;
  duration: number;
  clicks: number;
  desc: string;
  createdAt: string;
}
