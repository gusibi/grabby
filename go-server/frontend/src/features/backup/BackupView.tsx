import { useEffect, useRef, useState } from "react";
import {
  CheckCircle2,
  Download,
  FileJson,
  HardDriveDownload,
  ShieldAlert,
  Upload,
  X,
} from "lucide-react";
import type { Source } from "@/types";
import { api } from "@/lib/api";
import { useFeedback } from "@/components/ui/feedback-context";
import { PageFrame } from "@/components/ui/PageFrame";
import { useCopy } from "@/lib/copy";
import { useTranslation } from "@/lib/i18n";
import { backupCopy } from "./copy";
import type { BackupCopy } from "./copy";

const MAX_BACKUP_BYTES = 16 * 1024 * 1024;
const MAX_PREVIEW_SOURCES = 6;

interface BackupAIPreview {
  enabled?: boolean;
  provider?: string;
  model?: string;
  profiles?: Array<{ id?: string; name?: string }>;
  system_prompt?: string;
  daily_prompt?: string;
}

interface BackupSourcePreview {
  id?: string;
  name?: string;
  url?: string;
  enabled?: number;
}

interface BackupDocument {
  version?: number;
  app?: string;
  exported_at?: string;
  ai?: BackupAIPreview | null;
  sources?: BackupSourcePreview[] | null;
}

interface ImportResponse {
  success?: boolean;
  detail?: string;
  error?: string;
  ai_imported?: boolean;
  sources_created?: number;
  sources_updated?: number;
  warnings?: string[];
}

interface BackupViewProps {
  sources: Source[];
  onImported: () => void;
}

/** Validate the uploaded JSON locally and return a localized error message. */
function validateBackup(raw: unknown, c: BackupCopy): { doc?: BackupDocument; error?: string } {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
    return { error: c.errParse };
  }
  const doc = raw as BackupDocument;
  if (doc.app && doc.app !== "grabby") {
    return { error: c.errForeignApp };
  }
  if (typeof doc.version === "number" && doc.version > 1) {
    return { error: c.errNewerVersion };
  }
  const hasAI = !!doc.ai && typeof doc.ai === "object";
  const hasSources = Array.isArray(doc.sources) && doc.sources.length > 0;
  if (!hasAI && !hasSources) {
    return { error: c.errNoContent };
  }
  return { doc };
}

export function BackupView({ sources, onImported }: BackupViewProps) {
  const { toast, confirm } = useFeedback();
  const c = useCopy(backupCopy);
  const { language } = useTranslation();
  const inputRef = useRef<HTMLInputElement | null>(null);

  const [profileCount, setProfileCount] = useState<number | null>(null);
  const [isExporting, setIsExporting] = useState(false);
  const [isImporting, setIsImporting] = useState(false);
  const [isDragging, setIsDragging] = useState(false);
  const [fileName, setFileName] = useState("");
  const [doc, setDoc] = useState<BackupDocument | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    api.getAISettings()
      .then((data) => {
        if (!active) return;
        const profiles = Array.isArray(data?.settings?.profiles) ? data.settings.profiles : [];
        setProfileCount(profiles.length);
      })
      .catch(() => {
        if (active) setProfileCount(null);
      });
    return () => {
      active = false;
    };
  }, []);

  const handleExport = async () => {
    setIsExporting(true);
    try {
      const { blob, filename } = await api.exportConfig();
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = filename;
      document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      URL.revokeObjectURL(url);
      toast(c.exportDoneTitle, c.exportDoneBody, "success");
    } catch (err) {
      console.error("Failed to export config", err);
      toast(c.exportFailedTitle, c.errExportNetwork, "error");
    } finally {
      setIsExporting(false);
    }
  };

  const acceptFile = async (file: File) => {
    setError("");
    setFileName(file.name);
    setDoc(null);
    if (file.size > MAX_BACKUP_BYTES) {
      setError(c.dropHint);
      return;
    }
    try {
      const text = await file.text();
      const { doc: parsed, error: parseError } = validateBackup(JSON.parse(text), c);
      if (parseError || !parsed) {
        setError(parseError || c.errParse);
        return;
      }
      setDoc(parsed);
    } catch {
      setError(c.errParse);
    }
  };

  const handleInputChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (file) {
      void acceptFile(file);
    }
  };

  const handleDrop = (event: React.DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    setIsDragging(false);
    const file = event.dataTransfer.files?.[0];
    if (file) {
      void acceptFile(file);
    }
  };

  const clearSelection = () => {
    setDoc(null);
    setFileName("");
    setError("");
  };

  const handleImport = async () => {
    if (!doc) {
      setError(c.errEmpty);
      return;
    }
    const ok = await confirm({
      title: c.confirmTitle,
      description: c.confirmDesc,
      confirmLabel: c.confirmLabel,
      destructive: true,
    });
    if (!ok) return;

    setIsImporting(true);
    setError("");
    try {
      const response = (await api.importConfig(doc)) as ImportResponse;
      if (!response.success) {
        toast(c.importFailedTitle, response.detail || response.error || c.errNetwork, "error");
        return;
      }
      const parts = [
        response.ai_imported ? c.importDoneAI : "",
        c.importDoneSources(response.sources_created ?? 0, response.sources_updated ?? 0),
        response.warnings?.length ? c.importWarnings(response.warnings.join("；")) : "",
      ].filter(Boolean);
      toast(c.importDoneTitle, parts.join(" · "), "success");
      if (response.ai_imported) {
        setProfileCount(Array.isArray(doc.ai?.profiles) ? doc.ai.profiles.length : 0);
      }
      onImported();
      clearSelection();
    } catch (err) {
      console.error("Failed to import config", err);
      toast(c.importFailedTitle, c.errNetwork, "error");
    } finally {
      setIsImporting(false);
    }
  };

  const profiles = doc?.ai?.profiles?.length ?? 0;
  const docSources = Array.isArray(doc?.sources) ? doc.sources : [];
  const exportedAt = doc?.exported_at
    ? new Date(doc.exported_at).toLocaleString(language === "zh" ? "zh-CN" : "en-US")
    : "";

  return (
    <PageFrame kicker={c.kicker} title={c.title} description={c.description}>
      <div className="grid gap-5 lg:grid-cols-2 items-start">
        {/* Export */}
        <section className="g-panel-flat overflow-hidden">
          <header className="p-5" style={{ borderBottom: "1px solid var(--g-line)", background: "var(--g-panel-2)" }}>
            <h2 className="text-[15px] font-bold flex items-center gap-2">
              <HardDriveDownload size={16} style={{ color: "var(--g-signal)" }} />
              {c.exportTitle}
            </h2>
            <p className="text-[11.5px] mt-1.5 leading-relaxed" style={{ color: "var(--g-muted)" }}>
              {c.exportDesc}
            </p>
          </header>

          <div className="p-5 space-y-3">
            <div className="space-y-2 text-[12px]">
              <div className="flex items-center gap-2">
                <CheckCircle2 size={14} style={{ color: "var(--g-signal)" }} />
                <span>{c.exportProfiles(profileCount ?? 0)}</span>
              </div>
              <div className="flex items-center gap-2">
                <CheckCircle2 size={14} style={{ color: "var(--g-signal)" }} />
                <span>{c.exportSources(sources.length)}</span>
              </div>
              <div className="flex items-center gap-2">
                <CheckCircle2 size={14} style={{ color: "var(--g-signal)" }} />
                <span>{c.exportPrompts}</span>
              </div>
            </div>

            <div
              className="flex items-start gap-2 rounded-xl p-3 text-[11px] leading-relaxed"
              style={{
                background: "color-mix(in srgb, var(--g-warning, #b45309) 8%, transparent)",
                border: "1px solid color-mix(in srgb, var(--g-warning, #b45309) 24%, transparent)",
                color: "var(--g-muted)",
              }}
            >
              <ShieldAlert size={14} className="shrink-0 mt-0.5" style={{ color: "var(--g-warning, #b45309)" }} />
              <span>{c.exportWarning}</span>
            </div>

            <button
              className="g-btn g-btn-primary w-full"
              onClick={handleExport}
              disabled={isExporting}
              type="button"
            >
              <Download size={14} />
              {isExporting ? c.exporting : c.exportAction}
            </button>
          </div>
        </section>

        {/* Import */}
        <section className="g-panel-flat overflow-hidden">
          <header className="p-5" style={{ borderBottom: "1px solid var(--g-line)", background: "var(--g-panel-2)" }}>
            <h2 className="text-[15px] font-bold flex items-center gap-2">
              <Upload size={16} style={{ color: "var(--g-signal)" }} />
              {c.importTitle}
            </h2>
            <p className="text-[11.5px] mt-1.5 leading-relaxed" style={{ color: "var(--g-muted)" }}>
              {c.importDesc}
            </p>
          </header>

          <div className="p-5 space-y-4">
            <input
              ref={inputRef}
              type="file"
              accept="application/json,.json"
              className="hidden"
              onChange={handleInputChange}
            />

            {!doc ? (
              <div
                role="button"
                tabIndex={0}
                onClick={() => inputRef.current?.click()}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") inputRef.current?.click();
                }}
                onDragOver={(e) => {
                  e.preventDefault();
                  setIsDragging(true);
                }}
                onDragLeave={() => setIsDragging(false)}
                onDrop={handleDrop}
                className="rounded-xl px-5 py-9 text-center cursor-pointer transition-colors"
                style={{
                  border: `1.5px dashed ${isDragging ? "var(--g-signal)" : "var(--g-line-strong)"}`,
                  background: isDragging ? "var(--g-signal-soft)" : "transparent",
                }}
              >
                <FileJson size={26} className="mx-auto mb-3" style={{ color: "var(--g-signal)" }} />
                <div className="text-[12.5px] font-semibold">
                  {isDragging ? c.dropActive : c.dropTitle}
                </div>
                <div className="text-[10.5px] mt-1.5" style={{ color: "var(--g-faint)" }}>
                  {fileName || c.dropHint}
                </div>
              </div>
            ) : (
              <div className="g-inset p-4 space-y-2.5">
                <div className="flex items-center justify-between gap-3">
                  <div className="flex items-center gap-2 min-w-0">
                    <FileJson size={15} style={{ color: "var(--g-signal)" }} />
                    <span className="text-[12px] font-semibold truncate">{fileName}</span>
                  </div>
                  <button className="g-icon-btn shrink-0" onClick={clearSelection} title={c.importOther} type="button">
                    <X size={13} />
                  </button>
                </div>

                <div className="g-eyebrow !text-[9px] pt-1">{c.previewTitle}</div>
                {exportedAt && (
                  <div className="flex items-center justify-between text-[11px]">
                    <span style={{ color: "var(--g-muted)" }}>{c.previewExportedAt}</span>
                    <span className="g-mono text-[10.5px]">{exportedAt}</span>
                  </div>
                )}
                <div className="text-[11px]">
                  <span style={{ color: "var(--g-muted)" }}>{c.previewAISection}</span>
                  <span className="mx-1" style={{ color: "var(--g-faint)" }}>·</span>
                  {doc.ai ? (
                    <span>{profiles > 0 ? c.previewProfiles(profiles) : c.previewPromptInfo}</span>
                  ) : (
                    <span style={{ color: "var(--g-faint)" }}>{c.previewNoAI}</span>
                  )}
                </div>
                <div className="text-[11px]">
                  <span style={{ color: "var(--g-muted)" }}>{c.previewSourcesSection}</span>
                  <span className="mx-1" style={{ color: "var(--g-faint)" }}>·</span>
                  {docSources.length > 0 ? (
                    <span>{c.previewSourceCount(docSources.length)}</span>
                  ) : (
                    <span style={{ color: "var(--g-faint)" }}>{c.previewNoSources}</span>
                  )}
                </div>
                {docSources.length > 0 && (
                  <div className="g-mono text-[10px] leading-relaxed break-all" style={{ color: "var(--g-faint)" }}>
                    {c.previewSourceList(
                      docSources
                        .slice(0, MAX_PREVIEW_SOURCES)
                        .map((src) => src.name || src.id || "?")
                        .join(" · ") + (docSources.length > MAX_PREVIEW_SOURCES ? " …" : ""),
                    )}
                  </div>
                )}
              </div>
            )}

            {error && (
              <div
                className="rounded-xl px-3 py-2.5 text-[11px]"
                style={{
                  background: "color-mix(in srgb, var(--g-danger) 8%, transparent)",
                  border: "1px solid color-mix(in srgb, var(--g-danger) 26%, transparent)",
                  color: "var(--g-danger)",
                }}
              >
                {error}
              </div>
            )}

            <div className="text-[10.5px] leading-relaxed" style={{ color: "var(--g-faint)" }}>
              {c.overwriteNote}
            </div>

            <button
              className="g-btn g-btn-primary w-full"
              onClick={handleImport}
              disabled={isImporting || !doc}
              type="button"
            >
              <Upload size={14} />
              {isImporting ? c.importing : c.importAction}
            </button>
          </div>
        </section>
      </div>
    </PageFrame>
  );
}
