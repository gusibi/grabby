import { RefreshCw, Trash2, Edit, Loader2, Calendar, Database } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { formatTimeAgo } from "@/lib/format";
import type { Source } from "@/types";

interface SourcesViewProps {
  sources: Source[];
  handleToggleSourceEnabled: (source: Source) => void;
  handleRunSource: (source: Source) => void;
  openEditSourceDialog: (source: Source) => void;
  handleDeleteSource: (id: string) => void;
}

export function SourcesView({
  sources,
  handleToggleSourceEnabled,
  handleRunSource,
  openEditSourceDialog,
  handleDeleteSource
}: SourcesViewProps) {
  return (
            <div className="flex-1 overflow-y-auto bg-zinc-50/50 dark:bg-[#121212] p-8">
              <div className="max-w-4xl mx-auto space-y-8 pb-12">
                <Card className="border border-black/5 dark:border-white/5 bg-white dark:bg-[#1c1c1e] shadow-sm rounded-2xl overflow-hidden">
                  <CardHeader className="border-b border-black/5 p-6 bg-zinc-50/50 dark:bg-zinc-900/50">
                    <CardTitle className="text-base font-bold">已订阅订阅源 ({sources.length})</CardTitle>
                    <CardDescription className="text-xs">管理你的 RSS Feed、JSON API 和智能网页爬虫规则。</CardDescription>
                  </CardHeader>
                  <CardContent className="p-0">
                    <div className="divide-y divide-black/5 dark:divide-white/5">
                      {sources.map(source => (
                        <div key={source.id} className={`p-4 flex flex-wrap gap-4 items-center justify-between transition-opacity ${source.enabled === 0 ? "opacity-50" : ""}`}>
                          <div className="flex items-center gap-3">
                            <div className={`w-10 h-10 rounded-xl flex items-center justify-center shrink-0 ${source.enabled === 0 ? "bg-zinc-100 dark:bg-zinc-800 text-zinc-400" : "bg-blue-50 dark:bg-zinc-800 text-blue-500"}`}>
                              <Database className="w-5 h-5" />
                            </div>
                            <div>
                              <div className="flex items-center gap-2">
                                <h5 className="font-bold text-sm leading-none">{source.name}</h5>
                                <span className={`text-[9px] px-1.5 py-0.2 rounded font-bold uppercase ${
                                  source.type === "web_scrape"
                                    ? "bg-purple-100 text-purple-600 dark:bg-purple-900/20"
                                    : source.type === "api"
                                      ? "bg-amber-100 text-amber-600 dark:bg-amber-900/20"
                                      : "bg-blue-100 text-blue-600 dark:bg-blue-900/20"
                                }`}>
                                  {source.type}
                                </span>
                                {source.enabled === 0 && (
                                  <span className="text-[9px] px-1.5 py-0.2 rounded font-bold bg-zinc-200 text-zinc-500 dark:bg-zinc-700 dark:text-zinc-400">
                                    已禁用
                                  </span>
                                )}
                              </div>
                              <p className="text-[10px] text-zinc-400 mt-1 truncate max-w-xs md:max-w-md font-mono">{source.url}</p>

                              <div className="flex items-center gap-3 mt-1.5 text-[10px] text-zinc-500 font-medium">
                                <span className="flex items-center gap-0.5"><Calendar className="w-3 h-3" /> Cron: {source.schedule}</span>
                                <span>•</span>
                                <span>主题分类: {source.category || "General"}</span>
                                <span>•</span>
                                <span>默认分类: {source.default_category}</span>
                                {source.last_fetch_at && (
                                  <>
                                    <span>•</span>
                                    <span className={`flex items-center gap-1 ${
                                      source.last_fetch_status === "success"
                                        ? "text-emerald-500"
                                        : source.last_fetch_status === "running"
                                          ? "text-blue-500"
                                          : "text-rose-500"
                                    }`}>
                                      上次抓取: {source.last_fetch_status === "running" ? "抓取中..." : formatTimeAgo(source.last_fetch_at)}
                                    </span>
                                  </>
                                )}
                              </div>
                            </div>
                          </div>

                          <div className="flex items-center gap-3">
                            <div className="flex items-center gap-2">
                              <span className="text-xs text-zinc-400 font-medium select-none">
                                {source.enabled === 1 ? "启用" : "禁用"}
                              </span>
                              <Switch
                                checked={source.enabled === 1}
                                onCheckedChange={() => handleToggleSourceEnabled(source)}
                              />
                            </div>

                            <Button
                              onClick={() => handleRunSource(source)}
                              size="sm"
                              variant="outline"
                              disabled={source.last_fetch_status === "running" || source.enabled === 0}
                              className="h-7 text-xs gap-1"
                            >
                              {source.last_fetch_status === "running" ? (
                                <Loader2 className="w-3 h-3 animate-spin" />
                              ) : (
                                <RefreshCw className="w-3 h-3" />
                              )}
                              立即抓取
                            </Button>

                            <Button onClick={() => openEditSourceDialog(source)} size="icon" variant="ghost" className="w-8 h-8">
                              <Edit className="w-3.5 h-3.5 text-zinc-500" />
                            </Button>
                            <Button onClick={() => handleDeleteSource(source.id)} size="icon" variant="ghost" className="w-8 h-8 hover:bg-rose-50 dark:hover:bg-rose-950/20">
                              <Trash2 className="w-3.5 h-3.5 text-rose-500" />
                            </Button>
                          </div>
                        </div>
                      ))}
                    </div>
                  </CardContent>
                </Card>
              </div>
            </div>
  );
}
