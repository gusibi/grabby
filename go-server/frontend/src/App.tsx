/* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */
import { useState, useEffect, useRef } from "react";
import { Theme } from "@astryxdesign/core/theme";
import { stoneTheme } from "@astryxdesign/theme-stone/built";
import { neutralTheme } from "@astryxdesign/theme-neutral/built";
import { matchaTheme } from "@astryxdesign/theme-matcha/built";
import { gothicTheme } from "@astryxdesign/theme-gothic/built";
import { AppShell } from "@astryxdesign/core/AppShell";
import { Layout } from "@astryxdesign/core/Layout";
import { Sidebar } from "@/components/layout/Sidebar";
import { AppHeader } from "@/components/layout/AppHeader";
import { GridView } from "@/features/items/GridView";
import { ItemDetailModal } from "@/features/items/ItemDetailModal";
import { SourcesView } from "@/features/sources/SourcesView";
import { SourceDialog } from "@/features/sources/SourceDialog";
import { AISettingsView } from "@/features/ai-settings/AISettingsView";
import { LogsView } from "@/features/logs/LogsView";
import { DailyReportView } from "@/features/daily-report/DailyReportView";
import { DeviceSettingsView } from "@/features/device/DeviceSettingsView";
import { ExtractCapturesView } from "@/features/captures/ExtractCapturesView";
import { TwitterCapturesView } from "@/features/captures/TwitterCapturesView";
import { AuthDialog } from "@/features/auth/AuthDialog";
import { api } from "@/lib/api";
import type { AICategory, AIProviderProfile, AppView, DailyReport, FetchLog, ReportListItem, ScrapedItem, Source, SourceForm, Stats } from "@/types";

function getLocalDateString(d: Date = new Date()): string {
  const year = d.getFullYear();
  const month = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

const ITEMS_PAGE_SIZE = 18;
const READ_ITEMS_STORAGE_KEY = "grabby_read_item_ids";

function loadReadItemIds(): Set<number> {
  try {
    const raw = localStorage.getItem(READ_ITEMS_STORAGE_KEY);
    const ids = raw ? JSON.parse(raw) : [];
    if (!Array.isArray(ids)) {
      return new Set();
    }
    return new Set(ids.filter((id) => typeof id === "number"));
  } catch {
    return new Set();
  }
}

function saveReadItemIds(ids: Set<number>) {
  localStorage.setItem(READ_ITEMS_STORAGE_KEY, JSON.stringify(Array.from(ids)));
}

export default function App() {
  const [currentView, setCurrentView] = useState<AppView>("grid");
  const [items, setItems] = useState<ScrapedItem[]>([]);
  const [cursor, setCursor] = useState<string>("");
  const [pageCursors, setPageCursors] = useState<string[]>([""]);
  const [currentPage, setCurrentPage] = useState<number>(0);
  const [hasMore, setHasMore] = useState<boolean>(true);
  const [isLoadingItems, setIsLoadingItems] = useState<boolean>(false);
  const [readItemIds, setReadItemIds] = useState<Set<number>>(() => loadReadItemIds());

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
  const [selectedCategory] = useState<string>("all");
  const [selectedSourceCategory, setSelectedSourceCategory] = useState<string>("all");
  const [isSidebarCollapsed, setIsSidebarCollapsed] = useState<boolean>(false);
  const [darkMode, setDarkMode] = useState<boolean>(false);
  const [themeId, setThemeId] = useState<string>(() => localStorage.getItem("grabby_theme") || "stone");

  const themesMap: Record<string, any> = {
    stone: stoneTheme,
    neutral: neutralTheme,
    matcha: matchaTheme,
    gothic: gothicTheme,
  };
  const activeTheme = themesMap[themeId] || stoneTheme;
  const [browserConnected, setBrowserConnected] = useState<boolean>(false);
  const [authRequired, setAuthRequired] = useState<boolean>(false);
  const [isAuthenticated, setIsAuthenticated] = useState<boolean>(false);
  const [isAuthDialogOpen, setIsAuthDialogOpen] = useState<boolean>(false);
  const [isScrapingAll, setIsScrapingAll] = useState<boolean>(false);
  const [isShowOnlyAIQuality, setIsShowOnlyAIQuality] = useState<boolean>(false);
  const [selectedAICategory, setSelectedAICategory] = useState<string>("all");
  const [aiCategories, setAiCategories] = useState<AICategory[]>([]);
  const [dailyReport, setDailyReport] = useState<DailyReport | null>(null);
  const [dailyReportHtml, setDailyReportHtml] = useState<string>("");
  const [selectedReportDate, setSelectedReportDate] = useState<string>(getLocalDateString());
  const [isGeneratingReport, setIsGeneratingReport] = useState<boolean>(false);
  const [reportList, setReportList] = useState<ReportListItem[]>([]);
  const [selectedReportType, setSelectedReportType] = useState<string>("morning");

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
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const [aiTestResult, setAiTestResult] = useState<any | null>(null);
  const [aiRequestsPerMinute, setAiRequestsPerMinute] = useState<number>(10);
  const [morningReportEnabled, setMorningReportEnabled] = useState<boolean>(false);
  const [eveningReportEnabled, setEveningReportEnabled] = useState<boolean>(false);
  const [morningReportTime, setMorningReportTime] = useState<string>("08:30");
  const [eveningReportTime, setEveningReportTime] = useState<string>("22:30");

  const lastSetHashRef = useRef<string>('');

  const isLoginPath = () => window.location.pathname.replace(/\/+$/, "") === "/login";

  // Source Add/Edit modal state
  const [isSourceDialogOpen, setIsSourceDialogOpen] = useState<boolean>(false);
  const [editingSource, setEditingSource] = useState<Source | null>(null);
  const [sourceForm, setSourceForm] = useState<SourceForm>({
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
      const data = await api.getHealth();
      setBrowserConnected(data.browser_connected);
    } catch {
      setBrowserConnected(false);
    }
  };

  const fetchAuthSession = async () => {
    try {
      const data = await api.getAuthSession();
      setAuthRequired(!!data.auth_required);
      setIsAuthenticated(!!data.authenticated);
    } catch (err) {
      console.error("Failed to fetch auth session", err);
      setAuthRequired(true);
      setIsAuthenticated(false);
    }
  };

  const handleLogout = async () => {
    try {
      await api.logout();
    } catch (err) {
      console.error("Failed to logout", err);
    } finally {
      setIsAuthenticated(false);
      fetchAuthSession();
    }
  };

  const fetchStats = async () => {
    try {
      const data = await api.getStats();
      setStats(data);
    } catch (err) {
      console.error("Failed to fetch stats", err);
    }
  };

  const fetchSources = async () => {
    try {
      const data = await api.getSources();
      setSources(data);
    } catch (err) {
      console.error("Failed to fetch sources", err);
    }
  };

  const fetchLogs = async () => {
    try {
      const data = await api.getLogs();
      setLogs(data);
    } catch (err) {
      console.error("Failed to fetch logs", err);
    }
  };

  const fetchAICategories = async () => {
    try {
      const data = await api.getAICategories();
      if (data.success) {
        setAiCategories(data.categories || []);
      }
    } catch (err) {
      console.error("Failed to fetch AI categories", err);
    }
  };

  const fetchDailyReport = async (dateStr?: string, reportType?: string) => {
    const date = dateStr || selectedReportDate;
    const rType = reportType || selectedReportType || "morning";
    try {
      const data = await api.getDailyReport(date, rType);
      if (data.success && data.sections) {
        const reportData = { ...data };
        delete reportData.success;
        setDailyReport({
          ...reportData,
          content: JSON.stringify(reportData),
        } as DailyReport);
        setDailyReportHtml("");
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
      const data = await api.getReportList(30);
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

  const buildProfileFromSettings = (settings: Partial<AIProviderProfile> & { active_profile_id?: string }): AIProviderProfile => ({
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
      const data = await api.getAISettings();
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

      const data = await api.saveAISettings({
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
      });
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
      const data = await api.testAI();
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
      const data = await api.startEvaluation();
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

  const handleGenerateDailyReport = async (reportType?: string) => {
    setIsGeneratingReport(true);
    const rType = reportType || selectedReportType || "morning";
    try {
      const data = await api.generateDailyReport(selectedReportDate, rType);
      if (data.success) {
        alert("早报/晚报生成已在后台启动，生成需要一些时间，完成后将自动加载。");
        // Wait a few seconds to let backend start processing, then fetch updates
        setTimeout(() => {
          fetchStats();
          fetchReportList();
          fetchDailyReport(selectedReportDate, rType);
        }, 3000);
      } else {
        alert("生成失败: " + (data.error || "未知原因"));
      }
    } catch (err) {
      console.error("Failed to generate daily report", err);
      alert("生成日报出现网络错误");
    } finally {
      setIsGeneratingReport(false);
    }
  };

  const fetchItems = async (page = currentPage, cursorForPage = pageCursors[page] || "") => {
    if (isLoadingItems) return;
    setIsLoadingItems(true);

    try {
      let url = `/open/api/items?limit=${ITEMS_PAGE_SIZE}`;
      if (isShowOnlyAIQuality || (selectedAICategory && selectedAICategory !== "all")) {
        url = `/open/api/ai/items?limit=${ITEMS_PAGE_SIZE}`;
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

      if (cursorForPage) {
        url += `&cursor=${cursorForPage}`;
      }

      const data = await api.getItems(url);

      setItems(data.items || []);
      setSelectedItem(null);
      setItemDetailHtml("");
      setCurrentPage(page);
      setCursor(data.cursor || data.next_cursor || "");
      setHasMore(!!(data.cursor || data.next_cursor));
    } catch (err) {
      console.error("Failed to fetch items", err);
    } finally {
      setIsLoadingItems(false);
    }
  };

  const resetItemsPagination = () => {
    setPageCursors([""]);
    setCurrentPage(0);
    setCursor("");
    fetchItems(0, "");
  };

  const handleNextPage = () => {
    if (!cursor || isLoadingItems) return;
    const nextPage = currentPage + 1;
    const nextCursors = [...pageCursors];
    nextCursors[nextPage] = cursor;
    setPageCursors(nextCursors);
    fetchItems(nextPage, cursor);
  };

  const handlePreviousPage = () => {
    if (currentPage === 0 || isLoadingItems) return;
    const previousPage = currentPage - 1;
    fetchItems(previousPage, pageCursors[previousPage] || "");
  };

  const handleSelectItem = async (item: ScrapedItem) => {
    setSelectedItem(item);
    setIsLoadingDetail(true);
    setReadItemIds((prev) => {
      const next = new Set(prev);
      next.add(item.id);
      saveReadItemIds(next);
      return next;
    });
    try {
      const data = await api.getItemDetail(item.id);
      setItemDetailHtml(data.html_content);
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
      await api.setItemStarred(item.id, newStarred);

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

  const handleToggleSourceEnabled = async (source: Source) => {
    const newEnabled = source.enabled === 1 ? 0 : 1;
    try {
      const res = await api.toggleSource(source.id, newEnabled);
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
      const data = await api.runSource(source.id);
      if (data.success) {
        // Refresh sources & items
        fetchSources();
        resetItemsPagination();
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
          api.runSource(s.id).catch(e => console.error(e))
        )
      );
      fetchSources();
      resetItemsPagination();
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
      await api.deleteSource(id);
      setSources(prev => prev.filter(s => s.id !== id));
      fetchStats();
    } catch (err) {
      console.error("Failed to delete source", err);
    }
  };

  const openAddSourceDialog = () => {
    if (authRequired && !isAuthenticated) {
      setIsAuthDialogOpen(true);
      return;
    }
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
      const res = await api.saveSource(sourceForm, editingSource?.id);

      if (!res.ok) {
        const errText = await res.text();
        throw new Error(errText || "Request failed");
      }

      setIsSourceDialogOpen(false);
      fetchSources();
    } catch (err: unknown) {
      setFormError(err instanceof Error ? err.message : "Failed to save data source.");
    }
  };

  // Debounce search query
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(searchQuery);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchQuery]);

  // Initial fetch operations
  useEffect(() => {
    fetchAuthSession();
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

    // Initialize from URL hash
    if (window.location.hash) {
      const rawHash = window.location.hash.replace(/^#/, '');
      const [pathPart, queryPart] = rawHash.split('?');
      const parts = pathPart.replace(/^\//, '').split('/');
      const params = new URLSearchParams(queryPart || '');
      const validViews: AppView[] = ['grid', 'settings', 'ai-settings', 'logs', 'device', 'daily', 'captures-extract', 'captures-twitter'];
      const hashView = validViews.includes(parts[0] as AppView) ? (parts[0] as AppView) : 'grid';
      const hashDate = parts[1];
      const hashType = parts[2];
      lastSetHashRef.current = rawHash;
      setCurrentView(hashView);
      if (hashView === 'daily') {
        if (hashDate) setSelectedReportDate(hashDate);
        if (hashType) setSelectedReportType(hashType);
        fetchDailyReport(hashDate, hashType || 'morning');
        fetchReportList();
      } else if (hashView === 'logs') {
        fetchLogs();
      } else if (hashView === 'grid' && queryPart) {
        const aiCat = params.get('aiCat') || 'all';
        const quality = params.get('quality') === '1';
        const srcCat = params.get('srcCat') || 'all';
        if (aiCat !== 'all') setSelectedAICategory(aiCat);
        if (quality) setIsShowOnlyAIQuality(true);
        if (srcCat !== 'all') setSelectedSourceCategory(srcCat);
      }
    }
    if (isLoginPath()) {
      setIsAuthDialogOpen(true);
    }

    // Health check polling
    const healthInterval = setInterval(checkHealth, 10000);
    return () => clearInterval(healthInterval);
  }, []);

  useEffect(() => {
    const protectedViews: AppView[] = ["settings", "ai-settings", "device", "daily"];
    if (!authRequired || isAuthenticated || !protectedViews.includes(currentView)) {
      return;
    }

    let buffer = "";
    const handleKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      if (target && ["INPUT", "TEXTAREA", "SELECT"].includes(target.tagName)) {
        return;
      }
      if (e.key.length !== 1) {
        return;
      }
      buffer = (buffer + e.key.toLowerCase()).slice(-5);
      if (buffer === "login") {
        setIsAuthDialogOpen(true);
        buffer = "";
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [authRequired, isAuthenticated, currentView]);

  // Sync state → URL hash
  useEffect(() => {
    let hash = `/${currentView}`;
    if (currentView === 'daily') {
      hash += `/${selectedReportDate}/${selectedReportType}`;
    } else if (currentView === 'grid') {
      const params = new URLSearchParams();
      if (selectedAICategory && selectedAICategory !== 'all') params.set('aiCat', selectedAICategory);
      if (isShowOnlyAIQuality) params.set('quality', '1');
      if (selectedSourceCategory && selectedSourceCategory !== 'all') params.set('srcCat', selectedSourceCategory);
      const qs = params.toString();
      if (qs) hash += `?${qs}`;
    }
    if (window.location.hash !== `#${hash}`) {
      lastSetHashRef.current = hash;
      window.location.hash = hash;
    }
  }, [currentView, selectedReportDate, selectedReportType, selectedAICategory, isShowOnlyAIQuality, selectedSourceCategory]);

  // Sync URL hash → state (browser back/forward)
  useEffect(() => {
    const handleHashChange = () => {
      const currentHash = window.location.hash.replace('#', '');
      if (currentHash === lastSetHashRef.current) return;
      const [pathPart, queryPart] = currentHash.split('?');
      const parts = pathPart.replace(/^\//, '').split('/');
      const params = new URLSearchParams(queryPart || '');
      const validViews: AppView[] = ['grid', 'settings', 'ai-settings', 'logs', 'device', 'daily', 'captures-extract', 'captures-twitter'];
      const view = validViews.includes(parts[0] as AppView) ? (parts[0] as AppView) : 'grid';
      const date = parts[1];
      const type = parts[2];
      lastSetHashRef.current = currentHash;
      setCurrentView(view);
      if (view === 'daily') {
        if (date) setSelectedReportDate(date);
        if (type) setSelectedReportType(type);
        fetchDailyReport(date, type || 'morning');
        fetchReportList();
      } else if (view === 'grid') {
        setSelectedAICategory(params.get('aiCat') || 'all');
        setIsShowOnlyAIQuality(params.get('quality') === '1');
        setSelectedSourceCategory(params.get('srcCat') || 'all');
      }
      if (isLoginPath()) {
        setIsAuthDialogOpen(true);
      }
    };
    window.addEventListener('hashchange', handleHashChange);
    return () => window.removeEventListener('hashchange', handleHashChange);
  }, []);

  useEffect(() => {
    resetItemsPagination();
    fetchAICategories();
  }, [debouncedSearch, selectedCategory, selectedSourceCategory, isShowOnlyAIQuality, selectedAICategory]);

  useEffect(() => {
    if (currentView === "settings" || currentView === "ai-settings") {
      fetchAISettings();
    }
  }, [currentView]);

  useEffect(() => {
    const adminViews: AppView[] = ["settings", "ai-settings", "logs", "device", "captures-extract", "captures-twitter"];
    if (authRequired && !isAuthenticated && adminViews.includes(currentView)) {
      setCurrentView("grid");
    }
  }, [authRequired, isAuthenticated, currentView]);


  return (
    <Theme theme={activeTheme} mode={darkMode ? "dark" : "light"}>
      <AppShell
        contentPadding={0}
        sideNav={
          <Sidebar
            isSidebarCollapsed={isSidebarCollapsed}
            currentView={currentView}
            setCurrentView={setCurrentView}
            stats={stats}
            fetchDailyReport={fetchDailyReport}
            fetchReportList={fetchReportList}
            fetchLogs={fetchLogs}
            selectedSourceCategory={selectedSourceCategory}
            setSelectedSourceCategory={setSelectedSourceCategory}
            toggleDarkMode={toggleDarkMode}
            darkMode={darkMode}
            themeId={themeId}
            onChangeTheme={(t) => {
              setThemeId(t);
              localStorage.setItem("grabby_theme", t);
            }}
            setIsSidebarCollapsed={setIsSidebarCollapsed}
            authRequired={authRequired}
            isAuthenticated={isAuthenticated}
          />
        }
      >
        <Layout
          height="fill"
          content={
            <div className="flex-1 flex flex-col overflow-hidden">
              <AppHeader
                currentView={currentView}
                browserConnected={browserConnected}
                openAddSourceDialog={openAddSourceDialog}
                handleScrapeAllEnabled={handleScrapeAllEnabled}
                isScrapingAll={isScrapingAll}
                authRequired={authRequired}
                isAuthenticated={isAuthenticated}
                onLogout={handleLogout}
              />

              <div className="flex-1 flex overflow-hidden">
                {currentView === "grid" && (
                  <GridView
                    items={items}
                    selectedAICategory={selectedAICategory}
                    isShowOnlyAIQuality={isShowOnlyAIQuality}
                    setSelectedAICategory={setSelectedAICategory}
                    setIsShowOnlyAIQuality={setIsShowOnlyAIQuality}
                    aiCategories={aiCategories}
                    searchQuery={searchQuery}
                    setSearchQuery={setSearchQuery}
                    setCurrentView={setCurrentView}
                    handleSelectItem={handleSelectItem}
                    toggleStar={toggleStar}
                    hasMore={hasMore}
                    currentPage={currentPage}
                    readItemIds={readItemIds}
                    handlePreviousPage={handlePreviousPage}
                    handleNextPage={handleNextPage}
                    isLoadingItems={isLoadingItems}
                  />
                )}

                <ItemDetailModal
                  item={selectedItem}
                  isOpen={selectedItem !== null}
                  onClose={() => setSelectedItem(null)}
                  isLoadingDetail={isLoadingDetail}
                  itemDetailHtml={itemDetailHtml}
                  toggleStar={toggleStar}
                />

                {currentView === "settings" && (
                  <SourcesView
                    sources={sources}
                    handleToggleSourceEnabled={handleToggleSourceEnabled}
                    handleRunSource={handleRunSource}
                    openEditSourceDialog={openEditSourceDialog}
                    handleDeleteSource={handleDeleteSource}
                    isAuthenticated={isAuthenticated}
                  />
                )}

                {currentView === "ai-settings" && (
                  <AISettingsView
                    isLoadingSettings={isLoadingSettings}
                    handleStartEvaluation={handleStartEvaluation}
                    aiEnabled={aiEnabled}
                    setAiEnabled={setAiEnabled}
                    activeProfileId={activeProfileId}
                    handleSelectAIProfile={handleSelectAIProfile}
                    aiProfiles={aiProfiles}
                    handleAddAIProfile={handleAddAIProfile}
                    handleDeleteAIProfile={handleDeleteAIProfile}
                    aiStrategy={aiStrategy}
                    setAiStrategy={setAiStrategy}
                    setAiProfiles={setAiProfiles}
                    aiProfileName={aiProfileName}
                    handleProfileNameChange={handleProfileNameChange}
                    aiProvider={aiProvider}
                    setAiProvider={setAiProvider}
                    aiQualityThreshold={aiQualityThreshold}
                    setAiQualityThreshold={setAiQualityThreshold}
                    aiModel={aiModel}
                    setAiModel={setAiModel}
                    aiApiKey={aiApiKey}
                    setAiApiKey={setAiApiKey}
                    aiRequestsPerMinute={aiRequestsPerMinute}
                    setAiRequestsPerMinute={setAiRequestsPerMinute}
                    aiBaseUrl={aiBaseUrl}
                    setAiBaseUrl={setAiBaseUrl}
                    aiSystemPrompt={aiSystemPrompt}
                    setAiSystemPrompt={setAiSystemPrompt}
                    aiDailyPrompt={aiDailyPrompt}
                    setAiDailyPrompt={setAiDailyPrompt}
                    morningReportEnabled={morningReportEnabled}
                    setMorningReportEnabled={setMorningReportEnabled}
                    morningReportTime={morningReportTime}
                    setMorningReportTime={setMorningReportTime}
                    eveningReportEnabled={eveningReportEnabled}
                    setEveningReportEnabled={setEveningReportEnabled}
                    eveningReportTime={eveningReportTime}
                    setEveningReportTime={setEveningReportTime}
                    aiTestResult={aiTestResult}
                    handleTestAI={handleTestAI}
                    isTestingAI={isTestingAI}
                    isSavingSettings={isSavingSettings}
                    saveAISettings={saveAISettings}
                    isAuthenticated={isAuthenticated}
                  />
                )}

                {currentView === "logs" && (
                  <LogsView fetchLogs={fetchLogs} logs={logs} />
                )}

                {currentView === "daily" && (
                  <DailyReportView
                    dailyReport={dailyReport}
                    dailyReportHtml={dailyReportHtml}
                    selectedReportDate={selectedReportDate}
                    setSelectedReportDate={setSelectedReportDate}
                    fetchDailyReport={fetchDailyReport}
                    handleGenerateDailyReport={handleGenerateDailyReport}
                    isGeneratingReport={isGeneratingReport}
                    reportList={reportList}
                    selectedReportType={selectedReportType}
                    setSelectedReportType={setSelectedReportType}
                    isAuthenticated={isAuthenticated}
                  />
                )}

                {currentView === "device" && (
                  <DeviceSettingsView isAuthenticated={isAuthenticated} />
                )}

                {currentView === "captures-extract" && (
                  <ExtractCapturesView />
                )}

                {currentView === "captures-twitter" && (
                  <TwitterCapturesView />
                )}
              </div>
            </div>
          }
        />

        <SourceDialog
          isSourceDialogOpen={isSourceDialogOpen}
          setIsSourceDialogOpen={setIsSourceDialogOpen}
          handleSaveSource={handleSaveSource}
          editingSource={editingSource}
          formError={formError}
          sourceForm={sourceForm}
          setSourceForm={setSourceForm}
        />
        <AuthDialog
          open={isAuthDialogOpen}
          onOpenChange={setIsAuthDialogOpen}
          onAuthenticated={() => {
            fetchAuthSession();
            if (isLoginPath()) {
              window.history.replaceState(null, "", "/#/grid");
            }
          }}
        />
      </AppShell>
    </Theme>
  );
}
