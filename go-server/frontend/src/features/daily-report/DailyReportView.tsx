import * as React from "react";
import { Sparkles, Rss, ChevronLeft, ChevronRight, Calendar, Sun, Moon, CalendarClock, History, Newspaper } from "lucide-react";
import { Button } from "@astryxdesign/core/Button";
import { Card } from "@astryxdesign/core/Card";
import { Markdown } from "@astryxdesign/core/Markdown";
import { VStack, HStack } from "@astryxdesign/core/Layout";
import { parseDailyReportContent } from "@/lib/daily-report";
import { JsonDailyReportView } from "./JsonDailyReportView";
import type { DailyReport, ReportListItem } from "@/types";
import { useCopy } from "@/lib/copy";
import { dailyCopy } from "./copy";
import type { DailyCopy } from "./copy";
import { useFeedback } from "@/components/ui/feedback-context";
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
  isAuthenticated: boolean;
}

const reportTabs = (c: DailyCopy) => [
  { key: "all", label: c.tabAll, icon: <Newspaper size={13} /> },
  { key: "morning", label: c.tabMorning, icon: <Sun size={13} /> },
  { key: "evening", label: c.tabEvening, icon: <Moon size={13} /> },
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

function formatDisplayDate(c: DailyCopy, dateStr: string): string {
  const today = getLocalDateString();
  const yesterday = getShiftedDate(today, -1);
  const d = new Date(dateStr + "T00:00:00");
  const md = `${d.getMonth() + 1}/${d.getDate()}`;
  if (dateStr === today) return c.today(md);
  if (dateStr === yesterday) return c.yesterday(md);
  return `${md} ${c.weekdayShort[d.getDay()]}`;
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
  isAuthenticated,
}: DailyReportViewProps) {
  const { toast } = useFeedback();
  const c = useCopy(dailyCopy);
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
      toast(c.toastGenFailed, undefined, "error");
    } finally {
      setIsBusy(false);
    }
  };

  const handleBackfill = async () => {
    setIsBusy(true);
    try {
      const todayStr = getLocalDateString();
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
      toast(c.toastBackfillDone, undefined, "success");
    } catch (err) {
      console.error(err);
      toast(c.toastBackfillFailed, undefined, "error");
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
      <div className="w-60 shrink-0 border-r border-border flex flex-col overflow-hidden bg-surface">
        {/* Date Navigation */}
        <VStack gap={2} className="p-3 border-b border-border">
          <HStack gap={1} vAlign="center" hAlign="between">
            <Button
              variant="ghost"
              size="sm"
              isIconOnly
              label={c.prevDay}
              icon={<ChevronLeft className="w-4 h-4" />}
              onClick={() => goToDate(prevDate)}
            />
            <VStack 
              onClick={() => !isAtToday && goToDate(today)}
              className={`flex-1 text-center cursor-pointer p-1 rounded hover:bg-muted ${
                isAtToday ? "cursor-default pointer-events-none" : ""
              }`}
            >
              <span className="text-xs font-bold text-primary">
                {formatDisplayDate(c, selectedReportDate)}
              </span>
              {!isAtToday && (
                <span className="text-[9px] text-indigo-500">{c.backToToday}</span>
              )}
            </VStack>
            <Button
              variant="ghost"
              size="sm"
              isIconOnly
              label={c.nextDay}
              icon={<ChevronRight className="w-4 h-4" />}
              onClick={() => goToDate(nextDate)}
              isDisabled={isAtToday}
            />
          </HStack>

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
            className="w-full h-7 text-[11px] px-2 rounded-lg border border-border bg-muted text-secondary cursor-pointer focus:outline-none"
          />
        </VStack>

        {/* Report type tabs */}
        <div className="p-2 border-b border-border grid grid-cols-3 gap-1">
          {reportTabs(c).map((tab) => (
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
            {c.historyTitle}
          </span>
        </div>
        <div className="flex-1 overflow-y-auto px-2 pb-2 space-y-0.5">
          {filteredReports.length === 0 ? (
            <div className="text-center py-8 text-xs text-zinc-400">{c.historyEmpty}</div>
          ) : (
            filteredReports.map((r, idx) => {
              const TypeIcon =
                r.report_type === "morning" ? Sun : r.report_type === "evening" ? Moon : Newspaper;
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
                  <span className="mt-0.5 shrink-0 flex items-center justify-center w-5 h-5 rounded-md" style={{ background: "var(--g-panel-2)", color: "var(--g-signal)" }}>
                    <TypeIcon size={11} />
                  </span>
                  <div className="min-w-0 flex-1">
                    <div className="text-[11px] font-bold text-zinc-800 dark:text-zinc-200 truncate leading-tight">
                      {r.title}
                    </div>
                    <div className="text-[10px] text-zinc-400 mt-0.5 flex items-center gap-1.5">
                      <span>{r.report_date}</span>
                      <span className="text-zinc-300 dark:text-zinc-600">·</span>
                      <span>{c.qualityItems(r.quality_items)}</span>
                    </div>
                  </div>
                </button>
              );
            })
          )}
        </div>
      </div>

      {/* ── Center: Report Content ── */}
      <div className="flex-1 overflow-y-auto bg-body min-w-0">
        <VStack gap={5} className="max-w-3xl mx-auto p-6 pb-12">
          <div>
            <div className="g-eyebrow">Briefing · {formatDisplayDate(c, selectedReportDate)}</div>
            <h3 className="g-display mt-2 text-[30px] flex items-center gap-2.5">
              <Sparkles size={17} style={{ color: "var(--g-signal)" }} />
              {c.title}
            </h3>
            <p className="text-[11.5px] mt-2" style={{ color: "var(--g-muted)" }}>
              {c.subtitle}
            </p>
          </div>

          {dailyReport ? (
            <Card className="overflow-hidden">
              <div className="border-b border-border p-6 bg-muted">
                <h4 className="text-lg font-extrabold flex items-center gap-2">
                  <span className="flex items-center justify-center w-6 h-6 rounded-lg" style={{ background: "var(--g-signal-soft)", color: "var(--g-signal)" }}>
                    {dailyReport.report_type === "morning" ? (
                      <Sun size={13} />
                    ) : dailyReport.report_type === "evening" ? (
                      <Moon size={13} />
                    ) : (
                      <Newspaper size={13} />
                    )}
                  </span>
                  {dailyReport.title}
                </h4>
                <div className="text-xs text-indigo-600/80 dark:text-indigo-400/80 flex flex-wrap gap-x-4 gap-y-1 mt-1">
                  <span>{c.generatedAt(new Date(dailyReport.generated_at).toLocaleString())}</span>
                  <span>{c.modelUsed(dailyReport.model_used)}</span>
                </div>
              </div>
              <div className="p-6 md:p-8">
                {(() => {
                  const reportData = parseDailyReportContent(dailyReport?.content);
                  if (reportData) {
                    return <JsonDailyReportView reportData={reportData} />;
                  }
                  if (dailyReport.html_content) {
                    return (
                      <div
                        className="g-reader max-w-none"
                        dangerouslySetInnerHTML={{ __html: dailyReport.html_content }}
                      />
                    );
                  }
                  if (dailyReportHtml) {
                    return (
                      <div
                        className="g-reader max-w-none"
                        dangerouslySetInnerHTML={{ __html: dailyReportHtml }}
                      />
                    );
                  }
                  return (
                    <div className="g-reader max-w-none">
                      <Markdown headingLevelStart={3} contentWidth="none" autolink="gfm">
                        {dailyReport.content}
                      </Markdown>
                    </div>
                  );
                })()}
              </div>
            </Card>
          ) : (
            <div className="g-fade flex flex-col items-center justify-center py-28 rounded-2xl" style={{ background: "var(--g-panel)", border: "1px solid var(--g-line)" }}>
              <span className="w-14 h-14 rounded-2xl flex items-center justify-center" style={{ background: "var(--g-panel-2)", color: "var(--g-faint)" }}>
                <Calendar size={24} strokeWidth={1.3} />
              </span>
              <p className="g-serif text-[19px] mt-5">{c.emptyTitle}</p>
              <p className="text-[11.5px] mt-2 text-center px-4" style={{ color: "var(--g-muted)" }}>
                {c.emptyBody}
              </p>
            </div>
          )}
        </VStack>
      </div>

      {/* ── Right Panel: Actions & Stats ── */}
      <div className="w-52 shrink-0 border-l border-border flex flex-col overflow-y-auto bg-surface">
        <VStack gap={5} className="p-4">
          {isAuthenticated && (
            <VStack gap={2}>
              <span className="text-[10px] font-extrabold text-zinc-400 dark:text-zinc-500 uppercase tracking-wider">
                {c.actionsTitle}
              </span>
              <Button
                onClick={() => handleGenerateDailyReport("morning")}
                isDisabled={loading}
                isLoading={loading && selectedReportType === "morning"}
                size="sm"
                variant={selectedReportType === "morning" ? "primary" : "secondary"}
                label={c.genMorning}
                icon={<Sun className="w-3.5 h-3.5" />}
              />
              <Button
                onClick={() => handleGenerateDailyReport("evening")}
                isDisabled={loading}
                isLoading={loading && selectedReportType === "evening"}
                size="sm"
                variant={selectedReportType === "evening" ? "primary" : "secondary"}
                label={c.genEvening}
                icon={<Moon className="w-3.5 h-3.5" />}
              />
              <Button
                onClick={handleGenerateBoth}
                isDisabled={loading}
                isLoading={isBusy}
                size="sm"
                variant="secondary"
                label={c.genBoth}
                icon={<CalendarClock className="w-3.5 h-3.5" />}
              />
              <Button
                onClick={handleBackfill}
                isDisabled={loading}
                isLoading={isBusy}
                size="sm"
                variant="secondary"
                label={c.backfill}
                icon={<History className="w-3.5 h-3.5" />}
              />
            </VStack>
          )}

          {/* Stats */}
          {dailyReport && (
            <VStack gap={2}>
              <span className="text-[10px] font-extrabold text-zinc-400 dark:text-zinc-500 uppercase tracking-wider">
                {c.statsTitle}
              </span>
              <VStack gap={1.5}>
                <div className="bg-muted px-3 py-2 rounded-xl">
                  <div className="text-[10px] text-zinc-400 font-bold uppercase">{c.statProcessed}</div>
                  <div className="text-base font-black text-zinc-900 dark:text-zinc-100">
                    {c.statUnit(dailyReport.total_items)}
                  </div>
                </div>
                <div className="bg-indigo-50/60 dark:bg-indigo-950/20 px-3 py-2 rounded-xl">
                  <div className="text-[10px] text-indigo-400 font-bold uppercase">{c.statQuality}</div>
                  <div className="text-base font-black text-indigo-600 dark:text-indigo-400">
                    {c.statUnit(dailyReport.quality_items)}
                  </div>
                </div>
                <div className="bg-muted px-3 py-2 rounded-xl">
                  <div className="text-[10px] text-zinc-400 font-bold uppercase">{c.statRatio}</div>
                  <div className="text-base font-black text-zinc-900 dark:text-zinc-100">
                    {qualityPct}%
                  </div>
                </div>
              </VStack>
              <div className="text-[10px] text-zinc-400 space-y-0.5 pt-0.5">
                <div>
                  {c.metaModel(dailyReport.model_used)}
                </div>
                <div>
                  {c.metaGenerated(new Date(dailyReport.generated_at).toLocaleTimeString())}
                </div>
              </div>
            </VStack>
          )}

          {/* RSS */}
          <VStack gap={2}>
            <span className="text-[10px] font-extrabold text-zinc-400 dark:text-zinc-500 uppercase tracking-wider">
              {c.subscribeTitle}
            </span>
            <Button
              href="/open/api/ai/daily/rss"
              target="_blank"
              rel="noopener noreferrer"
              variant="secondary"
              size="sm"
              label={c.subscribeRss}
              icon={<Rss className="w-3.5 h-3.5 shrink-0 text-orange-500" />}
            />
          </VStack>
        </VStack>
      </div>
    </div>
  );
}
