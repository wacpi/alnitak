/** 与后端 dto.SearchKeywordPageReq 一致 */
interface SearchKeywordPageType {
  page: number;
  pageSize: number;
  keywords: string;
}
