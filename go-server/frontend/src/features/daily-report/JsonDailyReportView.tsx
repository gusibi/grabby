import { LayoutGrid, FileText, ExternalLink, CheckCircle, Activity, Layers, Database, Sparkles } from "lucide-react";
import type { JsonDailyReportData } from "@/types";

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type ReportItem = any;

interface JsonDailyReportViewProps {
  reportData: JsonDailyReportData;
}

export function JsonDailyReportView({ reportData }: JsonDailyReportViewProps) {
  const sections = Object.entries(reportData.sections || {});

  const getSectionIcon = (key: string, title: string) => {
    const t = (title || "").toLowerCase();
    const k = (key || "").toLowerCase();
    if (k.includes("top") || t.includes("头条") || t.includes("要闻")) {
      return <Sparkles className="w-5 h-5 text-indigo-500 animate-pulse" />;
    }
    if (k.includes("tech") || k.includes("ai") || t.includes("科技") || t.includes("ai") || t.includes("探索") || t.includes("前沿")) {
      return <Activity className="w-5 h-5 text-cyan-500" />;
    }
    if (k.includes("finance") || k.includes("macro") || t.includes("财经") || t.includes("宏观") || t.includes("金")) {
      return <Database className="w-5 h-5 text-emerald-500" />;
    }
    if (k.includes("international") || k.includes("social") || t.includes("国际") || t.includes("社会") || t.includes("世界")) {
      return <Layers className="w-5 h-5 text-violet-500" />;
    }
    if (k.includes("dashboard") || t.includes("看板") || t.includes("数据") || t.includes("统计")) {
      return <LayoutGrid className="w-5 h-5 text-amber-500" />;
    }
    return <FileText className="w-5 h-5 text-zinc-500" />;
  };

  return (
    <div className="space-y-8">
      {(reportData.editor || reportData.date) && (
        <div className="flex items-center justify-between text-xs text-zinc-500 border-b border-black/5 dark:border-white/5 pb-4">
          {reportData.editor && (
            <div className="flex items-center gap-1.5 bg-indigo-500/5 dark:bg-indigo-500/10 px-2.5 py-1 rounded-full text-indigo-700 dark:text-indigo-300 font-bold border border-indigo-500/10">
              <span className="w-1.5 h-1.5 rounded-full bg-indigo-500 animate-pulse"></span>
              <span>主编: {reportData.editor}</span>
            </div>
          )}
          {reportData.date && (
            <div className="font-mono bg-zinc-100 dark:bg-zinc-800 px-2.5 py-1 rounded-full text-zinc-600 dark:text-zinc-400">
              📅 {reportData.date}
            </div>
          )}
        </div>
      )}

      {sections.map(([key, section]) => {
        if (!section || !section.items || section.items.length === 0) return null;

        const sectionTitle = section.title || "";
        const isDashboard = key.includes("dashboard") || sectionTitle.includes("看板") || sectionTitle.includes("数据");
        const isTopStories = key.includes("top") || sectionTitle.includes("今日头条") || sectionTitle.includes("要闻");

        if (isDashboard) {
          return (
            <div key={key} className="bg-gradient-to-br from-indigo-50/40 via-purple-50/20 to-transparent dark:from-indigo-950/10 dark:via-zinc-900/10 dark:to-transparent border border-indigo-500/10 rounded-2xl p-6 space-y-4">
              <div className="flex items-center gap-2 font-bold text-indigo-950 dark:text-indigo-200">
                {getSectionIcon(key, sectionTitle)}
                <h4 className="text-sm font-black tracking-tight">{sectionTitle}</h4>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {section.items.map((item: ReportItem, idx: number) => {
                  const text = typeof item === "string" ? item : (item.title || "");
                  return (
                    <div key={idx} className="flex items-center gap-3 bg-white dark:bg-zinc-900/60 p-4 rounded-xl shadow-sm border border-black/5 dark:border-white/5">
                      <div className="p-2 bg-indigo-500/10 rounded-lg text-indigo-600 dark:text-indigo-400">
                        <CheckCircle className="w-4 h-4" />
                      </div>
                      <span className="text-sm font-semibold text-zinc-700 dark:text-zinc-300">{text}</span>
                    </div>
                  );
                })}
              </div>
            </div>
          );
        }

        return (
          <div key={key} className="space-y-4">
            <div className="flex items-center gap-2 border-b border-black/5 dark:border-white/5 pb-2">
              {getSectionIcon(key, sectionTitle)}
              <h4 className="text-base font-black text-indigo-950 dark:text-zinc-100">{sectionTitle}</h4>
            </div>

            <div className="space-y-4">
              {section.items.map((item: ReportItem, idx: number) => {
                if (typeof item === "string") {
                  return (
                    <div key={idx} className="p-3 bg-zinc-50 dark:bg-zinc-900/40 rounded-lg text-xs text-zinc-600 dark:text-zinc-400">
                      {item}
                    </div>
                  );
                }

                const story = item as ReportItem;

                const commentaryText = story.commentary || story.comment || "";

                if (isTopStories) {
                  return (
                    <div
                      key={story.id || idx}
                      className="group relative border border-indigo-500/10 hover:border-indigo-500/20 bg-white dark:bg-[#1a1a1c] hover:shadow-lg transition-all duration-300 rounded-2xl overflow-hidden p-6 space-y-4"
                    >
                      <div className="absolute top-0 left-0 w-1 h-full bg-gradient-to-b from-indigo-500 via-purple-500 to-pink-500" />
                      
                      <div className="flex flex-wrap items-start justify-between gap-3 pl-2">
                        <h5 className="text-base font-black leading-snug text-zinc-900 dark:text-white group-hover:text-indigo-600 dark:group-hover:text-indigo-400 transition-colors flex-1 min-w-[280px]">
                          {story.title}
                        </h5>
                        <div className="flex items-center gap-2 shrink-0">
                          {story.source && (
                            <span className="text-[10px] font-bold px-2 py-0.5 rounded-full bg-zinc-100 dark:bg-zinc-800 text-zinc-600 dark:text-zinc-300">
                              {story.source}
                            </span>
                          )}
                          {story.score && (
                            <span className="text-[10px] font-extrabold px-2 py-0.5 rounded-full bg-amber-500/15 text-amber-600 dark:text-amber-400">
                              评分: {story.score}
                            </span>
                          )}
                        </div>
                      </div>

                      <p className="text-sm text-zinc-600 dark:text-zinc-300 pl-2 leading-relaxed">
                        {story.summary}
                      </p>

                      {commentaryText && (
                        <div className="pl-2">
                          <div className="border-l-4 border-indigo-500 bg-indigo-50/50 dark:bg-indigo-950/20 p-4 rounded-r-xl text-xs text-zinc-700 dark:text-zinc-300 space-y-1">
                            <div className="font-bold text-indigo-600 dark:text-indigo-400 flex items-center gap-1">
                              <Sparkles className="w-3.5 h-3.5" />
                              深度解析
                            </div>
                            <p className="italic leading-relaxed">
                              {commentaryText.replace(/^【深度解析】\s*/, "")}
                            </p>
                          </div>
                        </div>
                      )}

                      {story.link && (
                        <div className="flex justify-end pl-2">
                          <a
                            href={story.link}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="inline-flex items-center gap-1 text-xs text-indigo-600 dark:text-indigo-400 hover:text-indigo-800 dark:hover:text-indigo-300 font-bold transition-colors"
                          >
                            阅读原文
                            <ExternalLink className="w-3.5 h-3.5" />
                          </a>
                        </div>
                      )}
                    </div>
                  );
                }

                // Regular Section Stories (e.g. tech, finance, international)
                return (
                  <div
                    key={story.id || idx}
                    className="border border-black/5 dark:border-white/5 hover:border-indigo-500/15 bg-white dark:bg-[#1a1a1c] hover:shadow-md transition-all duration-300 rounded-xl p-5 space-y-3"
                  >
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <h5 className="text-sm font-bold text-zinc-900 dark:text-white leading-snug flex-1 min-w-[240px]">
                        {story.title}
                      </h5>
                      <div className="flex items-center gap-1.5 shrink-0">
                        {story.source && (
                          <span className="text-[10px] font-medium px-2 py-0.5 rounded bg-zinc-100 dark:bg-zinc-800 text-zinc-500 dark:text-zinc-400">
                            {story.source}
                          </span>
                        )}
                        {story.score && (
                          <span className="text-[10px] font-bold px-1.5 py-0.5 rounded bg-indigo-50 dark:bg-indigo-950/30 text-indigo-600 dark:text-indigo-400">
                            {story.score}
                          </span>
                        )}
                      </div>
                    </div>

                    <p className="text-xs text-zinc-600 dark:text-zinc-400 leading-relaxed">
                      {story.summary}
                    </p>

                    {commentaryText && (
                      <div className="border-l-2 border-indigo-500/40 bg-zinc-50/50 dark:bg-zinc-900/30 p-2.5 rounded-r text-[11px] text-zinc-500 dark:text-zinc-400 italic">
                        {commentaryText.replace(/^【深度解析】\s*/, "")}
                      </div>
                    )}

                    {story.link && (
                      <div className="flex justify-end">
                        <a
                          href={story.link}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex items-center gap-1 text-[11px] text-indigo-600 dark:text-indigo-400 hover:text-indigo-800 dark:hover:text-indigo-300 font-semibold transition-colors"
                        >
                          阅读原文
                          <ExternalLink className="w-3 h-3" />
                        </a>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          </div>
        );
      })}
    </div>
  );
}

