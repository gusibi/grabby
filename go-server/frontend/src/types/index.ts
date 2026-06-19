export type AppView = "grid" | "list" | "settings" | "logs" | "daily" | "ai-settings" | "device" | "captures-extract" | "captures-twitter";

// 抓取记录：网页提取缓存的一条记录
export interface ExtractCaptureRecord {
  url: string;
  title: string;
  chars: number;
  created_at: string;
  updated_at: string;
}

// 抓取记录：归档的一条推文
export interface TweetCaptureRecord {
  id: string;
  text: string;
  author: string;
  author_name: string;
  tweet_created_at: string;
  favorite_count: number;
  retweet_count: number;
  reply_count: number;
  quote_count: number;
  url: string;
  media: string[] | null;
  source: string;
  fetched_at: string;
}

export interface Source {
  id: string;
  name: string;
  type: string;
  url: string;
  schedule: string;
  enabled: number;
  default_category: string;
  config: string;
  last_etag?: string;
  last_modified?: string;
  last_fetch_at?: string;
  last_fetch_status?: string;
  category?: string;
  created_at: string;
  updated_at: string;
}

export interface ScrapedItem {
  id: number;
  source_id: string;
  origin_source: string;
  title: string;
  url: string;
  summary: string;
  content: string;
  category: string;
  source_category?: string;
  published_at?: string;
  fetched_at: string;
  read_status: number;
  starred: number;
  tags: string;
  ai_category?: string;
  ai_subcategory?: string;
  quality_score?: number;
  ai_summary?: string;
  ai_comment?: string;
  ai_tags?: string;
  ai_model_used?: string;
  ai_processed_at?: string;
}

export interface FetchLog {
  id: number;
  source_id: string;
  started_at: string;
  finished_at?: string;
  status: string;
  items_found: number;
  items_added: number;
  error_message: string;
}

export interface Stats {
  total_count: number;
  unread_count: number;
  starred_count: number;
  categories: Record<string, number>;
  source_categories?: string[];
  source_category_unread?: Record<string, number>;
}

export interface AIProviderProfile {
  id: string;
  name: string;
  provider: string;
  api_key: string;
  model: string;
  base_url: string;
  disabled?: boolean;
  priority?: number;
  requests_per_minute?: number;
}

export interface AICategory {
  name: string;
  count: number;
  avg_score: number;
}

export interface DailyReportSection {
  title: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  items: any[];
}

export interface JsonDailyReportData {
  title?: string;
  date?: string;
  editor?: string;
  sections?: Record<string, DailyReportSection>;
}

export interface DailyReport {
  title: string;
  report_date: string;
  report_type: string;
  generated_at: string;
  model_used: string;
  total_items: number;
  quality_items: number;
  content: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  [key: string]: any;
}

export interface ReportListItem {
  title: string;
  report_date: string;
  report_type: string;
  quality_items: number;
  generated_at: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  [key: string]: any;
}

export interface SourceForm {
  id: string;
  name: string;
  type: string;
  url: string;
  schedule: string;
  default_category: string;
  config: string;
  category: string;
}
