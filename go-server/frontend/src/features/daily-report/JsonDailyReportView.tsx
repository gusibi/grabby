import { ArrowUpRight, BarChart3, Layers, Sparkles, Star, Target, TrendingUp } from "lucide-react";
import type { ReactNode } from "react";
import type { JsonDailyReportData } from "@/types";
import { useCopy } from "@/lib/copy";
import { dailyCopy } from "./copy";

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type ReportItem = any;

interface JsonDailyReportViewProps {
  reportData: JsonDailyReportData;
}

const sectionIcon = (key: string, title: string): ReactNode => {
  const t = (title || "").toLowerCase();
  const k = (key || "").toLowerCase();
  if (k.includes("top") || t.includes("头条") || t.includes("要闻")) return <Star size={13} />;
  if (k.includes("dashboard") || t.includes("看板") || t.includes("数据") || t.includes("统计")) return <BarChart3 size={13} />;
  if (k.includes("tech") || k.includes("ai") || t.includes("科技") || t.includes("探索") || t.includes("前沿")) return <TrendingUp size={13} />;
  if (k.includes("finance") || k.includes("macro") || t.includes("财经") || t.includes("宏观")) return <Target size={13} />;
  return <Layers size={13} />;
};

function itemText(item: ReportItem): string {
  if (typeof item === "string") return item;
  return [item.title, item.summary].filter(Boolean).join("：");
}

/**
 * Report summaries sometimes embed the source URL as a Markdown link
 * (`... [阅读原文](https://…)`). Pull the URL out so it can be rendered as a
 * real anchor and keep the prose free of syntax.
 */
function tidySummary(summary?: string): { text: string; link?: string } {
  const raw = summary || "";
  if (!raw) return { text: "" };
  let link: string | undefined;
  const text = raw
    .replace(/\[([^\]]*)\]\((https?:\/\/[^\s)]+)\)/g, (_m, _label: string, url: string) => {
      if (!link) link = url;
      return "";
    })
    .replace(/\s{2,}/g, " ")
    .trim();
  return { text, link };
}

function MetaChip({ children }: { children: ReactNode }) {
  return (
    <span className="g-chip !h-[21px] !text-[9.5px] g-mono" style={{ background: "var(--g-panel-2)" }}>
      {children}
    </span>
  );
}

export function JsonDailyReportView({ reportData }: JsonDailyReportViewProps) {
  const c = useCopy(dailyCopy);
  const sections = Object.entries(reportData.sections || {});

  return (
    <div className="space-y-9">
      {(reportData.editor || reportData.date) && (
        <div className="flex flex-wrap items-center gap-2 pb-4" style={{ borderBottom: "1px solid var(--g-line)" }}>
          {reportData.editor && (
            <MetaChip>
              <span className="w-1.5 h-1.5 rounded-full" style={{ background: "var(--g-signal)" }} />
              {c.editorByline(reportData.editor)}
            </MetaChip>
          )}
          {reportData.date && <MetaChip>{reportData.date}</MetaChip>}
        </div>
      )}

      {sections.map(([key, section]) => {
        if (!section || !section.items || section.items.length === 0) return null;

        const sectionTitle = section.title || "";
        const isDashboard = key.includes("dashboard") || sectionTitle.includes("看板") || sectionTitle.includes("数据");
        const isTopStories = key.includes("top") || sectionTitle.includes("今日头条") || sectionTitle.includes("要闻");

        return (
          <section key={key}>
            {/* Section head */}
            <div className="flex items-baseline gap-2.5 pb-2.5" style={{ borderBottom: "1px solid var(--g-line)" }}>
              <span style={{ color: "var(--g-signal)" }}>{sectionIcon(key, sectionTitle)}</span>
              <h4 className="g-serif text-[19px] tracking-[-0.015em]">{sectionTitle}</h4>
              <span className="g-mono text-[9.5px] ml-auto" style={{ color: "var(--g-faint)" }}>
                {c.itemCount(section.items.length)}
              </span>
            </div>

            {/* Dashboard */}
            {isDashboard ? (
              <div className="mt-4 grid grid-cols-1 sm:grid-cols-2 gap-2.5">
                {section.items.map((item: ReportItem, idx: number) => (
                  <div
                    key={idx}
                    className="flex items-center gap-3 px-4 py-3 rounded-xl"
                    style={{ background: "var(--g-panel-2)", border: "1px solid var(--g-line)" }}
                  >
                    <span className="w-8 h-8 rounded-lg flex items-center justify-center shrink-0" style={{ background: "var(--g-panel)", color: "var(--g-signal)" }}>
                      <BarChart3 size={14} />
                    </span>
                    <span className="text-[12px] leading-relaxed" style={{ color: "var(--g-ink)" }}>{itemText(item)}</span>
                  </div>
                ))}
              </div>
            ) : (
              <div className="mt-4 space-y-2.5">
                {section.items.map((item: ReportItem, idx: number) => {
                  if (typeof item === "string") {
                    return (
                      <p key={idx} className="text-[12.5px] leading-relaxed" style={{ color: "var(--g-muted)" }}>
                        {item}
                      </p>
                    );
                  }

                  const story = item as ReportItem;
                  const commentary = (story.commentary || story.comment || "").replace(/^【深度解析】\s*/, "");
                  const summary = tidySummary(story.summary);
                  const link = story.link || summary.link;
                  const score = story.score || story.rating || "";
                  const lead = isTopStories && idx === 0;

                  return (
                    <article
                      key={story.id || idx}
                      className="group rounded-xl overflow-hidden transition-all duration-300"
                      style={{
                        background: lead ? "var(--g-panel-2)" : "var(--g-panel)",
                        border: `1px solid ${lead ? "var(--g-line-strong)" : "var(--g-line)"}`,
                        borderLeft: `2px solid ${lead ? "var(--g-signal)" : "var(--g-line)"}`,
                      }}
                    >
                      <div className={lead ? "p-5" : "p-4"}>
                        <div className="flex flex-wrap items-start justify-between gap-3">
                          <h5
                            className={`g-serif leading-snug tracking-[-0.012em] flex-1 min-w-[220px] ${lead ? "text-[18px]" : "text-[14.5px]"}`}
                            style={{ color: "var(--g-ink)" }}
                          >
                            {story.title}
                          </h5>
                          <div className="flex items-center gap-1.5 shrink-0">
                            {story.source && (
                              <span className="g-chip !h-[19px] !text-[9px]">{story.source}</span>
                            )}
                            {score && (
                              <span
                                className="g-chip !h-[19px] !text-[9px] g-mono"
                                style={{ background: "var(--g-signal-soft)", color: "var(--g-signal)", borderColor: "transparent" }}
                              >
                                {typeof score === "number" ? `${score}/10` : String(score)}
                              </span>
                            )}
                          </div>
                        </div>

                        {summary.text && (
                          <p className={`mt-2.5 leading-relaxed ${lead ? "text-[12.5px]" : "text-[11.5px]"}`} style={{ color: "var(--g-muted)" }}>
                            {summary.text}
                          </p>
                        )}

                        {commentary && (
                          <div
                            className="mt-3 rounded-lg px-3 py-2.5"
                            style={{ background: "var(--g-panel-2)", borderLeft: "2px solid color-mix(in srgb, var(--g-signal) 45%, transparent)" }}
                          >
                            <div className="flex items-center gap-1.5 text-[9.5px] g-eyebrow mb-1">
                              <Sparkles size={10} style={{ color: "var(--g-signal)" }} />
                              {c.deepDive}
                            </div>
                            <p className="text-[11.5px] leading-relaxed" style={{ color: "var(--g-muted)" }}>
                              {commentary}
                            </p>
                          </div>
                        )}

                        {link && (
                          <a
                            href={link}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="inline-flex items-center gap-1 mt-3 text-[10.5px] font-medium transition-colors"
                            style={{ color: "var(--g-signal)" }}
                          >
                            {c.readOriginal}
                            <ArrowUpRight size={12} />
                          </a>
                        )}
                      </div>
                    </article>
                  );
                })}
              </div>
            )}
          </section>
        );
      })}
    </div>
  );
}
