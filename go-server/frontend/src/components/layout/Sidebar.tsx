import { LayoutGrid, Settings, FileText, ChevronLeft, ChevronRight, Moon, Sun, Sparkles, Laptop, Globe, AtSign } from "lucide-react";
import type { AppView, Stats } from "@/types";

interface SidebarProps {
  isSidebarCollapsed: boolean;
  currentView: AppView;
  setCurrentView: (view: AppView) => void;
  stats: Stats;
  fetchDailyReport: () => void;
  fetchReportList: () => void;
  fetchLogs: () => void;
  selectedSourceCategory: string;
  setSelectedSourceCategory: (value: string) => void;
  toggleDarkMode: () => void;
  darkMode: boolean;
  setIsSidebarCollapsed: (value: boolean) => void;
  authRequired: boolean;
  isAuthenticated: boolean;
}

export function Sidebar({
  isSidebarCollapsed,
  currentView,
  setCurrentView,
  stats,
  fetchDailyReport,
  fetchReportList,
  fetchLogs,
  selectedSourceCategory,
  setSelectedSourceCategory,
  toggleDarkMode,
  darkMode,
  setIsSidebarCollapsed,
  authRequired,
  isAuthenticated
}: SidebarProps) {
  const canAccessAdmin = !authRequired || isAuthenticated;

  return (
        <aside className={`${isSidebarCollapsed ? "w-16" : "w-64"} sidebar-vibrancy flex flex-col shrink-0 transition-all duration-300`}>
          <div className="h-14 flex items-center px-6 shrink-0 border-b border-black/5 dark:border-white/5">
            <h1 className="font-extrabold text-sm tracking-wider bg-gradient-to-r from-blue-600 to-indigo-500 bg-clip-text text-transparent">
              {!isSidebarCollapsed ? "GRABBY PANELS" : "G"}
            </h1>
          </div>

          <div className="flex-1 px-3 py-4 space-y-6 overflow-y-auto">
            <nav className="space-y-1">
              <button
                onClick={() => {
                  setCurrentView("grid");
                }}
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
            </nav>

            <div className="pt-4 border-t border-black/5 dark:border-white/5">
              {!isSidebarCollapsed && (
                <h3 className="px-3 py-2 text-[10px] font-bold text-zinc-400 dark:text-zinc-500 uppercase tracking-wider">
                  抓取记录 Captures
                </h3>
              )}
              <nav className="space-y-1">
                <button
                  onClick={() => setCurrentView("captures-extract")}
                  title="网页提取记录"
                  className={`w-full flex items-center gap-3 px-3 py-2 rounded-xl text-sm transition-all font-medium ${
                    currentView === "captures-extract"
                      ? "bg-blue-600 text-white shadow-md font-semibold"
                      : "text-zinc-600 dark:text-zinc-400 hover:bg-black/5 dark:hover:bg-white/5"
                  }`}
                >
                  <Globe className="w-4 h-4" />
                  {!isSidebarCollapsed && <span>网页提取 Extract</span>}
                </button>
                <button
                  onClick={() => setCurrentView("captures-twitter")}
                  title="推文归档"
                  className={`w-full flex items-center gap-3 px-3 py-2 rounded-xl text-sm transition-all font-medium ${
                    currentView === "captures-twitter"
                      ? "bg-blue-600 text-white shadow-md font-semibold"
                      : "text-zinc-600 dark:text-zinc-400 hover:bg-black/5 dark:hover:bg-white/5"
                  }`}
                >
                  <AtSign className="w-4 h-4" />
                  {!isSidebarCollapsed && <span>推文归档 Twitter</span>}
                </button>
              </nav>
            </div>

            {!isSidebarCollapsed && (
              <div className="pt-4 border-t border-black/5 dark:border-white/5">
                <h3 className="px-3 py-2 text-[10px] font-bold text-zinc-400 dark:text-zinc-500 uppercase tracking-wider">
                  分类筛选 Topic Category
                </h3>
                <nav className="space-y-1">
                  <button
                    onClick={() => { setSelectedSourceCategory("all"); if (currentView !== "grid") setCurrentView("grid"); }}
                    className={`w-full flex items-center justify-between px-3 py-1.5 rounded-lg text-xs hover:bg-black/5 dark:hover:bg-white/5 transition-all ${selectedSourceCategory === "all" ? "text-blue-500 font-semibold" : "text-zinc-600 dark:text-zinc-400"}`}
                  >
                    <span>全部 All</span>
                    <span>{stats.total_count}</span>
                  </button>
                  {(stats.source_categories || []).map((cat) => (
                    <button
                      key={cat}
                      onClick={() => { setSelectedSourceCategory(cat); if (currentView !== "grid") setCurrentView("grid"); }}
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

          <div className="p-3 border-t border-black/5 dark:border-white/5 space-y-1.5 shrink-0">
            {canAccessAdmin && (
              <>
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

                <button
                  onClick={() => {
                    setCurrentView("logs");
                    fetchLogs();
                  }}
                  title="抓取日志"
                  className={`w-full flex items-center gap-3 px-3 py-2 rounded-xl text-xs font-semibold transition-all ${
                    currentView === "logs"
                      ? "bg-blue-600 text-white shadow-sm font-semibold"
                      : "text-zinc-500 dark:text-zinc-400 hover:bg-black/5 dark:hover:bg-white/5"
                  }`}
                >
                  <FileText className="w-4 h-4" />
                  {!isSidebarCollapsed && <span>抓取日志 Logs</span>}
                </button>

                <button
                  onClick={() => setCurrentView("device")}
                  title="设备连接状态"
                  className={`w-full flex items-center gap-3 px-3 py-2 rounded-xl text-xs font-semibold transition-all ${
                    currentView === "device"
                      ? "bg-blue-600 text-white shadow-sm font-semibold"
                      : "text-zinc-500 dark:text-zinc-400 hover:bg-black/5 dark:hover:bg-white/5"
                  }`}
                >
                  <Laptop className="w-4 h-4" />
                  {!isSidebarCollapsed && <span>设备与连接 Devices</span>}
                </button>
              </>
            )}

            <div className={`flex ${isSidebarCollapsed ? "flex-col" : "flex-row"} gap-2 pt-1.5`}>
              <button
                onClick={toggleDarkMode}
                title={darkMode ? "切换亮色模式" : "切换暗色模式"}
                className={`flex-1 flex items-center justify-center p-2 text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-300 transition-colors bg-black/5 dark:bg-white/5 rounded-xl ${isSidebarCollapsed ? "h-9 w-full" : ""}`}
              >
                {darkMode ? <Sun className="w-4 h-4" /> : <Moon className="w-4 h-4" />}
                {!isSidebarCollapsed && <span className="text-[10px] ml-1.5 font-medium">{darkMode ? "亮色" : "暗色"}</span>}
              </button>
              <button
                onClick={() => setIsSidebarCollapsed(!isSidebarCollapsed)}
                title={isSidebarCollapsed ? "展开侧边栏" : "收起侧边栏"}
                className={`flex-1 flex items-center justify-center p-2 text-zinc-400 hover:text-zinc-600 dark:hover:text-zinc-300 transition-colors bg-black/5 dark:bg-white/5 rounded-xl ${isSidebarCollapsed ? "h-9 w-full" : ""}`}
              >
                {isSidebarCollapsed ? <ChevronRight className="w-4 h-4" /> : <ChevronLeft className="w-4 h-4" />}
                {!isSidebarCollapsed && <span className="text-[10px] ml-1.5 font-medium">折叠</span>}
              </button>
            </div>
          </div>
        </aside>

        
  );
}
