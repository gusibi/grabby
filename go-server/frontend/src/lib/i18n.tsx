import React, { createContext, useContext, useState } from "react";

export type Language = "zh" | "en";

const translations = {
  zh: {
    // Navigation / Shell
    "nav.grid": "聚合发现",
    "nav.daily": "AI 智能日报",
    "nav.captures": "抓取记录",
    "nav.extract": "网页提取",
    "nav.twitter": "推文归档",
    "nav.category": "分类筛选",
    "nav.all": "全部",
    "nav.admin": "管理配置",
    "nav.sources": "订阅数据源",
    "nav.settings": "AI 模型配置",
    "nav.logs": "抓取日志",
    "nav.devices": "设备与连接",
    "nav.theme": "主题",
    "nav.darkMode": "暗色模式",
    "nav.lightMode": "亮色模式",
    "nav.logout": "退出",
    "nav.login": "登录",

    // Common / Buttons
    "btn.scrape": "立即抓取数据",
    "btn.refresh": "刷新",
    "btn.save": "保存",
    "btn.cancel": "取消",
    "btn.delete": "删除",
    "btn.edit": "编辑",
    "btn.run": "执行",
    "btn.test": "测试",
    "btn.close": "关闭",
    "btn.openLink": "打开链接",

    // Headers / Titles
    "title.extract": "网页提取记录 Extract Captures",
    "title.twitter": "推文归档 Twitter Captures",
    "title.sources": "订阅数据源 Sources",
    "title.logs": "抓取日志 Logs",
    "title.daily": "AI 智能日报 Daily",
    "title.device": "设备与连接 Device",
    "title.aiSettings": "AI 模型配置 Settings",

    // Subtitles / Info
    "subtitle.extract": "通过 /api/extract 抓取并缓存的网页（按 URL 去重）。再次请求同一 URL 将直接命中缓存。",
    "subtitle.twitter": "search / timeline / likes 抓到的推文按 ID 去重存档。",
    "subtitle.sources": "配置抓取订阅源（RSS、网页、Twitter 等）。",
    "subtitle.logs": "查看抓取执行历史和日志。",
    "subtitle.daily": "查看生成的 AI Daily Report。",
    "subtitle.device": "浏览器扩展插件和 WebSocket 连接状态设置。",
    "subtitle.aiSettings": "配置大语言模型、提示词和定时日报生成规则。",

    // Grid View
    "grid.searchPlaceholder": "在标题和摘要中搜索...",
    "grid.showOnlyAI": "只显示高价值文章",
    "grid.star": "收藏",
    "grid.prev": "上一页",
    "grid.next": "下一页",
    "grid.noItems": "没有找到文章",

    // Item Detail Modal
    "item.original": "查看原文",
    "item.wordCount": "字",
    "item.score": "AI 评分",
    "item.summary": "AI 摘要",
    "item.tags": "标签",

    // Sources View
    "sources.add": "添加数据源",
    "sources.active": "已启用",
    "sources.inactive": "已禁用",
    "sources.schedule": "计划任务",
    "sources.category": "默认分类",
    "sources.type": "类型",

    // Add/Edit Source Modal
    "sources.editTitle": "编辑订阅源",
    "sources.addTitle": "新建订阅源",
    "sources.nameLabel": "数据源名称",
    "sources.urlLabel": "链接 URL",
    "sources.scheduleLabel": "定时任务 Cron 表达式",
    "sources.categoryLabel": "分类",
    "sources.configLabel": "额外配置 JSON (可选)",
    "sources.validationError": "表单字段验证失败",

    // Logs View
    "logs.status": "状态",
    "logs.source": "数据源",
    "logs.message": "日志信息",
    "logs.time": "执行时间",

    // Daily Report View
    "daily.generate": "手动生成日报",
    "daily.generating": "生成中...",
    "daily.selectType": "选择报告类型",
    "daily.morning": "早报",
    "daily.evening": "晚报",
    "daily.selectDate": "选择日期",
    "daily.reportTitle": "日报详情",
    "daily.noReport": "该日暂无报告",

    // Device View
    "device.status": "WebSocket 状态",
    "device.connected": "已连接",
    "device.disconnected": "未连接",
    "device.extId": "扩展 Extension ID",
    "device.wsUrl": "WebSocket 服务器 URL",
    "device.desc": "网页采集插件将通过以上 WebSocket 服务上传捕获的 HTML 和截图。",

    // AI Settings View
    "ai.enabled": "启用 AI 自动生成日报",
    "ai.strategy": "决策策略",
    "ai.strategySingle": "单篇即时评分",
    "ai.strategyBatch": "批量关联分析",
    "ai.profiles": "LLM 供应商配置文件",
    "ai.addProfile": "添加供应商",
    "ai.profileName": "供应商配置名称",
    "ai.provider": "供应商 API 类型",
    "ai.model": "模型名称 (如 gemini-2.5-pro)",
    "ai.apiKey": "API Key",
    "ai.baseUrl": "自定义 Base URL (可选)",
    "ai.threshold": "高价值入选评分阈值",
    "ai.rpm": "每分钟请求数限制 (RPM)",
    "ai.systemPrompt": "系统预设 Prompt",
    "ai.dailyPrompt": "日报生成 Prompt",
    "ai.morningReport": "早晨报告配置",
    "ai.eveningReport": "晚间报告配置",
    "ai.reportTime": "报告生成时间",
    "ai.test": "测试 AI 连通性",
    "ai.testing": "测试中...",
    "ai.success": "配置已保存",

    // Auth Dialog
    "auth.title": "管理员身份验证",
    "auth.desc": "此页面需要输入密码以进行写操作与后台配置。",
    "auth.pwd": "输入管理员密码",
    "auth.btn": "登录验证",
    "auth.error": "密码错误，请重试",
  },
  en: {
    // Navigation / Shell
    "nav.grid": "Discovery Grid",
    "nav.daily": "AI Daily Report",
    "nav.captures": "Captures",
    "nav.extract": "Web Extract",
    "nav.twitter": "Twitter Archive",
    "nav.category": "Topic Category",
    "nav.all": "All",
    "nav.admin": "Admin",
    "nav.sources": "Sources Settings",
    "nav.settings": "LLM Settings",
    "nav.logs": "Execution Logs",
    "nav.devices": "Devices & Sync",
    "nav.theme": "Theme",
    "nav.darkMode": "Dark Mode",
    "nav.lightMode": "Light Mode",
    "nav.logout": "Logout",
    "nav.login": "Login",

    // Common / Buttons
    "btn.scrape": "Scrape Now",
    "btn.refresh": "Refresh",
    "btn.save": "Save",
    "btn.cancel": "Cancel",
    "btn.delete": "Delete",
    "btn.edit": "Edit",
    "btn.run": "Run",
    "btn.test": "Test AI",
    "btn.close": "Close",
    "btn.openLink": "Open Link",

    // Headers / Titles
    "title.extract": "Extract Captures",
    "title.twitter": "Twitter Captures",
    "title.sources": "Data Sources",
    "title.logs": "System Logs",
    "title.daily": "AI Daily Report",
    "title.device": "Device Connectivity",
    "title.aiSettings": "AI Model Settings",

    // Subtitles / Info
    "subtitle.extract": "Webpages crawled and cached via /api/extract (deduplicated by URL). Future requests to the same URL hit the cache directly.",
    "subtitle.twitter": "Deduplicated tweet archive captured via search, timeline, and likes.",
    "subtitle.sources": "Configure subscription feeds (RSS, Web scraper, Twitter, etc.).",
    "subtitle.logs": "View historical execution stats and logs.",
    "subtitle.daily": "Browse generated AI Daily Reports.",
    "subtitle.device": "Browser extension connection settings and WebSocket configurations.",
    "subtitle.aiSettings": "Configure LLM provider profile, prompts, and report schedule triggers.",

    // Grid View
    "grid.searchPlaceholder": "Search by title and summary...",
    "grid.showOnlyAI": "High Value Articles Only",
    "grid.star": "Star",
    "grid.prev": "Previous",
    "grid.next": "Next",
    "grid.noItems": "No articles found",

    // Item Detail Modal
    "item.original": "View Original",
    "item.wordCount": "words",
    "item.score": "AI Score",
    "item.summary": "AI Summary",
    "item.tags": "Tags",

    // Sources View
    "sources.add": "Add Source",
    "sources.active": "Active",
    "sources.inactive": "Inactive",
    "sources.schedule": "Cron Schedule",
    "sources.category": "Default Category",
    "sources.type": "Type",

    // Add/Edit Source Modal
    "sources.editTitle": "Edit Source",
    "sources.addTitle": "New Source",
    "sources.nameLabel": "Source Name",
    "sources.urlLabel": "Target URL",
    "sources.scheduleLabel": "Cron Schedule Expression",
    "sources.categoryLabel": "Category",
    "sources.configLabel": "Extra Config JSON (Optional)",
    "sources.validationError": "Form field validation failed",

    // Logs View
    "logs.status": "Status",
    "logs.source": "Source",
    "logs.message": "Log Message",
    "logs.time": "Time Executed",

    // Daily Report View
    "daily.generate": "Generate Manually",
    "daily.generating": "Generating...",
    "daily.selectType": "Report Type",
    "daily.morning": "Morning Report",
    "daily.evening": "Evening Report",
    "daily.selectDate": "Select Date",
    "daily.reportTitle": "Report Details",
    "daily.noReport": "No report generated for this date",

    // Device View
    "device.status": "WebSocket Connection Status",
    "device.connected": "Connected",
    "device.disconnected": "Disconnected",
    "device.extId": "Web Extension ID",
    "device.wsUrl": "WebSocket Server URL",
    "device.desc": "The Web capturing browser extension uploads HTML snapshots and screenshots using this WebSocket service.",

    // AI Settings View
    "ai.enabled": "Enable Automatical AI Daily Reports",
    "ai.strategy": "Decision Strategy",
    "ai.strategySingle": "Instant Score (Per Article)",
    "ai.strategyBatch": "Batch Association Analysis",
    "ai.profiles": "LLM Provider Profiles",
    "ai.addProfile": "Add Provider",
    "ai.profileName": "Profile Name",
    "ai.provider": "API Provider Type",
    "ai.model": "Model ID (e.g. gemini-2.5-pro)",
    "ai.apiKey": "API Key",
    "ai.baseUrl": "Custom API Base URL (Optional)",
    "ai.threshold": "High-Value Ingestion Threshold",
    "ai.rpm": "Request Limit per Minute (RPM)",
    "ai.systemPrompt": "System Prompt",
    "ai.dailyPrompt": "Daily Report Prompt",
    "ai.morningReport": "Morning Report Config",
    "ai.eveningReport": "Evening Report Config",
    "ai.reportTime": "Trigger Time",
    "ai.test": "Test Connectivity",
    "ai.testing": "Testing...",
    "ai.success": "Settings saved successfully",

    // Auth Dialog
    "auth.title": "Admin Authentication",
    "auth.desc": "This page requires administrator access to edit configurations.",
    "auth.pwd": "Enter Admin Password",
    "auth.btn": "Verify & Login",
    "auth.error": "Invalid password, please try again",
  }
};

interface I18nContextProps {
  language: Language;
  setLanguage: (lang: Language) => void;
  t: (key: keyof typeof translations.zh | string) => string;
}

const I18nContext = createContext<I18nContextProps | undefined>(undefined);

export function LanguageProvider({ children }: { children: React.ReactNode }) {
  const [language, setLanguageState] = useState<Language>(() => {
    return (localStorage.getItem("grabby_lang") as Language) || "zh";
  });

  const setLanguage = (lang: Language) => {
    setLanguageState(lang);
    localStorage.setItem("grabby_lang", lang);
  };

  const t = (key: string): string => {
    const dict = translations[language] || translations.zh;
    return (dict as any)[key] || key;
  };

  return (
    <I18nContext.Provider value={{ language, setLanguage, t }}>
      {children}
    </I18nContext.Provider>
  );
}

export function useTranslation() {
  const context = useContext(I18nContext);
  if (!context) {
    throw new Error("useTranslation must be used within a LanguageProvider");
  }
  return context;
}
