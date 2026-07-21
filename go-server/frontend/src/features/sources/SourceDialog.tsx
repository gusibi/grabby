import { Dialog, DialogHeader } from "@astryxdesign/core/Dialog";
import { TextInput } from "@astryxdesign/core/TextInput";
import { TextArea } from "@astryxdesign/core/TextArea";
import { Selector } from "@astryxdesign/core/Selector";
import { Button } from "@astryxdesign/core/Button";
import { VStack, HStack } from "@astryxdesign/core/Layout";
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
    <Dialog 
      isOpen={isSourceDialogOpen} 
      onOpenChange={setIsSourceDialogOpen}
      purpose="form"
      width={500}
    >
      <form onSubmit={handleSaveSource}>
        <DialogHeader
          title={editingSource ? "编辑订阅数据源" : "添加新订阅数据源"}
          subtitle="配置 RSS Feed、JSON API 或指定 Chrome Extension 网页提取规则。"
          onOpenChange={setIsSourceDialogOpen}
          hasDivider
        />

        <VStack gap={4} className="py-4">
          {formError && (
            <div className="bg-rose-50 dark:bg-rose-950/20 text-rose-600 dark:text-rose-400 text-xs p-3 rounded-lg border border-rose-200 dark:border-rose-800">
              {formError}
            </div>
          )}

          <TextInput
            label="唯一标识 ID (英文/拼音)*"
            placeholder="如: hackernews, techcrunch"
            value={sourceForm.id}
            onChange={val => setSourceForm((prev: SourceForm) => ({ ...prev, id: val }))}
            isDisabled={!!editingSource}
          />

          <TextInput
            label="显示名称 Name*"
            placeholder="如: Hacker News"
            value={sourceForm.name}
            onChange={val => setSourceForm((prev: SourceForm) => ({ ...prev, name: val }))}
          />

          <TextInput
            label="主题分类 Topic Category* (如: AI, 科技新闻等)"
            placeholder="如: AI"
            value={sourceForm.category}
            onChange={val => setSourceForm((prev: SourceForm) => ({ ...prev, category: val }))}
          />

          <div className="grid grid-cols-2 gap-4">
            <Selector
              label="抓取类型 Type*"
              options={[
                { value: "rss", label: "RSS Feed" },
                { value: "api", label: "JSON API" },
                { value: "web_scrape", label: "网页爬虫 (Extension)" }
              ]}
              value={sourceForm.type}
              onChange={val => val && setSourceForm((prev: SourceForm) => ({ ...prev, type: val }))}
            />

            <Selector
              label="默认分类 Category*"
              options={[
                { value: "auto", label: "自动识别 (Auto)" },
                { value: "article", label: "文章 (Article)" },
                { value: "tweet", label: "推特 (Tweet)" },
                { value: "paper", label: "论文 (Paper)" },
                { value: "project", label: "项目 (Project)" }
              ]}
              value={sourceForm.default_category}
              onChange={val => val && setSourceForm((prev: SourceForm) => ({ ...prev, default_category: val }))}
            />
          </div>

          <TextInput
            label="入口 URL 地址*"
            placeholder="https://example.com/rss.xml"
            value={sourceForm.url}
            onChange={val => setSourceForm((prev: SourceForm) => ({ ...prev, url: val }))}
          />

          <TextInput
            label="定时调度 Cron 表达式*"
            placeholder="如: 0 */2 * * * (每2小时)"
            value={sourceForm.schedule}
            onChange={val => setSourceForm((prev: SourceForm) => ({ ...prev, schedule: val }))}
          />

          <TextArea
            label="高级数据解析 JSON 配置"
            placeholder="{}"
            value={sourceForm.config}
            onChange={val => setSourceForm((prev: SourceForm) => ({ ...prev, config: val }))}
            rows={3}
          />
        </VStack>

        <HStack gap={3} hAlign="end" className="pt-4 border-t border-border">
          <Button 
            type="button" 
            variant="ghost" 
            label="取消" 
            onClick={() => setIsSourceDialogOpen(false)} 
          />
          <Button 
            type="submit" 
            variant="primary" 
            label="保存" 
          />
        </HStack>
      </form>
    </Dialog>
  );
}
