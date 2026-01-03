import request from '@/utils/request';

//修改资源标题
export const modifyTitleAPI = (resourceTitle: BaseResourceType) => {
  return request.put('v1/resource/modifyTitle', resourceTitle);
}

//删除资源
export const deleteResourceAPI = (id: number) => {
  return request.delete(`v1/resource/deleteResource/${id}`);
}

//检查资源是否需要替换（比较hash）
export const checkReplaceResourceAPI = (resourceId: number, hash: string) => {
  return request.post('v1/resource/checkReplace', { resourceId, hash });
}

//替换资源
export const replaceResourceAPI = (resourceId: number, hash: string) => {
  return request.post('v1/resource/replaceResource', { resourceId, hash });
}