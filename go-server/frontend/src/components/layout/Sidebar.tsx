import { useMemo } from "react";
import type { ReactNode } from "react";
import {
  AtSign,
  ChevronLeft,
  ChevronRight,
  FileText,
  Globe,
  LayoutGrid,
  Laptop,
  Moon,
  Settings,
  Sparkles,
  Sun,
} from "lucide-react";
import type { AppView, Stats } from "@/types";
import { useTranslation } from "@/lib/i18n";
import { useCopy } from "@/lib/copy";
import { shellCopy } from "./copy";

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

interface NavItemProps {
  icon?: ReactNode;
  label: string;
  active?: boolean;
  count?: number | string;
  collapsed: boolean;
  onClick: () => void;
  indent?: boolean;
}

function NavItem({ icon, label, active, count, collapsed, onClick, indent }: NavItemProps) {
  return (
    <button
      onClick={onClick}
      title={collapsed ? label : undefined}
      className="group relative w-full flex items-center gap-2.5 h-[34px] rounded-lg text-left transition-all duration-200"
      style={{
        paddingLeft: indent ? 27 : 10,
        paddingRight: 10,
        background: active ? "var(--g-signal-soft)" : "transparent",
        color: active ? "var(--g-ink)" : "var(--g-muted)",
      }}
    >
      {!active && (
        <span className="absolute inset-0 rounded-lg opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none" style={{ background: "var(--g-panel-2)" }} />
      )}
      {active && (
        <span
          className="absolute left-0 top-1/2 -translate-y-1/2 w-[2px] h-4 rounded-full z-10"
          style={{ background: "var(--g-signal)" }}
        />
      )}
      {icon && (
        <span
          className="shrink-0 flex items-center justify-center w-4 relative z-10"
          style={{ color: active ? "var(--g-signal)" : "var(--g-muted)" }}
        >
          {icon}
        </span>
      )}
      {!collapsed && (
        <>
          <span className="flex-1 min-w-0 text-[12px] font-medium truncate relative z-10">{label}</span>
          {count !== undefined && (
            <span className="g-mono text-[10px] shrink-0 relative z-10" style={{ color: active ? "var(--g-signal)" : "var(--g-faint)" }}>
              {count}
            </span>
          )}
        </>
      )}
    </button>
  );
}

function NavGroup({ label, collapsed, children }: { label: string; collapsed: boolean; children: ReactNode }) {
  return (
    <div className="mt-5 first:mt-0">
      {!collapsed && <div className="g-eyebrow px-2.5 mb-2">{label}</div>}
      {collapsed && <div className="g-rule mx-2 mb-2" />}
      <div className="space-y-0.5">{children}</div>
    </div>
  );
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
  isAuthenticated,
}: SidebarProps) {
  const { language, setLanguage, t } = useTranslation();
  const c = useCopy(shellCopy);
  const collapsed = isSidebarCollapsed;
  const canAccessAdmin = !authRequired || isAuthenticated;

  const categories = useMemo(() => stats.source_categories || [], [stats.source_categories]);

  return (
    <aside
      className="flex flex-col h-full g-scroll"
      style={{
        width: collapsed ? 64 : 232,
        background: "var(--g-panel)",
        borderRight: "1px solid var(--g-line)",
        transition: "width 320ms cubic-bezier(0.16, 1, 0.3, 1)",
      }}
    >
      {/* Brand */}
      <div className="h-[62px] shrink-0 flex items-center gap-2.5 px-3.5" style={{ borderBottom: "1px solid var(--g-line)" }}>
        <img
          src="/logo-32.png"
          alt=""
          aria-hidden="true"
          className="w-8 h-8 rounded-[10px] shrink-0 object-cover"
        />
        {!collapsed && (
          <div className="min-w-0 g-fade">
            <div className="text-[13px] font-semibold tracking-[-0.02em] leading-none">grabby</div>
            <div className="g-mono text-[9px] mt-1 tracking-[0.08em]" style={{ color: "var(--g-faint)" }}>
              {c.brandTagline}
            </div>
          </div>
        )}
      </div>

      {/* Navigation */}
      <nav className="flex-1 overflow-y-auto g-scroll px-2.5 py-4">
        <NavGroup label={c.groupRead} collapsed={collapsed}>
          <NavItem
            icon={<LayoutGrid size={15} />}
            label={t("nav.grid")}
            active={currentView === "grid"}
            count={stats.total_count}
            collapsed={collapsed}
            onClick={() => setCurrentView("grid")}
          />
          <NavItem
            icon={<Sparkles size={15} />}
            label={t("nav.daily")}
            active={currentView === "daily"}
            collapsed={collapsed}
            onClick={() => {
              setCurrentView("daily");
              fetchDailyReport();
              fetchReportList();
            }}
          />
        </NavGroup>

        <NavGroup label={c.groupCapture} collapsed={collapsed}>
          <NavItem
            icon={<Globe size={15} />}
            label={t("nav.extract")}
            active={currentView === "captures-extract"}
            collapsed={collapsed}
            onClick={() => setCurrentView("captures-extract")}
          />
          <NavItem
            icon={<AtSign size={15} />}
            label={t("nav.twitter")}
            active={currentView === "captures-twitter"}
            collapsed={collapsed}
            onClick={() => setCurrentView("captures-twitter")}
          />
        </NavGroup>

        {canAccessAdmin && (
          <NavGroup label={c.groupAdmin} collapsed={collapsed}>
            <NavItem
              icon={<Settings size={15} />}
              label={c.navSources}
              active={currentView === "settings"}
              collapsed={collapsed}
              onClick={() => setCurrentView("settings")}
            />
            <NavItem
              icon={<Sparkles size={15} />}
              label={c.navModel}
              active={currentView === "ai-settings"}
              collapsed={collapsed}
              onClick={() => setCurrentView("ai-settings")}
            />
            <NavItem
              icon={<FileText size={15} />}
              label={c.navLogs}
              active={currentView === "logs"}
              collapsed={collapsed}
              onClick={() => {
                setCurrentView("logs");
                fetchLogs();
              }}
            />
            <NavItem
              icon={<Laptop size={15} />}
              label={c.navDevice}
              active={currentView === "device"}
              collapsed={collapsed}
              onClick={() => setCurrentView("device")}
            />
          </NavGroup>
        )}

        {!collapsed && categories.length > 0 && (
          <NavGroup label={c.groupCategory} collapsed={collapsed}>
            <NavItem
              label={t("nav.all")}
              active={selectedSourceCategory === "all" && currentView === "grid"}
              count={stats.total_count}
              collapsed={collapsed}
              onClick={() => {
                setSelectedSourceCategory("all");
                if (currentView !== "grid") setCurrentView("grid");
              }}
            />
            {categories.map((cat) => (
              <NavItem
                key={cat}
                indent
                label={cat}
                active={selectedSourceCategory === cat && currentView === "grid"}
                count={stats.source_category_unread?.[cat] ?? 0}
                collapsed={collapsed}
                onClick={() => {
                  setSelectedSourceCategory(cat);
                  if (currentView !== "grid") setCurrentView("grid");
                }}
              />
            ))}
          </NavGroup>
        )}
      </nav>

      {/* Footer */}
      <div className="shrink-0 px-2.5 py-3" style={{ borderTop: "1px solid var(--g-line)" }}>
        <div className={`flex items-center ${collapsed ? "flex-col gap-1.5" : "gap-1.5"}`}>
          <button
            className="g-btn g-btn-ghost g-btn-sm !px-2 flex-1"
            onClick={toggleDarkMode}
            title={darkMode ? c.lightMode : c.darkMode}
          >
            {darkMode ? <Sun size={14} /> : <Moon size={14} />}
            {!collapsed && <span>{darkMode ? c.light : c.dark}</span>}
          </button>
          <button
            className="g-btn g-btn-ghost g-btn-sm !px-2 flex-1 g-mono"
            onClick={() => setLanguage(language === "zh" ? "en" : "zh")}
            title={c.switchLanguage}
          >
            {c.zhShort}
            {!collapsed && <span className="!font-sans">{c.zhName}</span>}
          </button>
          <button
            className="g-icon-btn shrink-0"
            onClick={() => setIsSidebarCollapsed(!collapsed)}
            title={collapsed ? c.expandSidebar : c.collapseSidebar}
          >
            {collapsed ? <ChevronRight size={14} /> : <ChevronLeft size={14} />}
          </button>
        </div>
        {!collapsed && (
          <div className="g-mono text-[9px] mt-3 px-1.5 tracking-[0.08em]" style={{ color: "var(--g-faint)" }}>
            {c.footer}
          </div>
        )}
      </div>
    </aside>
  );
}
