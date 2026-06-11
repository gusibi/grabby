import { Inbox, Search, ExternalLink, Star, CheckCircle, Circle, Loader2, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { getCategoryColor, getCategoryLabel } from "@/lib/category";
import { formatTimeAgo } from "@/lib/format";
import type { AICategory, ScrapedItem } from "@/types";

interface ReaderViewProps {
  items: ScrapedItem[];
  searchQuery: string;
  setSearchQuery: (value: string) => void;
  selectedAICategory: string;
  isShowOnlyAIQuality: boolean;
  setSelectedAICategory: (value: string) => void;
  setIsShowOnlyAIQuality: (value: boolean) => void;
  aiCategories: AICategory[];
  selectedItem: ScrapedItem | null;
  handleSelectItem: (item: ScrapedItem) => void;
  toggleStar: (item: ScrapedItem, e?: React.MouseEvent) => void;
  hasMore: boolean;
  fetchItems: () => void;
  isLoadingItems: boolean;
  toggleReadStatus: (item: ScrapedItem, e?: React.MouseEvent) => void;
  isLoadingDetail: boolean;
  itemDetailHtml: string;
}

export function ReaderView({
  items,
  searchQuery,
  setSearchQuery,
  selectedAICategory,
  isShowOnlyAIQuality,
  setSelectedAICategory,
  setIsShowOnlyAIQuality,
  aiCategories,
  selectedItem,
  handleSelectItem,
  toggleStar,
  hasMore,
  fetchItems,
  isLoadingItems,
  toggleReadStatus,
  isLoadingDetail,
  itemDetailHtml
}: ReaderViewProps) {
  return (
            <div className="flex-1 flex overflow-hidden">
              <div className="w-80 border-r border-black/5 dark:border-white/5 flex flex-col bg-zinc-50/50 dark:bg-[#18181b] shrink-0">
                <div className="p-3 border-b border-black/5 dark:border-white/5 space-y-2">
                  <div className="relative">
                    <Search className="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-zinc-400" />
                    <Input
                      placeholder="快速过滤..."
                      value={searchQuery}
                      onChange={e => setSearchQuery(e.target.value)}
                      className="pl-8 h-8 text-xs"
                    />
                  </div>
                  <div className="flex items-center gap-1 overflow-x-auto no-scrollbar py-0.5">
                    <button
                      onClick={() => { setSelectedAICategory("all"); setIsShowOnlyAIQuality(false); }}
                      className={`px-2.5 py-1 text-[10px] rounded-full transition-all font-medium border shrink-0 ${
                        selectedAICategory === "all" && !isShowOnlyAIQuality
                          ? "bg-zinc-800 text-white dark:bg-white dark:text-zinc-900 border-transparent font-bold"
                          : "bg-transparent text-zinc-600 dark:text-zinc-400 border-zinc-200 dark:border-zinc-800 hover:bg-black/5 dark:hover:bg-white/5"
                      }`}
                    >
                      全部
                    </button>
                    {aiCategories.map((cat) => (
                      <button
                        key={cat.name}
                        onClick={() => { setSelectedAICategory(cat.name); setIsShowOnlyAIQuality(false); }}
                        className={`px-2.5 py-1 text-[10px] rounded-full transition-all font-medium border shrink-0 ${
                          selectedAICategory === cat.name
                            ? "bg-zinc-800 text-white dark:bg-white dark:text-zinc-900 border-transparent font-bold"
                            : "bg-transparent text-zinc-600 dark:text-zinc-400 border-zinc-200 dark:border-zinc-800 hover:bg-black/5 dark:hover:bg-white/5"
                        }`}
                      >
                        {cat.name}
                      </button>
                    ))}
                  </div>
                </div>
                
                <div className="flex-1 overflow-y-auto">
                  {items.length === 0 ? (
                    <div className="flex flex-col items-center justify-center p-8 text-zinc-400">
                      <Inbox className="w-8 h-8 mb-2 stroke-1" />
                      <p className="text-xs text-center">暂无消息内容</p>
                    </div>
                  ) : (
                    items.map(item => (
                      <div
                        key={item.id}
                        onClick={() => handleSelectItem(item)}
                        className={`p-4 border-b border-black/5 dark:border-white/5 cursor-pointer transition-all ${
                          selectedItem?.id === item.id
                            ? "bg-white dark:bg-white/5 border-l-4 border-l-blue-500 shadow-sm"
                            : "hover:bg-black/5 dark:hover:bg-white/5 border-l-4 border-l-transparent"
                        }`}
                      >
                        <div className="flex justify-between items-center mb-1">
                          <span className="text-[9px] font-bold text-blue-500 uppercase tracking-widest truncate max-w-[120px]">
                            {item.origin_source}
                          </span>
                          <span className="text-[9px] text-zinc-400 font-medium">
                            {formatTimeAgo(item.published_at)}
                          </span>
                        </div>
                        <h4 className={`text-xs font-bold leading-snug line-clamp-2 ${
                          selectedItem?.id === item.id 
                            ? "text-blue-600 dark:text-blue-400" 
                            : item.read_status === 0 ? "text-zinc-900 dark:text-zinc-100" : "text-zinc-500"
                        }`}>
                          {item.title}
                        </h4>
                        <div className="flex justify-between items-center mt-2">
                          <div className="flex gap-1 items-center">
                            <span className={`px-1.5 py-0.5 rounded text-[8px] font-bold ${getCategoryColor(item.category)}`}>
                              {item.category}
                            </span>
                            {item.source_category && (
                              <span className="px-1.5 py-0.5 rounded text-[8px] font-bold bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400">
                                {item.source_category}
                              </span>
                            )}
                          </div>
                          <div className="flex items-center gap-1.5">
                            <Star 
                              onClick={(e) => toggleStar(item, e)}
                              className={`w-3 h-3 ${item.starred === 1 ? "fill-amber-400 text-amber-400" : "text-zinc-300"}`} 
                            />
                            {item.read_status === 0 && <span className="w-1.5 h-1.5 rounded-full bg-blue-500" />}
                          </div>
                        </div>
                      </div>
                    ))
                  )}

                  {hasMore && items.length > 0 && (
                    <div className="p-3 text-center">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => fetchItems()}
                        disabled={isLoadingItems}
                        className="w-full text-xs h-7 hover:bg-black/5"
                      >
                        {isLoadingItems ? "加载中..." : "载入更多..."}
                      </Button>
                    </div>
                  )}
                </div>
              </div>

              <div className="flex-1 flex flex-col bg-white dark:bg-[#121212] overflow-hidden">
                {selectedItem ? (
                  <div className="flex-1 flex flex-col overflow-hidden">
                    <div className="h-10 flex items-center justify-between px-6 border-b border-black/5 dark:border-white/5 bg-zinc-50/50 dark:bg-zinc-900/50 shrink-0">
                      <div className="flex items-center gap-2">
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => toggleReadStatus(selectedItem)}
                          className="h-7 text-xs px-2 gap-1"
                        >
                          {selectedItem.read_status === 1 ? (
                            <>
                              <CheckCircle className="w-3.5 h-3.5 text-blue-500" />
                              <span>已读</span>
                            </>
                          ) : (
                            <>
                              <Circle className="w-3.5 h-3.5 text-zinc-400" />
                              <span>标记已读</span>
                            </>
                          )}
                        </Button>
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => toggleStar(selectedItem)}
                          className="h-7 text-xs px-2 gap-1"
                        >
                          <Star className={`w-3.5 h-3.5 ${selectedItem.starred === 1 ? "fill-amber-400 text-amber-400" : "text-zinc-400"}`} />
                          <span>{selectedItem.starred === 1 ? "已收藏" : "收藏"}</span>
                        </Button>
                      </div>

                      <a
                        href={selectedItem.url}
                        target="_blank"
                        rel="noreferrer"
                        className="flex items-center gap-1 text-xs text-blue-600 hover:text-blue-700 font-medium"
                      >
                        <ExternalLink className="w-3.5 h-3.5" />
                        <span>打开原文</span>
                      </a>
                    </div>

                    <div className="flex-1 overflow-y-auto p-8 md:p-12">
                      <div className="max-w-2xl mx-auto space-y-6">
                        <div className="space-y-2">
                          <div className="flex gap-1.5 items-center">
                            <span className={`px-2 py-0.5 rounded-full text-[10px] font-bold tracking-tight ${getCategoryColor(selectedItem.category)}`}>
                              {getCategoryLabel(selectedItem.category)}
                            </span>
                            {selectedItem.source_category && (
                              <span className="px-2 py-0.5 rounded-full text-[10px] font-bold tracking-tight bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400">
                                {selectedItem.source_category}
                              </span>
                            )}
                          </div>
                          <h1 className="text-2xl md:text-3xl font-black tracking-tight leading-tight">
                            {selectedItem.title}
                          </h1>
                          <div className="flex items-center gap-2 text-xs text-zinc-400 pt-2 pb-4 border-b border-zinc-100 dark:border-zinc-800">
                            <span className="font-semibold text-zinc-600 dark:text-zinc-300">{selectedItem.origin_source}</span>
                            <span>•</span>
                            <span>发布于: {selectedItem.published_at ? new Date(selectedItem.published_at).toLocaleString() : "未知时间"}</span>
                          </div>
                        </div>

                        {selectedItem.quality_score !== undefined && selectedItem.quality_score > 0 ? (
                          <div className="bg-indigo-50/50 dark:bg-indigo-950/10 border-l-4 border-indigo-500 p-4 rounded-r-xl space-y-3">
                            <div className="flex justify-between items-center">
                              <h5 className="text-xs font-bold text-indigo-600 dark:text-indigo-400 uppercase tracking-widest flex items-center gap-1.5">
                                <Sparkles className="w-3.5 h-3.5" />
                                AI 智能深度分析 AI Insights
                              </h5>
                              <span className="text-xs font-black bg-indigo-600 text-white dark:bg-indigo-700 px-2.5 py-0.5 rounded-full">
                                评分: {selectedItem.quality_score} / 10
                              </span>
                            </div>
                            
                            {selectedItem.ai_category && (
                              <div className="flex flex-wrap gap-1.5 items-center">
                                <span className="text-[10px] bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400 px-2 py-0.5 rounded-md font-semibold">
                                  分类: {selectedItem.ai_category} {selectedItem.ai_subcategory ? `(${selectedItem.ai_subcategory})` : ""}
                                </span>
                                {selectedItem.ai_tags && selectedItem.ai_tags.split(',').map((tag: string) => (
                                  <span key={tag} className="text-[9px] bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400 px-1.5 py-0.5 rounded">
                                    #{tag}
                                  </span>
                                ))}
                              </div>
                            )}

                            {selectedItem.ai_summary && (
                              <div>
                                <p className="text-xs leading-relaxed text-zinc-700 dark:text-zinc-300 font-medium">
                                  <span className="font-bold text-indigo-600 dark:text-indigo-400">智能摘要: </span>
                                  {selectedItem.ai_summary}
                                </p>
                              </div>
                            )}

                            {selectedItem.ai_comment && (
                              <div className="text-[11px] text-zinc-500 dark:text-zinc-400 bg-white/50 dark:bg-black/10 rounded-lg p-2.5 border border-indigo-500/5 leading-relaxed">
                                <span className="font-bold text-zinc-600 dark:text-zinc-300">深度点评: </span>
                                {selectedItem.ai_comment}
                              </div>
                            )}
                            
                            {selectedItem.ai_model_used && (
                              <p className="text-[9px] text-zinc-400 text-right">
                                分析模型: {selectedItem.ai_model_used}
                              </p>
                            )}
                          </div>
                        ) : (
                          /* Fallback Normal Summary Banner */
                          selectedItem.summary && (
                            <div className="bg-blue-50 dark:bg-blue-950/20 border-l-4 border-blue-500 p-4 rounded-r-xl">
                              <h5 className="text-xs font-bold text-blue-600 dark:text-blue-400 uppercase tracking-widest mb-1">
                                摘要 Summary
                              </h5>
                              <p 
                                className="text-xs leading-relaxed text-zinc-700 dark:text-zinc-300"
                                dangerouslySetInnerHTML={{ __html: selectedItem.summary }}
                              />
                            </div>
                          )
                        )}

                        {isLoadingDetail ? (
                          <div className="flex items-center justify-center py-20 text-zinc-400">
                            <Loader2 className="w-8 h-8 animate-spin" />
                          </div>
                        ) : (
                          <article 
                            className="prose dark:prose-invert max-w-none text-sm md:text-base leading-relaxed text-zinc-800 dark:text-zinc-200"
                            dangerouslySetInnerHTML={{ __html: itemDetailHtml || `<p>${selectedItem.content || "无法解析正文"}</p>` }}
                          />
                        )}
                      </div>
                    </div>
                  </div>
                ) : (
                  <div className="flex-1 flex flex-col items-center justify-center text-zinc-400">
                    <Inbox className="w-12 h-12 mb-2 stroke-1" />
                    <p className="text-sm">选择左侧项目开始阅读</p>
                  </div>
                )}
              </div>
            </div>
  );
}
