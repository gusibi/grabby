import { Dialog, DialogHeader } from "@astryxdesign/core/Dialog";
import { TextInput } from "@astryxdesign/core/TextInput";
import { TextArea } from "@astryxdesign/core/TextArea";
import { Selector } from "@astryxdesign/core/Selector";
import { VStack, HStack } from "@astryxdesign/core/Layout";
import type { Source, SourceForm } from "@/types";
import { useCopy } from "@/lib/copy";
import { sourcesCopy } from "./copy";

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
  const c = useCopy(sourcesCopy);
  return (
    <Dialog 
      isOpen={isSourceDialogOpen} 
      onOpenChange={setIsSourceDialogOpen}
      purpose="form"
      width={500}
      maxHeight="88vh"
    >
      <form onSubmit={handleSaveSource} className="flex flex-col">
        <DialogHeader
          title={editingSource ? c.editTitle : c.addTitle}
          subtitle={c.dialogSubtitle}
          onOpenChange={setIsSourceDialogOpen}
          hasDivider
        />

        <VStack gap={4} className="py-4 flex-1 overflow-y-auto g-scroll">
          {formError && (
            <div className="bg-rose-50 dark:bg-rose-950/20 text-rose-600 dark:text-rose-400 text-xs p-3 rounded-lg border border-rose-200 dark:border-rose-800">
              {formError}
            </div>
          )}

          <TextInput
            label={c.labelId}
            placeholder={c.placeholderId}
            value={sourceForm.id}
            onChange={val => setSourceForm((prev: SourceForm) => ({ ...prev, id: val }))}
            isDisabled={!!editingSource}
          />

          <TextInput
            label={c.labelName}
            placeholder={c.placeholderName}
            value={sourceForm.name}
            onChange={val => setSourceForm((prev: SourceForm) => ({ ...prev, name: val }))}
          />

          <TextInput
            label={c.labelCategory}
            description={c.labelCategoryHint}
            placeholder={c.placeholderCategory}
            value={sourceForm.category}
            onChange={val => setSourceForm((prev: SourceForm) => ({ ...prev, category: val }))}
          />

          <div className="grid grid-cols-2 gap-4">
            <Selector
              label={c.labelType}
              options={[
                { value: "rss", label: c.typeRssOption },
                { value: "api", label: c.typeApiOption },
                { value: "web_scrape", label: c.typeWebOption }
              ]}
              value={sourceForm.type}
              onChange={val => val && setSourceForm((prev: SourceForm) => ({ ...prev, type: val }))}
            />

            <Selector
              label={c.labelDefaultCategory}
              options={[
                { value: "auto", label: c.categoryAuto },
                { value: "article", label: c.categoryArticle },
                { value: "tweet", label: c.categoryTweet },
                { value: "paper", label: c.categoryPaper },
                { value: "project", label: c.categoryProject }
              ]}
              value={sourceForm.default_category}
              onChange={val => val && setSourceForm((prev: SourceForm) => ({ ...prev, default_category: val }))}
            />
          </div>

          <TextInput
            label={c.labelUrl}
            placeholder={c.placeholderUrl}
            value={sourceForm.url}
            onChange={val => setSourceForm((prev: SourceForm) => ({ ...prev, url: val }))}
          />

          <TextInput
            label={c.labelSchedule}
            placeholder={c.placeholderSchedule}
            value={sourceForm.schedule}
            onChange={val => setSourceForm((prev: SourceForm) => ({ ...prev, schedule: val }))}
          />

          <TextArea
            label={c.labelConfig}
            description={c.labelConfigHint}
            placeholder="{}"
            value={sourceForm.config}
            onChange={val => setSourceForm((prev: SourceForm) => ({ ...prev, config: val }))}
            rows={3}
          />
        </VStack>

        <HStack gap={3} hAlign="end" className="pt-4 mt-2 border-t border-border shrink-0">
          <button
            type="button"
            className="g-btn g-btn-ghost g-btn-sm"
            onClick={() => setIsSourceDialogOpen(false)}
          >
            {c.cancel}
          </button>
          <button type="submit" className="g-btn g-btn-primary g-btn-sm">
            {c.save}
          </button>
        </HStack>
      </form>
    </Dialog>
  );
}
