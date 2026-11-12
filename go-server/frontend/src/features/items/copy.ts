import type { Copy } from "@/lib/copy";

const zh = {
  lede: (n: number | string) => `从 ${n} 条已评估内容中，筛选值得你花时间阅读的部分。`,
  title: "今日情报",
  lead: "LEAD",
  articles: "Articles",
  page: "Page",
  highValue: "高价值",
  highValueHint: "只显示 AI 评分 ≥ 7 的内容",
  star: "收藏",
  unknownSource: "未知来源",
  noSummary: "暂无摘要，点击阅读全文。",
  emptyTitle: "这里还很安静",
  emptyBody: "检查订阅源，或点击右上角「立即抓取」获取最新内容。",
  publishedAt: (t: string) => `发布于: ${t}`,
  unknownTime: "未知时间",
  starred: "已收藏",
  aiInsights: "AI 深度分析",
  scoreOfTen: (v: number | string) => `评分 ${v} / 10`,
  coreSummary: "核心摘要",
  loadingContent: "正文加载中…",
  categoryArticle: "文章",
  categoryTweet: "推文",
  categoryProject: "项目",
  categoryPaper: "论文",
};

export type ItemsCopy = typeof zh;

const en: ItemsCopy = {
  lede: (n: number | string) => `A considered selection from ${n} evaluated stories.`,
  title: "Today's signal",
  lead: "LEAD",
  articles: "Articles",
  page: "Page",
  highValue: "High value",
  highValueHint: "Only items scoring 7+",
  star: "Star",
  unknownSource: "Unknown source",
  noSummary: "No summary available. Click to read.",
  emptyTitle: "Nothing here yet",
  emptyBody: "Check your sources or run a scrape to fill this space.",
  publishedAt: (t: string) => `Published at: ${t}`,
  unknownTime: "Unknown time",
  starred: "Starred",
  aiInsights: "AI insights",
  scoreOfTen: (v: number | string) => `Score ${v} / 10`,
  coreSummary: "Core summary",
  loadingContent: "Loading content…",
  categoryArticle: "Article",
  categoryTweet: "Tweet",
  categoryProject: "Project",
  categoryPaper: "Paper",
};

export const itemsCopy: Copy<ItemsCopy> = { zh, en };

export const categoryLabel = (c: ItemsCopy, category: string) => {
  switch (category) {
    case "article": return c.categoryArticle;
    case "tweet": return c.categoryTweet;
    case "project": return c.categoryProject;
    case "paper": return c.categoryPaper;
    default: return category;
  }
};
