import request from '@/utils/request';

// 获取用户信息
export const getUserInfoAPI = () => {
  return request.get('v1/user/getUserInfo');
}

// 获取用户列表
export const getUserListAPI = (data: UserListParam) => {
  return request.post("v1/user/getUserListManage", data);
}

// 编辑用户角色
export const editUserRoleAPI = (data: EditUserRoleParam) => {
  return request.put("v1/user/editUserRole", data);
}

// 编辑用户信息
export const editUserInfoAPI = (data: UserFormType) => {
  return request.put("v1/user/editUserInfoManage", data);
}

// 封禁用户
export const banUserAPI = (data: BanUserType) => {
  return request.put(`v1/user/banUser`, data);
}

// 解封用户
export const unbanUserAPI = (id: number) => {
  return request.put(`v1/user/unBanUser`, { id });
}

// 获取封禁记录
export const getUserBanRecordAPI = (id: number) => {
  return request.get(`v1/user/getUserBanRecord?uid=${id}`);
}

// 删除用户
export const deleteUserAPI = (id: number) => {
  return request.delete(`v1/user/deleteUser/${id}`);
}