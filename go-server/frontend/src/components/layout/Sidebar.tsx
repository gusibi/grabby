import { SideNav, SideNavSection, SideNavHeading, SideNavItem } from "@astryxdesign/core/SideNav";
import { LayoutGrid, Settings, FileText, Moon, Sun, Sparkles, Laptop, Globe, AtSign, Palette, Languages } from "lucide-react";
import type { AppView, Stats } from "@/types";
import { useTranslation } from "@/lib/i18n";

const themeNames: Record<string, string> = {
  stone: "石板暖色 Stone",
  matcha: "抹茶浅绿 Matcha",
  gothic: "哥特暗影 Gothic",
  neutral: "中性灰蓝 Neutral"
};

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
  themeId: string;
  onChangeTheme: (theme: string) => void;
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
  themeId,
  onChangeTheme,
  setIsSidebarCollapsed,
  authRequired,
  isAuthenticated
}: SidebarProps) {
  const canAccessAdmin = !authRequired || isAuthenticated;
  const { language, setLanguage, t } = useTranslation();

  return (
    <SideNav
      header={<SideNavHeading heading={!isSidebarCollapsed ? "GRABBY PANELS" : "G"} />}
      collapsible={{
        isCollapsed: isSidebarCollapsed,
        onCollapsedChange: setIsSidebarCollapsed,
        hasButton: true
      }}
      footer={
        <>
          {canAccessAdmin && (
            <SideNavSection title={!isSidebarCollapsed ? t("nav.admin") : ""}>
              <SideNavItem
                label={t("nav.sources")}
                icon={<Settings className="w-4 h-4" />}
                isSelected={currentView === "settings"}
                onClick={() => setCurrentView("settings")}
              />
              <SideNavItem
                label={t("nav.settings")}
                icon={<Sparkles className="w-4 h-4" />}
                isSelected={currentView === "ai-settings"}
                onClick={() => setCurrentView("ai-settings")}
              />
              <SideNavItem
                label={t("nav.logs")}
                icon={<FileText className="w-4 h-4" />}
                isSelected={currentView === "logs"}
                onClick={() => {
                  setCurrentView("logs");
                  fetchLogs();
                }}
              />
              <SideNavItem
                label={t("nav.devices")}
                icon={<Laptop className="w-4 h-4" />}
                isSelected={currentView === "device"}
                onClick={() => setCurrentView("device")}
              />
            </SideNavSection>
          )}
          <SideNavSection title="" isHeaderHidden>
            <SideNavItem
              label={!isSidebarCollapsed ? `${t("nav.theme")}: ${themeNames[themeId] || themeId}` : ""}
              icon={<Palette className="w-4 h-4" />}
              collapsible={{defaultIsCollapsed: true}}
            >
              <SideNavItem
                label="石板暖色 Stone"
                isSelected={themeId === "stone"}
                onClick={() => onChangeTheme("stone")}
              />
              <SideNavItem
                label="抹茶浅绿 Matcha"
                isSelected={themeId === "matcha"}
                onClick={() => onChangeTheme("matcha")}
              />
              <SideNavItem
                label="哥特暗影 Gothic"
                isSelected={themeId === "gothic"}
                onClick={() => onChangeTheme("gothic")}
              />
              <SideNavItem
                label="中性灰蓝 Neutral"
                isSelected={themeId === "neutral"}
                onClick={() => onChangeTheme("neutral")}
              />
            </SideNavItem>
            <SideNavItem
              label={!isSidebarCollapsed ? `语言: ${language === "zh" ? "中文" : "English"}` : ""}
              icon={<Languages className="w-4 h-4" />}
              collapsible={{defaultIsCollapsed: true}}
            >
              <SideNavItem
                label="简体中文 (ZH)"
                isSelected={language === "zh"}
                onClick={() => setLanguage("zh")}
              />
              <SideNavItem
                label="English (EN)"
                isSelected={language === "en"}
                onClick={() => setLanguage("en")}
              />
            </SideNavItem>
            <SideNavItem
              label={darkMode ? t("nav.lightMode") : t("nav.darkMode")}
              icon={darkMode ? <Sun className="w-4 h-4" /> : <Moon className="w-4 h-4" />}
              onClick={toggleDarkMode}
            />
          </SideNavSection>
        </>
      }
    >
      <SideNavSection title="">
        <SideNavItem
          label={t("nav.grid")}
          icon={<LayoutGrid className="w-4 h-4" />}
          isSelected={currentView === "grid"}
          onClick={() => setCurrentView("grid")}
        />
        <SideNavItem
          label={t("nav.daily")}
          icon={<Sparkles className="w-4 h-4" />}
          isSelected={currentView === "daily"}
          onClick={() => {
            setCurrentView("daily");
            fetchDailyReport();
            fetchReportList();
          }}
        />
      </SideNavSection>

      <SideNavSection title={!isSidebarCollapsed ? t("nav.captures") : ""}>
        <SideNavItem
          label={t("nav.extract")}
          icon={<Globe className="w-4 h-4" />}
          isSelected={currentView === "captures-extract"}
          onClick={() => setCurrentView("captures-extract")}
        />
        <SideNavItem
          label={t("nav.twitter")}
          icon={<AtSign className="w-4 h-4" />}
          isSelected={currentView === "captures-twitter"}
          onClick={() => setCurrentView("captures-twitter")}
        />
      </SideNavSection>

      {!isSidebarCollapsed && (
        <SideNavSection title={t("nav.category")}>
          <SideNavItem
            label={t("nav.all")}
            endContent={<span className="text-[10px] text-muted-foreground">{stats.total_count}</span>}
            isSelected={selectedSourceCategory === "all" && currentView === "grid"}
            onClick={() => {
              setSelectedSourceCategory("all");
              if (currentView !== "grid") setCurrentView("grid");
            }}
          />
          {(stats.source_categories || []).map((cat) => (
            <SideNavItem
              key={cat}
              label={cat}
              endContent={<span className="text-[10px] text-muted-foreground">{stats.source_category_unread?.[cat] || 0}</span>}
              isSelected={selectedSourceCategory === cat && currentView === "grid"}
              onClick={() => {
                setSelectedSourceCategory(cat);
                if (currentView !== "grid") setCurrentView("grid");
              }}
            />
          ))}
        </SideNavSection>
      )}
    </SideNav>
  );
}
