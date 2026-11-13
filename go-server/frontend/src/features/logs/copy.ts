import type { Copy } from "@/lib/copy";

const zh = {
  kicker: "Activity · 运行记录",
  title: "抓取日志",
  description: "每次抓取的执行状态、条目数量与诊断信息。",
  refresh: "刷新",
  colStarted: "开始时间",
  colSource: "数据源",
  colStatus: "状态",
  colFound: "发现",
  colAdded: "新增",
  colDiagnostic: "诊断",
  empty: "暂无抓取执行日志。",
};

type LogsCopy = typeof zh;

const en: LogsCopy = {
  kicker: "Activity · Runs",
  title: "Fetch logs",
  description: "Execution status, item counts and diagnostics for every scrape run.",
  refresh: "Refresh",
  colStarted: "Started",
  colSource: "Source",
  colStatus: "Status",
  colFound: "Found",
  colAdded: "Added",
  colDiagnostic: "Diagnostic",
  empty: "No execution logs available.",
};

export const logsCopy: Copy<LogsCopy> = { zh, en };
export type { LogsCopy };
