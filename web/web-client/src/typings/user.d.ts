interface UserInfoType {
  uid: number
  name: string
  avatar: string
  spaceCover?: string
  email?: string
  gender?: number
  status?: number
  sign?: string
  birthday?: string
  createdAt?: string
  role?: number
  /** 搜索用户等接口可能返回 */
  fans?: number
}

interface EditUserInfoType {
  avatar: string;
  name: string;
  gender?: number;
  sign?: string;
  birthday?: string;
  spaceCover?: string
}

interface ModifyPwdCheckType {
  email: string;
  captchaId: string;
}

interface ModifyPwdType {
  email: string;
  password: string;
  code: string; //验证码
  captchaId: string;
}

interface BanUserType {
  uid: number;
  endTime: string;
  reason: string;
}