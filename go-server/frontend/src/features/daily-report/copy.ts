import type { Copy } from "@/lib/copy";

const zh = {
  tabAll: "全部",
  tabMorning: "早报",
  tabEvening: "晚报",

  weekdayShort: ["周日", "周一", "周二", "周三", "周四", "周五", "周六"],
  today: (md: string) => `今天 ${md}`,
  yesterday: (md: string) => `昨天 ${md}`,
  backToToday: "← 回到今天",
  prevDay: "前一天",
  nextDay: "后一天",

  historyTitle: "历史日报",
  historyEmpty: "暂无日报记录",
  qualityItems: (n: number) => `${n} 优质`,

  title: "智能日报",
  subtitle: "AI 汇编整理的深度资讯与数据看板。",
  generatedAt: (t: string) => `生成时间: ${t}`,
  modelUsed: (m: string) => `使用模型: ${m}`,

  emptyTitle: "该日期暂无日报",
  emptyBody: "选择日期后，点击右侧「生成早报 / 晚报」即可编排当日内容。",

  actionsTitle: "生成操作",
  genMorning: "生成早报",
  genEvening: "生成晚报",
  genBoth: "生成早 + 晚报",
  backfill: "补齐历史早 / 晚报",

  statsTitle: "数据概览",
  statProcessed: "全天处理",
  statQuality: "入选优质",
  statRatio: "质量占比",
  statUnit: (n: number) => `${n} 条`,
  metaModel: (m: string) => `模型: ${m}`,
  metaGenerated: (t: string) => `生成: ${t}`,

  editorByline: (name: string) => `主编 ${name}`,
  itemCount: (n: number) => `${n} 条`,
  deepDive: "深度解析",
  readOriginal: "阅读原文",

  subscribeTitle: "订阅",
  subscribeRss: "RSS 订阅日报",

  toastGenFailed: "生成早报和晚报失败",
  toastBackfillDone: "补齐历史数据完成！",
  toastBackfillFailed: "补齐历史数据失败",
};

type DailyCopy = typeof zh;

const en: DailyCopy = {
  tabAll: "All",
  tabMorning: "Morning",
  tabEvening: "Evening",

  weekdayShort: ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"],
  today: (md: string) => `Today ${md}`,
  yesterday: (md: string) => `Yesterday ${md}`,
  backToToday: "← Back to today",
  prevDay: "Previous day",
  nextDay: "Next day",

  historyTitle: "Past briefings",
  historyEmpty: "No briefings yet",
  qualityItems: (n: number) => `${n} picked`,

  title: "Daily briefing",
  subtitle: "Deep reads and dashboards compiled by the AI.",
  generatedAt: (t: string) => `Generated ${t}`,
  modelUsed: (m: string) => `Model ${m}`,

  emptyTitle: "No briefing for this date",
  emptyBody: "Pick a date, then generate a morning or evening briefing from the panel on the right.",

  actionsTitle: "Generate",
  genMorning: "Generate morning",
  genEvening: "Generate evening",
  genBoth: "Generate both",
  backfill: "Backfill past briefings",

  statsTitle: "Overview",
  statProcessed: "Processed",
  statQuality: "Selected",
  statRatio: "Quality share",
  statUnit: (n: number) => `${n} items`,
  metaModel: (m: string) => `Model: ${m}`,
  metaGenerated: (t: string) => `Generated: ${t}`,

  editorByline: (name: string) => `Editor: ${name}`,
  itemCount: (n: number) => `${n} items`,
  deepDive: "Deep dive",
  readOriginal: "Read the original",

  subscribeTitle: "Subscribe",
  subscribeRss: "RSS feed",

  toastGenFailed: "Could not generate the briefings",
  toastBackfillDone: "Backfill complete.",
  toastBackfillFailed: "Backfill failed",
};

export const dailyCopy: Copy<DailyCopy> = { zh, en };
export type { DailyCopy };
