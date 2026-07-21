import { ExternalLink, Star } from "lucide-react";
import { Dialog, DialogHeader } from "@astryxdesign/core/Dialog";
import { Button } from "@astryxdesign/core/Button";
import { Badge } from "@astryxdesign/core/Badge";
import { Spinner } from "@astryxdesign/core/Spinner";
import { VStack, HStack } from "@astryxdesign/core/Layout";
import { getCategoryLabel } from "@/lib/category";
import { useTranslation } from "@/lib/i18n";
import type { ScrapedItem } from "@/types";

interface ItemDetailModalProps {
  item: ScrapedItem | null;
  isOpen: boolean;
  onClose: () => void;
  isLoadingDetail: boolean;
  itemDetailHtml: string;
  toggleStar: (item: ScrapedItem, e?: React.MouseEvent) => void;
}

export function ItemDetailModal({
  item,
  isOpen,
  onClose,
  isLoadingDetail,
  itemDetailHtml,
  toggleStar,
}: ItemDetailModalProps) {
  const { t, language } = useTranslation();
  if (!item) return null;

  return (
    <Dialog 
      isOpen={isOpen} 
      onOpenChange={onClose}
      purpose="info"
      width={900}
      maxHeight="90vh"
    >
      <div className="flex flex-col h-full overflow-hidden">
        {/* Header Action Bar */}
        <DialogHeader
          title={item.title}
          subtitle={item.origin_source ? `${item.origin_source} • ${language === "zh" ? "发布于:" : "Published at:"} ${item.published_at ? new Date(item.published_at).toLocaleString() : (language === "zh" ? "未知时间" : "Unknown Time")}` : ""}
          onOpenChange={onClose}
          hasDivider
          endContent={
            <HStack gap={2} vAlign="center">
              <Button
                size="sm"
                variant={item.starred === 1 ? "primary" : "secondary"}
                label={item.starred === 1 ? (language === "zh" ? "已收藏" : "Starred") : t("grid.star")}
                icon={<Star className={`w-4 h-4 ${item.starred === 1 ? "fill-amber-400 text-amber-400" : ""}`} />}
                onClick={(e) => toggleStar(item, e)}
              />
              <Button
                size="sm"
                variant="ghost"
                label={t("item.original")}
                icon={<ExternalLink className="w-4 h-4" />}
                href={item.url}
                target="_blank"
                rel="noreferrer"
              />
            </HStack>
          }
        />

        {/* Scrollable Reader Area */}
        <div className="flex-1 overflow-y-auto p-6 md:p-10">
          <VStack gap={6} className="max-w-3xl mx-auto">
            
            {/* Category Info */}
            <HStack gap={2} vAlign="center">
              <Badge variant="info" label={getCategoryLabel(item.category)} />
              {item.source_category && (
                <Badge variant="neutral" label={item.source_category} />
              )}
            </HStack>

            {/* AI Summary and Analysis */}
            {item.quality_score !== undefined && item.quality_score > 0 ? (
              <div className="bg-muted border-l-4 border-indigo-500 p-4 rounded-r-xl space-y-3">
                <div className="flex justify-between items-center">
                  <h5 className="text-xs font-bold text-indigo-600 dark:text-indigo-400 uppercase tracking-widest">
                    {language === "zh" ? "AI 智能深度分析 AI Insights" : "AI Insights & Deep Analysis"}
                  </h5>
                  <Badge variant="success" label={language === "zh" ? `评分: ${item.quality_score} / 10` : `Score: ${item.quality_score} / 10`} />
                </div>
                
                {item.ai_category && (
                  <div className="flex flex-wrap gap-1.5 items-center">
                    <span className="text-[10px] bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400 px-2 py-0.5 rounded-md font-semibold">
                      {language === "zh" ? "分类:" : "Category:"} {item.ai_category} {item.ai_subcategory ? `(${item.ai_subcategory})` : ""}
                    </span>
                    {item.ai_tags && item.ai_tags.split(',').map((tag: string) => (
                      <span key={tag} className="text-[9px] bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400 px-1.5 py-0.5 rounded">
                        #{tag.trim()}
                      </span>
                    ))}
                  </div>
                )}

                {item.ai_summary && (
                  <div className="space-y-1 mt-2">
                    <div className="text-[10px] font-bold text-zinc-400 uppercase tracking-wider">{language === "zh" ? "核心摘要 Summary" : "Core Summary"}</div>
                    <p className="text-xs text-zinc-700 dark:text-zinc-300 leading-relaxed font-sans">
                      {item.ai_summary}
                    </p>
                  </div>
                )}
              </div>
            ) : null}

            {/* Article Content Rendered via raw HTML */}
            {isLoadingDetail ? (
              <HStack gap={2} hAlign="center" vAlign="center" className="py-12">
                <Spinner size="md" />
                <span className="text-xs text-zinc-400">{language === "zh" ? "详细内容加载中..." : "Loading content..."}</span>
              </HStack>
            ) : (
              <article 
                className="prose prose-sm max-w-none dark:prose-invert prose-headings:font-bold prose-a:text-indigo-600 prose-img:rounded-xl leading-relaxed text-zinc-700 dark:text-zinc-300"
                dangerouslySetInnerHTML={{ __html: itemDetailHtml }} 
              />
            )}
          </VStack>
        </div>
      </div>
    </Dialog>
  );
}
