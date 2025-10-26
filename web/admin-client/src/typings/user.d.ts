interface UserListParam {
  page: number;
  pageSize: number;
}

interface EditUserRoleParam {
  uid: number;
  code: string;
}

interface UserInfoType {
  uid: number;
  name: string;
  avatar: string;
  spaceCover?: string;
  email?: string;
  gender?: number;
  sign?: string;
  birthday?: string;
  createdAt?: string;
  role?: string;
  status: number;
}

interface UserFormType {
  uid: number;
  name: string;
  avatar: string;
  spaceCover: string;
  email: string;
  sign: string;
  role: string;
}

interface BanUserType {
  uid: number;
  endTime: string;
  reason: string;
}

interface BanRecordType {
  id: number;
  endTime: string;
  reason: string;
  createdAt: string;
  status: number;
  operator: number;
}