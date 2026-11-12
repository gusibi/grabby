import { useEffect, useMemo, useRef } from "react";
import { ArrowRight, ChevronLeft, ChevronRight, Inbox, Search, SlidersHorizontal, Sparkles, Star } from "lucide-react";
import { stripHtml, formatTimeAgo } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import { useCopy } from "@/lib/copy";
import { categoryLabel, itemsCopy } from "./copy";
import { ScoreRing } from "@/components/ui/Settings";
import type { AICategory, ScrapedItem } from "@/types";

interface GridViewProps {
  items: ScrapedItem[];
  selectedAICategory: string;
  isShowOnlyAIQuality: boolean;
  setSelectedAICategory: (value: string) => void;
  setIsShowOnlyAIQuality: (value: boolean) => void;
  aiCategories: AICategory[];
  searchQuery: string;
  setSearchQuery: (value: string) => void;
  setCurrentView: (value: import("@/types").AppView) => void;
  handleSelectItem: (item: ScrapedItem) => void;
  toggleStar: (item: ScrapedItem, e?: React.MouseEvent) => void;
  hasMore: boolean;
  currentPage: number;
  readItemIds: Set<number>;
  handlePreviousPage: () => void;
  handleNextPage: () => void;
  isLoadingItems: boolean;
}

const avatarColors = ["#2F6B45", "#8A5A2B", "#3A5B8C", "#7A3F63", "#5B5B8C", "#2F6B6B"];

function sourceColor(name: string) {
  let hash = 0;
  for (let i = 0; i < name.length; i++) hash = (hash * 31 + name.charCodeAt(i)) | 0;
  return avatarColors[Math.abs(hash) % avatarColors.length];
}

function Card({
  item,
  featured,
  isRead,
  zh,
  onOpen,
  onStar,
  index,
}: {
  item: ScrapedItem;
  featured: boolean;
  isRead: boolean;
  zh: boolean;
  onOpen: () => void;
  onStar: (e: React.MouseEvent) => void;
  index: number;
}) {
  const c = useCopy(itemsCopy);
  const summary = stripHtml(item.ai_summary || item.summary || "");
  const source = item.origin_source || c.unknownSource;
  const score = item.quality_score ?? 0;

  return (
    <article
      role="button"
      tabIndex={0}
      onClick={onOpen}
      onKeyDown={(e) => e.key === "Enter" && onOpen()}
      className={`group relative flex flex-col cursor-pointer overflow-hidden rounded-2xl transition-all duration-500 g-rise ${featured ? "md:col-span-2" : ""}`}
      style={{
        animationDelay: `${Math.min(index, 11) * 40}ms`,
        background: featured ? "var(--g-panel-2)" : "var(--g-panel)",
        border: `1px solid ${featured ? "var(--g-line-strong)" : "var(--g-line)"}`,
        boxShadow: "var(--g-shadow)",
        opacity: isRead ? 0.72 : 1,
      }}
      onMouseEnter={(e) => {
        e.currentTarget.style.transform = "translateY(-4px)";
        e.currentTarget.style.boxShadow = "var(--g-shadow-lift)";
        e.currentTarget.style.borderColor = "var(--g-line-strong)";
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.transform = "";
        e.currentTarget.style.boxShadow = "var(--g-shadow)";
        e.currentTarget.style.borderColor = "var(--g-line)";
      }}
    >
      <span
        className="absolute top-0 left-0 right-0 h-[2px] origin-left scale-x-0 group-hover:scale-x-100 transition-transform duration-500"
        style={{ background: "var(--g-signal)", transitionTimingFunction: "cubic-bezier(0.16,1,0.3,1)" }}
      />

      <div className={`flex flex-col flex-1 p-5 ${featured ? "md:p-7" : ""}`}>
        {/* meta */}
        <div className="flex items-start justify-between gap-3">
          <div className="flex items-center gap-1.5 flex-wrap">
            {featured && (
              <span className="g-chip !h-[20px] !text-[9px] g-mono tracking-[0.14em]" style={{ background: "var(--g-signal)", color: "var(--g-on-signal)", borderColor: "transparent" }}>
                {c.lead}
              </span>
            )}
            <span className="g-chip !h-[20px] !text-[9.5px]" style={{ background: "var(--g-signal-soft)", color: "var(--g-signal)", borderColor: "color-mix(in srgb, var(--g-signal) 22%, transparent)" }}>
              {categoryLabel(c, item.category)}
            </span>
            {item.source_category && <span className="g-chip !h-[20px] !text-[9.5px]">{item.source_category}</span>}
            {item.ai_category && <span className="g-chip !h-[20px] !text-[9.5px]">{item.ai_category}</span>}
          </div>
          <div className="flex items-center gap-2 shrink-0">
            <span className="g-mono text-[9.5px]" style={{ color: "var(--g-faint)" }}>
              {formatTimeAgo(item.published_at || item.fetched_at, zh ? "zh" : "en")}
            </span>
            <button
              className="g-icon-btn !w-6 !h-6"
              onClick={onStar}
              title={c.star}
              aria-label={c.star}
            >
              <Star size={13} className={item.starred === 1 ? "fill-current" : ""} style={{ color: item.starred === 1 ? "#e5a83b" : "var(--g-faint)" }} />
            </button>
          </div>
        </div>

        {/* title */}
        <h3
          className={`g-serif mt-4 leading-[1.28] tracking-[-0.015em] group-hover:opacity-80 transition-opacity ${featured ? "text-[24px] md:text-[28px]" : "text-[17px]"}`}
          style={{ color: "var(--g-ink)", display: "-webkit-box", WebkitLineClamp: featured ? 3 : 3, WebkitBoxOrient: "vertical", overflow: "hidden" }}
        >
          {item.title}
        </h3>

        {/* summary */}
        <p
          className={`mt-3 leading-relaxed ${featured ? "text-[13px]" : "text-[12px]"}`}
          style={{ color: "var(--g-muted)", display: "-webkit-box", WebkitLineClamp: featured ? 3 : 3, WebkitBoxOrient: "vertical", overflow: "hidden" }}
        >
          {summary || c.noSummary}
        </p>

        {/* AI summary callout */}
        {item.ai_summary && (
          <div
            className="mt-4 rounded-lg px-3 py-2.5 flex items-start gap-2"
            style={{ background: "var(--g-panel-2)", borderLeft: "2px solid var(--g-signal)" }}
          >
            <Sparkles size={12} className="mt-0.5 shrink-0" style={{ color: "var(--g-signal)" }} />
            <p className="text-[11px] leading-relaxed line-clamp-2" style={{ color: "var(--g-muted)" }}>
              {stripHtml(item.ai_summary)}
            </p>
          </div>
        )}

        {/* footer */}
        <div className="mt-auto pt-5 flex items-center justify-between gap-3">
          <div className="flex items-center gap-2 min-w-0">
            <span
              className="w-5 h-5 rounded-md flex items-center justify-center shrink-0 text-[10px] font-bold text-white"
              style={{ background: sourceColor(source) }}
            >
              {source[0]}
            </span>
            <span className="text-[10.5px] font-medium truncate" style={{ color: "var(--g-muted)" }}>{source}</span>
          </div>
          <div className="flex items-center gap-2.5 shrink-0">
            {score > 0 ? <ScoreRing score={score} /> : null}
            <ArrowRight size={14} className="transition-transform duration-300 group-hover:translate-x-1" style={{ color: "var(--g-faint)" }} />
          </div>
        </div>
      </div>
    </article>
  );
}

export function GridView({
  items,
  selectedAICategory,
  isShowOnlyAIQuality,
  setSelectedAICategory,
  setIsShowOnlyAIQuality,
  aiCategories,
  searchQuery,
  setSearchQuery,
  handleSelectItem,
  toggleStar,
  hasMore,
  currentPage,
  readItemIds,
  handlePreviousPage,
  handleNextPage,
  isLoadingItems,
}: GridViewProps) {
  const { t, language } = useTranslation();
  const c = useCopy(itemsCopy);
  const zh = language === "zh";
  const searchRef = useRef<HTMLInputElement>(null);

  // Keyboard: "/" focuses the search field anywhere in the feed.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      if (target && ["INPUT", "TEXTAREA", "SELECT"].includes(target.tagName)) return;
      if (e.key === "/") {
        e.preventDefault();
        searchRef.current?.focus();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const dateLabel = useMemo(
    () => new Intl.DateTimeFormat(zh ? "zh-CN" : "en-US", { year: "numeric", month: "long", day: "numeric", weekday: "long" }).format(new Date()),
    [zh]
  );

  const totalAI = useMemo(() => aiCategories.reduce((sum, c) => sum + c.count, 0), [aiCategories]);

  return (
    <div className="flex-1 flex flex-col overflow-hidden" style={{ background: "var(--g-paper)" }}>
      {/* Masthead */}
      <div className="shrink-0 px-8 pt-7">
        <div className="flex flex-wrap items-end justify-between gap-5">
          <div>
            <div className="g-eyebrow">The daily feed · {dateLabel}</div>
            <h1 className="g-display mt-2.5 text-[clamp(2rem,3.4vw,2.9rem)]">{c.title}</h1>
            <p className="mt-2 text-[12px]" style={{ color: "var(--g-muted)" }}>
              {c.lede(totalAI || "—")}
            </p>
          </div>
          <div className="flex items-center gap-6 pb-1">
            <div>
              <div className="g-eyebrow">{c.articles}</div>
              <div className="g-mono text-[20px] font-semibold mt-1" style={{ color: "var(--g-ink)" }}>{items.length}</div>
            </div>
            <span className="w-px h-8" style={{ background: "var(--g-line)" }} />
            <div>
              <div className="g-eyebrow">{c.page}</div>
              <div className="g-mono text-[20px] font-semibold mt-1" style={{ color: "var(--g-ink)" }}>{String(currentPage + 1).padStart(2, "0")}</div>
            </div>
          </div>
        </div>

        {/* Filters */}
        <div className="mt-6 flex flex-wrap items-center justify-between gap-3 pb-3.5" style={{ borderBottom: "1px solid var(--g-line)" }}>
          <div className="flex items-center gap-1.5 overflow-x-auto g-scroll max-w-full">
            <button
              onClick={() => { setSelectedAICategory("all"); setIsShowOnlyAIQuality(false); }}
              className="g-chip !h-[28px] !px-3 !text-[11px] shrink-0 transition-all"
              style={{
                background: selectedAICategory === "all" && !isShowOnlyAIQuality ? "var(--g-signal)" : "var(--g-panel)",
                color: selectedAICategory === "all" && !isShowOnlyAIQuality ? "var(--g-on-signal)" : "var(--g-muted)",
                borderColor: selectedAICategory === "all" && !isShowOnlyAIQuality ? "var(--g-signal)" : "var(--g-line)",
              }}
            >
              {t("nav.all")}
            </button>
            {aiCategories.filter((cat) => cat.name).map((cat) => {
              const active = selectedAICategory === cat.name && !isShowOnlyAIQuality;
              return (
                <button
                  key={cat.name}
                  onClick={() => { setSelectedAICategory(cat.name); setIsShowOnlyAIQuality(false); }}
                  className="g-chip !h-[28px] !px-3 !text-[11px] shrink-0 transition-all"
                  style={{
                    background: active ? "var(--g-signal)" : "var(--g-panel)",
                    color: active ? "var(--g-on-signal)" : "var(--g-muted)",
                    borderColor: active ? "var(--g-signal)" : "var(--g-line)",
                  }}
                >
                  {cat.name} <span className="g-mono !text-[9px] opacity-65">{cat.count}</span>
                </button>
              );
            })}
          </div>

          <div className="flex items-center gap-2 shrink-0">
            <button
              onClick={() => setIsShowOnlyAIQuality(!isShowOnlyAIQuality)}
              className="g-chip !h-[28px] !px-2.5 !text-[10.5px] gap-1.5 transition-all"
              style={{
                background: isShowOnlyAIQuality ? "var(--g-signal-soft)" : "var(--g-panel)",
                color: isShowOnlyAIQuality ? "var(--g-signal)" : "var(--g-muted)",
                borderColor: isShowOnlyAIQuality ? "color-mix(in srgb, var(--g-signal) 30%, transparent)" : "var(--g-line)",
              }}
              title={c.highValueHint}
            >
              <SlidersHorizontal size={11} /> {c.highValue}
            </button>
            <div className="relative w-[210px]">
              <Search size={13} className="absolute left-2.5 top-1/2 -translate-y-1/2" style={{ color: "var(--g-faint)" }} />
              <input
                ref={searchRef}
                className="g-input !h-[30px] !pl-8 !pr-8 !text-[11.5px]"
                placeholder={t("grid.searchPlaceholder")}
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                aria-label={t("grid.searchPlaceholder")}
              />
              <span className="g-kbd absolute right-2 top-1/2 -translate-y-1/2 !h-[15px] !text-[9px] pointer-events-none">/</span>
            </div>
          </div>
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto g-scroll px-8 py-6">
        {isLoadingItems ? (
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
            {Array.from({ length: 6 }).map((_, i) => (
              <div key={i} className="rounded-2xl p-5" style={{ border: "1px solid var(--g-line)", background: "var(--g-panel)" }}>
                <div className="g-skeleton h-4 w-20" />
                <div className="g-skeleton h-5 w-full mt-5" />
                <div className="g-skeleton h-5 w-4/5 mt-2" />
                <div className="g-skeleton h-3 w-full mt-5" />
                <div className="g-skeleton h-3 w-3/4 mt-2" />
                <div className="g-skeleton h-6 w-24 mt-8" />
              </div>
            ))}
          </div>
        ) : items.length === 0 ? (
          <div className="h-[46vh] flex flex-col items-center justify-center text-center g-fade">
            <span className="w-14 h-14 rounded-2xl flex items-center justify-center" style={{ background: "var(--g-panel-2)", color: "var(--g-faint)" }}>
              <Inbox size={24} strokeWidth={1.3} />
            </span>
            <h3 className="g-serif text-[20px] mt-5">{c.emptyTitle}</h3>
            <p className="mt-2 text-[12px] max-w-xs" style={{ color: "var(--g-muted)" }}>
              {c.emptyBody}
            </p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4 items-stretch">
            {items.map((item, index) => (
              <Card
                key={item.id}
                item={item}
                featured={index === 0 && currentPage === 0 && !searchQuery}
                isRead={readItemIds.has(item.id)}
                zh={zh}
                index={index}
                onOpen={() => handleSelectItem(item)}
                onStar={(e) => { e.stopPropagation(); toggleStar(item, e); }}
              />
            ))}
          </div>
        )}

        {items.length > 0 && !isLoadingItems && (
          <div className="flex items-center justify-center gap-4 mt-9 pb-4">
            <button className="g-btn g-btn-secondary g-btn-sm" onClick={handlePreviousPage} disabled={isLoadingItems || currentPage === 0}>
              <ChevronLeft size={13} /> {t("grid.prev")}
            </button>
            <span className="g-mono text-[11px]" style={{ color: "var(--g-faint)" }}>
              {String(currentPage + 1).padStart(2, "0")} / —
            </span>
            <button className="g-btn g-btn-secondary g-btn-sm" onClick={handleNextPage} disabled={isLoadingItems || !hasMore}>
              {t("grid.next")} <ChevronRight size={13} />
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
