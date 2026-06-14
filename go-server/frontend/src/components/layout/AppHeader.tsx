import { RefreshCw, Wifi, WifiOff, Plus, Loader2, LogOut } from "lucide-react";
import { Button } from "@/components/ui/button";

interface AppHeaderProps {
  currentView: import("@/types").AppView;
  browserConnected: boolean;
  openAddSourceDialog: () => void;
  handleScrapeAllEnabled: () => void;
  isScrapingAll: boolean;
  authRequired: boolean;
  isAuthenticated: boolean;
  onLogout: () => void;
}

export function AppHeader({
  currentView,
  browserConnected,
  openAddSourceDialog,
  handleScrapeAllEnabled,
  isScrapingAll,
  authRequired,
  isAuthenticated,
  onLogout
}: AppHeaderProps) {
  return (
          <header className="h-14 flex items-center justify-between px-6 border-b border-black/5 dark:border-white/5 bg-white/80 dark:bg-[#1c1c1e]/80 backdrop-blur-md sticky top-0 z-10 shrink-0">
            <div className="flex items-center gap-3">
              <h2 className="text-lg font-bold tracking-tight">
                {currentView === "grid" && "聚合发现 Discovery"}
                {currentView === "settings" && "订阅数据源 Settings"}
                {currentView === "ai-settings" && "AI 模型配置 Settings"}
                {currentView === "logs" && "抓取执行日志 Logs"}
                {currentView === "daily" && "AI 智能日报 Daily"}
                {currentView === "device" && "设备与连接设置 Device"}
              </h2>
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
              {currentView === "settings" && isAuthenticated && (
                <Button onClick={openAddSourceDialog} size="sm" className="bg-blue-600 hover:bg-blue-700 text-white font-medium gap-1.5 h-8 text-xs">
                  <Plus className="w-3.5 h-3.5" /> 添加数据源
                </Button>
              )}

              {currentView === "grid" && isAuthenticated && (
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

              {authRequired && isAuthenticated && (
                <Button
                  onClick={onLogout}
                  size="sm"
                  variant="outline"
                  className="h-8 text-xs gap-1.5 border-zinc-200 dark:border-zinc-800"
                >
                  <LogOut className="w-3.5 h-3.5" />
                  退出
                </Button>
              )}
            </div>
          </header>

          
  );
}
