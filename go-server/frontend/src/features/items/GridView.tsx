import { ChevronLeft, ChevronRight, Inbox, Search, Star, Sparkles } from "lucide-react";
import { Button } from "@astryxdesign/core/Button";
import { TextInput } from "@astryxdesign/core/TextInput";
import { ClickableCard } from "@astryxdesign/core/ClickableCard";
import { Badge } from "@astryxdesign/core/Badge";
import { Spinner } from "@astryxdesign/core/Spinner";
import { VStack, HStack } from "@astryxdesign/core/Layout";
import { getCategoryLabel } from "@/lib/category";
import { formatTimeAgo } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
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
  isLoadingItems
}: GridViewProps) {
  const { t, language } = useTranslation();

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      {/* Search & Filter header */}
      <div className="p-4 bg-surface border-b border-border flex flex-wrap gap-4 items-center justify-between">
        <HStack gap={2} vAlign="center" className="overflow-x-auto py-1">
          <Button
            variant={selectedAICategory === "all" && !isShowOnlyAIQuality ? "primary" : "secondary"}
            onClick={() => { setSelectedAICategory("all"); setIsShowOnlyAIQuality(false); }}
            size="sm"
            label={t("nav.all")}
          />
          {aiCategories.map((cat) => (
            <Button
              key={cat.name}
              variant={selectedAICategory === cat.name ? "primary" : "secondary"}
              onClick={() => { setSelectedAICategory(cat.name); setIsShowOnlyAIQuality(false); }}
              size="sm"
              label={`${cat.name} (${cat.count})`}
            />
          ))}
        </HStack>

        <div className="w-full max-w-xs">
          <TextInput
            placeholder={t("grid.searchPlaceholder")}
            value={searchQuery}
            onChange={setSearchQuery}
            startIcon={<Search className="h-4 w-4 text-zinc-400" />}
            label={language === "zh" ? "搜索" : "Search"}
            isLabelHidden
          />
        </div>
      </div>

      {/* Main content grid */}
      <div className="flex-1 overflow-y-auto bg-body p-6">
        {items.length === 0 ? (
          <VStack gap={3} hAlign="center" vAlign="center" className="h-[50vh] text-zinc-400">
            <Inbox className="w-12 h-12 stroke-1" />
            <p className="text-sm">
              {language === "zh" 
                ? "暂无内容，请检查订阅源或点击右上角抓取数据" 
                : "No content, please check subscription sources or click Scrape Now in the header."}
            </p>
          </VStack>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 max-w-6xl mx-auto">
            {items.map(item => {
              const isRead = readItemIds.has(item.id);
              return (
                <ClickableCard 
                  key={item.id} 
                  label={item.title}
                  onClick={() => handleSelectItem(item)}
                  variant={isRead ? "muted" : "default"}
                  className="flex flex-col h-full overflow-hidden"
                >
                  <VStack gap={3} className="h-full flex-1">
                    {/* Header metadata */}
                    <div className="flex justify-between items-start gap-2 w-full">
                      <HStack gap={1.5} vAlign="center" className="flex-wrap">
                        <Badge variant="info" label={getCategoryLabel(item.category)} />
                        {item.source_category && (
                          <Badge variant="neutral" label={item.source_category} />
                        )}
                        {item.quality_score !== undefined && item.quality_score > 0 && (
                          <Badge 
                            variant={item.quality_score >= 8 ? "success" : "info"} 
                            label={language === "zh" ? `AI ${item.quality_score}分` : `AI ${item.quality_score}`} 
                          />
                        )}
                      </HStack>
                      <HStack gap={1.5} vAlign="center">
                        <span className="text-[10px] text-zinc-400 font-medium">{formatTimeAgo(item.published_at)}</span>
                        <Button
                          size="sm"
                          variant="ghost"
                          isIconOnly
                          label={t("grid.star")}
                          icon={<Star className={`w-3.5 h-3.5 ${item.starred === 1 ? "fill-amber-400 text-amber-400" : "text-zinc-400"}`} />}
                          onClick={(e) => { e.stopPropagation(); toggleStar(item, e); }}
                        />
                      </HStack>
                    </div>

                    {/* Title */}
                    <h3 className="text-sm font-bold leading-snug line-clamp-2 mt-2">
                      {item.title}
                    </h3>

                    {/* Summary */}
                    <div className="flex-1">
                      {item.ai_summary ? (
                        <div className="text-xs bg-indigo-50/30 dark:bg-indigo-950/5 border border-indigo-500/5 rounded-lg p-2.5 space-y-1">
                          <p className="text-[10px] font-bold text-indigo-600 dark:text-indigo-400 uppercase tracking-wider flex items-center gap-1">
                            <Sparkles className="w-3 h-3" />
                            {t("item.summary")} {item.ai_category ? `· ${item.ai_category}` : ""}
                          </p>
                          <p className="text-zinc-600 dark:text-zinc-400 leading-relaxed line-clamp-3">
                            {item.ai_summary}
                          </p>
                        </div>
                      ) : (
                        <p className="text-xs text-zinc-500 line-clamp-3 leading-relaxed">
                          {item.summary || (language === "zh" ? "未抓取到描述摘要，点击以进入详情阅读全文。" : "No description captured. Click to read the full article.")}
                        </p>
                      )}
                    </div>

                    {/* Footer */}
                    <div className="pt-2 border-t border-border flex items-center justify-between text-[10px] w-full">
                      <HStack gap={1.5} vAlign="center" className="text-zinc-400">
                        <div className="w-4 h-4 rounded-full bg-blue-100 dark:bg-blue-900/30 flex items-center justify-center text-[8px] font-bold text-blue-600 dark:text-blue-400">
                          {item.origin_source ? item.origin_source[0] : "?"}
                        </div>
                        <span className="font-semibold uppercase tracking-tight line-clamp-1 max-w-[120px]">{item.origin_source || (language === "zh" ? "未知出处" : "Unknown Source")}</span>
                      </HStack>
                    </div>
                  </VStack>
                </ClickableCard>
              );
            })}
          </div>
        )}
        
        {/* Pagination buttons */}
        {items.length > 0 && (
          <HStack gap={3} hAlign="center" vAlign="center" className="mt-8 pb-12">
            <Button
              variant="secondary"
              size="sm"
              onClick={handlePreviousPage}
              isDisabled={isLoadingItems || currentPage === 0}
              label={t("grid.prev")}
              icon={<ChevronLeft className="w-3.5 h-3.5" />}
            />
            <span className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">
              {language === "zh" ? `第 ${currentPage + 1} 页` : `Page ${currentPage + 1}`}
            </span>
            <Button
              variant="secondary"
              size="sm"
              onClick={handleNextPage}
              isDisabled={isLoadingItems || !hasMore}
              label={t("grid.next")}
              icon={isLoadingItems ? <Spinner size="sm" /> : <ChevronRight className="w-3.5 h-3.5" />}
            />
          </HStack>
        )}
      </div>
    </div>
  );
}
