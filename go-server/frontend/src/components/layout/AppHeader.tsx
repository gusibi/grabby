import { RefreshCw, Wifi, WifiOff, Plus, Loader2, LogOut } from "lucide-react";
import { Button } from "@astryxdesign/core/Button";
import { Heading } from "@astryxdesign/core/Text";
import { useTranslation } from "@/lib/i18n";

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
  const { t, language } = useTranslation();

  return (
    <header className="h-14 flex items-center justify-between px-6 border-b border-border bg-body sticky top-0 z-10 shrink-0">
      <div className="flex items-center gap-3">
        <Heading level={2}>
          {currentView === "grid" && t("title.daily").split(" ")[0]}
          {currentView === "settings" && t("title.sources")}
          {currentView === "ai-settings" && t("title.aiSettings")}
          {currentView === "logs" && t("title.logs")}
          {currentView === "daily" && t("title.daily")}
          {currentView === "device" && t("title.device")}
          {currentView === "captures-extract" && t("title.extract")}
          {currentView === "captures-twitter" && t("title.twitter")}
        </Heading>
        <div className={`flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-medium transition-all ${
          browserConnected 
            ? "bg-success/10 text-success" 
            : "bg-warning/10 text-warning"
        }`}>
          {browserConnected ? <Wifi className="w-3 h-3" /> : <WifiOff className="w-3 h-3" />}
          {browserConnected 
            ? (language === "zh" ? "插件已连接" : "Plugin Connected") 
            : (language === "zh" ? "插件未连接" : "Plugin Offline")}
        </div>
      </div>

      <div className="flex items-center gap-3">
        {currentView === "settings" && isAuthenticated && (
          <Button 
            onClick={openAddSourceDialog} 
            size="sm" 
            variant="primary"
            label={t("sources.add")}
            icon={<Plus className="w-3.5 h-3.5" />}
          />
        )}

        {currentView === "grid" && isAuthenticated && (
          <Button 
            onClick={handleScrapeAllEnabled} 
            isDisabled={isScrapingAll}
            size="sm" 
            variant="primary"
            label={t("btn.scrape")}
            icon={isScrapingAll ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <RefreshCw className="w-3.5 h-3.5" />}
          />
        )}

        {authRequired && isAuthenticated && (
          <Button
            onClick={onLogout}
            size="sm"
            variant="secondary"
            label={t("nav.logout")}
            icon={<LogOut className="w-3.5 h-3.5" />}
          />
        )}
      </div>
    </header>
  );
}
