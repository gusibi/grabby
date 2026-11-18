import { Dialog, DialogHeader } from "@astryxdesign/core/Dialog";
import { TextInput } from "@astryxdesign/core/TextInput";
import { TextArea } from "@astryxdesign/core/TextArea";
import { Selector } from "@astryxdesign/core/Selector";
import { VStack, HStack } from "@astryxdesign/core/Layout";
import { Toggle } from "@/components/ui/Settings";
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

  let parsedConfig: Record<string, unknown> = {};
  try {
    parsedConfig = sourceForm.config ? JSON.parse(sourceForm.config) : {};
  } catch {
    parsedConfig = {};
  }
  const fullContentEnabled = Boolean(parsedConfig.full_content || parsedConfig.fetch_full_via_scrape);
  // Audio is only fetched when the user explicitly opts in; scraping podcast
  // sources must never download audio in the background.
  const transcribeAudioEnabled = parsedConfig.transcribe_audio === true && sourceForm.type !== "podcast";

  const updateConfigFlag = (key: string, value: boolean) => {
    setSourceForm((prev: SourceForm) => {
      let next: Record<string, unknown> = {};
      try {
        next = prev.config ? JSON.parse(prev.config) : {};
      } catch {
        next = {};
      }
      next[key] = value;
      return { ...prev, config: JSON.stringify(next) };
    });
  };

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
                { value: "podcast", label: c.typePodcastOption },
                { value: "youtube", label: c.typeYoutubeOption },
                { value: "api", label: c.typeApiOption },
                { value: "web_scrape", label: c.typeWebOption }
              ]}
              value={sourceForm.type}
              onChange={val => val && setSourceForm((prev: SourceForm) => {
                const next = { ...prev, type: val };
                // Podcast / YouTube sources already imply a content type; mirror
                // it so the feed filters pick new items up immediately.
                if (val === "podcast" || val === "youtube") {
                  next.default_category = val;
                  try {
                    const cfg = prev.config ? JSON.parse(prev.config) : {};
                    cfg.transcribe_audio = false;
                    next.config = JSON.stringify(cfg);
                  } catch {
                    next.config = prev.config;
                  }
                } else if (prev.default_category === "podcast" || prev.default_category === "youtube") {
                  next.default_category = "auto";
                }
                return next;
              })}
            />

            <Selector
              label={c.labelDefaultCategory}
              options={[
                { value: "auto", label: c.categoryAuto },
                { value: "article", label: c.categoryArticle },
                { value: "blog", label: c.categoryBlog },
                { value: "youtube", label: c.categoryYoutube },
                { value: "podcast", label: c.categoryPodcast },
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

          <div className="space-y-3 rounded-xl p-3" style={{ background: "var(--g-panel-2)", border: "1px solid var(--g-line)" }}>
            <div className="flex items-center justify-between gap-3">
              <div>
                <div className="text-[12px] font-medium">{c.toggleFullContent}</div>
                <div className="text-[10.5px]" style={{ color: "var(--g-faint)" }}>{c.toggleFullContentHint}</div>
              </div>
              <Toggle checked={fullContentEnabled} onChange={(v) => updateConfigFlag("full_content", v)} label={c.toggleFullContent} />
            </div>
            <div className="flex items-center justify-between gap-3">
              <div>
                <div className="text-[12px] font-medium">{c.toggleTranscribeAudio}</div>
                <div className="text-[10.5px]" style={{ color: "var(--g-faint)" }}>{c.toggleTranscribeAudioHint}</div>
              </div>
              {sourceForm.type === "podcast" ? (
                <span className="text-[10.5px]" style={{ color: "var(--g-faint)" }}>{c.playbackOnlyHint}</span>
              ) : (
                <Toggle checked={transcribeAudioEnabled} onChange={(v) => updateConfigFlag("transcribe_audio", v)} label={c.toggleTranscribeAudio} />
              )}
            </div>
          </div>

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
