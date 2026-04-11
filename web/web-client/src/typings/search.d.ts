/** 与后端 dto.SearchKeywordPageReq 一致 */
interface SearchKeywordPageType {
  page: number;
  pageSize: number;
  keywords: string;
  /** newest | most_viewed | relevance（MVP：relevance≈newest） */
  sort?: string;
  /** all | 24h | week | month | year */
  timeRange?: string;
}
