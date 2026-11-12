import { useMemo } from "react";
import { Loader2, LogOut, Plus, RefreshCw, Wifi, WifiOff } from "lucide-react";
import { useTranslation } from "@/lib/i18n";
import { useCopy } from "@/lib/copy";
import { shellCopy } from "./copy";
import type { AppView } from "@/types";

interface AppHeaderProps {
  currentView: AppView;
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
  onLogout,
}: AppHeaderProps) {
  const { language } = useTranslation();
  const zh = language === "zh";
  const c = useCopy(shellCopy);
  const meta = c.titles[currentView] ?? c.titles.grid;

  const dateLabel = useMemo(
    () =>
      new Intl.DateTimeFormat(zh ? "zh-CN" : "en-US", {
        month: "long",
        day: "numeric",
        weekday: "long",
      }).format(new Date()),
    [zh]
  );

  return (
    <header
      className="h-[62px] shrink-0 flex items-center justify-between gap-4 px-7 relative"
      style={{ background: "var(--g-paper)", borderBottom: "1px solid var(--g-line)" }}
    >
      <div className="flex items-baseline gap-3.5 min-w-0">
        <span className="g-eyebrow hidden sm:inline">{meta.kicker}</span>
        <h1 key={currentView} className="g-rise text-[17px] font-semibold tracking-[-0.025em] truncate">
          {meta.title}
        </h1>
        <span className="hidden md:inline g-rule !w-px !h-3.5 self-center" />
        <span className="hidden md:inline text-[11px] whitespace-nowrap" style={{ color: "var(--g-faint)" }}>
          {dateLabel}
        </span>
      </div>

      <div className="flex items-center gap-2.5 shrink-0">
        <span
          className="hidden sm:inline-flex items-center gap-1.5 h-[26px] px-2.5 rounded-full text-[10.5px] font-medium"
          style={{
            background: browserConnected ? "var(--g-signal-soft)" : "var(--g-panel-2)",
            color: browserConnected ? "var(--g-signal)" : "var(--g-faint)",
            border: `1px solid ${browserConnected ? "color-mix(in srgb, var(--g-signal) 24%, transparent)" : "var(--g-line)"}`,
          }}
        >
          <span className="relative flex h-1.5 w-1.5">
            {browserConnected && (
              <span className="absolute inline-flex h-full w-full rounded-full g-pulse" style={{ background: "var(--g-signal)", opacity: 0.45 }} />
            )}
            <span className="relative inline-flex rounded-full h-1.5 w-1.5" style={{ background: browserConnected ? "var(--g-signal)" : "var(--g-faint)" }} />
          </span>
          {browserConnected ? c.pluginConnected : c.pluginOffline}
          {browserConnected ? <Wifi size={11} /> : <WifiOff size={11} />}
        </span>

        {currentView === "settings" && isAuthenticated && (
          <button className="g-btn g-btn-primary g-btn-sm" onClick={openAddSourceDialog}>
            <Plus size={13} /> {c.addSource}
          </button>
        )}

        {currentView === "grid" && isAuthenticated && (
          <button className="g-btn g-btn-primary g-btn-sm" onClick={handleScrapeAllEnabled} disabled={isScrapingAll}>
            {isScrapingAll ? <Loader2 size={13} className="g-spin" /> : <RefreshCw size={13} />}
            {c.scrapeNow}
          </button>
        )}

        {authRequired && isAuthenticated && (
          <button className="g-icon-btn" title={c.logOut} onClick={onLogout}>
            <LogOut size={14} />
          </button>
        )}
      </div>
    </header>
  );
}
