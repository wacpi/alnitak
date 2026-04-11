import request from '@/utils/request';

// 获取待审核合集列表
export const getReviewPlaylistListAPI = (data: ReviewPlaylistListParam) => {
  return request.post("v1/playlist/getReviewPlaylistList", data);
}

// 审核通过
export const reviewPlaylistApprovedAPI = (data: ReviewPlaylistParam) => {
  return request.post("v1/playlist/reviewApproved", data);
}

// 审核不通过
export const reviewPlaylistFailedAPI = (data: ReviewPlaylistParam) => {
  return request.post("v1/playlist/reviewFailed", data);
}

// 获取合集视频列表
export const getPlaylistVideoListAPI = (id: number, page: number, pageSize: number) => {
  return request.get(`v1/playlist/video/list?playlistId=${id}&page=${page}&pageSize=${pageSize}`);
}

// 获取全站合集列表（后台管理）
export const getPlaylistListManageAPI = (data: ReviewPlaylistListParam) => {
  return request.post("v1/playlist/getPlaylistListManage", data);
}

// 删除合集（后台管理）
export const deletePlaylistManageAPI = (id: number) => {
  return request.delete(`v1/playlist/deletePlaylistManage/${id}`);
}
