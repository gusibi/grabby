import * as React from "react";
import { RefreshCw, Loader2, Sparkles, Rss, ChevronLeft, ChevronRight, Calendar } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { parseDailyReportContent } from "@/lib/daily-report";
import { JsonDailyReportView } from "./JsonDailyReportView";
import type { DailyReport, ReportListItem } from "@/types";
import { api } from "@/lib/api";

interface DailyReportViewProps {
  dailyReport: DailyReport | null;
  dailyReportHtml: string;
  selectedReportDate: string;
  setSelectedReportDate: (value: string) => void;
  fetchDailyReport: (date?: string, reportType?: string) => void;
  handleGenerateDailyReport: (reportType?: string) => void;
  isGeneratingReport: boolean;
  reportList: ReportListItem[];
  selectedReportType: string;
  setSelectedReportType: (value: string) => void;
}

const REPORT_TABS = [
  { key: "all", label: "全部", icon: "📋" },
  { key: "morning", label: "早报", icon: "🌅" },
  { key: "evening", label: "晚报", icon: "🌙" },
];

function getLocalDateString(d: Date = new Date()): string {
  const year = d.getFullYear();
  const month = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function getShiftedDate(dateStr: string, days: number): string {
  const d = new Date(dateStr + "T00:00:00");
  d.setDate(d.getDate() + days);
  return getLocalDateString(d);
}

function formatDisplayDate(dateStr: string): string {
  const today = getLocalDateString();
  const yesterday = getShiftedDate(today, -1);
  const d = new Date(dateStr + "T00:00:00");
  const m = d.getMonth() + 1;
  const day = d.getDate();
  const weekdays = ["周日", "周一", "周二", "周三", "周四", "周五", "周六"];
  if (dateStr === today) return `今天 ${m}/${day}`;
  if (dateStr === yesterday) return `昨天 ${m}/${day}`;
  return `${m}/${day} ${weekdays[d.getDay()]}`;
}

export function DailyReportView({
  dailyReport,
  dailyReportHtml,
  selectedReportDate,
  setSelectedReportDate,
  fetchDailyReport,
  handleGenerateDailyReport,
  isGeneratingReport,
  reportList,
  selectedReportType,
  setSelectedReportType,
}: DailyReportViewProps) {
  const [isBusy, setIsBusy] = React.useState(false);
  const loading = isGeneratingReport || isBusy;

  const today = getLocalDateString();
  const isAtToday = selectedReportDate >= today;
  const prevDate = getShiftedDate(selectedReportDate, -1);
  const nextDate = getShiftedDate(selectedReportDate, +1);

  const handleGenerateBoth = async () => {
    setIsBusy(true);
    try {
      await api.generateDailyReport(selectedReportDate, "morning");
      await api.generateDailyReport(selectedReportDate, "evening");
      fetchDailyReport(selectedReportDate, selectedReportType === "all" ? "morning" : selectedReportType);
    } catch (err) {
      console.error(err);
      alert("生成早报和晚报失败");
    } finally {
      setIsBusy(false);
    }
  };

  const handleBackfill = async () => {
    setIsBusy(true);
    try {
      const todayStr = getLocalDateString();
      // Backfill last 7 days
      for (let i = 6; i >= 0; i--) {
        const d = getShiftedDate(todayStr, -i);
        const isToday = d === todayStr;

        const hasDaily = reportList.some(r => r.report_date === d && r.report_type === "daily");
        const hasMorning = reportList.some(r => r.report_date === d && r.report_type === "morning");
        const hasEvening = reportList.some(r => r.report_date === d && r.report_type === "evening");

        if (isToday || !hasDaily) {
          try {
            await api.generateDailyReport(d, "daily");
          } catch (e) {
            console.error(`Failed to generate daily for ${d}`, e);
          }
        }
        if (isToday || !hasMorning) {
          try {
            await api.generateDailyReport(d, "morning");
          } catch (e) {
            console.error(`Failed to generate morning for ${d}`, e);
          }
        }
        if (isToday || !hasEvening) {
          try {
            await api.generateDailyReport(d, "evening");
          } catch (e) {
            console.error(`Failed to generate evening for ${d}`, e);
          }
        }
      }
      fetchDailyReport(selectedReportDate, selectedReportType === "all" ? "morning" : selectedReportType);
      alert("补齐历史数据完成！");
    } catch (err) {
      console.error(err);
      alert("补齐历史数据失败");
    } finally {
      setIsBusy(false);
    }
  };

  const goToDate = (date: string) => {
    setSelectedReportDate(date);
    const typeToFetch = selectedReportType === "all" ? "morning" : selectedReportType;
    fetchDailyReport(date, typeToFetch);
  };

  const handleTypeChange = (type: string) => {
    setSelectedReportType(type);
    if (type !== "all") {
      fetchDailyReport(selectedReportDate, type);
    }
  };

  const filteredReports = reportList.filter(
    (r) => selectedReportType === "all" || r.report_type === selectedReportType
  );

  const qualityPct =
    dailyReport && dailyReport.total_items > 0
      ? ((dailyReport.quality_items / dailyReport.total_items) * 100).toFixed(1)
      : "0";

  return (
    <div className="flex-1 flex overflow-hidden">
      {/* ── Left Panel: Navigation ── */}
      <div className="w-60 shrink-0 border-r border-black/5 dark:border-white/5 flex flex-col overflow-hidden bg-white dark:bg-[#1c1c1e]">
        {/* Date Navigation */}
        <div className="p-3 border-b border-black/5 dark:border-white/5 space-y-2">
          <div className="flex items-center gap-1">
            <button
              onClick={() => goToDate(prevDate)}
              className="p-1.5 rounded-lg hover:bg-zinc-100 dark:hover:bg-zinc-800 text-zinc-400 hover:text-zinc-700 dark:hover:text-zinc-200 transition-colors"
              title="前一天"
            >
              <ChevronLeft className="w-4 h-4" />
            </button>
            <button
              onClick={() => !isAtToday && goToDate(today)}
              className={`flex-1 px-2 py-1.5 rounded-lg text-center transition-colors ${
                !isAtToday
                  ? "hover:bg-zinc-100 dark:hover:bg-zinc-800 cursor-pointer"
                  : "cursor-default"
              }`}
            >
              <div className="text-xs font-bold text-zinc-800 dark:text-zinc-200 leading-tight">
                {formatDisplayDate(selectedReportDate)}
              </div>
              {!isAtToday && (
                <div className="text-[10px] text-indigo-500 mt-0.5">← 回到今天</div>
              )}
            </button>
            <button
              onClick={() => goToDate(nextDate)}
              disabled={isAtToday}
              className="p-1.5 rounded-lg hover:bg-zinc-100 dark:hover:bg-zinc-800 text-zinc-400 hover:text-zinc-700 dark:hover:text-zinc-200 transition-colors disabled:opacity-30 disabled:cursor-not-allowed"
              title="后一天"
            >
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>

          {/* Quick day pills — last 7 days */}
          <div className="grid grid-cols-7 gap-0.5">
            {Array.from({ length: 7 }, (_, i) => {
              const date = getShiftedDate(today, -i);
              const d = new Date(date + "T00:00:00");
              const isSelected = selectedReportDate === date;
              const hasReport = reportList.some((r) => r.report_date === date);
              return (
                <button
                  key={date}
                  onClick={() => goToDate(date)}
                  title={date}
                  className={`py-1.5 rounded text-[10px] font-bold transition-all relative ${
                    isSelected
                      ? "bg-indigo-600 text-white shadow-sm"
                      : "bg-zinc-100 dark:bg-zinc-800 text-zinc-500 dark:text-zinc-400 hover:bg-zinc-200 dark:hover:bg-zinc-700"
                  }`}
                >
                  <div>{d.getDate()}</div>
                  {hasReport && !isSelected && (
                    <div className="absolute bottom-0.5 left-1/2 -translate-x-1/2 w-1 h-1 rounded-full bg-indigo-400 dark:bg-indigo-500" />
                  )}
                </button>
              );
            })}
          </div>

          {/* Date input for older dates */}
          <input
            type="date"
            value={selectedReportDate}
            max={today}
            onChange={(e) => e.target.value && goToDate(e.target.value)}
            className="w-full h-7 text-[11px] px-2 rounded-lg border border-black/5 dark:border-white/5 bg-zinc-50 dark:bg-zinc-900 text-zinc-500 dark:text-zinc-400 cursor-pointer focus:outline-none focus:ring-1 focus:ring-indigo-500/40"
          />
        </div>

        {/* Report type tabs */}
        <div className="p-2 border-b border-black/5 dark:border-white/5 grid grid-cols-4 gap-1">
          {REPORT_TABS.map((tab) => (
            <button
              key={tab.key}
              onClick={() => handleTypeChange(tab.key)}
              className={`flex flex-col items-center py-1.5 rounded-lg text-[10px] font-bold transition-all ${
                selectedReportType === tab.key
                  ? "bg-indigo-100 dark:bg-indigo-900/30 text-indigo-700 dark:text-indigo-300"
                  : "text-zinc-500 hover:bg-zinc-100 dark:hover:bg-zinc-800"
              }`}
            >
              <span className="text-sm leading-tight">{tab.icon}</span>
              <span className="leading-tight">{tab.label}</span>
            </button>
          ))}
        </div>

        {/* Report history list */}
        <div className="px-3 py-1.5 shrink-0">
          <span className="text-[10px] font-extrabold text-zinc-400 dark:text-zinc-500 uppercase tracking-wider">
            历史日报
          </span>
        </div>
        <div className="flex-1 overflow-y-auto px-2 pb-2 space-y-0.5">
          {filteredReports.length === 0 ? (
            <div className="text-center py-8 text-xs text-zinc-400">暂无日报记录</div>
          ) : (
            filteredReports.map((r, idx) => {
              const typeIcon =
                r.report_type === "morning" ? "🌅" : r.report_type === "evening" ? "🌙" : "📰";
              const isActive =
                dailyReport?.report_date === r.report_date &&
                dailyReport?.report_type === r.report_type;
              return (
                <button
                  key={idx}
                  onClick={() => {
                    setSelectedReportDate(r.report_date);
                    setSelectedReportType(r.report_type);
                    fetchDailyReport(r.report_date, r.report_type);
                  }}
                  className={`w-full flex items-start gap-2 px-2 py-2 rounded-lg text-left transition-all ${
                    isActive
                      ? "bg-indigo-50 dark:bg-indigo-950/30 ring-1 ring-indigo-200 dark:ring-indigo-800/50"
                      : "hover:bg-zinc-50 dark:hover:bg-zinc-900/40"
                  }`}
                >
                  <span className="text-sm mt-0.5 shrink-0">{typeIcon}</span>
                  <div className="min-w-0 flex-1">
                    <div className="text-[11px] font-bold text-zinc-800 dark:text-zinc-200 truncate leading-tight">
                      {r.title}
                    </div>
                    <div className="text-[10px] text-zinc-400 mt-0.5 flex items-center gap-1.5">
                      <span>{r.report_date}</span>
                      <span className="text-zinc-300 dark:text-zinc-600">·</span>
                      <span>{r.quality_items} 优质</span>
                    </div>
                  </div>
                </button>
              );
            })
          )}
        </div>
      </div>

      {/* ── Center: Report Content ── */}
      <div className="flex-1 overflow-y-auto bg-zinc-50/50 dark:bg-[#121212] min-w-0">
        <div className="max-w-3xl mx-auto p-6 pb-12 space-y-5">
          <div>
            <h3 className="text-lg font-black tracking-tight flex items-center gap-2 text-indigo-950 dark:text-white">
              <Sparkles className="w-5 h-5 text-indigo-500" />
              AI 智能日报 Daily Report
            </h3>
            <p className="text-xs text-zinc-500 mt-0.5">
              阅读 AI 汇编整理的深度资讯和数据看板。
            </p>
          </div>

          {dailyReport ? (
            <Card className="border border-indigo-500/10 bg-white dark:bg-[#1c1c1e] shadow-md rounded-2xl overflow-hidden">
              <CardHeader className="border-b border-black/5 dark:border-white/5 p-6 bg-indigo-50/20 dark:bg-indigo-950/10">
                <CardTitle className="text-lg font-extrabold text-indigo-950 dark:text-indigo-200 flex items-center gap-2">
                  <span>
                    {dailyReport.report_type === "morning"
                      ? "🌅"
                      : dailyReport.report_type === "evening"
                      ? "🌙"
                      : "📰"}
                  </span>
                  {dailyReport.title}
                </CardTitle>
                <CardDescription className="text-xs text-indigo-600/80 dark:text-indigo-400/80 flex flex-wrap gap-x-4 gap-y-1 mt-1">
                  <span>生成时间: {new Date(dailyReport.generated_at).toLocaleString()}</span>
                  <span>使用模型: {dailyReport.model_used}</span>
                </CardDescription>
              </CardHeader>
              <CardContent className="p-6 md:p-8 space-y-6">
                {(() => {
                  const reportData = parseDailyReportContent(dailyReport?.content);
                  if (reportData) {
                    return <JsonDailyReportView reportData={reportData} />;
                  }
                  return (
                    <div
                      className="prose dark:prose-invert max-w-none text-sm leading-relaxed text-zinc-800 dark:text-zinc-200"
                      dangerouslySetInnerHTML={{
                        __html:
                          dailyReportHtml ||
                          `<pre class="whitespace-pre-wrap font-sans text-sm">${dailyReport.content}</pre>`,
                      }}
                    />
                  );
                })()}
              </CardContent>
            </Card>
          ) : (
            <div className="flex flex-col items-center justify-center text-zinc-400 py-32 bg-white dark:bg-[#1c1c1e] rounded-2xl border border-black/5 dark:border-white/5">
              <Calendar className="w-12 h-12 mb-2 stroke-1 text-zinc-300" />
              <p className="text-sm font-medium text-zinc-600 dark:text-zinc-400">
                该日期暂无日报内容
              </p>
              <p className="text-xs text-zinc-500 mt-1 text-center px-4">
                选择日期后点击右侧"生成日报"按钮生成
              </p>
            </div>
          )}
        </div>
      </div>

      {/* ── Right Panel: Actions & Stats ── */}
      <div className="w-52 shrink-0 border-l border-black/5 dark:border-white/5 flex flex-col overflow-y-auto bg-white dark:bg-[#1c1c1e]">
        <div className="p-4 space-y-5">
          {/* Generate actions */}
          <div className="space-y-2">
            <div className="text-[10px] font-extrabold text-zinc-400 dark:text-zinc-500 uppercase tracking-wider">
              生成操作
            </div>
            <Button
              onClick={() => handleGenerateDailyReport("morning")}
              disabled={loading}
              size="sm"
              className="w-full h-8 gap-1 text-xs font-semibold bg-indigo-600 hover:bg-indigo-700 text-white font-bold"
            >
              {loading && selectedReportType === "morning" ? (
                <Loader2 className="w-3.5 h-3.5 animate-spin" />
              ) : (
                "🌅 生成早报"
              )}
            </Button>
            <Button
              onClick={() => handleGenerateDailyReport("evening")}
              disabled={loading}
              size="sm"
              className="w-full h-8 gap-1 text-xs font-semibold bg-indigo-600 hover:bg-indigo-700 text-white font-bold"
            >
              {loading && selectedReportType === "evening" ? (
                <Loader2 className="w-3.5 h-3.5 animate-spin" />
              ) : (
                "🌙 生成晚报"
              )}
            </Button>
            <Button
              onClick={handleGenerateBoth}
              disabled={loading}
              size="sm"
              variant="outline"
              className="w-full h-8 gap-1 text-xs font-semibold"
            >
              {isBusy ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : "📋 生成早+晚报"}
            </Button>
            <Button
              onClick={handleBackfill}
              disabled={loading}
              size="sm"
              variant="outline"
              className="w-full h-8 gap-1 text-xs font-semibold text-zinc-600 dark:text-zinc-300 hover:bg-zinc-100 dark:hover:bg-zinc-800"
            >
              {isBusy ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : "🔄 补齐历史早/晚报"}
            </Button>
          </div>

          {/* Stats */}
          {dailyReport && (
            <div className="space-y-2">
              <div className="text-[10px] font-extrabold text-zinc-400 dark:text-zinc-500 uppercase tracking-wider">
                数据概览
              </div>
              <div className="space-y-1.5">
                <div className="bg-zinc-50 dark:bg-zinc-900/40 px-3 py-2 rounded-xl">
                  <div className="text-[10px] text-zinc-400 font-bold uppercase">全天处理</div>
                  <div className="text-base font-black text-zinc-900 dark:text-zinc-100">
                    {dailyReport.total_items} 条
                  </div>
                </div>
                <div className="bg-indigo-50/60 dark:bg-indigo-950/20 px-3 py-2 rounded-xl">
                  <div className="text-[10px] text-indigo-400 font-bold uppercase">入选优质</div>
                  <div className="text-base font-black text-indigo-600 dark:text-indigo-400">
                    {dailyReport.quality_items} 条
                  </div>
                </div>
                <div className="bg-zinc-50 dark:bg-zinc-900/40 px-3 py-2 rounded-xl">
                  <div className="text-[10px] text-zinc-400 font-bold uppercase">质量占比</div>
                  <div className="text-base font-black text-zinc-900 dark:text-zinc-100">
                    {qualityPct}%
                  </div>
                </div>
              </div>
              <div className="text-[10px] text-zinc-400 space-y-0.5 pt-0.5">
                <div>
                  模型:{" "}
                  <span className="text-zinc-600 dark:text-zinc-300">{dailyReport.model_used}</span>
                </div>
                <div>
                  生成:{" "}
                  <span className="text-zinc-600 dark:text-zinc-300">
                    {new Date(dailyReport.generated_at).toLocaleTimeString()}
                  </span>
                </div>
              </div>
            </div>
          )}

          {/* RSS */}
          <div className="space-y-2">
            <div className="text-[10px] font-extrabold text-zinc-400 dark:text-zinc-500 uppercase tracking-wider">
              订阅
            </div>
            <a
              href="/api/ai/daily/rss"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-2 px-3 py-2 rounded-xl bg-orange-50 dark:bg-orange-900/10 text-orange-600 dark:text-orange-400 text-xs font-semibold hover:bg-orange-100 dark:hover:bg-orange-900/20 transition-colors no-underline"
            >
              <Rss className="w-3.5 h-3.5 shrink-0" />
              RSS 订阅日报
            </a>
          </div>
        </div>
      </div>
    </div>
  );
}
