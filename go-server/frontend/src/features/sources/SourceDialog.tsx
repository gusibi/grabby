import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { Source, SourceForm } from "@/types";

interface SourceDialogProps {
  isSourceDialogOpen: boolean;
  setIsSourceDialogOpen: (value: boolean) => void;
  handleSaveSource: (e: React.FormEvent) => void;
  editingSource: Source | null;
  formError: string;
  sourceForm: SourceForm;
  setSourceForm: (value: SourceForm | ((prev: SourceForm) => SourceForm)) => void;
}

export function SourceDialog({
  isSourceDialogOpen,
  setIsSourceDialogOpen,
  handleSaveSource,
  editingSource,
  formError,
  sourceForm,
  setSourceForm
}: SourceDialogProps) {
  return (
      <Dialog open={isSourceDialogOpen} onOpenChange={setIsSourceDialogOpen}>
        <DialogContent className="sm:max-w-[500px]">
          <form onSubmit={handleSaveSource}>
            <DialogHeader>
              <DialogTitle className="text-base font-bold">
                {editingSource ? "编辑订阅数据源" : "添加新订阅数据源"}
              </DialogTitle>
              <DialogDescription className="text-xs">
                配置 RSS Feed、JSON API 或指定 Chrome Extension 网页提取规则。
              </DialogDescription>
            </DialogHeader>

            <div className="space-y-4 py-4">
              {formError && (
                <div className="bg-rose-50 dark:bg-rose-950/20 text-rose-600 dark:text-rose-400 text-xs p-3 rounded-lg border border-rose-200 dark:border-rose-800">
                  {formError}
                </div>
              )}

              <div className="space-y-1">
                <label className="text-xs font-bold text-zinc-500">唯一标识 ID (英文/拼音)*</label>
                <Input
                  disabled={!!editingSource}
                  placeholder="如: hackernews, techcrunch"
                  value={sourceForm.id}
                  onChange={e => setSourceForm((prev: SourceForm) => ({ ...prev, id: e.target.value }))}
                  className="h-9 text-xs"
                />
              </div>

              <div className="space-y-1">
                <label className="text-xs font-bold text-zinc-500">显示名称 Name*</label>
                <Input
                  placeholder="如: Hacker News"
                  value={sourceForm.name}
                  onChange={e => setSourceForm((prev: SourceForm) => ({ ...prev, name: e.target.value }))}
                  className="h-9 text-xs"
                />
              </div>

              <div className="space-y-1">
                <label className="text-xs font-bold text-zinc-500">主题分类 Topic Category* (如: AI, 财经新闻, 科技新闻, 国际新闻, 国内新闻)</label>
                <Input
                  placeholder="如: AI"
                  value={sourceForm.category}
                  onChange={e => setSourceForm((prev: SourceForm) => ({ ...prev, category: e.target.value }))}
                  className="h-9 text-xs"
                />
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-1">
                  <label className="text-xs font-bold text-zinc-500">抓取类型 Type*</label>
                  <Select
                    value={sourceForm.type}
                    onValueChange={val => setSourceForm((prev: SourceForm) => ({ ...prev, type: val }))}
                  >
                    <SelectTrigger className="h-9 text-xs">
                      <SelectValue placeholder="选择类型" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="rss">RSS Feed</SelectItem>
                      <SelectItem value="api">JSON API</SelectItem>
                      <SelectItem value="web_scrape">网页爬虫 (Extension)</SelectItem>
                    </SelectContent>
                  </Select>
                </div>

                <div className="space-y-1">
                  <label className="text-xs font-bold text-zinc-500">默认分类 Category*</label>
                  <Select
                    value={sourceForm.default_category}
                    onValueChange={val => setSourceForm((prev: SourceForm) => ({ ...prev, default_category: val }))}
                  >
                    <SelectTrigger className="h-9 text-xs">
                      <SelectValue placeholder="默认分类" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="auto">自动识别 (Auto)</SelectItem>
                      <SelectItem value="article">文章 (Article)</SelectItem>
                      <SelectItem value="tweet">推特 (Tweet)</SelectItem>
                      <SelectItem value="paper">论文 (Paper)</SelectItem>
                      <SelectItem value="project">项目 (Project)</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>

              <div className="space-y-1">
                <label className="text-xs font-bold text-zinc-500">入口 URL 地址*</label>
                <Input
                  placeholder="https://example.com/rss.xml"
                  value={sourceForm.url}
                  onChange={e => setSourceForm((prev: SourceForm) => ({ ...prev, url: e.target.value }))}
                  className="h-9 text-xs"
                />
              </div>

              <div className="space-y-1">
                <label className="text-xs font-bold text-zinc-500">定时调度 Cron 表达式*</label>
                <Input
                  placeholder="如: 0 */2 * * * (每2小时) 或 0 9 * * * (每天早9点)"
                  value={sourceForm.schedule}
                  onChange={e => setSourceForm((prev: SourceForm) => ({ ...prev, schedule: e.target.value }))}
                  className="h-9 text-xs font-mono"
                />
              </div>

              <div className="space-y-1">
                <label className="text-xs font-bold text-zinc-500">高级数据解析 JSON 配置</label>
                <textarea
                  placeholder="{}"
                  value={sourceForm.config}
                  onChange={e => setSourceForm((prev: SourceForm) => ({ ...prev, config: e.target.value }))}
                  className="w-full h-20 text-xs font-mono p-2 border border-zinc-200 dark:border-zinc-800 rounded-md bg-transparent resize-none"
                />
              </div>
            </div>

            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setIsSourceDialogOpen(false)} className="h-8 text-xs">
                取消
              </Button>
              <Button type="submit" className="bg-blue-600 hover:bg-blue-700 text-white h-8 text-xs">
                保存
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    
  );
}
