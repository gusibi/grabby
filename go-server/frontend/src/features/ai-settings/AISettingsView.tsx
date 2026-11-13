/* eslint-disable react-hooks/set-state-in-effect -- settings form syncs props → local field state */
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { ReactNode } from "react";
import {
  ArrowDown,
  ArrowUp,
  BookOpen,
  Check,
  ChevronDown,
  ChevronRight,
  CircleDot,
  Clock,
  Cloud,
  Copy,
  Cpu,
  Download,
  ExternalLink,
  Eye,
  EyeOff,
  HardDrive,
  Layers,
  Loader2,
  Network,
  Play,
  Plus,
  RotateCcw,
  Search,
  Shield,
  Shuffle,
  Sparkles,
  Terminal,
  Trash2,
  Upload,
  WandSparkles,
  X,
  Zap,
} from "lucide-react";
import type { AIProviderProfile, AITestResult, AIUsageSummary } from "@/types";
import {
  defaultModelRef,
  providerDescription,
  providerName,
  findModelInfo,
  fromModelRef,
  kindLabel,
  resolvePresetFrom,
  toModelRef,
  useProviderCatalog,
} from "@/lib/model-catalog";
import type { ModelInfo, ProviderKind, ProviderPreset } from "@/lib/model-catalog";
import { api } from "@/lib/api";
import { Mark, ScoreRing, SettingSection, Toggle } from "@/components/ui/Settings";
import { useFeedback } from "@/components/ui/feedback-context";
import { useTranslation } from "@/lib/i18n";
import { useAiSettingsCopy } from "./copy";
import type { AISettingsCopy } from "./copy";

interface AISettingsViewProps {
  isLoadingSettings: boolean;
  handleStartEvaluation: () => void;
  aiEnabled: boolean;
  setAiEnabled: (value: boolean) => void;
  activeProfileId: string;
  handleSelectAIProfile: (profileID: string) => void;
  aiProfiles: AIProviderProfile[];
  handleAddAIProfile: () => void;
  handleDeleteAIProfile: () => void;
  aiStrategy: string;
  setAiStrategy: (value: string) => void;
  setAiProfiles: (value: AIProviderProfile[]) => void;
  aiProfileName: string;
  handleProfileNameChange: (name: string) => void;
  aiProvider: string;
  setAiProvider: (value: string) => void;
  aiQualityThreshold: number;
  setAiQualityThreshold: (value: number) => void;
  aiModels: string[];
  setAiModels: (value: string[]) => void;
  aiApiKey: string;
  setAiApiKey: (value: string) => void;
  aiRequestsPerMinute: number;
  setAiRequestsPerMinute: (value: number) => void;
  aiBaseUrl: string;
  setAiBaseUrl: (value: string) => void;
  aiSystemPrompt: string;
  setAiSystemPrompt: (value: string) => void;
  aiDailyPrompt: string;
  setAiDailyPrompt: (value: string) => void;
  morningReportEnabled: boolean;
  setMorningReportEnabled: (value: boolean) => void;
  morningReportTime: string;
  setMorningReportTime: (value: string) => void;
  eveningReportEnabled: boolean;
  setEveningReportEnabled: (value: boolean) => void;
  eveningReportTime: string;
  setEveningReportTime: (value: string) => void;
  aiTestResult: AITestResult | null;
  handleTestAI: (draft?: { profile: AIProviderProfile; system_prompt: string }) => void;
  isTestingAI: boolean;
  isSavingSettings: boolean;
  saveAISettings: () => void;
  resetAISettings: () => void;
  reloadAISettings: () => Promise<void> | void;
  isAuthenticated: boolean;
}

const kindIcon: Record<ProviderKind, ReactNode> = {
  cloud: <Cloud size={11} />,
  local: <HardDrive size={11} />,
  aggregator: <Network size={11} />,
};

const thresholdCopy = (c: AISettingsCopy): Record<number, string> => ({
  5: c.threshold5,
  6: c.threshold6,
  7: c.threshold7,
  8: c.threshold8,
  9: c.threshold9,
});

const systemVariables = ["{{.Title}}", "{{.OriginSource}}", "{{.Summary}}", "{{.Content}}"];
const dailyVariables = ["{{.Count}}", "{{.FeedText}}", "{{.TotalItems}}", "{{.QualityItems}}"];

function FieldLabel({ children, hint }: { children: ReactNode; hint?: string }) {
  return (
    <div className="flex items-baseline justify-between gap-3 mb-2">
      <span className="g-label">{children}</span>
      {hint && <span className="g-mono text-[10px]" style={{ color: "var(--g-faint)" }}>{hint}</span>}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* Provider tile                                                       */
/* ------------------------------------------------------------------ */

function ProviderTile({ preset, active, onClick }: { preset: ProviderPreset; active: boolean; onClick: () => void }) {
  const { language } = useTranslation();
  return (
    <button
      type="button"
      onClick={onClick}
      className="group relative flex flex-col gap-2 p-2.5 rounded-xl text-left transition-all duration-300"
      style={{
        background: active ? "var(--g-signal-soft)" : "var(--g-panel)",
        border: `1px solid ${active ? "var(--g-signal)" : "var(--g-line)"}`,
        boxShadow: active ? `0 0 0 3px color-mix(in srgb, var(--g-signal) 12%, transparent)` : "none",
      }}
    >
      <div className="flex items-start justify-between gap-2">
        <Mark label={preset.mark} color={preset.color} size="sm" />
        {active ? (
          <span
            className="w-4 h-4 rounded-full flex items-center justify-center"
            style={{ background: "var(--g-signal)", color: "var(--g-on-signal)" }}
          >
            <Check size={10} strokeWidth={3} />
          </span>
        ) : (
          <span className="g-mono text-[9px] flex items-center gap-1" style={{ color: "var(--g-faint)" }}>
            {kindIcon[preset.kind]}
            {kindLabel[preset.kind][language]}
          </span>
        )}
      </div>
      <div className="min-w-0">
        <div className="text-[11.5px] font-semibold truncate" style={{ color: "var(--g-ink)" }}>{providerName(preset, language)}</div>
        <div className="mt-0.5 text-[9.5px] leading-snug truncate" style={{ color: "var(--g-faint)" }}>
          {providerDescription(preset, language)}
        </div>
      </div>
    </button>
  );
}

/* ------------------------------------------------------------------ */
/* Model combobox                                                      */
/* ------------------------------------------------------------------ */

function ModelPicker({
  preset,
  modelRef,
  onChange,
  mode = "edit",
  label,
  hint,
}: {
  preset: ProviderPreset;
  modelRef: string;
  onChange: (ref: string) => void;
  mode?: "edit" | "add";
  label?: string;
  hint?: string;
}) {
  const c = useAiSettingsCopy();
  const [query, setQuery] = useState(() => fromModelRef(modelRef));
  const [open, setOpen] = useState(false);
  const [manual, setManual] = useState(false);
  const wrapRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setQuery(fromModelRef(modelRef));
  }, [modelRef, preset.id]);

  useEffect(() => {
    const onDocClick = (e: MouseEvent) => {
      if (wrapRef.current && !wrapRef.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", onDocClick);
    return () => document.removeEventListener("mousedown", onDocClick);
  }, []);

  const commit = (shortId: string) => {
    setQuery(shortId);
    onChange(toModelRef(preset.backend, shortId));
    setOpen(false);
  };

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return preset.models;
    return preset.models.filter(
      (m) => m.name.toLowerCase().includes(q) || m.id.toLowerCase().includes(q) || (m.capabilities || []).some((c) => c.toLowerCase().includes(q))
    );
  }, [preset, query]);

  const selected = findModelInfo(preset, modelRef);
  const isKnown = !!selected;

  return (
    <div ref={wrapRef} className="relative">
      <FieldLabel hint={hint ?? (isKnown ? c.modelHintMatched : c.modelHintCustom)}>{label ?? c.modelLabel}</FieldLabel>
      <div className="relative">
        <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" style={{ color: "var(--g-faint)" }} />
        <input
          className="g-input mono pl-9 pr-24"
          value={query}
          placeholder={preset.id === "custom" ? c.modelSearchCustom : c.modelSearch}
          onFocus={() => setOpen(true)}
          onChange={(e) => {
            const v = e.target.value;
            setQuery(v);
            setOpen(true);
            // In "add" mode the parent only receives explicit commits, so
            // typing does not append a model on every keystroke.
            if (mode === "edit" && (manual || preset.id === "custom" || preset.models.length === 0)) {
              onChange(toModelRef(preset.backend, v));
            }
          }}
          onKeyDown={(e) => {
            if (e.key === "Enter" && open && filtered.length > 0 && !manual) {
              e.preventDefault();
              commit(filtered[0].id);
            }
            if (e.key === "Escape") setOpen(false);
          }}
          spellCheck={false}
        />
        <div className="absolute right-2 top-1/2 -translate-y-1/2 flex items-center gap-1">
          <button
            type="button"
            title={c.modelManualToggle}
            onClick={() => {
              setManual((v) => !v);
              setOpen(true);
            }}
            className="g-mono text-[10px] px-1.5 py-1 rounded-md transition-colors"
            style={{
              background: manual ? "var(--g-signal-soft)" : "transparent",
              color: manual ? "var(--g-signal)" : "var(--g-faint)",
            }}
          >
            ID
          </button>
          <ChevronDown size={14} style={{ color: "var(--g-faint)", transform: open ? "rotate(180deg)" : "none", transition: "transform 200ms" }} />
        </div>
      </div>

      {open && (
        <div
          className="g-pop absolute z-30 left-0 right-0 mt-2 rounded-xl overflow-hidden g-scroll"
          style={{
            background: "var(--g-panel)",
            border: "1px solid var(--g-line-strong)",
            boxShadow: "var(--g-shadow-lift)",
            maxHeight: 292,
            overflowY: "auto",
          }}
        >
          {filtered.length > 0 ? (
            <>
              {filtered.slice(0, 80).map((model) => (
                <ModelOption
                  key={model.id}
                  model={model}
                  active={fromModelRef(modelRef) === model.id}
                  onSelect={() => commit(model.id)}
                />
              ))}
              {filtered.length > 80 && (
                <div className="px-3.5 py-2.5 text-[10.5px]" style={{ color: "var(--g-faint)", borderTop: "1px solid var(--g-line)" }}>
                  仅显示前 80 个，继续输入可缩小范围
                </div>
              )}
            </>
          ) : (
            <button
              type="button"
              className="w-full px-3.5 py-3 text-left text-[12px] flex items-center gap-2"
              onClick={() => commit(query)}
              style={{ color: "var(--g-muted)" }}
            >
              <Plus size={13} /> {c.modelUseCustomId}<span className="g-mono" style={{ color: "var(--g-ink)" }}>{query || "…"}</span>
            </button>
          )}
          {mode === "add" && query.trim() !== "" && (
            <button
              type="button"
              className="w-full px-3.5 py-3 text-left text-[12px] flex items-center gap-2"
              onClick={() => commit(query.trim())}
              style={{ color: "var(--g-signal)", borderTop: "1px solid var(--g-line)" }}
            >
              <Plus size={13} /> {c.modelsAddCustom}
              <span className="g-mono" style={{ color: "var(--g-ink)" }}>{query.trim()}</span>
            </button>
          )}
          {preset.id === "custom" && (
            <div className="px-3.5 py-3 text-[11px] leading-relaxed" style={{ color: "var(--g-faint)", borderTop: "1px solid var(--g-line)" }}>
              {c.modelCustomNote}
            </div>
          )}
        </div>
      )}

      <div className="mt-2 flex items-center justify-between gap-3">
        <span className="g-hint">
          {selected ? (
            <>
              <span style={{ color: "var(--g-signal)" }}>●</span> {selected.name}
              {selected.context ? ` · ${selected.context} ${c.modelContextUnit}` : ""}
            </>
          ) : (
            c.modelHintNamespace
          )}
        </span>
        <span className="g-mono text-[10px] shrink-0" style={{ color: "var(--g-faint)" }}>{modelRef || "—"}</span>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* Multi-model list (one provider can hold several models)             */
/* ------------------------------------------------------------------ */

function ModelListEditor({
  preset,
  models,
  onChange,
  action,
}: {
  preset: ProviderPreset;
  models: string[];
  onChange: (models: string[]) => void;
  action?: ReactNode;
}) {
  const c = useAiSettingsCopy();

  const add = (ref: string) => {
    const value = ref.trim();
    if (!value) return;
    if (models.some((m) => m.toLowerCase() === value.toLowerCase())) return;
    onChange([...models, value]);
  };

  const remove = (index: number) => onChange(models.filter((_, i) => i !== index));

  const move = (index: number, direction: -1 | 1) => {
    const target = index + direction;
    if (target < 0 || target >= models.length) return;
    const next = [...models];
    [next[index], next[target]] = [next[target], next[index]];
    onChange(next);
  };

  return (
    <div className="g-field">
      <div className="flex items-center justify-between gap-3 mb-2">
        <FieldLabel hint={c.modelsHint}>{c.modelsLabel}</FieldLabel>
        {action}
      </div>

      {models.length === 0 ? (
        <div
          className="rounded-lg px-3 py-2.5 text-[11.5px]"
          style={{ border: "1px dashed var(--g-line-strong)", color: "var(--g-faint)" }}
        >
          {c.modelsEmpty}
        </div>
      ) : (
        <div className="space-y-1.5">
          {models.map((model, index) => (
            <div
              key={`${model}-${index}`}
              className="flex items-center gap-2 rounded-lg px-2.5 py-1.5"
              style={{ border: "1px solid var(--g-line)", background: index === 0 ? "var(--g-signal-soft)" : "transparent" }}
            >
              <span className="g-mono text-[11px] flex-1 truncate" style={{ color: "var(--g-ink)" }}>
                {fromModelRef(model) || model}
              </span>
              {index === 0 && (
                <span className="g-chip !h-[18px] !text-[9px] shrink-0">{c.modelsPrimary}</span>
              )}
              <button
                type="button"
                className="g-icon-btn !w-5 !h-5 disabled:opacity-30"
                title={c.moveUp}
                disabled={index === 0}
                onClick={() => move(index, -1)}
              >
                <ArrowUp size={11} />
              </button>
              <button
                type="button"
                className="g-icon-btn !w-5 !h-5 disabled:opacity-30"
                title={c.moveDown}
                disabled={index === models.length - 1}
                onClick={() => move(index, 1)}
              >
                <ArrowDown size={11} />
              </button>
              <button
                type="button"
                className="g-icon-btn !w-5 !h-5"
                title={c.modelsRemove}
                onClick={() => remove(index)}
              >
                <X size={11} />
              </button>
            </div>
          ))}
        </div>
      )}

      <div className="mt-2">
        <ModelPicker
          key={`add-${models.length}`}
          mode="add"
          preset={preset}
          modelRef=""
          onChange={add}
          label={c.modelsAddLabel}
          hint={c.modelHintNamespace}
        />
      </div>

      <span className="g-hint">{c.modelsRotationHint}</span>
    </div>
  );
}

function ModelOption({ model, active, onSelect }: { model: ModelInfo; active: boolean; onSelect: () => void }) {
  const c = useAiSettingsCopy();
  return (
    <button
      type="button"
      onClick={onSelect}
      className="w-full flex items-start gap-3 px-3.5 py-2.5 text-left transition-colors"
      style={{ background: active ? "var(--g-signal-soft)" : "transparent", borderBottom: "1px solid var(--g-line)" }}
    >
      <span
        className="mt-1 w-3.5 h-3.5 rounded-full border flex items-center justify-center shrink-0"
        style={{ borderColor: active ? "var(--g-signal)" : "var(--g-line-strong)" }}
      >
        {active && <span className="w-1.5 h-1.5 rounded-full" style={{ background: "var(--g-signal)" }} />}
      </span>
      <span className="min-w-0 flex-1">
        <span className="flex items-center gap-2 flex-wrap">
          <span className="text-[12px] font-semibold" style={{ color: "var(--g-ink)" }}>{model.name}</span>
          {model.recommended && <span className="g-chip g-chip-signal !h-[18px] !text-[9px]">{c.modelRecommended}</span>}
          {(model.capabilities || []).map((c) => (
            <span key={c} className="g-chip !h-[18px] !text-[9px]">{c}</span>
          ))}
        </span>
        <span className="block mt-1 g-mono text-[10px] truncate" style={{ color: "var(--g-faint)" }}>{model.id}</span>
      </span>
      <span className="g-mono text-[10px] shrink-0 pt-0.5" style={{ color: "var(--g-faint)" }}>
        {model.context ? `${model.context} · ` : ""}
        {model.input ? `$${model.input}/M` : "—"}
      </span>
    </button>
  );
}

/* ------------------------------------------------------------------ */
/* Prompt editor                                                       */
/* ------------------------------------------------------------------ */

function PromptEditor({
  id,
  title,
  description,
  value,
  onChange,
  variables,
  open,
  onToggle,
}: {
  id: string;
  title: string;
  description: string;
  value: string;
  onChange: (v: string) => void;
  variables: string[];
  open: boolean;
  onToggle: () => void;
}) {
  const c = useAiSettingsCopy();
  const ref = useRef<HTMLTextAreaElement>(null);

  const insert = (variable: string) => {
    const el = ref.current;
    if (!el) {
      onChange(value + variable);
      return;
    }
    const start = el.selectionStart ?? value.length;
    const end = el.selectionEnd ?? value.length;
    const next = value.slice(0, start) + variable + value.slice(end);
    onChange(next);
    requestAnimationFrame(() => {
      el.focus();
      el.setSelectionRange(start + variable.length, start + variable.length);
    });
  };

  return (
    <div className="rounded-xl overflow-hidden" style={{ border: "1px solid var(--g-line)" }}>
      <button
        type="button"
        onClick={onToggle}
        className="w-full flex items-center justify-between gap-4 px-4 py-3.5 text-left transition-colors"
        style={{ background: "var(--g-panel-2)" }}
      >
        <span className="flex items-center gap-3 min-w-0">
          <ChevronRight
            size={15}
            style={{ color: "var(--g-faint)", transform: open ? "rotate(90deg)" : "none", transition: "transform 260ms var(--g-ease)" }}
          />
          <span className="min-w-0">
            <span className="block text-[12.5px] font-semibold">{title}</span>
            <span className="block text-[10.5px] mt-0.5 truncate" style={{ color: "var(--g-faint)" }}>{description}</span>
          </span>
        </span>
        <span className="g-mono text-[10px] shrink-0" style={{ color: "var(--g-faint)" }}>{c.promptChars(value.length)}</span>
      </button>
      {open && (
        <div className="p-4 g-fade">
          <textarea
            id={id}
            ref={ref}
            className="g-textarea"
            value={value}
            onChange={(e) => onChange(e.target.value)}
            spellCheck={false}
            placeholder={c.promptPlaceholder}
          />
          <div className="mt-3 flex items-center gap-2 flex-wrap">
            <span className="g-eyebrow mr-1">{c.promptVars}</span>
            {variables.map((v) => (
              <button key={v} type="button" className="g-chip g-mono hover:opacity-70 transition-opacity" onClick={() => insert(v)}>
                <Plus size={10} /> {v}
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* Test panel                                                          */
/* ------------------------------------------------------------------ */

function TestPanel({
  result,
  running,
  onRun,
  canRun,
  disabledReason,
}: {
  result: AITestResult | null;
  running: boolean;
  onRun: () => void;
  canRun: boolean;
  disabledReason?: string;
}) {
  const c = useAiSettingsCopy();
  const analysis = result?.success ? result.analysis : null;
  return (
    <div
      className="rounded-2xl overflow-hidden g-rise"
      style={{ background: "#101310", border: "1px solid #262c24", color: "#eef2e8" }}
    >
      <div
        className="px-6 py-5 flex flex-wrap items-center justify-between gap-4"
        style={{ borderBottom: "1px solid #262c24", backgroundImage: "radial-gradient(circle at 85% -40%, rgba(201,242,103,.14), transparent 55%)" }}
      >
        <div className="flex items-center gap-3.5">
          <span className="w-9 h-9 rounded-xl flex items-center justify-center" style={{ background: "rgba(201,242,103,.12)", color: "#c9f267" }}>
            <Terminal size={17} />
          </span>
          <div>
            <div className="text-[13px] font-semibold tracking-tight">{c.testTitle}</div>
            <div className="text-[11px] mt-0.5" style={{ color: disabledReason ? "#e9be68" : "#8e9789" }}>
              {disabledReason ?? c.testDesc}
            </div>
          </div>
        </div>
        <button className="g-btn g-btn-primary g-btn-sm" onClick={onRun} disabled={running || !canRun}>
          {running ? <Loader2 size={13} className="g-spin" /> : <Play size={12} fill="currentColor" />}
          {running ? c.testRunning : c.testRun}
        </button>
      </div>

      <div className="px-6 py-5 min-h-[132px]">
        {running ? (
          <div className="flex items-center gap-3 text-[12px]" style={{ color: "#8e9789" }}>
            <Loader2 size={15} className="g-spin" style={{ color: "#c9f267" }} />
            <span>{c.testConnecting}</span>
          </div>
        ) : result?.success ? (
          <div className="g-fade">
            <div className="flex flex-wrap items-center gap-x-5 gap-y-2 text-[11px] g-mono" style={{ color: "#8e9789" }}>
              <span className="flex items-center gap-1.5" style={{ color: "#c9f267" }}><CircleDot size={12} /> {c.testSuccess}</span>
              {result.latency_ms != null && <span>{c.testLatency(result.latency_ms)}</span>}
              <span>{result.model_used || "—"}</span>
              {result.item_title && <span className="truncate max-w-[260px]">{c.testSample(result.item_title)}</span>}
            </div>
            {analysis && (
              <div className="mt-4 grid grid-cols-1 md:grid-cols-[auto_1fr] gap-5 items-start">
                <div className="flex items-center gap-3">
                  <ScoreRing score={Number(analysis.quality_score) || 0} />
                  <div>
                    <div className="g-eyebrow !text-[#8e9789]">{c.testScore}</div>
                    <div className="text-[12px] mt-1">{analysis.ai_category || c.testUnknownCategory}{analysis.ai_subcategory ? ` / ${analysis.ai_subcategory}` : ""}</div>
                  </div>
                </div>
                <div className="space-y-2.5 text-[12px] leading-relaxed" style={{ color: "#c6cdc0" }}>
                  {analysis.ai_summary && <p><span style={{ color: "#c9f267" }}>{c.testSummaryLabel} </span>{analysis.ai_summary}</p>}
                  {analysis.ai_comment && <p style={{ color: "#8e9789" }}>{analysis.ai_comment}</p>}
                </div>
              </div>
            )}
          </div>
        ) : result && !result.success ? (
          <div className="g-fade">
            <div className="flex items-center gap-2 text-[12px] font-semibold" style={{ color: "#ff8878" }}>
              <X size={14} /> {c.testFailed}
            </div>
            <pre
              className="mt-3 p-3 rounded-lg g-mono text-[11px] leading-relaxed whitespace-pre-wrap break-all"
              style={{ background: "rgba(255,136,120,.07)", color: "#ffb3a8", border: "1px solid rgba(255,136,120,.18)" }}
            >
              {result.error}
            </pre>
          </div>
        ) : (
          <div className="flex items-center gap-3 text-[11.5px]" style={{ color: "#69725f" }}>
            <span className="w-8 h-8 rounded-lg flex items-center justify-center" style={{ border: "1px dashed #394133" }}>
              <Sparkles size={14} />
            </span>
            {c.testIdle}
          </div>
        )}
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* Main view                                                           */
/* ------------------------------------------------------------------ */

export function AISettingsView(props: AISettingsViewProps) {
  const {
    isLoadingSettings,
    handleStartEvaluation,
    aiEnabled,
    setAiEnabled,
    activeProfileId,
    handleSelectAIProfile,
    aiProfiles,
    handleAddAIProfile,
    handleDeleteAIProfile,
    aiStrategy,
    setAiStrategy,
    setAiProfiles,
    aiProfileName,
    handleProfileNameChange,
    aiProvider,
    setAiProvider,
    aiQualityThreshold,
    setAiQualityThreshold,
    aiModels,
    setAiModels,
    aiApiKey,
    setAiApiKey,
    aiRequestsPerMinute,
    setAiRequestsPerMinute,
    aiBaseUrl,
    setAiBaseUrl,
    aiSystemPrompt,
    setAiSystemPrompt,
    aiDailyPrompt,
    setAiDailyPrompt,
    morningReportEnabled,
    setMorningReportEnabled,
    morningReportTime,
    setMorningReportTime,
    eveningReportEnabled,
    setEveningReportEnabled,
    eveningReportTime,
    setEveningReportTime,
    aiTestResult,
    handleTestAI,
    isTestingAI,
    isSavingSettings,
    saveAISettings,
    resetAISettings,
    reloadAISettings,
    isAuthenticated,
  } = props;

  const { toast, confirm } = useFeedback();
  const { language } = useTranslation();
  const c = useAiSettingsCopy();
  const catalog = useProviderCatalog();
  const [providerFilter, setProviderFilter] = useState<"all" | ProviderKind>("all");
  const [providerQuery, setProviderQuery] = useState("");
  const [showKey, setShowKey] = useState(false);
  const [openPrompt, setOpenPrompt] = useState<"system" | "daily" | null>(null);
  const [baseline, setBaseline] = useState<string | null>(null);
  const [discovering, setDiscovering] = useState(false);
  const [importing, setImporting] = useState(false);
  const [liveModels, setLiveModels] = useState<Record<string, ModelInfo[]>>({});
  const [usage, setUsage] = useState<AIUsageSummary | null>(null);

  const preset = useMemo(() => resolvePresetFrom(catalog.presets, aiProvider, aiBaseUrl), [catalog.presets, aiProvider, aiBaseUrl]);
  const effectivePreset = useMemo(() => {
    const discovered = liveModels[preset.id];
    return discovered && discovered.length > 0 ? { ...preset, models: discovered } : preset;
  }, [preset, liveModels]);
  const keyMissing = (aiProvider === "openai" || aiProvider === "gemini") && !aiApiKey.trim();
  const sortedProfiles = useMemo(
    () => [...aiProfiles].sort((a, b) => (a.priority || 999) - (b.priority || 999)),
    [aiProfiles]
  );

  const signature = JSON.stringify({
    aiEnabled, activeProfileId, aiProfiles, aiStrategy, aiProfileName, aiProvider,
    aiModels, aiApiKey, aiBaseUrl, aiRequestsPerMinute, aiQualityThreshold,
    aiSystemPrompt, aiDailyPrompt, morningReportEnabled, morningReportTime,
    eveningReportEnabled, eveningReportTime,
  });

  useEffect(() => {
    if (!isLoadingSettings && baseline === null) setBaseline(signature);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isLoadingSettings, signature]);

  useEffect(() => {
    let cancelled = false;
    api.getAIUsage(7)
      .then((data) => {
        if (!cancelled && data?.success && data.summary) setUsage(data.summary as AIUsageSummary);
      })
      .catch(() => { /* usage is optional */ });
    return () => { cancelled = true; };
  }, []);

  const isDirty = baseline !== null && signature !== baseline;

  const handleSave = useCallback(() => {
    saveAISettings();
    setBaseline(signature);
  }, [saveAISettings, signature]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "s") {
        e.preventDefault();
        if (isAuthenticated && !isSavingSettings) handleSave();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [handleSave, isAuthenticated, isSavingSettings]);

  const updateActiveProfile = (patch: Partial<AIProviderProfile>) => {
    setAiProfiles(aiProfiles.map((p) => (p.id === activeProfileId ? { ...p, ...patch } : p)));
  };

  const handlePresetChange = (next: ProviderPreset) => {
    if (next.id === preset.id) return;
    const model = defaultModelRef(next);
    setAiProvider(next.backend);
    setAiBaseUrl(next.baseUrl);
    setAiModels(model ? [model] : []);
    setAiApiKey("");
    setShowKey(false);
    toast(
      c.toastProviderSwitched(providerName(next, language)),
      next.id === "custom" || next.id === "lmstudio" || next.id === "ollama"
        ? c.toastProviderCustom
        : c.toastProviderPreset
    );
  };

  const handleRpmChange = (value: number) => {
    const rpm = Math.max(1, Math.min(120, value));
    setAiRequestsPerMinute(rpm);
    updateActiveProfile({ requests_per_minute: rpm });
  };

  const handleDiscoverModels = async () => {
    setDiscovering(true);
    try {
      const data = await api.discoverAIModels({
        provider: aiProvider,
        base_url: aiBaseUrl,
        api_key: aiApiKey,
        profile_id: activeProfileId,
      });
      if (data?.success && Array.isArray(data.models) && data.models.length > 0) {
        const models: ModelInfo[] = data.models.map((model: { id: string; name?: string }) => ({
          id: model.id,
          name: model.name || model.id,
          capabilities: [],
        }));
        setLiveModels((previous) => ({ ...previous, [preset.id]: models }));
        if (aiModels.length === 0) {
          setAiModels([toModelRef(preset.backend, models[0].id)]);
        }
        toast(c.discoverDoneTitle, c.discoverDoneBody(models.length), "success");
      } else {
        toast(c.discoverFailedTitle, data?.error || c.discoverFailedBody, "error");
      }
    } catch (err) {
      toast(c.discoverFailedTitle, err instanceof Error ? err.message : c.discoverFailedBody, "error");
    } finally {
      setDiscovering(false);
    }
  };

  const handleImportConfig = async () => {
    setImporting(true);
    try {
      const data = await api.importAIConfig("auto", true);
      if (data?.success) {
        toast(
          c.importDoneTitle,
          data.imported > 0 ? c.importDoneBody(data.imported) : c.importEmptyBody,
          data.imported > 0 ? "success" : "info"
        );
        if (data.imported > 0) await reloadAISettings();
      } else {
        toast(c.importFailedTitle, data?.error || c.importFailedBody, "error");
      }
    } catch (err) {
      toast(c.importFailedTitle, err instanceof Error ? err.message : c.importFailedBody, "error");
    } finally {
      setImporting(false);
    }
  };

  const runTest = () => {
    const draft: AIProviderProfile = {
      id: activeProfileId || "default",
      name: aiProfileName,
      provider: aiProvider,
      api_key: aiApiKey,
      model: aiModels[0] || "",
      models: aiModels,
      base_url: aiBaseUrl,
      requests_per_minute: aiRequestsPerMinute,
      disabled: false,
    };
    handleTestAI({ profile: draft, system_prompt: aiSystemPrompt });
  };

  const moveProfile = (id: string, direction: -1 | 1) => {
    const ordered = [...sortedProfiles];
    const index = ordered.findIndex((p) => p.id === id);
    const target = index + direction;
    if (index < 0 || target < 0 || target >= ordered.length) return;
    [ordered[index], ordered[target]] = [ordered[target], ordered[index]];
    setAiProfiles(ordered.map((p, i) => ({ ...p, priority: i + 1 })));
  };

  const filteredPresets = useMemo(() => {
    const q = providerQuery.trim().toLowerCase();
    return catalog.presets.filter((p) => {
      if (providerFilter !== "all" && p.kind !== providerFilter) return false;
      if (!q) return true;
      return p.name.toLowerCase().includes(q) || p.legalName.toLowerCase().includes(q) || p.id.includes(q);
    });
  }, [catalog.presets, providerFilter, providerQuery]);

  const visiblePresets = useMemo(() => filteredPresets.slice(0, 60), [filteredPresets]);

  if (isLoadingSettings) {
    return (
      <div className="flex-1 flex items-center justify-center" style={{ background: "var(--g-paper)" }}>
        <div className="flex flex-col items-center gap-3">
          <Loader2 size={22} className="g-spin" style={{ color: "var(--g-signal)" }} />
          <span className="g-eyebrow">{c.loading}</span>
        </div>
      </div>
    );
  }

  return (
    <div className="g-scroll flex-1 overflow-y-auto" style={{ background: "var(--g-paper)" }}>
      <div className="max-w-[1320px] mx-auto px-10 pt-9 pb-8">
        {/* Masthead */}
        <header className="g-rise flex flex-wrap items-end justify-between gap-6">
          <div>
            <div className="g-eyebrow">{c.kicker}</div>
            <h1 className="g-display mt-3 text-[clamp(2.4rem,4vw,3.4rem)]">{c.title}</h1>
            <p className="mt-3 max-w-xl text-[13px] leading-relaxed" style={{ color: "var(--g-muted)" }}>
              {c.lede}
            </p>
          </div>
          <div className="flex items-center gap-2.5">
            <span
              className="g-chip !h-8 !px-3"
              style={{
                background: aiEnabled ? "var(--g-signal-soft)" : "var(--g-panel-2)",
                color: aiEnabled ? "var(--g-signal)" : "var(--g-muted)",
                borderColor: aiEnabled ? "color-mix(in srgb, var(--g-signal) 30%, transparent)" : "var(--g-line)",
              }}
            >
              <span
                className={`w-1.5 h-1.5 rounded-full ${aiEnabled ? "g-pulse" : ""}`}
                style={{ background: aiEnabled ? "var(--g-signal)" : "var(--g-faint)" }}
              />
              {aiEnabled ? c.engineRunning : c.enginePaused}
            </span>
            <a
              className="g-btn g-btn-secondary g-btn-sm"
              href="https://models.dev"
              target="_blank"
              rel="noreferrer"
            >
              <BookOpen size={13} /> {c.catalog} <ExternalLink size={11} />
            </a>
          </div>
        </header>

        {/* Meta strip */}
        <div
          className="g-rise mt-8 flex flex-wrap items-center gap-x-10 gap-y-3 py-3.5 g-mono text-[10px]"
          style={{ borderTop: "1px solid var(--g-line)", borderBottom: "1px solid var(--g-line)", color: "var(--g-faint)" }}
        >
          <span>
            {c.metaProfiles} <b style={{ color: "var(--g-ink)" }}>{aiProfiles.filter((p) => !p.disabled).length}/{aiProfiles.length}</b> {c.metaProfilesEnabled}
          </span>
          <span>
            {c.metaRouting} <b style={{ color: "var(--g-ink)" }}>{aiStrategy === "failover" ? c.routeFailover : aiStrategy === "round-robin" ? c.routeRoundRobin : c.routeSingle}</b>
          </span>
          <span>{c.metaThreshold} <b style={{ color: "var(--g-ink)" }}>≥ {aiQualityThreshold}</b></span>
          <span>{c.metaRate} <b style={{ color: "var(--g-ink)" }}>{aiRequestsPerMinute} {c.rateUnit}</b></span>
          <span className="ml-auto hidden lg:flex items-center gap-2">
            <span className="flex items-center gap-1.5">
              <span
                className="w-1.5 h-1.5 rounded-full"
                style={{ background: catalog.source === "live" ? "var(--g-signal)" : "var(--g-faint)" }}
              />
              {catalog.source === "live" ? c.catalogLive : c.catalogBuiltin}
              {catalog.fetchedAt ? ` · ${new Date(catalog.fetchedAt).toLocaleDateString()}` : ""}
            </span>
            <button
              type="button"
              className="g-icon-btn !w-5 !h-5"
              title={catalog.error || c.catalogRefresh}
              onClick={catalog.refresh}
              disabled={catalog.loading}
            >
              <RotateCcw size={11} className={catalog.loading ? "g-spin" : ""} />
            </button>
          </span>
        </div>

        <div className="mt-8 grid grid-cols-1 lg:grid-cols-[286px_minmax(0,1fr)] gap-6 items-start">
          {/* ---------------- Left rail ---------------- */}
          <aside className="lg:sticky lg:top-6 space-y-5">
            <div className="g-panel-flat overflow-hidden">
              <div className="flex items-center justify-between px-4 pt-4 pb-3">
                <div>
                  <div className="g-eyebrow">{c.railProfiles}</div>
                  <div className="text-[13px] font-semibold mt-1.5">{c.railProfilesTitle}</div>
                </div>
                {isAuthenticated && (
                  <div className="flex items-center gap-1">
                    <button className="g-icon-btn" title={c.importConfig} onClick={handleImportConfig} disabled={importing}>
                      {importing ? <Loader2 size={14} className="g-spin" /> : <Upload size={14} />}
                    </button>
                    <button className="g-icon-btn" title={c.railAddProfile} onClick={handleAddAIProfile}>
                      <Plus size={15} />
                    </button>
                  </div>
                )}
              </div>
              <div className="px-2.5 pb-2.5 space-y-1 max-h-[340px] overflow-y-auto g-scroll">
                {sortedProfiles.map((profile, idx) => {
                  const active = profile.id === activeProfileId;
                  const p = resolvePresetFrom(catalog.presets, profile.provider, profile.base_url || "");
                  return (
                    <div
                      key={profile.id}
                      className="group relative flex items-center gap-2.5 px-2.5 py-2.5 rounded-lg cursor-pointer transition-all duration-200"
                      style={{
                        background: active ? "var(--g-signal-soft)" : "transparent",
                        boxShadow: active ? "inset 2px 0 0 var(--g-signal)" : "none",
                      }}
                      onClick={() => handleSelectAIProfile(profile.id)}
                      role="button"
                      tabIndex={0}
                      onKeyDown={(e) => e.key === "Enter" && handleSelectAIProfile(profile.id)}
                    >
                      {aiStrategy === "failover" && (
                        <span className="g-mono text-[9px] w-3 shrink-0" style={{ color: "var(--g-faint)" }}>{idx + 1}</span>
                      )}
                      <Mark label={p.mark} color={profile.disabled ? "#9ca3af" : p.color} size="sm" />
                      <div className="min-w-0 flex-1">
                        <div className="text-[11.5px] font-semibold truncate" style={{ color: profile.disabled ? "var(--g-faint)" : "var(--g-ink)" }}>
                          {profile.name || c.railUnnamed}
                        </div>
                        <div className="g-mono text-[9.5px] truncate mt-0.5" style={{ color: "var(--g-faint)" }}>
                          {profile.models && profile.models.length > 1 ? c.modelsCount(profile.models.length) : fromModelRef(profile.model || "") || p.name}
                        </div>
                      </div>
                      {active && <span className="w-1.5 h-1.5 rounded-full shrink-0" style={{ background: "var(--g-signal)" }} />}
                      {isAuthenticated && (
                        <div className="flex items-center gap-0.5 shrink-0">
                          {aiStrategy === "failover" && (
                            <span className="hidden group-hover:flex flex-col -space-y-1">
                              <button className="g-icon-btn !w-4 !h-4" title={c.moveUp} onClick={(e) => { e.stopPropagation(); moveProfile(profile.id, -1); }}>
                                <ArrowUp size={10} />
                              </button>
                              <button className="g-icon-btn !w-4 !h-4" title={c.moveDown} onClick={(e) => { e.stopPropagation(); moveProfile(profile.id, 1); }}>
                                <ArrowDown size={10} />
                              </button>
                            </span>
                          )}
                          <span onClick={(e) => e.stopPropagation()}>
                            <Toggle
                              checked={!profile.disabled}
                              label={c.toggleEnable(profile.name)}
                              onChange={(v) => updateActiveProfileFor(profile.id, { disabled: !v })}
                            />
                          </span>
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
              <div className="px-4 py-3 flex items-center justify-between" style={{ borderTop: "1px solid var(--g-line)" }}>
                <span className="g-hint !text-[10px]">
                  {aiStrategy === "failover" ? c.railProfileHintSorted : c.railProfileHintPlain}
                </span>
                {isAuthenticated && aiProfiles.length > 1 && (
                  <button
                    className="g-icon-btn !w-6 !h-6"
                    title={c.railDeleteProfile}
                    onClick={async () => {
                      const ok = await confirm({
                        title: c.deleteProfileTitle,
                        description: c.deleteProfileDesc(aiProfileName),
                        confirmLabel: c.confirmDelete,
                        destructive: true,
                      });
                      if (ok) handleDeleteAIProfile();
                    }}
                  >
                    <Trash2 size={13} />
                  </button>
                )}
              </div>
            </div>

            {/* Strategy */}
            <div className="g-panel-flat p-4">
              <div className="g-eyebrow">{c.railRouting}</div>
              <div className="text-[13px] font-semibold mt-1.5 mb-3">{c.railRoutingTitle}</div>
              <div className="space-y-1.5">
                {[
                  { value: "single", label: c.strategySingleLabel, desc: c.strategySingleDesc, icon: <CircleDot size={14} /> },
                  { value: "round-robin", label: c.strategyRoundRobinLabel, desc: c.strategyRoundRobinDesc, icon: <Shuffle size={14} /> },
                  { value: "failover", label: c.strategyFailoverLabel, desc: c.strategyFailoverDesc, icon: <Shield size={14} /> },
                ].map((opt) => {
                  const active = aiStrategy === opt.value;
                  return (
                    <button
                      key={opt.value}
                      onClick={() => setAiStrategy(opt.value)}
                      className="w-full flex items-center gap-2.5 px-2.5 py-2 rounded-lg text-left transition-all"
                      style={{
                        background: active ? "var(--g-signal-soft)" : "transparent",
                        color: active ? "var(--g-ink)" : "var(--g-muted)",
                      }}
                    >
                      <span style={{ color: active ? "var(--g-signal)" : "var(--g-faint)" }}>{opt.icon}</span>
                      <span className="min-w-0 flex-1">
                        <span className="block text-[11.5px] font-semibold">{opt.label}</span>
                        <span className="block text-[10px] mt-0.5 truncate" style={{ color: "var(--g-faint)" }}>{opt.desc}</span>
                      </span>
                      {active && <Check size={13} style={{ color: "var(--g-signal)" }} />}
                    </button>
                  );
                })}
              </div>
            </div>

            {/* Evaluation queue */}
            {isAuthenticated && (
              <div
                className="rounded-xl p-4 g-fade"
                style={{ background: "var(--g-panel-2)", border: "1px solid var(--g-line)" }}
              >
                <div className="flex items-center gap-2">
                  <Layers size={14} style={{ color: "var(--g-signal)" }} />
                  <span className="text-[12px] font-semibold">{c.railQueueTitle}</span>
                </div>
                <p className="mt-2 text-[11px] leading-relaxed" style={{ color: "var(--g-muted)" }}>
                  {c.railQueueDesc}
                </p>
                <button className="g-btn g-btn-secondary g-btn-sm w-full mt-3" onClick={handleStartEvaluation}>
                  <Zap size={12} /> {c.railQueueRun}
                </button>
              </div>
            )}

            {/* Usage */}
            {isAuthenticated && (
              <div className="g-panel-flat p-4">
                <div className="flex items-center justify-between gap-2">
                  <div className="g-eyebrow">{c.usageTitle(usage?.days ?? 7)}</div>
                  <span className="g-mono text-[9.5px]" style={{ color: usage?.failed_requests ? "#e9be68" : "var(--g-faint)" }}>
                    {usage?.failed_requests ? c.usageFailed(usage.failed_requests) : c.usageStable}
                  </span>
                </div>
                <div className="text-[13px] font-semibold mt-1.5">
                  {c.usageRequests(usage?.total_requests ?? 0)}
                </div>
                <div className="g-hint mt-1">
                  {c.usageTokens(
                    Number(usage?.total_tokens ?? 0).toLocaleString(),
                    Math.round(usage?.avg_latency_ms ?? 0)
                  )}
                </div>
              </div>
            )}

            {/* Section index */}
            <div className="g-panel-flat p-4">
              <div className="g-eyebrow">{c.railIndex}</div>
              <div className="text-[13px] font-semibold mt-1.5 mb-2.5">{c.railIndexTitle}</div>
              <div className="space-y-0.5">
                {[
                  { id: "sec-connection", n: "01", label: c.secConnectionTitle },
                  { id: "sec-behavior", n: "02", label: c.secBehaviorTitle },
                  { id: "sec-prompts", n: "03", label: c.secPromptsTitle },
                  { id: "sec-schedule", n: "04", label: c.secScheduleTitle },
                  { id: "sec-verify", n: "05", label: c.secVerifyTitle },
                ].map((item) => (
                  <button
                    key={item.id}
                    className="w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-lg text-left transition-colors hover:bg-[var(--g-panel-2)]"
                    onClick={() => document.getElementById(item.id)?.scrollIntoView({ behavior: "smooth", block: "start" })}
                  >
                    <span className="g-mono text-[9.5px]" style={{ color: "var(--g-faint)" }}>{item.n}</span>
                    <span className="text-[11.5px]" style={{ color: "var(--g-muted)" }}>{item.label}</span>
                  </button>
                ))}
              </div>
            </div>
          </aside>

          {/* ---------------- Main column ---------------- */}
          <main className="space-y-5 min-w-0">
            {/* Connection */}
            <SettingSection
              id="sec-connection"
              index="01"
              title={c.secConnectionTitle}
              description={c.secConnectionDesc}
              aside={
                <span className="g-chip g-chip-signal !h-7">
                  <Cpu size={11} /> {providerName(preset, language)}
                </span>
              }
            >
              <div className="space-y-6">
                <div>
                  <div className="flex flex-wrap items-center justify-between gap-3 mb-3">
                    <FieldLabel>{c.providerLabel}</FieldLabel>
                    <div className="flex items-center gap-1.5">
                      {(["all", "cloud", "aggregator", "local"] as const).map((k) => (
                        <button
                          key={k}
                          onClick={() => setProviderFilter(k)}
                          className="g-chip !h-6 !px-2 !text-[10px] transition-all"
                          style={{
                            background: providerFilter === k ? "var(--g-signal-soft)" : "transparent",
                            color: providerFilter === k ? "var(--g-signal)" : "var(--g-muted)",
                            borderColor: providerFilter === k ? "color-mix(in srgb, var(--g-signal) 30%, transparent)" : "var(--g-line)",
                          }}
                        >
                          {k === "all" ? c.providerFilterAll : kindLabel[k][language]}
                        </button>
                      ))}
                      <div className="relative ml-1">
                        <Search size={12} className="absolute left-2 top-1/2 -translate-y-1/2" style={{ color: "var(--g-faint)" }} />
                        <input
                          className="g-input !h-[26px] !w-[124px] !pl-7 !text-[11px]"
                          placeholder={c.providerSearch}
                          value={providerQuery}
                          onChange={(e) => setProviderQuery(e.target.value)}
                        />
                      </div>
                    </div>
                  </div>
                  <div className="grid grid-cols-2 sm:grid-cols-3 xl:grid-cols-4 gap-2.5 g-stagger">
                    {visiblePresets.map((p) => (
                      <ProviderTile key={p.id} preset={p} active={p.id === preset.id} onClick={() => handlePresetChange(p)} />
                    ))}
                  </div>
                  {filteredPresets.length > visiblePresets.length && (
                    <div className="mt-3 g-hint">
                      {c.providerMore(visiblePresets.length, filteredPresets.length)}
                    </div>
                  )}
                  {filteredPresets.length === 0 && (
                    <div className="py-8 text-center text-[12px]" style={{ color: "var(--g-faint)" }}>{c.providerEmpty}</div>
                  )}
                </div>

                <div className="flex flex-wrap items-center gap-x-5 gap-y-2 px-3.5 py-3 rounded-lg g-mono text-[10.5px]" style={{ background: "var(--g-panel-2)", color: "var(--g-muted)" }}>
                  <span>ENDPOINT <b style={{ color: "var(--g-ink)" }}>{aiBaseUrl || (preset.backend === "gemini" ? c.endpointNative : c.endpointOfficial)}</b></span>
                  <span>DRIVER <b style={{ color: "var(--g-ink)" }}>{preset.backend}</b></span>
                  {preset.docsUrl && (
                    <a href={preset.docsUrl} target="_blank" rel="noreferrer" className="flex items-center gap-1 hover:underline" style={{ color: "var(--g-signal)" }}>
                      {c.docsLink} <ExternalLink size={10} />
                    </a>
                  )}
                  {preset.apiKeyUrl && (
                    <a href={preset.apiKeyUrl} target="_blank" rel="noreferrer" className="flex items-center gap-1 hover:underline" style={{ color: "var(--g-signal)" }}>
                      {c.apiKeyLink} <ExternalLink size={10} />
                    </a>
                  )}
                </div>

                <hr className="g-rule" />

                <div className="g-field">
                  <FieldLabel>{c.profileName}</FieldLabel>
                  <input
                    className="g-input"
                    value={aiProfileName}
                    onChange={(e) => handleProfileNameChange(e.target.value)}
                    placeholder={c.profileNamePlaceholder}
                  />
                  <span className="g-hint">{c.profileNameHint}</span>
                </div>

                <div className="mt-5">
                  <ModelListEditor
                    preset={effectivePreset}
                    models={aiModels}
                    onChange={setAiModels}
                    action={(
                      <button
                        className="g-btn g-btn-secondary g-btn-sm"
                        type="button"
                        onClick={handleDiscoverModels}
                        disabled={discovering}
                      >
                        {discovering ? <Loader2 size={12} className="g-spin" /> : <Download size={12} />}
                        {c.discoverModels}
                      </button>
                    )}
                  />
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
                  <div className="g-field">
                    <FieldLabel hint={preset.requiresKey ? c.apiKeyRequired : c.apiKeyOptional}>{c.apiKeyLabel}</FieldLabel>
                    <div className="relative">
                      <input
                        className="g-input mono pr-[76px]"
                        type={showKey ? "text" : "password"}
                        value={aiApiKey}
                        onChange={(e) => setAiApiKey(e.target.value)}
                        placeholder={preset.apiKeyHint}
                        autoComplete="off"
                        disabled={!preset.requiresKey}
                      />
                      <div className="absolute right-1.5 top-1/2 -translate-y-1/2 flex items-center">
                        <button className="g-icon-btn !w-7 !h-7" title={showKey ? c.apiKeyHide : c.apiKeyReveal} onClick={() => setShowKey((v) => !v)}>
                          {showKey ? <EyeOff size={13} /> : <Eye size={13} />}
                        </button>
                        <button
                          className="g-icon-btn !w-7 !h-7"
                          title={c.apiKeyCopy}
                          onClick={() => {
                            if (!aiApiKey) return;
                            navigator.clipboard?.writeText(aiApiKey).then(() => toast(c.apiKeyCopied));
                          }}
                        >
                          <Copy size={13} />
                        </button>
                      </div>
                    </div>
                    <span className="g-hint">
                      {preset.requiresKey ? c.apiKeyHintStored : c.apiKeyHintLocal}
                    </span>
                  </div>

                  <div className="g-field">
                    <FieldLabel>{c.baseUrlLabel}</FieldLabel>
                    <div className="relative">
                      <input
                        className="g-input mono pr-[76px]"
                        value={aiBaseUrl}
                        onChange={(e) => setAiBaseUrl(e.target.value)}
                        placeholder={preset.baseUrl || "https://your-endpoint/v1"}
                        spellCheck={false}
                      />
                      <div className="absolute right-1.5 top-1/2 -translate-y-1/2 flex items-center">
                        <button
                          className="g-icon-btn !w-7 !h-7"
                          title={c.baseUrlReset}
                          onClick={() => {
                            setAiBaseUrl(preset.baseUrl);
                            toast(c.baseUrlResetDone);
                          }}
                        >
                          <RotateCcw size={13} />
                        </button>
                      </div>
                    </div>
                    <span className="g-hint">
                      {preset.backend === "gemini" || preset.backend === "openai"
                        ? c.baseUrlHintOfficial
                        : c.baseUrlHintCustom}
                    </span>
                  </div>
                </div>

                <div className="rounded-xl p-4" style={{ background: "var(--g-panel-2)", border: "1px solid var(--g-line)" }}>
                  <div className="flex items-center justify-between gap-4">
                    <div>
                      <div className="g-label">{c.rateTitle}</div>
                      <div className="g-hint mt-1">{c.rateHint}</div>
                    </div>
                    <div className="g-mono text-[18px] font-semibold" style={{ color: "var(--g-signal)" }}>
                      {aiRequestsPerMinute}<span className="text-[11px] ml-1" style={{ color: "var(--g-faint)" }}>{c.rateUnit}</span>
                    </div>
                  </div>
                  <input
                    type="range"
                    min={1}
                    max={120}
                    value={aiRequestsPerMinute}
                    onChange={(e) => handleRpmChange(Number(e.target.value))}
                    className="g-range mt-4"
                  />
                  <div className="flex items-center justify-between mt-3">
                    <div className="flex gap-1.5">
                      {[5, 10, 30, 60, 100].map((v) => (
                        <button
                          key={v}
                          className="g-chip !h-6 !px-2 g-mono !text-[10px]"
                          onClick={() => handleRpmChange(v)}
                          style={aiRequestsPerMinute === v ? { background: "var(--g-signal-soft)", color: "var(--g-signal)", borderColor: "color-mix(in srgb, var(--g-signal) 30%, transparent)" } : {}}
                        >
                          {v}
                        </button>
                      ))}
                    </div>
                    <span className="g-mono text-[10px]" style={{ color: "var(--g-faint)" }}>{c.rateRange}</span>
                  </div>
                </div>
              </div>
            </SettingSection>

            {/* Behavior */}
            <SettingSection id="sec-behavior" index="02" title={c.secBehaviorTitle} description={c.secBehaviorDesc}>
              <div className="space-y-6">
                <div
                  className="flex items-center justify-between gap-5 p-4 rounded-xl"
                  style={{ background: "var(--g-panel-2)", border: "1px solid var(--g-line)" }}
                >
                  <div className="flex items-start gap-3">
                    <span className="w-9 h-9 rounded-xl flex items-center justify-center shrink-0" style={{ background: "var(--g-signal-soft)", color: "var(--g-signal)" }}>
                      <WandSparkles size={16} />
                    </span>
                    <div>
                      <div className="text-[12.5px] font-semibold">{c.enableTitle}</div>
                      <div className="g-hint mt-1">{c.enableHint}</div>
                    </div>
                  </div>
                  <Toggle checked={aiEnabled} onChange={setAiEnabled} label={c.enableLabel} />
                </div>

                <div className={aiEnabled ? "" : "opacity-45 pointer-events-none"}>
                  <div className="flex items-center justify-between gap-4">
                    <div>
                      <div className="g-label">{c.thresholdTitle}</div>
                      <div className="g-hint mt-1">{thresholdCopy(c)[aiQualityThreshold] || c.thresholdCustom}</div>
                    </div>
                    <div className="g-mono text-[20px] font-semibold" style={{ color: "var(--g-signal)" }}>
                      ≥ {aiQualityThreshold}<span className="text-[11px] ml-1" style={{ color: "var(--g-faint)" }}>{c.thresholdOf}</span>
                    </div>
                  </div>
                  <input
                    type="range"
                    min={5}
                    max={9}
                    step={1}
                    value={aiQualityThreshold}
                    onChange={(e) => setAiQualityThreshold(Number(e.target.value))}
                    className="g-range mt-4"
                  />
                  <div className="flex justify-between mt-2 g-mono text-[10px]" style={{ color: "var(--g-faint)" }}>
                    <span>{c.tick5}</span><span>6</span><span>{c.tick7}</span><span>8</span><span>{c.tick9}</span>
                  </div>
                </div>
              </div>
            </SettingSection>

            {/* Prompts */}
            <SettingSection id="sec-prompts" index="03" title={c.secPromptsTitle} description={c.secPromptsDesc}>
              <div className="space-y-3">
                <PromptEditor
                  id="system-prompt"
                  title={c.promptSystemTitle}
                  description={c.promptSystemDesc}
                  value={aiSystemPrompt}
                  onChange={setAiSystemPrompt}
                  variables={systemVariables}
                  open={openPrompt === "system"}
                  onToggle={() => setOpenPrompt(openPrompt === "system" ? null : "system")}
                />
                <PromptEditor
                  id="daily-prompt"
                  title={c.promptDailyTitle}
                  description={c.promptDailyDesc}
                  value={aiDailyPrompt}
                  onChange={setAiDailyPrompt}
                  variables={dailyVariables}
                  open={openPrompt === "daily"}
                  onToggle={() => setOpenPrompt(openPrompt === "daily" ? null : "daily")}
                />
              </div>
            </SettingSection>

            {/* Schedule */}
            <SettingSection id="sec-schedule" index="04" title={c.secScheduleTitle} description={c.secScheduleDesc}>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                {[
                  {
                    enabled: morningReportEnabled,
                    setEnabled: setMorningReportEnabled,
                    time: morningReportTime,
                    setTime: setMorningReportTime,
                    title: c.morningTitle,
                    desc: c.morningDesc,
                    icon: <span className="text-[15px]">☀</span>,
                  },
                  {
                    enabled: eveningReportEnabled,
                    setEnabled: setEveningReportEnabled,
                    time: eveningReportTime,
                    setTime: setEveningReportTime,
                    title: c.eveningTitle,
                    desc: c.eveningDesc,
                    icon: <span className="text-[15px]">☾</span>,
                  },
                ].map((row) => (
                  <div key={row.title} className="rounded-xl p-4 flex items-center gap-3.5" style={{ background: "var(--g-panel-2)", border: "1px solid var(--g-line)" }}>
                    <span className="w-9 h-9 rounded-xl flex items-center justify-center shrink-0" style={{ background: "var(--g-panel)", color: "var(--g-muted)" }}>
                      {row.icon}
                    </span>
                    <div className="min-w-0 flex-1">
                      <div className="text-[12.5px] font-semibold">{row.title}</div>
                      <div className="g-hint mt-0.5 truncate">{row.desc}</div>
                    </div>
                    <div className="flex items-center gap-2.5 shrink-0">
                      <input
                        type="time"
                        className="g-input mono !w-[104px] !h-[32px] !px-2 !text-[11px]"
                        value={row.time}
                        onChange={(e) => row.setTime(e.target.value)}
                        disabled={!row.enabled}
                        aria-label={row.title}
                      />
                      <Toggle checked={row.enabled} onChange={row.setEnabled} label={row.title} />
                    </div>
                  </div>
                ))}
              </div>
              <div className="mt-3 flex items-center gap-1.5 g-hint">
                <Clock size={11} /> {c.scheduleTimezone}
              </div>
            </SettingSection>

            {/* Test */}
            <SettingSection id="sec-verify" index="05" title={c.secVerifyTitle} description={c.secVerifyDesc}>
              <TestPanel
                result={aiTestResult}
                running={isTestingAI}
                onRun={runTest}
                canRun={isAuthenticated && aiModels.length > 0 && !keyMissing}
                disabledReason={
                  !isAuthenticated
                    ? c.testDisabledLogin
                    : aiModels.length === 0
                      ? c.testDisabledModel
                      : keyMissing
                        ? c.testDisabledKey
                        : undefined
                }
              />
            </SettingSection>
          </main>
        </div>
      </div>

      {/* Sticky save bar */}
      {isAuthenticated && (
        <div
          className="sticky bottom-0 z-20 px-10 py-3.5 flex items-center justify-between gap-4"
          style={{
            background: "color-mix(in srgb, var(--g-paper) 82%, transparent)",
            backdropFilter: "blur(18px) saturate(160%)",
            borderTop: "1px solid var(--g-line)",
          }}
        >
          <div className="flex items-center gap-2.5 text-[11.5px]" style={{ color: "var(--g-muted)" }}>
            <span
              className="w-1.5 h-1.5 rounded-full"
              style={{ background: isDirty ? "var(--g-warn)" : "var(--g-signal)" }}
            />
            {isDirty ? c.saveDirty : c.saveClean}
            <span className="hidden sm:inline g-mono text-[10px]" style={{ color: "var(--g-faint)" }}>
              <span className="g-kbd mx-1">⌘</span><span className="g-kbd">S</span> {c.saveShortcut}
            </span>
          </div>
          <div className="flex items-center gap-2">
            <button
              className="g-btn g-btn-ghost g-btn-sm"
              onClick={() => {
                setBaseline(null);
                resetAISettings();
              }}
              disabled={isSavingSettings || !isDirty}
            >
              <RotateCcw size={12} /> {c.saveDiscard}
            </button>
            <button className="g-btn g-btn-primary g-btn-sm !h-[34px] !px-5" onClick={handleSave} disabled={isSavingSettings || !isDirty}>
              {isSavingSettings ? <Loader2 size={13} className="g-spin" /> : <Check size={13} />}
              {isSavingSettings ? c.saveSaving : c.saveAction}
            </button>
          </div>
        </div>
      )}
    </div>
  );

  function updateActiveProfileFor(id: string, patch: Partial<AIProviderProfile>) {
    setAiProfiles(aiProfiles.map((p) => (p.id === id ? { ...p, ...patch } : p)));
  }
}

export type { AISettingsViewProps };
