import { useState, useEffect, useMemo, useRef } from "react";
import {
  LayoutGrid,
  Inbox,
  Settings,
  FileText,
  ChevronLeft,
  ChevronRight,
  Search,
  PlusCircle,
  ExternalLink,
  Star,
  CheckCircle,
  Circle,
  RefreshCw,
  Trash2,
  Edit,
  Moon,
  Sun,
  Activity,
  Wifi,
  WifiOff,
  Check,
  Plus,
  Loader2,
  Calendar,
  Layers,
  Database,
  Sparkles,
  Rss
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

interface Source {
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

interface ScrapedItem {
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
  // Optional AI analysis fields
  ai_category?: string;
  ai_subcategory?: string;
  quality_score?: number;
  ai_summary?: string;
  ai_comment?: string;
  ai_tags?: string;
  ai_model_used?: string;
  ai_processed_at?: string;
}

interface FetchLog {
  id: number;
  source_id: string;
  started_at: string;
  finished_at?: string;
  status: string;
  items_found: number;
  items_added: number;
  error_message: string;
}

interface Stats {
  total_count: number;
  unread_count: number;
  starred_count: number;
  categories: Record<string, number>;
  source_categories?: string[];
  source_category_unread?: Record<string, number>;
}

interface AIProviderProfile {
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

interface JsonDailyReportViewProps {
  reportData: {
    title?: string;
    date?: string;
    editor?: string;
    sections?: Record<string, {
      title: string;
      items: any[];
    }>;
  };
}

function JsonDailyReportView({ reportData }: JsonDailyReportViewProps) {
  const sections = Object.entries(reportData.sections || {});

  const getSectionIcon = (key: string, title: string) => {
    const t = (title || "").toLowerCase();
    const k = (key || "").toLowerCase();
    if (k.includes("top") || t.includes("头条") || t.includes("要闻")) {
      return <Sparkles className="w-5 h-5 text-indigo-500 animate-pulse" />;
    }
    if (k.includes("tech") || k.includes("ai") || t.includes("科技") || t.includes("ai") || t.includes("探索") || t.includes("前沿")) {
      return <Activity className="w-5 h-5 text-cyan-500" />;
    }
    if (k.includes("finance") || k.includes("macro") || t.includes("财经") || t.includes("宏观") || t.includes("金")) {
      return <Database className="w-5 h-5 text-emerald-500" />;
    }
    if (k.includes("international") || k.includes("social") || t.includes("国际") || t.includes("社会") || t.includes("世界")) {
      return <Layers className="w-5 h-5 text-violet-500" />;
    }
    if (k.includes("dashboard") || t.includes("看板") || t.includes("数据") || t.includes("统计")) {
      return <LayoutGrid className="w-5 h-5 text-amber-500" />;
    }
    return <FileText className="w-5 h-5 text-zinc-500" />;
  };

  return (
    <div className="space-y-8">
      {/* Editor & Date bar */}
      {(reportData.editor || reportData.date) && (
        <div className="flex items-center justify-between text-xs text-zinc-500 border-b border-black/5 dark:border-white/5 pb-4">
          {reportData.editor && (
            <div className="flex items-center gap-1.5 bg-indigo-500/5 dark:bg-indigo-500/10 px-2.5 py-1 rounded-full text-indigo-700 dark:text-indigo-300 font-bold border border-indigo-500/10">
              <span className="w-1.5 h-1.5 rounded-full bg-indigo-500 animate-pulse"></span>
              <span>主编: {reportData.editor}</span>
            </div>
          )}
          {reportData.date && (
            <div className="font-mono bg-zinc-100 dark:bg-zinc-800 px-2.5 py-1 rounded-full text-zinc-600 dark:text-zinc-400">
              📅 {reportData.date}
            </div>
          )}
        </div>
      )}

      {sections.map(([key, section]) => {
        if (!section || !section.items || section.items.length === 0) return null;

        const sectionTitle = section.title || "";
        const isDashboard = key.includes("dashboard") || sectionTitle.includes("看板") || sectionTitle.includes("数据");
        const isTopStories = key.includes("top") || sectionTitle.includes("今日头条") || sectionTitle.includes("要闻");

        if (isDashboard) {
          return (
            <div key={key} className="bg-gradient-to-br from-indigo-50/40 via-purple-50/20 to-transparent dark:from-indigo-950/10 dark:via-zinc-900/10 dark:to-transparent border border-indigo-500/10 rounded-2xl p-6 space-y-4">
              <div className="flex items-center gap-2 font-bold text-indigo-950 dark:text-indigo-200">
                {getSectionIcon(key, sectionTitle)}
                <h4 className="text-sm font-black tracking-tight">{sectionTitle}</h4>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {section.items.map((item: any, idx: number) => {
                  const text = typeof item === "string" ? item : (item.title || "");
                  return (
                    <div key={idx} className="flex items-center gap-3 bg-white dark:bg-zinc-900/60 p-4 rounded-xl shadow-sm border border-black/5 dark:border-white/5">
                      <div className="p-2 bg-indigo-500/10 rounded-lg text-indigo-600 dark:text-indigo-400">
                        <CheckCircle className="w-4 h-4" />
                      </div>
                      <span className="text-sm font-semibold text-zinc-700 dark:text-zinc-300">{text}</span>
                    </div>
                  );
                })}
              </div>
            </div>
          );
        }

        return (
          <div key={key} className="space-y-4">
            <div className="flex items-center gap-2 border-b border-black/5 dark:border-white/5 pb-2">
              {getSectionIcon(key, sectionTitle)}
              <h4 className="text-base font-black text-indigo-950 dark:text-zinc-100">{sectionTitle}</h4>
            </div>

            <div className="space-y-4">
              {section.items.map((item: any, idx: number) => {
                if (typeof item === "string") {
                  return (
                    <div key={idx} className="p-3 bg-zinc-50 dark:bg-zinc-900/40 rounded-lg text-xs text-zinc-600 dark:text-zinc-400">
                      {item}
                    </div>
                  );
                }

                const commentaryText = item.commentary || item.comment || "";

                if (isTopStories) {
                  return (
                    <div
                      key={item.id || idx}
                      className="group relative border border-indigo-500/10 hover:border-indigo-500/20 bg-white dark:bg-[#1a1a1c] hover:shadow-lg transition-all duration-300 rounded-2xl overflow-hidden p-6 space-y-4"
                    >
                      {/* Left accent gradient line */}
                      <div className="absolute top-0 left-0 w-1 h-full bg-gradient-to-b from-indigo-500 via-purple-500 to-pink-500" />
                      
                      <div className="flex flex-wrap items-start justify-between gap-3 pl-2">
                        <h5 className="text-base font-black leading-snug text-zinc-900 dark:text-white group-hover:text-indigo-600 dark:group-hover:text-indigo-400 transition-colors flex-1 min-w-[280px]">
                          {item.title}
                        </h5>
                        <div className="flex items-center gap-2 shrink-0">
                          {item.source && (
                            <span className="text-[10px] font-bold px-2 py-0.5 rounded-full bg-zinc-100 dark:bg-zinc-800 text-zinc-600 dark:text-zinc-300">
                              {item.source}
                            </span>
                          )}
                          {item.score && (
                            <span className="text-[10px] font-extrabold px-2 py-0.5 rounded-full bg-amber-500/15 text-amber-600 dark:text-amber-400">
                              评分: {item.score}
                            </span>
                          )}
                        </div>
                      </div>

                      <p className="text-sm text-zinc-600 dark:text-zinc-300 pl-2 leading-relaxed">
                        {item.summary}
                      </p>

                      {commentaryText && (
                        <div className="pl-2">
                          <div className="border-l-4 border-indigo-500 bg-indigo-50/50 dark:bg-indigo-950/20 p-4 rounded-r-xl text-xs text-zinc-700 dark:text-zinc-300 space-y-1">
                            <div className="font-bold text-indigo-600 dark:text-indigo-400 flex items-center gap-1">
                              <Sparkles className="w-3.5 h-3.5" />
                              深度解析
                            </div>
                            <p className="italic leading-relaxed">
                              {commentaryText.replace(/^【深度解析】\s*/, "")}
                            </p>
                          </div>
                        </div>
                      )}

                      {item.link && (
                        <div className="flex justify-end pl-2">
                          <a
                            href={item.link}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="inline-flex items-center gap-1 text-xs text-indigo-600 dark:text-indigo-400 hover:text-indigo-800 dark:hover:text-indigo-300 font-bold transition-colors"
                          >
                            阅读原文
                            <ExternalLink className="w-3.5 h-3.5" />
                          </a>
                        </div>
                      )}
                    </div>
                  );
                }

                // Regular Section Stories (e.g. tech, finance, international)
                return (
                  <div
                    key={item.id || idx}
                    className="border border-black/5 dark:border-white/5 hover:border-indigo-500/15 bg-white dark:bg-[#1a1a1c] hover:shadow-md transition-all duration-300 rounded-xl p-5 space-y-3"
                  >
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <h5 className="text-sm font-bold text-zinc-900 dark:text-white leading-snug flex-1 min-w-[240px]">
                        {item.title}
                      </h5>
                      <div className="flex items-center gap-1.5 shrink-0">
                        {item.source && (
                          <span className="text-[10px] font-medium px-2 py-0.5 rounded bg-zinc-100 dark:bg-zinc-800 text-zinc-500 dark:text-zinc-400">
                            {item.source}
                          </span>
                        )}
                        {item.score && (
                          <span className="text-[10px] font-bold px-1.5 py-0.5 rounded bg-indigo-50 dark:bg-indigo-950/30 text-indigo-600 dark:text-indigo-400">
                            {item.score}
                          </span>
                        )}
                      </div>
                    </div>

                    <p className="text-xs text-zinc-600 dark:text-zinc-400 leading-relaxed">
                      {item.summary}
                    </p>

                    {commentaryText && (
                      <div className="border-l-2 border-indigo-500/40 bg-zinc-50/50 dark:bg-zinc-900/30 p-2.5 rounded-r text-[11px] text-zinc-500 dark:text-zinc-400 italic">
                        {commentaryText.replace(/^【深度解析】\s*/, "")}
                      </div>
                    )}

                    {item.link && (
                      <div className="flex justify-end">
                        <a
                          href={item.link}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex items-center gap-1 text-[11px] text-indigo-600 dark:text-indigo-400 hover:text-indigo-800 dark:hover:text-indigo-300 font-semibold transition-colors"
                        >
                          阅读原文
                          <ExternalLink className="w-3 h-3" />
                        </a>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          </div>
        );
      })}
    </div>
  );
}

export default function App() {
  const [currentView, setCurrentView] = useState<"grid" | "list" | "settings" | "logs" | "daily" | "ai-settings">("grid");
  const [items, setItems] = useState<ScrapedItem[]>([]);
  const [cursor, setCursor] = useState<string>("");
  const [hasMore, setHasMore] = useState<boolean>(true);
  const [isLoadingItems, setIsLoadingItems] = useState<boolean>(false);

  const [sources, setSources] = useState<Source[]>([]);
  const [logs, setLogs] = useState<FetchLog[]>([]);
  const [stats, setStats] = useState<Stats>({ 
    total_count: 0, 
    unread_count: 0, 
    starred_count: 0, 
    categories: {},
    source_categories: [],
    source_category_unread: {}
  });

  const [searchQuery, setSearchQuery] = useState<string>("");
  const [debouncedSearch, setDebouncedSearch] = useState<string>("");
  const [selectedCategory, setSelectedCategory] = useState<string>("all");
  const [selectedSourceCategory, setSelectedSourceCategory] = useState<string>("all");
  const [selectedReadStatus, setSelectedReadStatus] = useState<string>("all"); // "all", "unread", "starred"
  const [isSidebarCollapsed, setIsSidebarCollapsed] = useState<boolean>(false);
  const [darkMode, setDarkMode] = useState<boolean>(false);
  const [browserConnected, setBrowserConnected] = useState<boolean>(false);
  const [isScrapingAll, setIsScrapingAll] = useState<boolean>(false);
  const [isShowOnlyAIQuality, setIsShowOnlyAIQuality] = useState<boolean>(false);
  const [selectedAICategory, setSelectedAICategory] = useState<string>("all");
  const [aiCategories, setAiCategories] = useState<{name: string, count: number, avg_score: number}[]>([]);
  const [dailyReport, setDailyReport] = useState<any | null>(null);
  const [dailyReportHtml, setDailyReportHtml] = useState<string>("");
  const [selectedReportDate, setSelectedReportDate] = useState<string>(new Date().toISOString().split('T')[0]);
  const [isGeneratingReport, setIsGeneratingReport] = useState<boolean>(false);
  const [reportList, setReportList] = useState<any[]>([]);
  const [selectedReportType, setSelectedReportType] = useState<string>("daily");

  // Detail item reader state
  const [selectedItem, setSelectedItem] = useState<ScrapedItem | null>(null);
  const [itemDetailHtml, setItemDetailHtml] = useState<string>("");
  const [isLoadingDetail, setIsLoadingDetail] = useState<boolean>(false);

  // AI Settings states
  const [aiEnabled, setAiEnabled] = useState<boolean>(false);
  const [aiProfiles, setAiProfiles] = useState<AIProviderProfile[]>([]);
  const [activeProfileId, setActiveProfileId] = useState<string>("");
  const [aiProfileName, setAiProfileName] = useState<string>("默认服务商");
  const [aiProvider, setAiProvider] = useState<string>("gemini");
  const [aiApiKey, setAiApiKey] = useState<string>("");
  const [aiModel, setAiModel] = useState<string>("");
  const [aiBaseUrl, setAiBaseUrl] = useState<string>("");
  const [aiQualityThreshold, setAiQualityThreshold] = useState<number>(7);
  const [aiStrategy, setAiStrategy] = useState<string>("single");
  const [aiSystemPrompt, setAiSystemPrompt] = useState<string>("");
  const [aiDailyPrompt, setAiDailyPrompt] = useState<string>("");
  const [isSavingSettings, setIsSavingSettings] = useState<boolean>(false);
  const [isLoadingSettings, setIsLoadingSettings] = useState<boolean>(false);
  const [isTestingAI, setIsTestingAI] = useState<boolean>(false);
  const [aiTestResult, setAiTestResult] = useState<any | null>(null);
  const [aiRequestsPerMinute, setAiRequestsPerMinute] = useState<number>(10);
  const [morningReportEnabled, setMorningReportEnabled] = useState<boolean>(false);
  const [eveningReportEnabled, setEveningReportEnabled] = useState<boolean>(false);
  const [morningReportTime, setMorningReportTime] = useState<string>("08:30");
  const [eveningReportTime, setEveningReportTime] = useState<string>("22:30");

  // Source Add/Edit modal state
  const [isSourceDialogOpen, setIsSourceDialogOpen] = useState<boolean>(false);
  const [editingSource, setEditingSource] = useState<Source | null>(null);
  const [sourceForm, setSourceForm] = useState({
    id: "",
    name: "",
    type: "rss",
    url: "",
    schedule: "0 */2 * * *",
    default_category: "auto",
    config: "{}",
    category: "General"
  });
  const [formError, setFormError] = useState<string>("");

  // Debounce search query
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(searchQuery);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchQuery]);

  // Initial fetch operations
  useEffect(() => {
    fetchSources();
    fetchStats();
    checkHealth();
    fetchAICategories();
    
    // Check local storage for dark mode
    if (localStorage.getItem("theme") === "dark" || 
        (!localStorage.getItem("theme") && window.matchMedia("(prefers-color-scheme: dark)").matches)) {
      document.documentElement.classList.add("dark");
      setDarkMode(true);
    }

    // Health check polling
    const healthInterval = setInterval(checkHealth, 10000);
    return () => clearInterval(healthInterval);
  }, []);

  useEffect(() => {
    fetchItems(true);
    fetchAICategories();
  }, [debouncedSearch, selectedCategory, selectedSourceCategory, selectedReadStatus, isShowOnlyAIQuality, selectedAICategory]);

  const toggleDarkMode = () => {
    if (darkMode) {
      document.documentElement.classList.remove("dark");
      localStorage.setItem("theme", "light");
      setDarkMode(false);
    } else {
      document.documentElement.classList.add("dark");
      localStorage.setItem("theme", "dark");
      setDarkMode(true);
    }
  };

  const checkHealth = async () => {
    try {
      const res = await fetch("/api/health");
      const data = await res.json();
      setBrowserConnected(data.browser_connected);
    } catch {
      setBrowserConnected(false);
    }
  };

  const fetchStats = async () => {
    try {
      const res = await fetch("/api/stats");
      const data = await res.json();
      setStats(data);
    } catch (err) {
      console.error("Failed to fetch stats", err);
    }
  };

  const fetchSources = async () => {
    try {
      const res = await fetch("/api/sources");
      const data = await res.json();
      setSources(data);
    } catch (err) {
      console.error("Failed to fetch sources", err);
    }
  };

  const fetchLogs = async () => {
    try {
      const res = await fetch("/api/logs");
      const data = await res.json();
      setLogs(data);
    } catch (err) {
      console.error("Failed to fetch logs", err);
    }
  };

  const fetchAICategories = async () => {
    try {
      const resp = await fetch("/api/ai/categories");
      const data = await resp.json();
      if (data.success) {
        setAiCategories(data.categories || []);
      }
    } catch (err) {
      console.error("Failed to fetch AI categories", err);
    }
  };

  const fetchDailyReport = async (dateStr?: string, reportType?: string) => {
    const date = dateStr || selectedReportDate;
    const rType = reportType || selectedReportType || "daily";
    try {
      const resp = await fetch(`/api/ai/daily?date=${date}&type=${rType}`);
      const data = await resp.json();
      if (data.success && data.report) {
        setDailyReport(data.report);
        setDailyReportHtml(data.html_content || "");
      } else {
        setDailyReport(null);
        setDailyReportHtml("");
      }
    } catch (err) {
      console.error("Failed to fetch daily report", err);
    }
  };

  const fetchReportList = async () => {
    try {
      const resp = await fetch("/api/ai/daily/list?limit=30");
      const data = await resp.json();
      if (data.success) {
        setReportList(data.reports || []);
      }
    } catch (err) {
      console.error("Failed to fetch report list", err);
    }
  };

  const applyProfileToForm = (profile: AIProviderProfile) => {
    setActiveProfileId(profile.id);
    setAiProfileName(profile.name || "未命名服务商");
    setAiProvider(profile.provider || "gemini");
    setAiApiKey(profile.api_key || "");
    setAiModel(profile.model || "");
    setAiBaseUrl(profile.base_url || "");
    setAiRequestsPerMinute(profile.requests_per_minute || 10);
  };

  const buildProfileFromSettings = (settings: any): AIProviderProfile => ({
    id: settings.active_profile_id || "default",
    name: "默认服务商",
    provider: settings.provider || "gemini",
    api_key: settings.api_key || "",
    model: settings.model || "",
    base_url: settings.base_url || "",
    requests_per_minute: 10,
  });

  const upsertCurrentProfile = (profiles: AIProviderProfile[] = aiProfiles) => {
    const currentID = activeProfileId || "default";
    const currentProfile: AIProviderProfile = {
      id: currentID,
      name: aiProfileName.trim() || "未命名服务商",
      provider: aiProvider,
      api_key: aiApiKey,
      model: aiModel,
      base_url: aiBaseUrl,
      requests_per_minute: aiRequestsPerMinute,
    };

    if (profiles.some((profile) => profile.id === currentID)) {
      return profiles.map((profile) => profile.id === currentID ? currentProfile : profile);
    }
    return [...profiles, currentProfile];
  };

  const handleProfileNameChange = (name: string) => {
    setAiProfileName(name);
    if (!activeProfileId) {
      return;
    }
    setAiProfiles((profiles) => profiles.map((profile) => (
      profile.id === activeProfileId ? { ...profile, name } : profile
    )));
  };

  const handleSelectAIProfile = (profileID: string) => {
    const nextProfiles = upsertCurrentProfile();
    const nextProfile = nextProfiles.find((profile) => profile.id === profileID);
    setAiProfiles(nextProfiles);
    if (nextProfile) {
      applyProfileToForm(nextProfile);
    }
  };

  const handleAddAIProfile = () => {
    const nextProfiles = upsertCurrentProfile();
    const maxPriority = nextProfiles.reduce((max, p) => Math.max(max, p.priority || 0), 0);
    const newProfile: AIProviderProfile = {
      id: `profile-${Date.now()}`,
      name: `服务商 ${nextProfiles.length + 1}`,
      provider: "custom",
      api_key: "",
      model: "",
      base_url: "http://localhost:1234/v1",
      disabled: false,
      priority: maxPriority + 1,
      requests_per_minute: 10,
    };
    setAiProfiles([...nextProfiles, newProfile]);
    applyProfileToForm(newProfile);
    setAiTestResult(null);
  };

  const handleDeleteAIProfile = () => {
    const nextProfiles = upsertCurrentProfile();
    if (nextProfiles.length <= 1) {
      alert("至少需要保留一个服务商档案。");
      return;
    }

    const remainingProfiles = nextProfiles.filter((profile) => profile.id !== activeProfileId);
    const nextProfile = remainingProfiles[0];
    setAiProfiles(remainingProfiles);
    applyProfileToForm(nextProfile);
    setAiTestResult(null);
  };

  const fetchAISettings = async () => {
    setIsLoadingSettings(true);
    try {
      const resp = await fetch("/api/ai/settings");
      const data = await resp.json();
      if (data.success && data.settings) {
        const profiles = Array.isArray(data.settings.profiles) && data.settings.profiles.length > 0
          ? data.settings.profiles
          : [buildProfileFromSettings(data.settings)];
        const activeID = data.settings.active_profile_id || profiles[0].id;
        const activeProfile = profiles.find((profile: AIProviderProfile) => profile.id === activeID) || profiles[0];

        setAiEnabled(data.settings.enabled);
        setAiProfiles(profiles);
        applyProfileToForm(activeProfile);
        setAiQualityThreshold(data.settings.quality_threshold || 7);
        setAiStrategy(data.settings.strategy || "single");
        setAiSystemPrompt(data.settings.system_prompt || "");
        setAiDailyPrompt(data.settings.daily_prompt || "");
        setMorningReportEnabled(data.settings.morning_report_enabled || false);
        setEveningReportEnabled(data.settings.evening_report_enabled || false);
        setMorningReportTime(data.settings.morning_report_time || "07:30");
        setEveningReportTime(data.settings.evening_report_time || "20:00");
      }
    } catch (err) {
      console.error("Failed to fetch AI settings", err);
    } finally {
      setIsLoadingSettings(false);
    }
  };

  const saveAISettings = async () => {
    setIsSavingSettings(true);
    try {
      const nextProfiles = upsertCurrentProfile();
      setAiProfiles(nextProfiles);

      const resp = await fetch("/api/ai/settings", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          enabled: aiEnabled,
          provider: aiProvider,
          api_key: aiApiKey,
          model: aiModel,
          base_url: aiBaseUrl,
          quality_threshold: Number(aiQualityThreshold),
          strategy: aiStrategy,
          system_prompt: aiSystemPrompt,
          daily_prompt: aiDailyPrompt,
          active_profile_id: activeProfileId || nextProfiles[0]?.id || "default",
          profiles: nextProfiles,
          morning_report_enabled: morningReportEnabled,
          evening_report_enabled: eveningReportEnabled,
          morning_report_time: morningReportTime,
          evening_report_time: eveningReportTime,
        }),
      });
      const data = await resp.json();
      if (data.success) {
        alert("AI配置保存成功！");
      } else {
        alert("AI配置保存失败：" + (data.error || "未知错误"));
      }
    } catch (err) {
      console.error("Failed to save AI settings", err);
      alert("保存失败，请检查网络连接");
    } finally {
      setIsSavingSettings(false);
    }
  };

  const handleTestAI = async () => {
    setIsTestingAI(true);
    setAiTestResult(null);
    try {
      const resp = await fetch("/api/ai/test", { method: "POST" });
      const data = await resp.json();
      if (data.success) {
        setAiTestResult({
          success: true,
          title: data.item_title,
          analysis: data.analysis,
        });
      } else {
        setAiTestResult({
          success: false,
          error: data.error || "未知接口错误",
        });
      }
    } catch (err) {
      console.error("Failed to test AI connection", err);
      setAiTestResult({
        success: false,
        error: "网络连接失败，请检查后端服务是否正常启动。",
      });
    } finally {
      setIsTestingAI(false);
    }
  };

  const handleStartEvaluation = async () => {
    try {
      const resp = await fetch("/api/ai/start_eval", { method: "POST" });
      const data = await resp.json();
      if (data.success) {
        alert(data.message || "增量评估已启动！");
        fetchStats();
      } else {
        alert("启动失败：" + (data.error || "未知原因"));
      }
    } catch (err) {
      console.error("Failed to start evaluation", err);
      alert("网络错误，启动增量评测失败");
    }
  };

  useEffect(() => {
    if (currentView === "settings" || currentView === "ai-settings") {
      fetchAISettings();
    }
  }, [currentView]);

  const handleGenerateDailyReport = async (reportType?: string) => {
    setIsGeneratingReport(true);
    const rType = reportType || selectedReportType || "daily";
    try {
      const resp = await fetch("/api/ai/daily/generate", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ date: selectedReportDate, report_type: rType })
      });
      const data = await resp.json();
      if (data.success && data.report) {
        setDailyReport(data.report);
        setDailyReportHtml(data.html_content || "");
        fetchStats();
        fetchReportList();
      } else {
        alert("生成日报失败: " + (data.error || "未知原因"));
      }
    } catch (err) {
      console.error("Failed to generate daily report", err);
      alert("生成日报出现网络错误");
    } finally {
      setIsGeneratingReport(false);
    }
  };

  const fetchItems = async (reset = false) => {
    if (isLoadingItems) return;
    setIsLoadingItems(true);

    try {
      let url = `/api/items?limit=20`;
      if (isShowOnlyAIQuality || (selectedAICategory && selectedAICategory !== "all")) {
        url = `/api/ai/items?limit=20`;
        if (isShowOnlyAIQuality) {
          url += `&score_min=7`;
        }
        if (selectedAICategory && selectedAICategory !== "all") {
          url += `&category=${encodeURIComponent(selectedAICategory)}`;
        }
      } else {
        if (selectedCategory && selectedCategory !== "all") {
          url += `&category=${selectedCategory}`;
        }
      }
      if (selectedSourceCategory && selectedSourceCategory !== "all") {
        url += `&source_category=${encodeURIComponent(selectedSourceCategory)}`;
      }
      if (debouncedSearch) {
        url += `&q=${encodeURIComponent(debouncedSearch)}`;
      }
      if (selectedReadStatus === "unread") {
        url += `&read_status=0`;
      } else if (selectedReadStatus === "starred") {
        url += `&starred=1`;
      }

      const currentCursor = reset ? "" : cursor;
      if (currentCursor) {
        url += `&cursor=${currentCursor}`;
      }

      const res = await fetch(url);
      const data = await res.json();

      if (reset) {
        setItems(data.items || []);
        // Select first item by default if entering list view
        if (data.items && data.items.length > 0) {
          handleSelectItem(data.items[0]);
        } else {
          setSelectedItem(null);
          setItemDetailHtml("");
        }
      } else {
        setItems(prev => [...prev, ...(data.items || [])]);
      }

      setCursor(data.cursor || "");
      setHasMore(!!data.cursor);
    } catch (err) {
      console.error("Failed to fetch items", err);
    } finally {
      setIsLoadingItems(false);
    }
  };

  const handleSelectItem = async (item: ScrapedItem) => {
    setSelectedItem(item);
    setIsLoadingDetail(true);
    try {
      const res = await fetch(`/api/items/${item.id}`);
      const data = await res.json();
      setItemDetailHtml(data.html_content);
      
      // Update item read status locally
      setItems(prev =>
        prev.map(i => (i.id === item.id ? { ...i, read_status: 1 } : i))
      );
      
      // Trigger update stats
      fetchStats();
    } catch (err) {
      console.error("Failed to fetch item detail", err);
    } finally {
      setIsLoadingDetail(false);
    }
  };

  const toggleStar = async (item: ScrapedItem, e?: React.MouseEvent) => {
    if (e) e.stopPropagation();
    const newStarred = item.starred === 1 ? 0 : 1;
    try {
      await fetch(`/api/items/${item.id}/star`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ starred: newStarred })
      });

      // Update locally
      setItems(prev =>
        prev.map(i => (i.id === item.id ? { ...i, starred: newStarred } : i))
      );
      if (selectedItem && selectedItem.id === item.id) {
        setSelectedItem(prev => prev ? { ...prev, starred: newStarred } : null);
      }
      fetchStats();
    } catch (err) {
      console.error("Failed to toggle star", err);
    }
  };

  const toggleReadStatus = async (item: ScrapedItem, e?: React.MouseEvent) => {
    if (e) e.stopPropagation();
    const newRead = item.read_status === 1 ? 0 : 1;
    try {
      await fetch(`/api/items/${item.id}/read`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ read_status: newRead })
      });

      // Update locally
      setItems(prev =>
        prev.map(i => (i.id === item.id ? { ...i, read_status: newRead } : i))
      );
      if (selectedItem && selectedItem.id === item.id) {
        setSelectedItem(prev => prev ? { ...prev, read_status: newRead } : null);
      }
      fetchStats();
    } catch (err) {
      console.error("Failed to toggle read status", err);
    }
  };

  const handleToggleSourceEnabled = async (source: Source) => {
    const newEnabled = source.enabled === 1 ? 0 : 1;
    try {
      const res = await fetch(`/api/sources/${source.id}/toggle`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled: newEnabled })
      });
      if (!res.ok) throw new Error(`Toggle failed: ${res.status}`);
      setSources(prev =>
        prev.map(s => (s.id === source.id ? { ...s, enabled: newEnabled } : s))
      );
    } catch (err) {
      console.error("Failed to toggle source", err);
    }
  };

  const handleRunSource = async (source: Source) => {
    try {
      // Temporary loading indicator
      setSources(prev =>
        prev.map(s => (s.id === source.id ? { ...s, last_fetch_status: "running" } : s))
      );
      const res = await fetch(`/api/sources/${source.id}/run`, { method: "POST" });
      const data = await res.json();
      if (data.success) {
        // Refresh sources & items
        fetchSources();
        fetchItems(true);
        fetchStats();
      } else {
        alert("Scrape failed: " + data.error);
      }
    } catch (err) {
      console.error(err);
      fetchSources();
    }
  };

  const handleScrapeAllEnabled = async () => {
    if (isScrapingAll) return;
    setIsScrapingAll(true);
    const enabledSources = sources.filter(s => s.enabled === 1);
    try {
      await Promise.all(
        enabledSources.map(s =>
          fetch(`/api/sources/${s.id}/run`, { method: "POST" }).catch(e => console.error(e))
        )
      );
      fetchSources();
      fetchItems(true);
      fetchStats();
    } catch (err) {
      console.error(err);
    } finally {
      setIsScrapingAll(false);
    }
  };

  const handleDeleteSource = async (id: string) => {
    if (!confirm("Are you sure you want to delete this subscription source?")) return;
    try {
      await fetch(`/api/sources/${id}`, { method: "DELETE" });
      setSources(prev => prev.filter(s => s.id !== id));
      fetchStats();
    } catch (err) {
      console.error("Failed to delete source", err);
    }
  };

  const openAddSourceDialog = () => {
    setEditingSource(null);
    setSourceForm({
      id: "",
      name: "",
      type: "rss",
      url: "",
      schedule: "0 */2 * * *",
      default_category: "auto",
      config: "{}",
      category: "General"
    });
    setFormError("");
    setIsSourceDialogOpen(true);
  };

  const openEditSourceDialog = (source: Source) => {
    setEditingSource(source);
    setSourceForm({
      id: source.id,
      name: source.name,
      type: source.type,
      url: source.url,
      schedule: source.schedule,
      default_category: source.default_category,
      config: source.config,
      category: source.category || "General"
    });
    setFormError("");
    setIsSourceDialogOpen(true);
  };

  const handleSaveSource = async (e: React.FormEvent) => {
    e.preventDefault();
    setFormError("");

    // Simple validation
    if (!sourceForm.id || !sourceForm.name || !sourceForm.url || !sourceForm.schedule) {
      setFormError("Please fill out all required fields.");
      return;
    }

    try {
      JSON.parse(sourceForm.config);
    } catch {
      setFormError("Advanced configuration must be valid JSON.");
      return;
    }

    try {
      const method = editingSource ? "PUT" : "POST";
      const url = editingSource ? `/api/sources/${editingSource.id}` : "/api/sources";

      const res = await fetch(url, {
        method,
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(sourceForm)
      });

      if (!res.ok) {
        const errText = await res.text();
        throw new Error(errText || "Request failed");
      }

      setIsSourceDialogOpen(false);
      fetchSources();
    } catch (err: any) {
      setFormError(err.message || "Failed to save data source.");
    }
  };

  const getCategoryColor = (cat: string) => {
    switch (cat) {
      case "tweet": return "bg-sky-500/10 text-sky-600 dark:text-sky-400";
      case "project": return "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400";
      case "paper": return "bg-purple-500/10 text-purple-600 dark:text-purple-400";
      case "article": return "bg-amber-500/10 text-amber-600 dark:text-amber-400";
      default: return "bg-zinc-500/10 text-zinc-600 dark:text-zinc-400";
    }
  };

  const getCategoryLabel = (cat: string) => {
    switch (cat) {
      case "tweet": return "推特 Tweet";
      case "project": return "项目 Project";
      case "paper": return "论文 Paper";
      case "article": return "文章 Article";
      default: return cat;
    }
  };

  const formatTimeAgo = (timeStr?: string) => {
    if (!timeStr) return "Unknown";
    const date = new Date(timeStr);
    const seconds = Math.floor((new Date().getTime() - date.getTime()) / 1000);
    
    let interval = seconds / 31536000;
    if (interval > 1) return Math.floor(interval) + " years ago";
    interval = seconds / 2592000;
    if (interval > 1) return Math.floor(interval) + " months ago";
    interval = seconds / 86400;
    if (interval > 1) return Math.floor(interval) + " days ago";
    interval = seconds / 3600;
    if (interval > 1) return Math.floor(interval) + " hours ago";
    interval = seconds / 60;
    if (interval > 1) return Math.floor(interval) + " minutes ago";
    return "just now";
  };

  return (
    <div className="flex w-screen h-screen bg-zinc-100 dark:bg-[#0f0f0f] text-zinc-900 dark:text-zinc-100 font-sans">
      <div className="flex w-full h-full overflow-hidden bg-white dark:bg-[#1c1c1e] animate-fade-in">
        
        {/* --- Sidebar Component --- */}
        <aside className={`${isSidebarCollapsed ? "w-16" : "w-64"} sidebar-vibrancy flex flex-col shrink-0 transition-all duration-300`}>
          {/* Top Logo/Title Area */}
          <div className="h-14 flex items-center px-6 shrink-0 border-b border-black/5 dark:border-white/5">
            <h1 className="font-extrabold text-sm tracking-wider bg-gradient-to-r from-blue-600 to-indigo-500 bg-clip-text text-transparent">
              {!isSidebarCollapsed ? "GRABBY PANELS" : "G"}
            </h1>
          </div>

          {/* Navigation Items */}
          <div className="flex-1 px-3 py-4 space-y-6 overflow-y-auto">
            <nav className="space-y-1">
              <button
                onClick={() => setCurrentView("grid")}
                className={`w-full flex items-center gap-3 px-3 py-2 rounded-xl text-sm transition-all font-medium ${
                  currentView === "grid"
                    ? "bg-blue-600 text-white shadow-md font-semibold"
                    : "text-zinc-600 dark:text-zinc-400 hover:bg-black/5 dark:hover:bg-white/5"
                }`}
              >
                <LayoutGrid className="w-4 h-4" />
                {!isSidebarCollapsed && <span>聚合发现 Grid</span>}
              </button>
              
              <button
                onClick={() => setCurrentView("list")}
                className={`w-full flex items-center justify-between px-3 py-2 rounded-xl text-sm transition-all font-medium ${
                  currentView === "list"
                    ? "bg-blue-600 text-white shadow-md font-semibold"
                    : "text-zinc-600 dark:text-zinc-400 hover:bg-black/5 dark:hover:bg-white/5"
                }`}
              >
                <div className="flex items-center gap-3">
                  <Inbox className="w-4 h-4" />
                  {!isSidebarCollapsed && <span>阅读列表 Inbox</span>}
                </div>
                {!isSidebarCollapsed && stats.unread_count > 0 && (
                  <span className={`text-[10px] px-1.5 py-0.5 rounded-md font-bold ${
                    currentView === "list" ? "bg-white text-blue-600" : "bg-blue-100 text-blue-600 dark:bg-blue-900/30 dark:text-blue-400"
                  }`}>
                    {stats.unread_count}
                  </span>
                )}
              </button>

              <button
                onClick={() => {
                  setCurrentView("daily");
                  fetchDailyReport();
                  fetchReportList();
                }}
                className={`w-full flex items-center gap-3 px-3 py-2 rounded-xl text-sm transition-all font-medium ${
                  currentView === "daily"
                    ? "bg-indigo-600 text-white shadow-md font-semibold"
                    : "text-zinc-600 dark:text-zinc-400 hover:bg-black/5 dark:hover:bg-white/5"
                }`}
              >
                <Sparkles className="w-4 h-4" />
                {!isSidebarCollapsed && <span>AI 智能日报 Daily</span>}
              </button>

              <button
                onClick={() => {
                  setCurrentView("logs");
                  fetchLogs();
                }}
                className={`w-full flex items-center gap-3 px-3 py-2 rounded-xl text-sm transition-all font-medium ${
                  currentView === "logs"
                    ? "bg-blue-600 text-white shadow-md font-semibold"
                    : "text-zinc-600 dark:text-zinc-400 hover:bg-black/5 dark:hover:bg-white/5"
                }`}
              >
                <FileText className="w-4 h-4" />
                {!isSidebarCollapsed && <span>抓取日志 Logs</span>}
              </button>
            </nav>

            {/* AI Smart Filter Section */}
            {!isSidebarCollapsed && (
              <div className="pt-4 border-t border-black/5 dark:border-white/5">
                <h3 className="px-3 py-2 text-[10px] font-extrabold text-indigo-500 dark:text-indigo-400 uppercase tracking-wider flex items-center gap-1.5">
                  <Sparkles className="w-3.5 h-3.5" />
                  AI 智能筛选 AI Filter
                </h3>
                <nav className="space-y-1">
                  <button
                    onClick={() => {
                      setIsShowOnlyAIQuality(prev => !prev);
                      setSelectedAICategory("all");
                      if (currentView !== "grid" && currentView !== "list") setCurrentView("grid");
                    }}
                    className={`w-full flex items-center justify-between px-3 py-1.5 rounded-lg text-xs hover:bg-black/5 dark:hover:bg-white/5 transition-all ${isShowOnlyAIQuality ? "text-indigo-500 font-semibold" : "text-zinc-600 dark:text-zinc-400"}`}
                  >
                    <span>只看 AI 优质内容 (≥7分)</span>
                    <span className="text-[9px] px-1 bg-indigo-100 dark:bg-indigo-900/30 text-indigo-700 dark:text-indigo-400 rounded font-bold">AI</span>
                  </button>
                  
                  {aiCategories.map((cat) => (
                    <button
                      key={cat.name}
                      onClick={() => {
                        setSelectedAICategory(cat.name === selectedAICategory ? "all" : cat.name);
                        setIsShowOnlyAIQuality(false);
                        if (currentView !== "grid" && currentView !== "list") setCurrentView("grid");
                      }}
                      className={`w-full flex items-center justify-between px-3 py-1.5 rounded-lg text-xs hover:bg-black/5 dark:hover:bg-white/5 transition-all ${selectedAICategory === cat.name ? "text-indigo-500 font-semibold" : "text-zinc-600 dark:text-zinc-400"}`}
                    >
                      <span>{cat.name} ({cat.avg_score.toFixed(1)}分)</span>
                      <span>{cat.count}</span>
                    </button>
                  ))}
                </nav>
              </div>
            )}

            {/* Quick Stats list (Categories) */}
            {!isSidebarCollapsed && (
              <div className="pt-4 border-t border-black/5 dark:border-white/5">
                <h3 className="px-3 py-2 text-[10px] font-bold text-zinc-400 dark:text-zinc-500 uppercase tracking-wider">
                  分类筛选 Topic Category
                </h3>
                <nav className="space-y-1">
                  <button
                    onClick={() => { setSelectedSourceCategory("all"); if (currentView !== "grid" && currentView !== "list") setCurrentView("grid"); }}
                    className={`w-full flex items-center justify-between px-3 py-1.5 rounded-lg text-xs hover:bg-black/5 dark:hover:bg-white/5 transition-all ${selectedSourceCategory === "all" ? "text-blue-500 font-semibold" : "text-zinc-600 dark:text-zinc-400"}`}
                  >
                    <span>全部 All</span>
                    <span>{stats.unread_count}</span>
                  </button>
                  {(stats.source_categories || []).map((cat) => (
                    <button
                      key={cat}
                      onClick={() => { setSelectedSourceCategory(cat); if (currentView !== "grid" && currentView !== "list") setCurrentView("grid"); }}
                      className={`w-full flex items-center justify-between px-3 py-1.5 rounded-lg text-xs hover:bg-black/5 dark:hover:bg-white/5 transition-all ${selectedSourceCategory === cat ? "text-blue-500 font-semibold" : "text-zinc-600 dark:text-zinc-400"}`}
                    >
                      <span>{cat}</span>
                      <span>{stats.source_category_unread?.[cat] || 0}</span>
                    </button>
                  ))}
                </nav>
              </div>
            )}
          </div>

          {/* Bottom Area: Settings, AI Settings, Collapse & Dark Mode */}
          <div className="p-3 border-t border-black/5 dark:border-white/5 space-y-1.5 shrink-0">
            <button
              onClick={() => setCurrentView("settings")}
              title="订阅源配置"
              className={`w-full flex items-center gap-3 px-3 py-2 rounded-xl text-xs font-semibold transition-all ${
                currentView === "settings"
                  ? "bg-blue-600 text-white shadow-sm font-semibold"
                  : "text-zinc-500 dark:text-zinc-400 hover:bg-black/5 dark:hover:bg-white/5"
              }`}
            >
              <Settings className="w-4 h-4" />
              {!isSidebarCollapsed && <span>订阅数据源 Sources</span>}
            </button>

            <button
              onClick={() => setCurrentView("ai-settings")}
              title="AI 智能配置"
              className={`w-full flex items-center gap-3 px-3 py-2 rounded-xl text-xs font-semibold transition-all ${
                currentView === "ai-settings"
                  ? "bg-indigo-600 text-white shadow-sm font-semibold"
                  : "text-zinc-500 dark:text-zinc-400 hover:bg-black/5 dark:hover:bg-white/5"
              }`}
            >
              <Sparkles className="w-4 h-4" />
              {!isSidebarCollapsed && <span>AI 模型配置 Settings</span>}
            </button>

            <div className="flex gap-2 pt-1.5">
              <button
                onClick={toggleDarkMode}
                title={darkMode ? "切换亮色模式" : "切换暗色模式"}
                className="flex-1 flex items-center justify-center p-2 text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-300 transition-colors bg-black/5 dark:bg-white/5 rounded-xl"
              >
                {darkMode ? <Sun className="w-4 h-4" /> : <Moon className="w-4 h-4" />}
                {!isSidebarCollapsed && <span className="text-[10px] ml-1.5 font-medium">{darkMode ? "亮色" : "暗色"}</span>}
              </button>
              <button
                onClick={() => setIsSidebarCollapsed(!isSidebarCollapsed)}
                title={isSidebarCollapsed ? "展开侧边栏" : "收起侧边栏"}
                className="flex-1 flex items-center justify-center p-2 text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-300 transition-colors bg-black/5 dark:bg-white/5 rounded-xl"
              >
                {isSidebarCollapsed ? <ChevronRight className="w-4 h-4" /> : <ChevronLeft className="w-4 h-4" />}
                {!isSidebarCollapsed && <span className="text-[10px] ml-1.5 font-medium">折叠</span>}
              </button>
            </div>
          </div>
        </aside>

        {/* --- Main Dashboard Container --- */}
        <main className="flex-1 flex flex-col bg-zinc-50 dark:bg-[#121212] overflow-hidden relative">
          
          {/* Header Bar */}
          <header className="h-14 flex items-center justify-between px-6 border-b border-black/5 dark:border-white/5 bg-white/80 dark:bg-[#1c1c1e]/80 backdrop-blur-md sticky top-0 z-10 shrink-0">
            <div className="flex items-center gap-3">
              <h2 className="text-lg font-bold tracking-tight">
                {currentView === "grid" && "聚合发现 Discovery"}
                {currentView === "list" && "阅读列表 Inbox"}
                {currentView === "settings" && "订阅数据源 Settings"}
                {currentView === "ai-settings" && "AI 模型配置 Settings"}
                {currentView === "logs" && "抓取执行日志 Logs"}
                {currentView === "daily" && "AI 智能日报 Daily"}
              </h2>
              {/* Browser Connection Status Indicator */}
              <div className={`flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-medium transition-all ${
                browserConnected 
                  ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400" 
                  : "bg-amber-500/10 text-amber-600 dark:text-amber-400"
              }`}>
                {browserConnected ? <Wifi className="w-3 h-3" /> : <WifiOff className="w-3 h-3" />}
                {browserConnected ? "插件已连接" : "插件未连接"}
              </div>
            </div>

            <div className="flex items-center gap-3">
              {/* Filter selections inside header */}
              {(currentView === "grid" || currentView === "list") && (
                <div className="flex items-center gap-2">
                  <Select value={selectedReadStatus} onValueChange={setSelectedReadStatus}>
                    <SelectTrigger className="w-[100px] h-8 text-xs">
                      <SelectValue placeholder="筛选阅读" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">全部内容</SelectItem>
                      <SelectItem value="unread">未读消息</SelectItem>
                      <SelectItem value="starred">星标收藏</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              )}

              {/* Action Buttons */}
              {currentView === "settings" && (
                <Button onClick={openAddSourceDialog} size="sm" className="bg-blue-600 hover:bg-blue-700 text-white font-medium gap-1.5 h-8 text-xs">
                  <Plus className="w-3.5 h-3.5" /> 添加数据源
                </Button>
              )}

              {(currentView === "grid" || currentView === "list") && (
                <Button 
                  onClick={handleScrapeAllEnabled} 
                  disabled={isScrapingAll}
                  size="sm" 
                  className="bg-blue-600 hover:bg-blue-700 text-white font-medium gap-1.5 h-8 text-xs"
                >
                  {isScrapingAll ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <RefreshCw className="w-3.5 h-3.5" />}
                  立即抓取数据
                </Button>
              )}
            </div>
          </header>

          {/* --- Grid View --- */}
          {currentView === "grid" && (
            <div className="flex-1 flex flex-col overflow-hidden">
              {/* Filter & Search Bar */}
              <div className="p-4 bg-white dark:bg-[#18181b] border-b border-black/5 dark:border-white/5 flex flex-wrap gap-4 items-center justify-between">
                <div className="flex items-center gap-2 overflow-x-auto py-1">
                  <Button
                    variant={selectedAICategory === "all" && !isShowOnlyAIQuality ? "default" : "outline"}
                    onClick={() => { setSelectedAICategory("all"); setIsShowOnlyAIQuality(false); }}
                    className="h-7 text-xs rounded-full px-4"
                  >
                    全部
                  </Button>
                  {aiCategories.map((cat) => (
                    <Button
                      key={cat.name}
                      variant={selectedAICategory === cat.name ? "default" : "outline"}
                      onClick={() => { setSelectedAICategory(cat.name); setIsShowOnlyAIQuality(false); }}
                      className="h-7 text-xs rounded-full px-4 gap-1"
                    >
                      {cat.name}
                      <span className="text-[10px] opacity-60">{cat.count}</span>
                    </Button>
                  ))}
                </div>

                <div className="relative w-full max-w-xs">
                  <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-zinc-400" />
                  <Input
                    placeholder="在标题和摘要中搜索..."
                    value={searchQuery}
                    onChange={e => setSearchQuery(e.target.value)}
                    className="pl-9 h-9 text-xs"
                  />
                </div>
              </div>

              {/* Items Card List Grid */}
              <div className="flex-1 overflow-y-auto bg-zinc-50/50 dark:bg-[#121212] p-6">
                {items.length === 0 ? (
                  <div className="flex flex-col items-center justify-center h-[50vh] text-zinc-400">
                    <Inbox className="w-12 h-12 mb-2 stroke-1" />
                    <p className="text-sm">暂无内容，请检查订阅源或点击右上角抓取数据</p>
                  </div>
                ) : (
                  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 max-w-6xl mx-auto">
                    {items.map(item => (
                      <Card 
                        key={item.id} 
                        onClick={() => {
                          setCurrentView("list");
                          handleSelectItem(item);
                        }}
                        className="group relative flex flex-col bg-white dark:bg-[#1c1c1e] border border-black/5 dark:border-white/5 news-card-hover cursor-pointer overflow-hidden shadow-sm h-full"
                      >
                        <CardHeader className="p-4 pb-2">
                          <div className="flex justify-between items-start gap-2">
                            <div className="flex flex-wrap gap-1.5 items-center">
                              <span className={`px-2 py-0.5 rounded-full text-[9px] font-bold tracking-tight ${getCategoryColor(item.category)}`}>
                                {getCategoryLabel(item.category)}
                              </span>
                              {item.source_category && (
                                <span className="px-2 py-0.5 rounded-full text-[9px] font-bold tracking-tight bg-zinc-100 text-zinc-600 dark:bg-zinc-800/60 dark:text-zinc-400">
                                  {item.source_category}
                                </span>
                              )}
                              {item.quality_score !== undefined && item.quality_score > 0 && (
                                <span className={`px-2 py-0.5 rounded-full text-[9px] font-extrabold tracking-tight ${
                                  item.quality_score >= 8 
                                    ? "bg-indigo-600 text-white dark:bg-indigo-700" 
                                    : "bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400"
                                }`}>
                                  AI {item.quality_score}分
                                </span>
                              )}
                            </div>
                            <div className="flex items-center gap-1.5">
                              <span className="text-[10px] text-zinc-400 font-medium">{formatTimeAgo(item.published_at)}</span>
                              <Button
                                size="icon"
                                variant="ghost"
                                className="w-6 h-6 hover:bg-black/5 dark:hover:bg-white/5"
                                onClick={(e) => toggleStar(item, e)}
                              >
                                <Star className={`w-3.5 h-3.5 ${item.starred === 1 ? "fill-amber-400 text-amber-400" : "text-zinc-400"}`} />
                              </Button>
                            </div>
                          </div>
                          <CardTitle className="text-sm font-bold leading-snug group-hover:text-blue-500 transition-colors line-clamp-2 mt-2">
                            {item.title}
                          </CardTitle>
                        </CardHeader>
                        <CardContent className="p-4 pt-1 flex-1">
                          {item.ai_summary ? (
                            <div className="text-xs bg-indigo-50/30 dark:bg-indigo-950/5 border border-indigo-500/5 rounded-lg p-2.5 space-y-1">
                              <p className="text-[10px] font-bold text-indigo-600 dark:text-indigo-400 uppercase tracking-wider flex items-center gap-1">
                                <Sparkles className="w-3 h-3" />
                                AI 摘要 {item.ai_category ? `· ${item.ai_category}` : ""}
                              </p>
                              <p className="text-zinc-600 dark:text-zinc-400 leading-relaxed line-clamp-3">
                                {item.ai_summary}
                              </p>
                            </div>
                          ) : (
                            <p className="text-xs text-zinc-500 line-clamp-3 leading-relaxed">
                              {item.summary || "未抓取到描述摘要，点击以进入详情阅读全文。"}
                            </p>
                          )}
                        </CardContent>
                        <CardFooter className="p-4 pt-0 border-t border-black/5 dark:border-white/5 flex items-center justify-between text-[10px]">
                          <div className="flex items-center gap-1.5 text-zinc-400">
                            <div className="w-4 h-4 rounded-full bg-blue-100 dark:bg-blue-900/30 flex items-center justify-center text-[8px] font-bold text-blue-600 dark:text-blue-400">
                              {item.origin_source ? item.origin_source[0] : "?"}
                            </div>
                            <span className="font-semibold uppercase tracking-tight line-clamp-1 max-w-[120px]">{item.origin_source || "未知出处"}</span>
                          </div>
                          
                          {/* Unread marker dot */}
                          {item.read_status === 0 && (
                            <div className="flex items-center gap-1 text-blue-600 font-medium">
                              <span className="w-1.5 h-1.5 rounded-full bg-blue-500" />
                              <span>未读</span>
                            </div>
                          )}
                        </CardFooter>
                      </Card>
                    ))}
                  </div>
                )}
                
                {/* Load More Button */}
                {hasMore && items.length > 0 && (
                  <div className="flex justify-center mt-8 pb-12">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => fetchItems()}
                      disabled={isLoadingItems}
                      className="px-6 h-8 text-xs font-semibold"
                    >
                      {isLoadingItems ? <Loader2 className="w-3.5 h-3.5 animate-spin mr-1.5" /> : null}
                      加载更多数据...
                    </Button>
                  </div>
                )}
              </div>
            </div>
          )}

          {/* --- List/Reader View (Split Pane) --- */}
          {currentView === "list" && (
            <div className="flex-1 flex overflow-hidden">
              {/* Left Column: Inbox List */}
              <div className="w-80 border-r border-black/5 dark:border-white/5 flex flex-col bg-zinc-50/50 dark:bg-[#18181b] shrink-0">
                <div className="p-3 border-b border-black/5 dark:border-white/5 space-y-2">
                  <div className="relative">
                    <Search className="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-zinc-400" />
                    <Input
                      placeholder="快速过滤..."
                      value={searchQuery}
                      onChange={e => setSearchQuery(e.target.value)}
                      className="pl-8 h-8 text-xs"
                    />
                  </div>
                  <div className="flex items-center gap-1 overflow-x-auto no-scrollbar py-0.5">
                    <button
                      onClick={() => { setSelectedAICategory("all"); setIsShowOnlyAIQuality(false); }}
                      className={`px-2.5 py-1 text-[10px] rounded-full transition-all font-medium border shrink-0 ${
                        selectedAICategory === "all" && !isShowOnlyAIQuality
                          ? "bg-zinc-800 text-white dark:bg-white dark:text-zinc-900 border-transparent font-bold"
                          : "bg-transparent text-zinc-600 dark:text-zinc-400 border-zinc-200 dark:border-zinc-800 hover:bg-black/5 dark:hover:bg-white/5"
                      }`}
                    >
                      全部
                    </button>
                    {aiCategories.map((cat) => (
                      <button
                        key={cat.name}
                        onClick={() => { setSelectedAICategory(cat.name); setIsShowOnlyAIQuality(false); }}
                        className={`px-2.5 py-1 text-[10px] rounded-full transition-all font-medium border shrink-0 ${
                          selectedAICategory === cat.name
                            ? "bg-zinc-800 text-white dark:bg-white dark:text-zinc-900 border-transparent font-bold"
                            : "bg-transparent text-zinc-600 dark:text-zinc-400 border-zinc-200 dark:border-zinc-800 hover:bg-black/5 dark:hover:bg-white/5"
                        }`}
                      >
                        {cat.name}
                      </button>
                    ))}
                  </div>
                </div>
                
                <div className="flex-1 overflow-y-auto">
                  {items.length === 0 ? (
                    <div className="flex flex-col items-center justify-center p-8 text-zinc-400">
                      <Inbox className="w-8 h-8 mb-2 stroke-1" />
                      <p className="text-xs text-center">暂无消息内容</p>
                    </div>
                  ) : (
                    items.map(item => (
                      <div
                        key={item.id}
                        onClick={() => handleSelectItem(item)}
                        className={`p-4 border-b border-black/5 dark:border-white/5 cursor-pointer transition-all ${
                          selectedItem?.id === item.id
                            ? "bg-white dark:bg-white/5 border-l-4 border-l-blue-500 shadow-sm"
                            : "hover:bg-black/5 dark:hover:bg-white/5 border-l-4 border-l-transparent"
                        }`}
                      >
                        <div className="flex justify-between items-center mb-1">
                          <span className="text-[9px] font-bold text-blue-500 uppercase tracking-widest truncate max-w-[120px]">
                            {item.origin_source}
                          </span>
                          <span className="text-[9px] text-zinc-400 font-medium">
                            {formatTimeAgo(item.published_at)}
                          </span>
                        </div>
                        <h4 className={`text-xs font-bold leading-snug line-clamp-2 ${
                          selectedItem?.id === item.id 
                            ? "text-blue-600 dark:text-blue-400" 
                            : item.read_status === 0 ? "text-zinc-900 dark:text-zinc-100" : "text-zinc-500"
                        }`}>
                          {item.title}
                        </h4>
                        <div className="flex justify-between items-center mt-2">
                          <div className="flex gap-1 items-center">
                            <span className={`px-1.5 py-0.5 rounded text-[8px] font-bold ${getCategoryColor(item.category)}`}>
                              {item.category}
                            </span>
                            {item.source_category && (
                              <span className="px-1.5 py-0.5 rounded text-[8px] font-bold bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400">
                                {item.source_category}
                              </span>
                            )}
                          </div>
                          <div className="flex items-center gap-1.5">
                            <Star 
                              onClick={(e) => toggleStar(item, e)}
                              className={`w-3 h-3 ${item.starred === 1 ? "fill-amber-400 text-amber-400" : "text-zinc-300"}`} 
                            />
                            {item.read_status === 0 && <span className="w-1.5 h-1.5 rounded-full bg-blue-500" />}
                          </div>
                        </div>
                      </div>
                    ))
                  )}

                  {/* Load More Button inside left sidebar */}
                  {hasMore && items.length > 0 && (
                    <div className="p-3 text-center">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => fetchItems()}
                        disabled={isLoadingItems}
                        className="w-full text-xs h-7 hover:bg-black/5"
                      >
                        {isLoadingItems ? "加载中..." : "载入更多..."}
                      </Button>
                    </div>
                  )}
                </div>
              </div>

              {/* Right Column: Full Reader Panel */}
              <div className="flex-1 flex flex-col bg-white dark:bg-[#121212] overflow-hidden">
                {selectedItem ? (
                  <div className="flex-1 flex flex-col overflow-hidden">
                    {/* Reader Top Action Bar */}
                    <div className="h-10 flex items-center justify-between px-6 border-b border-black/5 dark:border-white/5 bg-zinc-50/50 dark:bg-zinc-900/50 shrink-0">
                      <div className="flex items-center gap-2">
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => toggleReadStatus(selectedItem)}
                          className="h-7 text-xs px-2 gap-1"
                        >
                          {selectedItem.read_status === 1 ? (
                            <>
                              <CheckCircle className="w-3.5 h-3.5 text-blue-500" />
                              <span>已读</span>
                            </>
                          ) : (
                            <>
                              <Circle className="w-3.5 h-3.5 text-zinc-400" />
                              <span>标记已读</span>
                            </>
                          )}
                        </Button>
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => toggleStar(selectedItem)}
                          className="h-7 text-xs px-2 gap-1"
                        >
                          <Star className={`w-3.5 h-3.5 ${selectedItem.starred === 1 ? "fill-amber-400 text-amber-400" : "text-zinc-400"}`} />
                          <span>{selectedItem.starred === 1 ? "已收藏" : "收藏"}</span>
                        </Button>
                      </div>

                      <a
                        href={selectedItem.url}
                        target="_blank"
                        rel="noreferrer"
                        className="flex items-center gap-1 text-xs text-blue-600 hover:text-blue-700 font-medium"
                      >
                        <ExternalLink className="w-3.5 h-3.5" />
                        <span>打开原文</span>
                      </a>
                    </div>

                    {/* Reader Area */}
                    <div className="flex-1 overflow-y-auto p-8 md:p-12">
                      <div className="max-w-2xl mx-auto space-y-6">
                        <div className="space-y-2">
                          <div className="flex gap-1.5 items-center">
                            <span className={`px-2 py-0.5 rounded-full text-[10px] font-bold tracking-tight ${getCategoryColor(selectedItem.category)}`}>
                              {getCategoryLabel(selectedItem.category)}
                            </span>
                            {selectedItem.source_category && (
                              <span className="px-2 py-0.5 rounded-full text-[10px] font-bold tracking-tight bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400">
                                {selectedItem.source_category}
                              </span>
                            )}
                          </div>
                          <h1 className="text-2xl md:text-3xl font-black tracking-tight leading-tight">
                            {selectedItem.title}
                          </h1>
                          <div className="flex items-center gap-2 text-xs text-zinc-400 pt-2 pb-4 border-b border-zinc-100 dark:border-zinc-800">
                            <span className="font-semibold text-zinc-600 dark:text-zinc-300">{selectedItem.origin_source}</span>
                            <span>•</span>
                            <span>发布于: {selectedItem.published_at ? new Date(selectedItem.published_at).toLocaleString() : "未知时间"}</span>
                          </div>
                        </div>

                        {/* AI Review Banner */}
                        {selectedItem.quality_score !== undefined && selectedItem.quality_score > 0 ? (
                          <div className="bg-indigo-50/50 dark:bg-indigo-950/10 border-l-4 border-indigo-500 p-4 rounded-r-xl space-y-3">
                            <div className="flex justify-between items-center">
                              <h5 className="text-xs font-bold text-indigo-600 dark:text-indigo-400 uppercase tracking-widest flex items-center gap-1.5">
                                <Sparkles className="w-3.5 h-3.5" />
                                AI 智能深度分析 AI Insights
                              </h5>
                              <span className="text-xs font-black bg-indigo-600 text-white dark:bg-indigo-700 px-2.5 py-0.5 rounded-full">
                                评分: {selectedItem.quality_score} / 10
                              </span>
                            </div>
                            
                            {selectedItem.ai_category && (
                              <div className="flex flex-wrap gap-1.5 items-center">
                                <span className="text-[10px] bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400 px-2 py-0.5 rounded-md font-semibold">
                                  分类: {selectedItem.ai_category} {selectedItem.ai_subcategory ? `(${selectedItem.ai_subcategory})` : ""}
                                </span>
                                {selectedItem.ai_tags && selectedItem.ai_tags.split(',').map((tag: string) => (
                                  <span key={tag} className="text-[9px] bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400 px-1.5 py-0.5 rounded">
                                    #{tag}
                                  </span>
                                ))}
                              </div>
                            )}

                            {selectedItem.ai_summary && (
                              <div>
                                <p className="text-xs leading-relaxed text-zinc-700 dark:text-zinc-300 font-medium">
                                  <span className="font-bold text-indigo-600 dark:text-indigo-400">智能摘要: </span>
                                  {selectedItem.ai_summary}
                                </p>
                              </div>
                            )}

                            {selectedItem.ai_comment && (
                              <div className="text-[11px] text-zinc-500 dark:text-zinc-400 bg-white/50 dark:bg-black/10 rounded-lg p-2.5 border border-indigo-500/5 leading-relaxed">
                                <span className="font-bold text-zinc-600 dark:text-zinc-300">深度点评: </span>
                                {selectedItem.ai_comment}
                              </div>
                            )}
                            
                            {selectedItem.ai_model_used && (
                              <p className="text-[9px] text-zinc-400 text-right">
                                分析模型: {selectedItem.ai_model_used}
                              </p>
                            )}
                          </div>
                        ) : (
                          /* Fallback Normal Summary Banner */
                          selectedItem.summary && (
                            <div className="bg-blue-50 dark:bg-blue-950/20 border-l-4 border-blue-500 p-4 rounded-r-xl">
                              <h5 className="text-xs font-bold text-blue-600 dark:text-blue-400 uppercase tracking-widest mb-1">
                                摘要 Summary
                              </h5>
                              <p 
                                className="text-xs leading-relaxed text-zinc-700 dark:text-zinc-300"
                                dangerouslySetInnerHTML={{ __html: selectedItem.summary }}
                              />
                            </div>
                          )
                        )}

                        {/* Content Area */}
                        {isLoadingDetail ? (
                          <div className="flex items-center justify-center py-20 text-zinc-400">
                            <Loader2 className="w-8 h-8 animate-spin" />
                          </div>
                        ) : (
                          <article 
                            className="prose dark:prose-invert max-w-none text-sm md:text-base leading-relaxed text-zinc-800 dark:text-zinc-200"
                            dangerouslySetInnerHTML={{ __html: itemDetailHtml || `<p>${selectedItem.content || "无法解析正文"}</p>` }}
                          />
                        )}
                      </div>
                    </div>
                  </div>
                ) : (
                  <div className="flex-1 flex flex-col items-center justify-center text-zinc-400">
                    <Inbox className="w-12 h-12 mb-2 stroke-1" />
                    <p className="text-sm">选择左侧项目开始阅读</p>
                  </div>
                )}
              </div>
            </div>
          )}

          {/* --- Settings Panel --- */}
          {currentView === "settings" && (
            <div className="flex-1 overflow-y-auto bg-zinc-50/50 dark:bg-[#121212] p-8">
              <div className="max-w-4xl mx-auto space-y-8 pb-12">
                <Card className="border border-black/5 dark:border-white/5 bg-white dark:bg-[#1c1c1e] shadow-sm rounded-2xl overflow-hidden">
                  <CardHeader className="border-b border-black/5 p-6 bg-zinc-50/50 dark:bg-zinc-900/50">
                    <CardTitle className="text-base font-bold">已订阅订阅源 ({sources.length})</CardTitle>
                    <CardDescription className="text-xs">管理你的 RSS Feed、JSON API 和智能网页爬虫规则。</CardDescription>
                  </CardHeader>
                  <CardContent className="p-0">
                    <div className="divide-y divide-black/5 dark:divide-white/5">
                      {sources.map(source => (
                        <div key={source.id} className={`p-4 flex flex-wrap gap-4 items-center justify-between transition-opacity ${source.enabled === 0 ? "opacity-50" : ""}`}>
                          <div className="flex items-center gap-3">
                            <div className={`w-10 h-10 rounded-xl flex items-center justify-center shrink-0 ${source.enabled === 0 ? "bg-zinc-100 dark:bg-zinc-800 text-zinc-400" : "bg-blue-50 dark:bg-zinc-800 text-blue-500"}`}>
                              <Database className="w-5 h-5" />
                            </div>
                            <div>
                              <div className="flex items-center gap-2">
                                <h5 className="font-bold text-sm leading-none">{source.name}</h5>
                                <span className={`text-[9px] px-1.5 py-0.2 rounded font-bold uppercase ${
                                  source.type === "web_scrape"
                                    ? "bg-purple-100 text-purple-600 dark:bg-purple-900/20"
                                    : source.type === "api"
                                      ? "bg-amber-100 text-amber-600 dark:bg-amber-900/20"
                                      : "bg-blue-100 text-blue-600 dark:bg-blue-900/20"
                                }`}>
                                  {source.type}
                                </span>
                                {source.enabled === 0 && (
                                  <span className="text-[9px] px-1.5 py-0.2 rounded font-bold bg-zinc-200 text-zinc-500 dark:bg-zinc-700 dark:text-zinc-400">
                                    已禁用
                                  </span>
                                )}
                              </div>
                              <p className="text-[10px] text-zinc-400 mt-1 truncate max-w-xs md:max-w-md font-mono">{source.url}</p>

                              <div className="flex items-center gap-3 mt-1.5 text-[10px] text-zinc-500 font-medium">
                                <span className="flex items-center gap-0.5"><Calendar className="w-3 h-3" /> Cron: {source.schedule}</span>
                                <span>•</span>
                                <span>主题分类: {source.category || "General"}</span>
                                <span>•</span>
                                <span>默认分类: {source.default_category}</span>
                                {source.last_fetch_at && (
                                  <>
                                    <span>•</span>
                                    <span className={`flex items-center gap-1 ${
                                      source.last_fetch_status === "success"
                                        ? "text-emerald-500"
                                        : source.last_fetch_status === "running"
                                          ? "text-blue-500"
                                          : "text-rose-500"
                                    }`}>
                                      上次抓取: {source.last_fetch_status === "running" ? "抓取中..." : formatTimeAgo(source.last_fetch_at)}
                                    </span>
                                  </>
                                )}
                              </div>
                            </div>
                          </div>

                          <div className="flex items-center gap-3">
                            {/* Enable/Disable Toggle */}
                            <div className="flex items-center gap-2">
                              <span className="text-xs text-zinc-400 font-medium select-none">
                                {source.enabled === 1 ? "启用" : "禁用"}
                              </span>
                              <Switch
                                checked={source.enabled === 1}
                                onCheckedChange={() => handleToggleSourceEnabled(source)}
                              />
                            </div>

                            {/* Run Now Trigger */}
                            <Button
                              onClick={() => handleRunSource(source)}
                              size="sm"
                              variant="outline"
                              disabled={source.last_fetch_status === "running" || source.enabled === 0}
                              className="h-7 text-xs gap-1"
                            >
                              {source.last_fetch_status === "running" ? (
                                <Loader2 className="w-3 h-3 animate-spin" />
                              ) : (
                                <RefreshCw className="w-3 h-3" />
                              )}
                              立即抓取
                            </Button>

                            {/* Edit & Delete Action Menu */}
                            <Button onClick={() => openEditSourceDialog(source)} size="icon" variant="ghost" className="w-8 h-8">
                              <Edit className="w-3.5 h-3.5 text-zinc-500" />
                            </Button>
                            <Button onClick={() => handleDeleteSource(source.id)} size="icon" variant="ghost" className="w-8 h-8 hover:bg-rose-50 dark:hover:bg-rose-950/20">
                              <Trash2 className="w-3.5 h-3.5 text-rose-500" />
                            </Button>
                          </div>
                        </div>
                      ))}
                    </div>
                  </CardContent>
                </Card>
              </div>
            </div>
          )}

          {/* --- AI Settings Panel --- */}
          {currentView === "ai-settings" && (
            <div className="flex-1 overflow-y-auto bg-zinc-50/50 dark:bg-[#121212] p-8">
              <div className="max-w-4xl mx-auto space-y-8 pb-12">
                <Card className="border border-black/5 dark:border-white/5 bg-white dark:bg-[#1c1c1e] shadow-sm rounded-2xl overflow-hidden">
                  <CardHeader className="border-b border-black/5 p-6 bg-zinc-50/50 dark:bg-zinc-900/50">
                    <CardTitle className="text-base font-bold flex items-center gap-2">
                      <Sparkles className="w-5 h-5 text-indigo-500" /> AI 评估与模型配置 (AI Settings)
                    </CardTitle>
                    <CardDescription className="text-xs">
                      配置个人大语言模型（LLM）服务以进行文章自动打分、提取精炼摘要与生成智能日报。
                    </CardDescription>
                  </CardHeader>
                  <CardContent className="p-6 space-y-6">
                    {isLoadingSettings ? (
                      <div className="flex items-center justify-center py-12">
                        <Loader2 className="w-6 h-6 animate-spin text-zinc-400" />
                        <span className="text-sm text-zinc-400 ml-2">正在加载配置...</span>
                      </div>
                    ) : (
                      <>
                        {/* Trigger AI Evaluation Backfill */}
                        <div className="flex items-center justify-between p-4 bg-indigo-50/30 dark:bg-indigo-950/10 rounded-xl border border-indigo-100 dark:border-indigo-900/30">
                          <div>
                            <h5 className="text-sm font-semibold text-indigo-950 dark:text-indigo-300">后台增量评测队列 (AI Evaluation Queue)</h5>
                            <p className="text-[11px] text-zinc-400 mt-0.5">可以手动触发对数据库中未被 AI 分析的文章进行增量评估。</p>
                          </div>
                          <Button
                            onClick={handleStartEvaluation}
                            size="sm"
                            className="bg-indigo-600 hover:bg-indigo-700 text-white rounded-lg px-4 h-8 text-xs gap-1.5 font-sans"
                          >
                            <Sparkles className="w-3.5 h-3.5" />
                            立即评测未评估内容
                          </Button>
                        </div>
                        {/* Toggle AI Enabled */}
                        <div className="flex items-center justify-between p-4 bg-zinc-50 dark:bg-zinc-900/40 rounded-xl border border-black/5 dark:border-white/5">
                          <div>
                            <h5 className="text-sm font-semibold">启用 AI 语义分析与评分</h5>
                            <p className="text-[11px] text-zinc-400 mt-0.5">关闭后，新抓取的文章将不再进行自动分类打分，也不会生成每日简报。</p>
                          </div>
                          <Switch
                            checked={aiEnabled}
                            onCheckedChange={setAiEnabled}
                          />
                        </div>

                        {aiEnabled && (
                          <div className="space-y-6 animate-in fade-in duration-200">
                            {/* Provider Profiles */}
                            <div className="space-y-4 p-4 bg-zinc-50 dark:bg-zinc-900/40 rounded-xl border border-black/5 dark:border-white/5">
                              <div className="flex flex-col md:flex-row md:items-end gap-3">
                                <div className="space-y-2 flex-1">
                                  <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">服务商档案</label>
                                  <Select value={activeProfileId} onValueChange={handleSelectAIProfile}>
                                    <SelectTrigger className="w-full h-9 border border-zinc-200 dark:border-zinc-800 text-sm">
                                      <SelectValue placeholder="选择一个服务商档案" />
                                    </SelectTrigger>
                                    <SelectContent>
                                      {aiProfiles.map((profile) => (
                                        <SelectItem key={profile.id} value={profile.id}>
                                          {profile.name || "未命名服务商"}
                                        </SelectItem>
                                      ))}
                                    </SelectContent>
                                  </Select>
                                </div>

                                <Button
                                  type="button"
                                  onClick={handleAddAIProfile}
                                  variant="outline"
                                  className="h-9 text-xs gap-1.5 border-zinc-200 dark:border-zinc-800"
                                >
                                  <Plus className="w-3.5 h-3.5" />
                                  添加档案
                                </Button>
                                <Button
                                  type="button"
                                  onClick={handleDeleteAIProfile}
                                  variant="outline"
                                  disabled={aiProfiles.length <= 1}
                                  className="h-9 text-xs gap-1.5 border-zinc-200 dark:border-zinc-800 text-rose-600 hover:text-rose-700"
                                >
                                  <Trash2 className="w-3.5 h-3.5" />
                                  删除
                                </Button>
                              </div>

                              {/* Strategy selector */}
                              <div className="space-y-2">
                                <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">多模型策略</label>
                                <Select value={aiStrategy} onValueChange={setAiStrategy}>
                                  <SelectTrigger className="w-full h-9 border border-zinc-200 dark:border-zinc-800 text-sm">
                                    <SelectValue placeholder="选择策略" />
                                  </SelectTrigger>
                                  <SelectContent>
                                    <SelectItem value="single">单一模式 — 只用选中的档案</SelectItem>
                                    <SelectItem value="round-robin">轮询模式 — 多个模型轮流使用</SelectItem>
                                    <SelectItem value="failover">故障转移 — 主模型不可用时自动切换</SelectItem>
                                  </SelectContent>
                                </Select>
                              </div>

                              {/* Profile list — always visible */}
                              <div className="space-y-2">
                                <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">
                                  已配置的服务商列表
                                  {aiStrategy === "failover" && "（数字越小优先级越高）"}
                                </label>
                                <div className="space-y-1.5">
                                  {aiProfiles
                                    .slice()
                                    .sort((a, b) => (a.priority || 999) - (b.priority || 999))
                                    .map((profile, idx) => (
                                    <div
                                      key={profile.id}
                                      className={`flex items-center gap-2 px-3 py-2 rounded-lg border text-sm transition-colors ${
                                        profile.id === activeProfileId
                                          ? "border-indigo-300 dark:border-indigo-700 bg-indigo-50/40 dark:bg-indigo-950/20"
                                          : profile.disabled
                                            ? "border-zinc-200 dark:border-zinc-800 opacity-50 bg-zinc-100 dark:bg-zinc-900"
                                            : "border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900/60"
                                      }`}
                                    >
                                      {/* Priority badge (failover only) */}
                                      {aiStrategy === "failover" && (
                                        <span className="flex-shrink-0 w-5 h-5 flex items-center justify-center rounded-full bg-zinc-200 dark:bg-zinc-700 text-[10px] font-bold text-zinc-600 dark:text-zinc-300">
                                          {idx + 1}
                                        </span>
                                      )}
                                      <span className="flex-1 truncate font-medium">{profile.name || "未命名"}</span>
                                      {/* Default badge */}
                                      {profile.id === activeProfileId && (
                                        <span className="flex-shrink-0 text-[10px] font-semibold px-1.5 py-0.5 rounded bg-indigo-100 dark:bg-indigo-900/40 text-indigo-600 dark:text-indigo-400">
                                          默认
                                        </span>
                                      )}
                                      <span className="text-[10px] text-zinc-400">{profile.provider}</span>
                                      <span className="text-[10px] text-zinc-400">{profile.requests_per_minute || 10}/min</span>
                                      {/* Enable/disable toggle */}
                                      <button
                                        type="button"
                                        onClick={() => {
                                          const next = aiProfiles.map(p =>
                                            p.id === profile.id ? { ...p, disabled: !p.disabled } : p
                                          );
                                          setAiProfiles(next);
                                        }}
                                        className={`relative w-9 h-5 rounded-full transition-colors flex-shrink-0 ${
                                          profile.disabled
                                            ? "bg-zinc-300 dark:bg-zinc-700"
                                            : "bg-emerald-500"
                                        }`}
                                      >
                                        <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow transition-transform ${
                                          profile.disabled ? "" : "translate-x-4"
                                        }`} />
                                      </button>
                                    </div>
                                  ))}
                                </div>
                              </div>

                              <div className="space-y-2">
                                <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">档案名称</label>
                                <Input
                                  value={aiProfileName}
                                  onChange={(e) => handleProfileNameChange(e.target.value)}
                                  placeholder="如 LM Studio 本地、OpenAI、Gemini"
                                  className="h-9 text-sm bg-white dark:bg-zinc-900/50"
                                />
                              </div>
                            </div>

                            {/* Provider, Model, APIKey */}
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                              <div className="space-y-2">
                                <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">AI 服务商 (AI Provider)</label>
                                <Select value={aiProvider} onValueChange={setAiProvider}>
                                  <SelectTrigger className="w-full h-9 border border-zinc-200 dark:border-zinc-800 text-sm">
                                    <SelectValue placeholder="选择服务商" />
                                  </SelectTrigger>
                                  <SelectContent>
                                    <SelectItem value="gemini">Google Gemini</SelectItem>
                                    <SelectItem value="openai">OpenAI</SelectItem>
                                    <SelectItem value="custom">自定义兼容 OpenAI (Custom)</SelectItem>
                                    <SelectItem value="lmstudio">LM Studio (本地)</SelectItem>
                                  </SelectContent>
                                </Select>
                              </div>

                              <div className="space-y-2">
                                <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">评分阈值 (Quality Threshold)</label>
                                <Select value={String(aiQualityThreshold)} onValueChange={(v) => setAiQualityThreshold(Number(v))}>
                                  <SelectTrigger className="w-full h-9 border border-zinc-200 dark:border-zinc-800 text-sm">
                                    <SelectValue placeholder="选择评分阈值" />
                                  </SelectTrigger>
                                  <SelectContent>
                                    <SelectItem value="5">5分及以上 (普通质量)</SelectItem>
                                    <SelectItem value="6">6分及以上 (中等质量)</SelectItem>
                                    <SelectItem value="7">7分及以上 (高分推荐 - 推荐)</SelectItem>
                                    <SelectItem value="8">8分及以上 (极其优质)</SelectItem>
                                    <SelectItem value="9">9分及以上 (行业特写)</SelectItem>
                                  </SelectContent>
                                </Select>
                              </div>

                              <div className="space-y-2">
                                <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">模型名称 (AI Model)</label>
                                <Input
                                  value={aiModel}
                                  onChange={(e) => setAiModel(e.target.value)}
                                  placeholder={aiProvider === "lmstudio" ? "如 gemma-4-12b 或 qwen2.5-7b" : "如 googleai/gemini-2.0-flash 或 openai/gpt-4o-mini"}
                                  className="h-9 text-sm"
                                />
                                {aiProvider === "lmstudio" ? (
                                  <p className="text-[10px] text-zinc-400">LM Studio 中加载的模型名称，可在 LM Studio 界面查看。</p>
                                ) : (
                                  <p className="text-[10px] text-zinc-400">对应 Genkit Go 模型格式，形式为 <code>provider/model-name</code>。</p>
                                )}
                              </div>

                              <div className="space-y-2">
                                <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">API 密钥 (API Key)</label>
                                <Input
                                  type="password"
                                  value={aiApiKey}
                                  onChange={(e) => setAiApiKey(e.target.value)}
                                  placeholder="输入 API Key 密钥"
                                  className="h-9 text-sm"
                                />
                              </div>

                              <div className="space-y-2">
                                <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">请求频率限制 (Requests/min)</label>
                                <Input
                                  type="number"
                                  min={1}
                                  max={1000}
                                  value={aiRequestsPerMinute}
                                  onChange={(e) => setAiRequestsPerMinute(Math.max(1, Number(e.target.value) || 10))}
                                  className="h-9 text-sm"
                                />
                                <p className="text-[10px] text-zinc-400">每分钟最大请求数，不同服务商可分别设置。本地模型可设高（如 100），云端 API 建议设低（如 5-10）。</p>
                              </div>
                            </div>

                            {/* Base URL (Custom / LM Studio Provider) */}
                            {(aiProvider === "custom" || aiProvider === "lmstudio") && (
                              <div className="space-y-2 animate-in slide-in-from-top-2 duration-200">
                                <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">接口 Base URL</label>
                                <Input
                                  value={aiBaseUrl}
                                  onChange={(e) => setAiBaseUrl(e.target.value)}
                                  placeholder={aiProvider === "lmstudio" ? "如 http://localhost:1234" : "如 https://api.moonshot.cn/v1 或 https://api.deepseek.com/v1"}
                                  className="h-9 text-sm"
                                />
                                {aiProvider === "lmstudio" && (
                                  <p className="text-[10px] text-zinc-400">LM Studio 本地服务地址，默认 <code>http://localhost:1234</code>，无需 API 密钥。</p>
                                )}
                                {aiProvider === "custom" && (
                                  <p className="text-[10px] text-zinc-400">只有在 AI 服务商选择"自定义兼容 OpenAI"时该配置项才生效。</p>
                                )}
                              </div>
                            )}

                            {/* System Prompt Customization */}
                            <div className="space-y-2">
                              <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">AI 资讯深度分析提示词 (System Prompt)</label>
                              <p className="text-[10px] text-zinc-400 mb-1">
                                自定义深度分析提示词。可用占位符：
                                <code className="mx-1 bg-zinc-100 dark:bg-zinc-800 px-1 py-0.5 rounded text-[10px]">{`{{.Title}}`}</code>, 
                                <code className="mx-1 bg-zinc-100 dark:bg-zinc-800 px-1 py-0.5 rounded text-[10px]">{`{{.OriginSource}}`}</code>, 
                                <code className="mx-1 bg-zinc-100 dark:bg-zinc-800 px-1 py-0.5 rounded text-[10px]">{`{{.Summary}}`}</code>, 
                                <code className="mx-1 bg-zinc-100 dark:bg-zinc-800 px-1 py-0.5 rounded text-[10px]">{`{{.Content}}`}</code>
                              </p>
                              <textarea
                                value={aiSystemPrompt}
                                onChange={(e) => setAiSystemPrompt(e.target.value)}
                                rows={8}
                                className="flex w-full rounded-xl border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900/50 px-3 py-2 text-xs focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-indigo-500 font-mono resize-y"
                                placeholder="输入系统分析提示词..."
                              />
                            </div>

                            {/* Daily Prompt Customization */}
                            <div className="space-y-2">
                              <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">AI 智能日报生成提示词 (Daily Prompt)</label>
                              <p className="text-[10px] text-zinc-400 mb-1">
                                自定义简报生成提示词。可用占位符：
                                <code className="mx-1 bg-zinc-100 dark:bg-zinc-800 px-1 py-0.5 rounded text-[10px]">{`{{.Count}}`}</code>, 
                                <code className="mx-1 bg-zinc-100 dark:bg-zinc-800 px-1 py-0.5 rounded text-[10px]">{`{{.FeedText}}`}</code>, 
                                <code className="mx-1 bg-zinc-100 dark:bg-zinc-800 px-1 py-0.5 rounded text-[10px]">{`{{.TotalItems}}`}</code>, 
                                <code className="mx-1 bg-zinc-100 dark:bg-zinc-800 px-1 py-0.5 rounded text-[10px]">{`{{.QualityItems}}`}</code>
                              </p>
                              <textarea
                                value={aiDailyPrompt}
                                onChange={(e) => setAiDailyPrompt(e.target.value)}
                                rows={8}
                                className="flex w-full rounded-xl border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900/50 px-3 py-2 text-xs focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-indigo-500 font-mono resize-y"
                                placeholder="输入日报生成提示词..."
                              />
                            </div>

                            {/* Morning/Evening Report Scheduling */}
                            <div className="space-y-4 pt-2 border-t border-zinc-200 dark:border-zinc-800">
                              <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">定时早晚报设置</label>

                              {/* Morning Report */}
                              <div className="flex items-center gap-4 p-3 bg-amber-50/50 dark:bg-amber-950/10 rounded-xl border border-amber-200/50 dark:border-amber-900/20">
                                <label className="flex items-center gap-2 cursor-pointer">
                                  <input
                                    type="checkbox"
                                    checked={morningReportEnabled}
                                    onChange={(e) => setMorningReportEnabled(e.target.checked)}
                                    className="rounded border-zinc-300 text-amber-600 focus:ring-amber-500"
                                  />
                                  <span className="text-xs font-semibold text-amber-700 dark:text-amber-400">🌅 启用早报</span>
                                </label>
                                <div className="flex items-center gap-2">
                                  <label className="text-[10px] text-zinc-500">时间</label>
                                  <Input
                                    type="time"
                                    value={morningReportTime}
                                    onChange={(e) => setMorningReportTime(e.target.value)}
                                    className="h-7 text-xs w-24"
                                  />
                                </div>
                                <p className="text-[10px] text-zinc-400 flex-1">覆盖最近 24 小时优质内容</p>
                              </div>

                              {/* Evening Report */}
                              <div className="flex items-center gap-4 p-3 bg-blue-50/50 dark:bg-blue-950/10 rounded-xl border border-blue-200/50 dark:border-blue-900/20">
                                <label className="flex items-center gap-2 cursor-pointer">
                                  <input
                                    type="checkbox"
                                    checked={eveningReportEnabled}
                                    onChange={(e) => setEveningReportEnabled(e.target.checked)}
                                    className="rounded border-zinc-300 text-blue-600 focus:ring-blue-500"
                                  />
                                  <span className="text-xs font-semibold text-blue-700 dark:text-blue-400">🌙 启用晚报</span>
                                </label>
                                <div className="flex items-center gap-2">
                                  <label className="text-[10px] text-zinc-500">时间</label>
                                  <Input
                                    type="time"
                                    value={eveningReportTime}
                                    onChange={(e) => setEveningReportTime(e.target.value)}
                                    className="h-7 text-xs w-24"
                                  />
                                </div>
                                <p className="text-[10px] text-zinc-400 flex-1">覆盖当日早报至晚报时段内容</p>
                              </div>
                            </div>

                            {/* AI Connection Test Result */}
                            {aiTestResult && (
                              <div className={`p-4 rounded-xl border text-xs space-y-2.5 mt-4 animate-in fade-in duration-200 ${
                                aiTestResult.success 
                                  ? "bg-emerald-50/50 dark:bg-emerald-950/10 border-emerald-200 dark:border-emerald-900/30 text-emerald-800 dark:text-emerald-300"
                                  : "bg-rose-50/50 dark:bg-rose-950/10 border-rose-200 dark:border-rose-900/30 text-rose-800 dark:text-rose-300"
                              }`}>
                                <div className="font-bold flex items-center gap-1.5 text-sm">
                                  {aiTestResult.success ? (
                                    <>
                                      <span className="text-emerald-500">●</span> AI 接口连接成功 (Success)
                                    </>
                                  ) : (
                                    <>
                                      <span className="text-rose-500">●</span> AI 接口连接失败 (Failed)
                                    </>
                                  )}
                                </div>
                                {aiTestResult.success ? (
                                  <div className="space-y-1.5 font-sans">
                                    <p className="font-semibold text-zinc-700 dark:text-zinc-300">
                                      测试文章标题: <span className="font-bold text-zinc-900 dark:text-white">{aiTestResult.title}</span>
                                    </p>
                                    <div className="grid grid-cols-2 gap-2 mt-2 pt-2 border-t border-black/5 dark:border-white/5 text-[11px]">
                                      <div>
                                        <span className="text-zinc-400">智能分类:</span> <span className="font-bold text-zinc-700 dark:text-zinc-300">{aiTestResult.analysis.ai_category} ({aiTestResult.analysis.ai_subcategory || "无"})</span>
                                      </div>
                                      <div>
                                        <span className="text-zinc-400">质量评分:</span> <span className="font-bold text-indigo-600 dark:text-indigo-400">{aiTestResult.analysis.quality_score} / 10 分</span>
                                      </div>
                                    </div>
                                    <div className="mt-2 pt-2 border-t border-black/5 dark:border-white/5">
                                      <span className="text-zinc-400 block mb-0.5">AI 极简摘要 (100字):</span>
                                      <p className="text-zinc-600 dark:text-zinc-400 leading-relaxed font-sans">{aiTestResult.analysis.ai_summary}</p>
                                    </div>
                                    {aiTestResult.analysis.ai_comment && (
                                      <div className="mt-2 pt-2 border-t border-black/5 dark:border-white/5">
                                        <span className="text-zinc-400 block mb-0.5">AI 推荐理由 / 避坑评价:</span>
                                        <p className="text-zinc-600 dark:text-zinc-400 leading-relaxed font-sans">{aiTestResult.analysis.ai_comment}</p>
                                      </div>
                                    )}
                                  </div>
                                ) : (
                                  <p className="font-mono bg-white/50 dark:bg-black/20 p-2.5 rounded border border-black/5 dark:border-white/5 leading-relaxed break-all font-sans">
                                    {aiTestResult.error}
                                  </p>
                                )}
                              </div>
                            )}
                          </div>
                        )}
                      </>
                    )}
                  </CardContent>
                  {!isLoadingSettings && (
                    <CardFooter className="border-t border-black/5 p-6 bg-zinc-50/50 dark:bg-zinc-900/50 flex justify-end">
                      {aiEnabled && (
                        <Button
                          onClick={handleTestAI}
                          disabled={isTestingAI || isSavingSettings}
                          variant="outline"
                          className="mr-3 border-zinc-200 dark:border-zinc-800 rounded-xl h-9 text-xs gap-1.5"
                        >
                          {isTestingAI ? (
                            <>
                              <Loader2 className="w-3.5 h-3.5 animate-spin" />
                              正在测试...
                            </>
                          ) : (
                            <>
                              <RefreshCw className="w-3 h-3" />
                              测试 AI 连通性
                            </>
                          )}
                        </Button>
                      )}
                      <Button
                        onClick={saveAISettings}
                        disabled={isSavingSettings}
                        className="bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl shadow-md px-5 h-9 text-sm"
                      >
                        {isSavingSettings ? (
                          <>
                            <Loader2 className="w-4 h-4 animate-spin mr-2" />
                            正在保存...
                          </>
                        ) : (
                          "保存 AI 配置"
                        )}
                      </Button>
                    </CardFooter>
                  )}
                </Card>
              </div>
            </div>
          )}

          {/* --- Logs View --- */}
          {currentView === "logs" && (
            <div className="flex-1 overflow-y-auto bg-zinc-50/50 dark:bg-[#121212] p-8">
              <div className="max-w-5xl mx-auto space-y-6 pb-12">
                <div className="flex justify-between items-center">
                  <div>
                    <h3 className="text-base font-bold">抓取执行日志 Log Records</h3>
                    <p className="text-xs text-zinc-500">查看最近 50 次数据源抓取的执行记录，以进行运维和故障排查。</p>
                  </div>
                  <Button onClick={fetchLogs} size="sm" variant="outline" className="h-8 gap-1 text-xs">
                    <RefreshCw className="w-3.5 h-3.5" /> 刷新日志
                  </Button>
                </div>

                <Card className="border border-black/5 dark:border-white/5 bg-white dark:bg-[#1c1c1e] shadow-sm rounded-2xl overflow-hidden">
                  <Table>
                    <TableHeader className="bg-zinc-50/50 dark:bg-zinc-900/50">
                      <TableRow>
                        <TableHead className="text-xs font-bold w-[120px]">开始时间</TableHead>
                        <TableHead className="text-xs font-bold w-[100px]">数据源</TableHead>
                        <TableHead className="text-xs font-bold w-[80px]">状态</TableHead>
                        <TableHead className="text-xs font-bold text-center w-[80px]">发现数</TableHead>
                        <TableHead className="text-xs font-bold text-center w-[80px]">新增数</TableHead>
                        <TableHead className="text-xs font-bold">错误信息 / 诊断</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {logs.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={6} className="text-center text-xs text-zinc-500 py-10">
                            暂无抓取执行日志。
                          </TableCell>
                        </TableRow>
                      ) : (
                        logs.map(log => (
                          <TableRow key={log.id}>
                            <TableCell className="text-xs text-zinc-500 font-mono">
                              {new Date(log.started_at).toLocaleString()}
                            </TableCell>
                            <TableCell className="text-xs font-semibold">
                              {log.source_id}
                            </TableCell>
                            <TableCell className="text-xs">
                              <span className={`px-2 py-0.5 rounded-full text-[9px] font-bold ${
                                log.status === "success" 
                                  ? "bg-emerald-500/10 text-emerald-600" 
                                  : log.status === "running"
                                    ? "bg-blue-500/10 text-blue-600"
                                    : log.status === "skipped"
                                      ? "bg-zinc-500/10 text-zinc-600"
                                      : "bg-rose-500/10 text-rose-600"
                              }`}>
                                {log.status}
                              </span>
                            </TableCell>
                            <TableCell className="text-xs text-center font-mono">{log.items_found}</TableCell>
                            <TableCell className="text-xs text-center font-mono">{log.items_added}</TableCell>
                            <TableCell className="text-xs text-zinc-500 font-mono max-w-xs truncate">
                              {log.error_message || "-"}
                            </TableCell>
                          </TableRow>
                        ))
                      )}
                    </TableBody>
                  </Table>
                </Card>
              </div>
            </div>
          )}

          {/* --- Daily Report View --- */}
          {currentView === "daily" && (
            <div className="flex-1 overflow-y-auto bg-zinc-50/50 dark:bg-[#121212] p-8">
              <div className="max-w-3xl mx-auto space-y-6 pb-12">
                <div className="flex flex-wrap justify-between items-center gap-4">
                  <div>
                    <h3 className="text-lg font-black tracking-tight flex items-center gap-2 text-indigo-950 dark:text-white">
                      <Sparkles className="w-5 h-5 text-indigo-500" />
                      AI 智能日报 Daily Report
                    </h3>
                    <p className="text-xs text-zinc-500 flex items-center gap-2">
                      阅读 AI 汇编整理的深度资讯和数据看板。
                      <a
                        href="/api/ai/daily/rss"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[10px] font-semibold bg-orange-100 text-orange-600 hover:bg-orange-200 dark:bg-orange-900/30 dark:text-orange-400 dark:hover:bg-orange-900/50 transition-colors no-underline"
                        title="复制或在 RSS 阅读器中打开"
                      >
                        <Rss className="w-3 h-3" />
                        RSS 订阅
                      </a>
                    </p>
                  </div>
                  <div className="flex items-center gap-2">
                    <Input
                      type="date"
                      value={selectedReportDate}
                      onChange={e => {
                        setSelectedReportDate(e.target.value);
                        fetchDailyReport(e.target.value);
                      }}
                      className="h-8 text-xs w-36"
                    />
                    <Button
                      onClick={() => handleGenerateDailyReport("daily")}
                      disabled={isGeneratingReport}
                      size="sm"
                      className="h-8 gap-1 text-xs bg-indigo-600 hover:bg-indigo-700 text-white font-bold"
                    >
                      {isGeneratingReport ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <RefreshCw className="w-3.5 h-3.5" />}
                      生成日报
                    </Button>
                  </div>
                </div>

                {/* Report Type Tabs + Report List */}
                {reportList.length > 0 && (
                  <div className="bg-white dark:bg-[#1c1c1e] rounded-2xl border border-black/5 dark:border-white/5 overflow-hidden">
                    <div className="flex items-center gap-1 px-4 pt-3 pb-0">
                      {[{key: "all", label: "全部", icon: "📋"}, {key: "morning", label: "早报", icon: "🌅"}, {key: "evening", label: "晚报", icon: "🌙"}, {key: "daily", label: "日报", icon: "📰"}].map(tab => (
                        <button
                          key={tab.key}
                          onClick={() => {
                            setSelectedReportType(tab.key);
                          }}
                          className={`px-3 py-1.5 text-xs rounded-lg font-semibold transition-all ${
                            selectedReportType === tab.key
                              ? "bg-indigo-100 dark:bg-indigo-900/30 text-indigo-700 dark:text-indigo-300"
                              : "text-zinc-500 hover:bg-zinc-100 dark:hover:bg-zinc-800"
                          }`}
                        >
                          {tab.icon} {tab.label}
                        </button>
                      ))}
                    </div>
                    <div className="p-3 grid gap-1.5 max-h-48 overflow-y-auto">
                      {reportList
                        .filter(r => selectedReportType === "all" || r.report_type === selectedReportType)
                        .map((r, idx) => {
                          const typeIcon = r.report_type === "morning" ? "🌅" : r.report_type === "evening" ? "🌙" : "📰";
                          const typeLabel = r.report_type === "morning" ? "早报" : r.report_type === "evening" ? "晚报" : "日报";
                          const isActive = dailyReport && dailyReport.report_date === r.report_date && dailyReport.report_type === r.report_type;
                          return (
                            <button
                              key={idx}
                              onClick={() => {
                                setSelectedReportDate(r.report_date);
                                setSelectedReportType(r.report_type);
                                fetchDailyReport(r.report_date, r.report_type);
                              }}
                              className={`flex items-center gap-3 px-3 py-2 rounded-xl text-left transition-all ${
                                isActive
                                  ? "bg-indigo-50 dark:bg-indigo-950/20 border border-indigo-200 dark:border-indigo-800"
                                  : "hover:bg-zinc-50 dark:hover:bg-zinc-900/40 border border-transparent"
                              }`}
                            >
                              <span className="text-base">{typeIcon}</span>
                              <div className="flex-1 min-w-0">
                                <div className="flex items-center gap-2">
                                  <span className="text-xs font-bold text-zinc-800 dark:text-zinc-200 truncate">{r.title}</span>
                                  <span className={`text-[10px] px-1.5 py-0.5 rounded-md font-bold ${
                                    r.report_type === "morning" ? "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400" :
                                    r.report_type === "evening" ? "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400" :
                                    "bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400"
                                  }`}>{typeLabel}</span>
                                </div>
                                <div className="text-[10px] text-zinc-400 mt-0.5">
                                  {r.report_date} · {r.quality_items} 条优质 · {new Date(r.generated_at).toLocaleTimeString()}
                                </div>
                              </div>
                            </button>
                          );
                        })}
                    </div>
                  </div>
                )}

                {dailyReport ? (
                  <Card className="border border-indigo-500/10 bg-white dark:bg-[#1c1c1e] shadow-md rounded-2xl overflow-hidden">
                    <CardHeader className="border-b border-black/5 dark:border-white/5 p-6 bg-indigo-50/20 dark:bg-indigo-950/10">
                      <CardTitle className="text-lg font-extrabold text-indigo-950 dark:text-indigo-200 flex items-center gap-2">
                        <span>
                          {dailyReport.report_type === "morning" ? "🌅" : dailyReport.report_type === "evening" ? "🌙" : "📰"}
                        </span>
                        {dailyReport.title}
                      </CardTitle>
                      <CardDescription className="text-xs text-indigo-600/80 dark:text-indigo-400/80 flex flex-wrap gap-x-4 gap-y-1 mt-1">
                        <span>生成时间: {new Date(dailyReport.generated_at).toLocaleString()}</span>
                        <span>使用模型: {dailyReport.model_used}</span>
                      </CardDescription>
                    </CardHeader>
                    <CardContent className="p-6 md:p-8 space-y-6">
                      {/* Summary Badges */}
                      <div className="grid grid-cols-3 gap-4 border-b border-black/5 dark:border-white/5 pb-6">
                        <div className="bg-zinc-50 dark:bg-zinc-900/40 p-3 rounded-xl">
                          <span className="text-[10px] text-zinc-400 dark:text-zinc-500 font-bold block uppercase">全天处理</span>
                          <span className="text-base font-black">{dailyReport.total_items} 条</span>
                        </div>
                        <div className="bg-zinc-50 dark:bg-zinc-900/40 p-3 rounded-xl">
                          <span className="text-[10px] text-zinc-400 dark:text-zinc-500 font-bold block uppercase">入选优质</span>
                          <span className="text-base font-black text-indigo-600 dark:text-indigo-400">{dailyReport.quality_items} 条</span>
                        </div>
                        <div className="bg-zinc-50 dark:bg-zinc-900/40 p-3 rounded-xl">
                          <span className="text-[10px] text-zinc-400 dark:text-zinc-500 font-bold block uppercase">质量占比</span>
                          <span className="text-base font-black">
                            {dailyReport.total_items > 0 
                              ? ((dailyReport.quality_items / dailyReport.total_items) * 100).toFixed(1) 
                              : 0}%
                          </span>
                        </div>
                      </div>
                      
                      {/* Rendered daily report content */}
                      {(() => {
                        try {
                          let rawContent = (dailyReport?.content || "").trim();
                          if (rawContent.startsWith("```json")) {
                            rawContent = rawContent.substring(7);
                            if (rawContent.endsWith("```")) {
                              rawContent = rawContent.substring(0, rawContent.length - 3);
                            }
                            rawContent = rawContent.trim();
                          } else if (rawContent.startsWith("```")) {
                            rawContent = rawContent.substring(3);
                            if (rawContent.endsWith("```")) {
                              rawContent = rawContent.substring(0, rawContent.length - 3);
                            }
                            rawContent = rawContent.trim();
                          }
                          
                          // Check if it contains JSON
                          const startIdx = rawContent.indexOf("{");
                          const endIdx = rawContent.lastIndexOf("}");
                          if (startIdx !== -1 && endIdx !== -1 && endIdx > startIdx) {
                            const jsonCandidate = rawContent.substring(startIdx, endIdx + 1);
                            const reportData = JSON.parse(jsonCandidate);
                            if (reportData && reportData.sections) {
                              return <JsonDailyReportView reportData={reportData} />;
                            }
                          }
                        } catch (e) {
                          console.error("Failed to parse report content as JSON", e);
                        }

                        // Fallback to HTML/markdown
                        return (
                          <div 
                            className="prose dark:prose-invert max-w-none text-sm leading-relaxed text-zinc-800 dark:text-zinc-200"
                            dangerouslySetInnerHTML={{ __html: dailyReportHtml || `<pre className="whitespace-pre-wrap font-sans text-sm">${dailyReport.content}</pre>` }}
                          />
                        );
                      })()}
                    </CardContent>
                  </Card>
                ) : (
                  <div className="flex-1 flex flex-col items-center justify-center text-zinc-400 py-32 bg-white dark:bg-[#1c1c1e] rounded-2xl border border-black/5 dark:border-white/5">
                    <Calendar className="w-12 h-12 mb-2 stroke-1 text-zinc-300" />
                    <p className="text-sm font-medium text-zinc-600 dark:text-zinc-400">该日期暂无日报内容</p>
                    <p className="text-xs text-zinc-500 mt-1 text-center px-4">您可以选择该日期或今日，点击右上角 "生成日报" 进行生成</p>
                  </div>
                )}
              </div>
            </div>
          )}

        </main>
      </div>

      {/* --- ADD/EDIT SOURCE DIALOG --- */}
      <Dialog open={isSourceDialogOpen} onOpenChange={setIsSourceDialogOpen}>
        <DialogContent className="sm:max-w-[500px]">
          <form onSubmit={handleSaveSource}>
            <DialogHeader>
              <DialogTitle className="text-base font-bold">
                {editingSource ? "编辑订阅数据源" : "添加新订阅数据源"}
              </DialogTitle>
              <DialogDescription className="text-xs">
                配置 RSS Feed、JSON API 或指定 Chrome Extension 网页提取规则。
              </DialogDescription>
            </DialogHeader>

            <div className="space-y-4 py-4">
              {formError && (
                <div className="bg-rose-50 dark:bg-rose-950/20 text-rose-600 dark:text-rose-400 text-xs p-3 rounded-lg border border-rose-200 dark:border-rose-800">
                  {formError}
                </div>
              )}

              {/* ID (Unique field, disabled when editing) */}
              <div className="space-y-1">
                <label className="text-xs font-bold text-zinc-500">唯一标识 ID (英文/拼音)*</label>
                <Input
                  disabled={!!editingSource}
                  placeholder="如: hackernews, techcrunch"
                  value={sourceForm.id}
                  onChange={e => setSourceForm(prev => ({ ...prev, id: e.target.value }))}
                  className="h-9 text-xs"
                />
              </div>

              {/* Name */}
              <div className="space-y-1">
                <label className="text-xs font-bold text-zinc-500">显示名称 Name*</label>
                <Input
                  placeholder="如: Hacker News"
                  value={sourceForm.name}
                  onChange={e => setSourceForm(prev => ({ ...prev, name: e.target.value }))}
                  className="h-9 text-xs"
                />
              </div>

              {/* Topic Category */}
              <div className="space-y-1">
                <label className="text-xs font-bold text-zinc-500">主题分类 Topic Category* (如: AI, 财经新闻, 科技新闻, 国际新闻, 国内新闻)</label>
                <Input
                  placeholder="如: AI"
                  value={sourceForm.category}
                  onChange={e => setSourceForm(prev => ({ ...prev, category: e.target.value }))}
                  className="h-9 text-xs"
                />
              </div>

              {/* Source Type */}
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-1">
                  <label className="text-xs font-bold text-zinc-500">抓取类型 Type*</label>
                  <Select
                    value={sourceForm.type}
                    onValueChange={val => setSourceForm(prev => ({ ...prev, type: val }))}
                  >
                    <SelectTrigger className="h-9 text-xs">
                      <SelectValue placeholder="选择类型" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="rss">RSS Feed</SelectItem>
                      <SelectItem value="api">JSON API</SelectItem>
                      <SelectItem value="web_scrape">网页爬虫 (Extension)</SelectItem>
                    </SelectContent>
                  </Select>
                </div>

                <div className="space-y-1">
                  <label className="text-xs font-bold text-zinc-500">默认分类 Category*</label>
                  <Select
                    value={sourceForm.default_category}
                    onValueChange={val => setSourceForm(prev => ({ ...prev, default_category: val }))}
                  >
                    <SelectTrigger className="h-9 text-xs">
                      <SelectValue placeholder="默认分类" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="auto">自动识别 (Auto)</SelectItem>
                      <SelectItem value="article">文章 (Article)</SelectItem>
                      <SelectItem value="tweet">推特 (Tweet)</SelectItem>
                      <SelectItem value="paper">论文 (Paper)</SelectItem>
                      <SelectItem value="project">项目 (Project)</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>

              {/* URL */}
              <div className="space-y-1">
                <label className="text-xs font-bold text-zinc-500">入口 URL 地址*</label>
                <Input
                  placeholder="https://example.com/rss.xml"
                  value={sourceForm.url}
                  onChange={e => setSourceForm(prev => ({ ...prev, url: e.target.value }))}
                  className="h-9 text-xs"
                />
              </div>

              {/* Schedule (Cron rule) */}
              <div className="space-y-1">
                <label className="text-xs font-bold text-zinc-500">定时调度 Cron 表达式*</label>
                <Input
                  placeholder="如: 0 */2 * * * (每2小时) 或 0 9 * * * (每天早9点)"
                  value={sourceForm.schedule}
                  onChange={e => setSourceForm(prev => ({ ...prev, schedule: e.target.value }))}
                  className="h-9 text-xs font-mono"
                />
              </div>

              {/* Config Area */}
              <div className="space-y-1">
                <label className="text-xs font-bold text-zinc-500">高级数据解析 JSON 配置</label>
                <textarea
                  placeholder="{}"
                  value={sourceForm.config}
                  onChange={e => setSourceForm(prev => ({ ...prev, config: e.target.value }))}
                  className="w-full h-20 text-xs font-mono p-2 border border-zinc-200 dark:border-zinc-800 rounded-md bg-transparent resize-none"
                />
              </div>
            </div>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setIsSourceDialogOpen(false)} className="h-8 text-xs">
                取消
              </Button>
              <Button type="submit" className="bg-blue-600 hover:bg-blue-700 text-white h-8 text-xs">
                保存
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  );
}
