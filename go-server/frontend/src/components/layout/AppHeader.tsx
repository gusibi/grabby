import { RefreshCw, Wifi, WifiOff, Plus, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

interface AppHeaderProps {
  currentView: import("@/types").AppView;
  browserConnected: boolean;
  selectedReadStatus: string;
  setSelectedReadStatus: (value: string) => void;
  openAddSourceDialog: () => void;
  handleScrapeAllEnabled: () => void;
  isScrapingAll: boolean;
}

export function AppHeader({
  currentView,
  browserConnected,
  selectedReadStatus,
  setSelectedReadStatus,
  openAddSourceDialog,
  handleScrapeAllEnabled,
  isScrapingAll
}: AppHeaderProps) {
  return (
          <header className="h-14 flex items-center justify-between px-6 border-b border-black/5 dark:border-white/5 bg-white/80 dark:bg-[#1c1c1e]/80 backdrop-blur-md sticky top-0 z-10 shrink-0">
            <div className="flex items-center gap-3">
              <h2 className="text-lg font-bold tracking-tight">
                {currentView === "grid" && (selectedReadStatus === "unread" ? "阅读列表 Inbox" : "聚合发现 Discovery")}
                {currentView === "settings" && "订阅数据源 Settings"}
                {currentView === "ai-settings" && "AI 模型配置 Settings"}
                {currentView === "logs" && "抓取执行日志 Logs"}
                {currentView === "daily" && "AI 智能日报 Daily"}
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
              {currentView === "grid" && (
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

              {currentView === "settings" && (
                <Button onClick={openAddSourceDialog} size="sm" className="bg-blue-600 hover:bg-blue-700 text-white font-medium gap-1.5 h-8 text-xs">
                  <Plus className="w-3.5 h-3.5" /> 添加数据源
                </Button>
              )}

              {currentView === "grid" && (
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

          
  );
}
