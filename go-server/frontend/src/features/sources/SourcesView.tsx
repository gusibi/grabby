import { Calendar, Database, Edit, Play, Trash2 } from "lucide-react";
import type { Source } from "@/types";
import { formatTimeAgo } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import { useCopy } from "@/lib/copy";
import { sourcesCopy } from "./copy";
import type { SourcesCopy } from "./copy";
import { PageFrame } from "@/components/ui/PageFrame";
import { Toggle } from "@/components/ui/Settings";

interface SourcesViewProps {
  sources: Source[];
  handleToggleSourceEnabled: (source: Source) => void;
  handleRunSource: (source: Source) => void;
  openEditSourceDialog: (source: Source) => void;
  handleDeleteSource: (id: string) => void;
  isAuthenticated: boolean;
}

const typeLabel = (c: SourcesCopy, type: string) => {
  switch (type) {
    case "rss": return c.typeRss;
    case "api": return c.typeApi;
    case "web_scrape": return c.typeWeb;
    case "twitter": return c.typeTwitter;
    default: return type;
  }
};

export function SourcesView({
  sources,
  handleToggleSourceEnabled,
  handleRunSource,
  openEditSourceDialog,
  handleDeleteSource,
  isAuthenticated,
}: SourcesViewProps) {
  const { language } = useTranslation();
  const c = useCopy(sourcesCopy);
  const enabledCount = sources.filter((s) => s.enabled === 1).length;

  return (
    <PageFrame
      kicker={c.kicker}
      title={c.title}
      description={c.description}
    >
      <div className="flex items-center gap-8 g-mono text-[10.5px] mb-5" style={{ color: "var(--g-faint)" }}>
        <span>{c.total} <b style={{ color: "var(--g-ink)" }}>{sources.length}</b></span>
        <span>{c.enabled} <b style={{ color: "var(--g-signal)" }}>{enabledCount}</b></span>
        <span>{c.paused} <b style={{ color: "var(--g-ink)" }}>{sources.length - enabledCount}</b></span>
      </div>

      <div className="space-y-2.5">
        {sources.map((source, i) => {
          const disabled = source.enabled === 0;
          const running = source.last_fetch_status === "running";
          return (
            <div
              key={source.id}
              className="g-rise g-panel-flat px-4 py-3.5 flex flex-wrap items-center gap-4"
              style={{ animationDelay: `${Math.min(i, 10) * 35}ms`, opacity: disabled ? 0.62 : 1 }}
            >
              <span
                className="w-9 h-9 rounded-xl flex items-center justify-center shrink-0"
                style={{ background: "var(--g-panel-2)", color: disabled ? "var(--g-faint)" : "var(--g-signal)" }}
              >
                <Database size={16} />
              </span>

              <div className="min-w-0 flex-1 basis-[240px]">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="text-[13px] font-semibold truncate">{source.name}</span>
                  <span className="g-chip !h-[19px] !text-[9.5px]">{typeLabel(c, source.type)}</span>
                  {disabled && <span className="g-chip !h-[19px] !text-[9.5px]">{c.pausedChip}</span>}
                </div>
                <div className="g-mono text-[10px] mt-1.5 truncate" style={{ color: "var(--g-faint)" }}>{source.url}</div>
                <div className="flex items-center gap-2.5 mt-2 text-[10px] flex-wrap" style={{ color: "var(--g-muted)" }}>
                  <span className="flex items-center gap-1"><Calendar size={10} /> {source.schedule}</span>
                  <span style={{ color: "var(--g-line-strong)" }}>·</span>
                  <span>{source.category || c.general}</span>
                  <span style={{ color: "var(--g-line-strong)" }}>·</span>
                  <span>{source.default_category}</span>
                  {source.last_fetch_at && (
                    <>
                      <span style={{ color: "var(--g-line-strong)" }}>·</span>
                      <span style={{ color: running ? "var(--g-signal)" : source.last_fetch_status === "success" ? "var(--g-signal)" : "var(--g-danger)" }}>
                        {running ? c.running : formatTimeAgo(source.last_fetch_at, language)}
                      </span>
                    </>
                  )}
                </div>
              </div>

              {isAuthenticated && (
                <div className="flex items-center gap-2 shrink-0">
                  <Toggle
                    checked={!disabled}
                    label={c.enableToggle(source.name)}
                    onChange={() => handleToggleSourceEnabled(source)}
                  />
                  <button
                    className="g-btn g-btn-secondary g-btn-sm"
                    onClick={() => handleRunSource(source)}
                    disabled={running || disabled}
                    title={c.runNow}
                  >
                    <Play size={12} /> {c.run}
                  </button>
                  <button className="g-icon-btn" title={c.edit} onClick={() => openEditSourceDialog(source)}>
                    <Edit size={13} />
                  </button>
                  <button className="g-icon-btn" title={c.delete} onClick={() => handleDeleteSource(source.id)}>
                    <Trash2 size={13} style={{ color: "var(--g-danger)" }} />
                  </button>
                </div>
              )}
            </div>
          );
        })}
        {sources.length === 0 && (
          <div className="g-panel-flat py-16 text-center text-[12.5px]" style={{ color: "var(--g-faint)" }}>
            {c.empty}
          </div>
        )}
      </div>
    </PageFrame>
  );
}
