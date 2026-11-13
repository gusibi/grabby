import type { Copy } from "@/lib/copy";
import type { AppView } from "@/types";

type TitleMap = Record<AppView, { title: string; kicker: string }>;

const zhTitles: TitleMap = {
  grid: { title: "聚合发现", kicker: "FEED" },
  list: { title: "列表", kicker: "FEED" },
  daily: { title: "智能日报", kicker: "BRIEFING" },
  settings: { title: "订阅数据源", kicker: "SOURCES" },
  "ai-settings": { title: "模型接入", kicker: "INTELLIGENCE" },
  logs: { title: "抓取日志", kicker: "ACTIVITY" },
  device: { title: "设备与连接", kicker: "SYSTEM" },
  backup: { title: "备份与恢复", kicker: "BACKUP" },
  "captures-extract": { title: "网页提取", kicker: "CAPTURES" },
  "captures-twitter": { title: "推文归档", kicker: "CAPTURES" },
};

const enTitles: TitleMap = {
  grid: { title: "Discovery", kicker: "FEED" },
  list: { title: "List", kicker: "FEED" },
  daily: { title: "Daily briefing", kicker: "BRIEFING" },
  settings: { title: "Data sources", kicker: "SOURCES" },
  "ai-settings": { title: "Model access", kicker: "INTELLIGENCE" },
  logs: { title: "Fetch logs", kicker: "ACTIVITY" },
  device: { title: "Devices", kicker: "SYSTEM" },
  backup: { title: "Backup & restore", kicker: "BACKUP" },
  "captures-extract": { title: "Web extracts", kicker: "CAPTURES" },
  "captures-twitter": { title: "Tweet archive", kicker: "CAPTURES" },
};

const zh = {
  titles: zhTitles,
  pluginConnected: "插件已连接",
  pluginOffline: "插件离线",
  addSource: "添加数据源",
  scrapeNow: "立即抓取",
  logOut: "退出登录",

  groupRead: "阅读",
  groupCapture: "采集",
  groupAdmin: "管理",
  groupCategory: "分类",
  navSources: "订阅数据源",
  navModel: "模型配置",
  navLogs: "抓取日志",
  navDevice: "设备与连接",
  navBackup: "备份与恢复",
  lightMode: "切换浅色",
  darkMode: "切换深色",
  light: "浅色",
  dark: "深色",
  switchLanguage: "切换语言",
  zhShort: "中",
  zhName: "中文",
  expandSidebar: "展开侧栏",
  collapseSidebar: "收起侧栏",
  brandTagline: "INTELLIGENCE DESK",
  footer: "GRABBY · LOCAL FIRST",
};

export type ShellCopy = typeof zh;

const en: ShellCopy = {
  titles: enTitles,
  pluginConnected: "Extension connected",
  pluginOffline: "Extension offline",
  addSource: "Add source",
  scrapeNow: "Scrape now",
  logOut: "Log out",

  groupRead: "Read",
  groupCapture: "Capture",
  groupAdmin: "Manage",
  groupCategory: "Topics",
  navSources: "Data sources",
  navModel: "Model settings",
  navLogs: "Fetch logs",
  navDevice: "Devices",
  navBackup: "Backup & restore",
  lightMode: "Switch to light",
  darkMode: "Switch to dark",
  light: "Light",
  dark: "Dark",
  switchLanguage: "Switch language",
  zhShort: "EN",
  zhName: "English",
  expandSidebar: "Expand sidebar",
  collapseSidebar: "Collapse sidebar",
  brandTagline: "INTELLIGENCE DESK",
  footer: "GRABBY · LOCAL FIRST",
};

export const shellCopy: Copy<ShellCopy> = { zh, en };
