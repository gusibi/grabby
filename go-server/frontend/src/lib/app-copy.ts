import type { Copy } from "@/lib/copy";

/** App-level copy: default profile names and the toast/confirm messages raised from App.tsx. */
const zh = {
  defaultProfileName: "默认服务商",
  unnamedProfile: "未命名服务商",
  newProfileName: (n: number) => `服务商 ${n}`,
  reportMorning: "AI 早报",
  reportEvening: "AI 晚报",

  errCannotDeleteTitle: "无法删除",
  errCannotDeleteBody: "至少需要保留一个连接档案。",
  savedTitle: "配置已保存",
  savedBody: "AI 引擎已按新的设置重新加载。",
  saveFailedTitle: "保存失败",
  saveFailedBody: "未知错误",
  saveNetworkBody: "请检查网络连接后重试。",
  testUnknownError: "未知接口错误",
  testNetworkError: "网络连接失败，请检查后端服务是否正常启动。",
  evalStartedTitle: "评测已启动",
  evalStartedBody: "正在后台增量评估未分析的文章。",
  evalFailedTitle: "启动失败",
  evalNetworkTitle: "网络错误",
  evalNetworkBody: "启动增量评测失败。",
  reportStartedTitle: "生成已启动",
  reportStartedBody: "日报正在后台生成，完成后将自动加载。",
  reportFailedTitle: "生成失败",
  reportNetworkBody: "生成日报出现网络错误。",
  scrapeFailedTitle: "抓取失败",
  scrapeFailedBody: "未知原因",

  deleteSourceTitle: "删除订阅源",
  deleteSourceBody: "该订阅源及其抓取计划将被移除，已抓取的内容会保留。",
  delete: "删除",
};

type AppCopy = typeof zh;

const en: AppCopy = {
  defaultProfileName: "Default provider",
  unnamedProfile: "Unnamed provider",
  newProfileName: (n: number) => `Provider ${n}`,
  reportMorning: "AI morning briefing",
  reportEvening: "AI evening briefing",

  errCannotDeleteTitle: "Cannot delete",
  errCannotDeleteBody: "At least one connection profile must remain.",
  savedTitle: "Settings saved",
  savedBody: "The AI engine has reloaded with the new configuration.",
  saveFailedTitle: "Save failed",
  saveFailedBody: "Unknown error",
  saveNetworkBody: "Check your connection and try again.",
  testUnknownError: "Unknown API error",
  testNetworkError: "Network unreachable — check that the backend is running.",
  evalStartedTitle: "Evaluation started",
  evalStartedBody: "Unanalysed articles are being scored in the background.",
  evalFailedTitle: "Could not start",
  evalNetworkTitle: "Network error",
  evalNetworkBody: "Could not start the evaluation queue.",
  reportStartedTitle: "Generation started",
  reportStartedBody: "The briefing is being generated and will load when ready.",
  reportFailedTitle: "Generation failed",
  reportNetworkBody: "Could not reach the service to generate the briefing.",
  scrapeFailedTitle: "Scrape failed",
  scrapeFailedBody: "Unknown reason",

  deleteSourceTitle: "Delete data source",
  deleteSourceBody: "The source and its schedule will be removed. Already captured content stays.",
  delete: "Delete",
};

export const appCopy: Copy<AppCopy> = { zh, en };
export type { AppCopy };
