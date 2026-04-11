interface PGCRecommendNewEp {
  index_show?: string;
  title?: string;
}

interface PGCRecommendItem {
  pgc_id: string;
  season_id: string;
  media_id: string;
  pgc_type: number;
  title: string;
  cover: string;
  desc?: string;
  year?: number;
  area?: string;
  rating?: number;
  is_ongoing?: boolean;
  total_episodes?: number;
  current_episodes?: number;
  badge?: string;
  latest_ep_number?: number;
  latest_ep_id?: number;
  latest_ep_title?: string;
  latest_vid?: number;
  latest_publish_time?: string;
  new_ep?: PGCRecommendNewEp;
}

