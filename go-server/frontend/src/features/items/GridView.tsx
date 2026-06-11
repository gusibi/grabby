import { Inbox, Search, Star, Loader2, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { getCategoryColor, getCategoryLabel } from "@/lib/category";
import { formatTimeAgo } from "@/lib/format";
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
  fetchItems: () => void;
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
  setCurrentView,
  handleSelectItem,
  toggleStar,
  hasMore,
  fetchItems,
  isLoadingItems
}: GridViewProps) {
  return (
            <div className="flex-1 flex flex-col overflow-hidden">
              <div className="p-4 bg-white dark:bg-[#18181b] border-b border-black/5 dark:border-white/5 flex flex-wrap gap-4 items-center justify-between">
                <div className="flex items-center gap-2 overflow-x-auto py-1">
                  <Button
                    variant={selectedAICategory === "all" && !isShowOnlyAIQuality ? "default" : "outline"}
                    onClick={() => { setSelectedAICategory("all"); setIsShowOnlyAIQuality(false); }}
                    className="h-7 text-xs rounded-full px-4"
                  >
                    全部
                  </Button>
                  {aiCategories.map((cat) => (
                    <Button
                      key={cat.name}
                      variant={selectedAICategory === cat.name ? "default" : "outline"}
                      onClick={() => { setSelectedAICategory(cat.name); setIsShowOnlyAIQuality(false); }}
                      className="h-7 text-xs rounded-full px-4 gap-1"
                    >
                      {cat.name}
                      <span className="text-[10px] opacity-60">{cat.count}</span>
                    </Button>
                  ))}
                </div>

                <div className="relative w-full max-w-xs">
                  <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-zinc-400" />
                  <Input
                    placeholder="在标题和摘要中搜索..."
                    value={searchQuery}
                    onChange={e => setSearchQuery(e.target.value)}
                    className="pl-9 h-9 text-xs"
                  />
                </div>
              </div>

              <div className="flex-1 overflow-y-auto bg-zinc-50/50 dark:bg-[#121212] p-6">
                {items.length === 0 ? (
                  <div className="flex flex-col items-center justify-center h-[50vh] text-zinc-400">
                    <Inbox className="w-12 h-12 mb-2 stroke-1" />
                    <p className="text-sm">暂无内容，请检查订阅源或点击右上角抓取数据</p>
                  </div>
                ) : (
                  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 max-w-6xl mx-auto">
                    {items.map(item => (
                      <Card 
                        key={item.id} 
                        onClick={() => {
                          setCurrentView("list");
                          handleSelectItem(item);
                        }}
                        className="group relative flex flex-col bg-white dark:bg-[#1c1c1e] border border-black/5 dark:border-white/5 news-card-hover cursor-pointer overflow-hidden shadow-sm h-full"
                      >
                        <CardHeader className="p-4 pb-2">
                          <div className="flex justify-between items-start gap-2">
                            <div className="flex flex-wrap gap-1.5 items-center">
                              <span className={`px-2 py-0.5 rounded-full text-[9px] font-bold tracking-tight ${getCategoryColor(item.category)}`}>
                                {getCategoryLabel(item.category)}
                              </span>
                              {item.source_category && (
                                <span className="px-2 py-0.5 rounded-full text-[9px] font-bold tracking-tight bg-zinc-100 text-zinc-600 dark:bg-zinc-800/60 dark:text-zinc-400">
                                  {item.source_category}
                                </span>
                              )}
                              {item.quality_score !== undefined && item.quality_score > 0 && (
                                <span className={`px-2 py-0.5 rounded-full text-[9px] font-extrabold tracking-tight ${
                                  item.quality_score >= 8 
                                    ? "bg-indigo-600 text-white dark:bg-indigo-700" 
                                    : "bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400"
                                }`}>
                                  AI {item.quality_score}分
                                </span>
                              )}
                            </div>
                            <div className="flex items-center gap-1.5">
                              <span className="text-[10px] text-zinc-400 font-medium">{formatTimeAgo(item.published_at)}</span>
                              <Button
                                size="icon"
                                variant="ghost"
                                className="w-6 h-6 hover:bg-black/5 dark:hover:bg-white/5"
                                onClick={(e) => toggleStar(item, e)}
                              >
                                <Star className={`w-3.5 h-3.5 ${item.starred === 1 ? "fill-amber-400 text-amber-400" : "text-zinc-400"}`} />
                              </Button>
                            </div>
                          </div>
                          <CardTitle className="text-sm font-bold leading-snug group-hover:text-blue-500 transition-colors line-clamp-2 mt-2">
                            {item.title}
                          </CardTitle>
                        </CardHeader>
                        <CardContent className="p-4 pt-1 flex-1">
                          {item.ai_summary ? (
                            <div className="text-xs bg-indigo-50/30 dark:bg-indigo-950/5 border border-indigo-500/5 rounded-lg p-2.5 space-y-1">
                              <p className="text-[10px] font-bold text-indigo-600 dark:text-indigo-400 uppercase tracking-wider flex items-center gap-1">
                                <Sparkles className="w-3 h-3" />
                                AI 摘要 {item.ai_category ? `· ${item.ai_category}` : ""}
                              </p>
                              <p className="text-zinc-600 dark:text-zinc-400 leading-relaxed line-clamp-3">
                                {item.ai_summary}
                              </p>
                            </div>
                          ) : (
                            <p className="text-xs text-zinc-500 line-clamp-3 leading-relaxed">
                              {item.summary || "未抓取到描述摘要，点击以进入详情阅读全文。"}
                            </p>
                          )}
                        </CardContent>
                        <CardFooter className="p-4 pt-0 border-t border-black/5 dark:border-white/5 flex items-center justify-between text-[10px]">
                          <div className="flex items-center gap-1.5 text-zinc-400">
                            <div className="w-4 h-4 rounded-full bg-blue-100 dark:bg-blue-900/30 flex items-center justify-center text-[8px] font-bold text-blue-600 dark:text-blue-400">
                              {item.origin_source ? item.origin_source[0] : "?"}
                            </div>
                            <span className="font-semibold uppercase tracking-tight line-clamp-1 max-w-[120px]">{item.origin_source || "未知出处"}</span>
                          </div>
                          
                          {item.read_status === 0 && (
                            <div className="flex items-center gap-1 text-blue-600 font-medium">
                              <span className="w-1.5 h-1.5 rounded-full bg-blue-500" />
                              <span>未读</span>
                            </div>
                          )}
                        </CardFooter>
                      </Card>
                    ))}
                  </div>
                )}
                
                {hasMore && items.length > 0 && (
                  <div className="flex justify-center mt-8 pb-12">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => fetchItems()}
                      disabled={isLoadingItems}
                      className="px-6 h-8 text-xs font-semibold"
                    >
                      {isLoadingItems ? <Loader2 className="w-3.5 h-3.5 animate-spin mr-1.5" /> : null}
                      加载更多数据...
                    </Button>
                  </div>
                )}
              </div>
            </div>
  );
}
