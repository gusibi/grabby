import type { Copy } from "@/lib/copy";

const zh = {
  refresh: "刷新",
  open: "打开",
  all: "全部",
  srcSearch: "搜索",
  srcTimeline: "时间线",
  srcLikes: "点赞",

  // Web extracts
  extractKicker: "Captures · 网页缓存",
  extractTitle: "网页提取",
  extractDescription: (total: number) =>
    `通过 /api/extract 抓取并按 URL 去重的网页缓存，共 ${total} 条。再次请求同一地址会直接命中缓存。`,
  extractColTitle: "标题 / URL",
  extractColChars: "字数",
  extractColUpdated: "更新时间",
  extractEmpty: "暂无网页提取记录。",
  extractNoTitle: "（无标题）",

  // Tweets
  tweetKicker: "Captures · 社交归档",
  tweetTitle: "推文归档",
  tweetDescription: (total: number) => `按 ID 去重的推文档案，覆盖 search / timeline / likes，共 ${total} 条。`,
  tweetColAuthor: "作者",
  tweetColText: "正文",
  tweetColEngagement: "互动",
  tweetColSource: "来源",
  tweetEmpty: "暂无推文记录。",
  tweetOpen: "打开推文",
};

type CapturesCopy = typeof zh;

const en: CapturesCopy = {
  refresh: "Refresh",
  open: "Open",
  all: "All",
  srcSearch: "Search",
  srcTimeline: "Timeline",
  srcLikes: "Likes",

  extractKicker: "Captures · Web cache",
  extractTitle: "Web extracts",
  extractDescription: (total: number) =>
    `Pages cached via /api/extract and deduplicated by URL — ${total} in total. Repeat requests hit the cache directly.`,
  extractColTitle: "Title / URL",
  extractColChars: "Chars",
  extractColUpdated: "Updated",
  extractEmpty: "No captured pages yet.",
  extractNoTitle: "(No title)",

  tweetKicker: "Captures · Social archive",
  tweetTitle: "Tweet archive",
  tweetDescription: (total: number) => `Tweets deduplicated by ID across search, timeline and likes — ${total} in total.`,
  tweetColAuthor: "Author",
  tweetColText: "Text",
  tweetColEngagement: "Engagement",
  tweetColSource: "Source",
  tweetEmpty: "No captured tweets yet.",
  tweetOpen: "Open tweet",
};

export const capturesCopy: Copy<CapturesCopy> = { zh, en };
export type { CapturesCopy };
