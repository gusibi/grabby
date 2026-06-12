import { X, ExternalLink, Star, CheckCircle, Circle, Loader2, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";
import { getCategoryColor, getCategoryLabel } from "@/lib/category";
import type { ScrapedItem } from "@/types";

interface ItemDetailModalProps {
  item: ScrapedItem | null;
  isOpen: boolean;
  onClose: () => void;
  isLoadingDetail: boolean;
  itemDetailHtml: string;
  toggleStar: (item: ScrapedItem, e?: React.MouseEvent) => void;
  toggleReadStatus: (item: ScrapedItem, e?: React.MouseEvent) => void;
}

export function ItemDetailModal({
  item,
  isOpen,
  onClose,
  isLoadingDetail,
  itemDetailHtml,
  toggleStar,
  toggleReadStatus,
}: ItemDetailModalProps) {
  if (!isOpen || !item) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm animate-fade-in">
      {/* Backdrop click to close */}
      <div className="absolute inset-0" onClick={onClose} />

      {/* Modal Content container */}
      <div className="relative w-full max-w-4xl h-[90vh] bg-white dark:bg-[#1c1c1e] rounded-2xl shadow-2xl border border-black/5 dark:border-white/5 flex flex-col overflow-hidden animate-zoom-in">
        
        {/* Header Action Bar */}
        <div className="h-14 flex items-center justify-between px-6 border-b border-black/5 dark:border-white/5 bg-zinc-50/50 dark:bg-zinc-900/50 shrink-0">
          <div className="flex items-center gap-2">
            <Button
              size="sm"
              variant="ghost"
              onClick={(e) => toggleReadStatus(item, e)}
              className="h-8 text-xs px-3 gap-1.5 hover:bg-black/5 dark:hover:bg-white/5"
            >
              {item.read_status === 1 ? (
                <>
                  <CheckCircle className="w-4 h-4 text-blue-500" />
                  <span className="font-medium text-zinc-700 dark:text-zinc-300">已读</span>
                </>
              ) : (
                <>
                  <Circle className="w-4 h-4 text-zinc-400" />
                  <span className="font-medium text-zinc-500">标记已读</span>
                </>
              )}
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onClick={(e) => toggleStar(item, e)}
              className="h-8 text-xs px-3 gap-1.5 hover:bg-black/5 dark:hover:bg-white/5"
            >
              <Star className={`w-4 h-4 ${item.starred === 1 ? "fill-amber-400 text-amber-400" : "text-zinc-400"}`} />
              <span className="font-medium text-zinc-700 dark:text-zinc-300">
                {item.starred === 1 ? "已收藏" : "收藏"}
              </span>
            </Button>
          </div>

          <div className="flex items-center gap-4">
            <a
              href={item.url}
              target="_blank"
              rel="noreferrer"
              className="flex items-center gap-1 text-xs text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300 font-medium"
            >
              <ExternalLink className="w-4 h-4" />
              <span>打开原文</span>
            </a>
            
            <button
              onClick={onClose}
              className="p-1.5 rounded-lg hover:bg-black/5 dark:hover:bg-white/5 text-zinc-500 hover:text-zinc-800 dark:text-zinc-400 dark:hover:text-zinc-100 transition-colors"
            >
              <X className="w-5 h-5" />
            </button>
          </div>
        </div>

        {/* Scrollable Reader Area */}
        <div className="flex-1 overflow-y-auto p-6 md:p-10 scrollbar-thin">
          <div className="max-w-3xl mx-auto space-y-6">
            
            {/* Title & Category Info */}
            <div className="space-y-3">
              <div className="flex flex-wrap gap-1.5 items-center">
                <span className={`px-2 py-0.5 rounded-full text-[10px] font-bold tracking-tight ${getCategoryColor(item.category)}`}>
                  {getCategoryLabel(item.category)}
                </span>
                {item.source_category && (
                  <span className="px-2 py-0.5 rounded-full text-[10px] font-bold tracking-tight bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400">
                    {item.source_category}
                  </span>
                )}
              </div>
              <h1 className="text-xl md:text-3xl font-black tracking-tight leading-snug text-zinc-900 dark:text-zinc-50">
                {item.title}
              </h1>
              <div className="flex items-center gap-2 text-xs text-zinc-400 pt-2 pb-4 border-b border-zinc-100 dark:border-zinc-800">
                <span className="font-semibold text-zinc-600 dark:text-zinc-300">{item.origin_source}</span>
                <span>•</span>
                <span>发布于: {item.published_at ? new Date(item.published_at).toLocaleString() : "未知时间"}</span>
              </div>
            </div>

            {/* AI Summary and Analysis */}
            {item.quality_score !== undefined && item.quality_score > 0 ? (
              <div className="bg-indigo-50/50 dark:bg-indigo-950/10 border-l-4 border-indigo-500 p-4 rounded-r-xl space-y-3">
                <div className="flex justify-between items-center">
                  <h5 className="text-xs font-bold text-indigo-600 dark:text-indigo-400 uppercase tracking-widest flex items-center gap-1.5">
                    <Sparkles className="w-3.5 h-3.5" />
                    AI 智能深度分析 AI Insights
                  </h5>
                  <span className="text-xs font-black bg-indigo-600 text-white dark:bg-indigo-700 px-2.5 py-0.5 rounded-full">
                    评分: {item.quality_score} / 10
                  </span>
                </div>
                
                {item.ai_category && (
                  <div className="flex flex-wrap gap-1.5 items-center">
                    <span className="text-[10px] bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400 px-2 py-0.5 rounded-md font-semibold">
                      分类: {item.ai_category} {item.ai_subcategory ? `(${item.ai_subcategory})` : ""}
                    </span>
                    {item.ai_tags && item.ai_tags.split(',').map((tag: string) => (
                      <span key={tag} className="text-[9px] bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400 px-1.5 py-0.5 rounded">
                        #{tag.trim()}
                      </span>
                    ))}
                  </div>
                )}

                {item.ai_summary && (
                  <div>
                    <p className="text-xs leading-relaxed text-zinc-700 dark:text-zinc-300 font-medium">
                      <span className="font-bold text-indigo-600 dark:text-indigo-400">智能摘要: </span>
                      {item.ai_summary}
                    </p>
                  </div>
                )}

                {item.ai_comment && (
                  <div className="text-[11px] text-zinc-500 dark:text-zinc-400 bg-white/50 dark:bg-black/10 rounded-lg p-2.5 border border-indigo-500/5 leading-relaxed">
                    <span className="font-bold text-zinc-600 dark:text-zinc-300">深度点评: </span>
                    {item.ai_comment}
                  </div>
                )}
                
                {item.ai_model_used && (
                  <p className="text-[9px] text-zinc-400 text-right">
                    分析模型: {item.ai_model_used}
                  </p>
                )}
              </div>
            ) : (
              /* Fallback Normal Summary Banner */
              item.summary && (
                <div className="bg-blue-50 dark:bg-blue-950/20 border-l-4 border-blue-500 p-4 rounded-r-xl">
                  <h5 className="text-xs font-bold text-blue-600 dark:text-blue-400 uppercase tracking-widest mb-1">
                    摘要 Summary
                  </h5>
                  <p 
                    className="text-xs leading-relaxed text-zinc-700 dark:text-zinc-300"
                    dangerouslySetInnerHTML={{ __html: item.summary }}
                  />
                </div>
              )
            )}

            {/* Main Article Content */}
            {isLoadingDetail ? (
              <div className="flex items-center justify-center py-20 text-zinc-400">
                <Loader2 className="w-8 h-8 animate-spin" />
              </div>
            ) : (
              <article 
                className="prose dark:prose-invert max-w-none text-sm md:text-base leading-relaxed text-zinc-800 dark:text-zinc-200"
                dangerouslySetInnerHTML={{ __html: itemDetailHtml || `<p>${item.content || "无法解析正文"}</p>` }}
              />
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
