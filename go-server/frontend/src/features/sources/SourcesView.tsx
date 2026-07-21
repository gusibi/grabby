import { RefreshCw, Trash2, Edit, Database, Calendar } from "lucide-react";
import { Button } from "@astryxdesign/core/Button";
import { Switch } from "@astryxdesign/core/Switch";
import { Card } from "@astryxdesign/core/Card";
import { Badge } from "@astryxdesign/core/Badge";
import { VStack, HStack } from "@astryxdesign/core/Layout";
import { formatTimeAgo } from "@/lib/format";
import type { Source } from "@/types";

interface SourcesViewProps {
  sources: Source[];
  handleToggleSourceEnabled: (source: Source) => void;
  handleRunSource: (source: Source) => void;
  openEditSourceDialog: (source: Source) => void;
  handleDeleteSource: (id: string) => void;
  isAuthenticated: boolean;
}

export function SourcesView({
  sources,
  handleToggleSourceEnabled,
  handleRunSource,
  openEditSourceDialog,
  handleDeleteSource,
  isAuthenticated
}: SourcesViewProps) {
  return (
    <div className="flex-1 overflow-y-auto bg-body p-8">
      <div className="max-w-4xl mx-auto space-y-8 pb-12">
        <Card className="overflow-hidden">
          <div className="border-b border-border p-6 bg-muted">
            <h3 className="text-base font-bold text-primary">已订阅订阅源 ({sources.length})</h3>
            <p className="text-xs text-secondary mt-1">管理你的 RSS Feed、JSON API 和智能网页爬虫规则。</p>
          </div>
          
          <div className="divide-y divide-border">
            {sources.map(source => (
              <div 
                key={source.id} 
                className={`p-4 flex flex-wrap gap-4 items-center justify-between transition-opacity ${
                  source.enabled === 0 ? "opacity-50" : ""
                }`}
              >
                <HStack gap={3} vAlign="center" className="flex-1 min-w-0">
                  <div className={`w-10 h-10 rounded-xl flex items-center justify-center shrink-0 ${
                    source.enabled === 0 
                      ? "bg-zinc-100 dark:bg-zinc-800 text-zinc-400" 
                      : "bg-blue-subtle text-accent"
                  }`}>
                    <Database className="w-5 h-5" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <HStack gap={2} vAlign="center" className="flex-wrap">
                      <h5 className="font-bold text-sm leading-none">{source.name}</h5>
                      <Badge 
                        variant={
                          source.type === "web_scrape"
                            ? "purple"
                            : source.type === "api"
                              ? "orange"
                              : "blue"
                        } 
                        label={source.type} 
                      />
                      {source.enabled === 0 && (
                        <Badge variant="neutral" label="已禁用" />
                      )}
                    </HStack>
                    <p className="text-[10px] text-zinc-400 mt-1 truncate max-w-xs md:max-w-md font-mono">
                      {source.url}
                    </p>

                    <HStack gap={3} vAlign="center" className="mt-1.5 text-[10px] text-zinc-500 font-medium flex-wrap">
                      <span className="flex items-center gap-0.5">
                        <Calendar className="w-3 h-3" /> Cron: {source.schedule}
                      </span>
                      <span>•</span>
                      <span>主题分类: {source.category || "General"}</span>
                      <span>•</span>
                      <span>默认分类: {source.default_category}</span>
                      {source.last_fetch_at && (
                        <>
                          <span>•</span>
                          <span className={
                            source.last_fetch_status === "success"
                              ? "text-success"
                              : source.last_fetch_status === "running"
                                ? "text-accent"
                                : "text-error"
                          }>
                            上次抓取: {source.last_fetch_status === "running" ? "抓取中..." : formatTimeAgo(source.last_fetch_at)}
                          </span>
                        </>
                      )}
                    </HStack>
                  </div>
                </HStack>

                <HStack gap={3} vAlign="center" className="shrink-0">
                  {isAuthenticated && (
                    <HStack gap={2} vAlign="center">
                      <span className="text-xs text-zinc-400 font-medium select-none">
                        {source.enabled === 1 ? "启用" : "禁用"}
                      </span>
                      <Switch
                        label="启用"
                        isLabelHidden
                        value={source.enabled === 1}
                        onChange={() => handleToggleSourceEnabled(source)}
                      />
                    </HStack>
                  )}

                  {isAuthenticated && (
                    <HStack gap={1.5} vAlign="center">
                      <Button
                        onClick={() => handleRunSource(source)}
                        size="sm"
                        variant="secondary"
                        isDisabled={source.last_fetch_status === "running" || source.enabled === 0}
                        isLoading={source.last_fetch_status === "running"}
                        label="立即抓取"
                        icon={<RefreshCw className="w-3 h-3" />}
                      />

                      <Button 
                        onClick={() => openEditSourceDialog(source)} 
                        size="sm" 
                        variant="ghost" 
                        isIconOnly
                        label="编辑"
                        icon={<Edit className="w-3.5 h-3.5 text-zinc-500" />}
                      />
                      <Button 
                        onClick={() => handleDeleteSource(source.id)} 
                        size="sm" 
                        variant="ghost" 
                        isIconOnly
                        label="删除"
                        icon={<Trash2 className="w-3.5 h-3.5 text-error" />}
                      />
                    </HStack>
                  )}
                </HStack>
              </div>
            ))}
          </div>
        </Card>
      </div>
    </div>
  );
}
