interface ReviewListParam {
  page: number;
  pageSize: number;
}

interface ReviewArticleListParam {
  page: number;
  pageSize: number;
}

interface ReviewVideoParam {
  vid: number;
  status?: number;
  remark?: string;
  conflictResourceId?: number; // 冲突稿件的资源ID（可选，用于重复内容驳回）
  conflictReason?: string;     // 冲突原因说明（可选）
}

interface ReviewArticleParam {
  aid: number;
  status?: number;
  remark?: string;
}