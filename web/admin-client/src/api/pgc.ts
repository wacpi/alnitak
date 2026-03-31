import request from '@/utils/request';

export const getPGCReviewListAPI = (data: PGCReviewListParam) => {
  return request.post('v1/pgc/getReviewList', data);
};

export const reviewPGCApprovedAPI = (data: PGCReviewActionParam) => {
  return request.post('v1/pgc/reviewApproved', data);
};

export const reviewPGCFailedAPI = (data: PGCReviewActionParam) => {
  return request.post('v1/pgc/reviewFailed', data);
};
