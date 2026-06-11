import { RefreshCw, Loader2, Calendar, Sparkles, Rss } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { parseDailyReportContent } from "@/lib/daily-report";
import { JsonDailyReportView } from "./JsonDailyReportView";
import type { DailyReport, ReportListItem } from "@/types";

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
  setSelectedReportType
}: DailyReportViewProps) {
  return (
            <div className="flex-1 overflow-y-auto bg-zinc-50/50 dark:bg-[#121212] p-8">
              <div className="max-w-3xl mx-auto space-y-6 pb-12">
                <div className="flex flex-wrap justify-between items-center gap-4">
                  <div>
                    <h3 className="text-lg font-black tracking-tight flex items-center gap-2 text-indigo-950 dark:text-white">
                      <Sparkles className="w-5 h-5 text-indigo-500" />
                      AI 智能日报 Daily Report
                    </h3>
                    <p className="text-xs text-zinc-500 flex items-center gap-2">
                      阅读 AI 汇编整理的深度资讯和数据看板。
                      <a
                        href="/api/ai/daily/rss"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[10px] font-semibold bg-orange-100 text-orange-600 hover:bg-orange-200 dark:bg-orange-900/30 dark:text-orange-400 dark:hover:bg-orange-900/50 transition-colors no-underline"
                        title="复制或在 RSS 阅读器中打开"
                      >
                        <Rss className="w-3 h-3" />
                        RSS 订阅
                      </a>
                    </p>
                  </div>
                  <div className="flex items-center gap-2">
                    <Input
                      type="date"
                      value={selectedReportDate}
                      onChange={e => {
                        setSelectedReportDate(e.target.value);
                        fetchDailyReport(e.target.value);
                      }}
                      className="h-8 text-xs w-36"
                    />
                    <Button
                      onClick={() => handleGenerateDailyReport("daily")}
                      disabled={isGeneratingReport}
                      size="sm"
                      className="h-8 gap-1 text-xs bg-indigo-600 hover:bg-indigo-700 text-white font-bold"
                    >
                      {isGeneratingReport ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <RefreshCw className="w-3.5 h-3.5" />}
                      生成日报
                    </Button>
                  </div>
                </div>

                {reportList.length > 0 && (
                  <div className="bg-white dark:bg-[#1c1c1e] rounded-2xl border border-black/5 dark:border-white/5 overflow-hidden">
                    <div className="flex items-center gap-1 px-4 pt-3 pb-0">
                      {[{key: "all", label: "全部", icon: "📋"}, {key: "morning", label: "早报", icon: "🌅"}, {key: "evening", label: "晚报", icon: "🌙"}, {key: "daily", label: "日报", icon: "📰"}].map(tab => (
                        <button
                          key={tab.key}
                          onClick={() => {
                            setSelectedReportType(tab.key);
                          }}
                          className={`px-3 py-1.5 text-xs rounded-lg font-semibold transition-all ${
                            selectedReportType === tab.key
                              ? "bg-indigo-100 dark:bg-indigo-900/30 text-indigo-700 dark:text-indigo-300"
                              : "text-zinc-500 hover:bg-zinc-100 dark:hover:bg-zinc-800"
                          }`}
                        >
                          {tab.icon} {tab.label}
                        </button>
                      ))}
                    </div>
                    <div className="p-3 grid gap-1.5 max-h-48 overflow-y-auto">
                      {reportList
                        .filter(r => selectedReportType === "all" || r.report_type === selectedReportType)
                        .map((r, idx) => {
                          const typeIcon = r.report_type === "morning" ? "🌅" : r.report_type === "evening" ? "🌙" : "📰";
                          const typeLabel = r.report_type === "morning" ? "早报" : r.report_type === "evening" ? "晚报" : "日报";
                          const isActive = dailyReport && dailyReport.report_date === r.report_date && dailyReport.report_type === r.report_type;
                          return (
                            <button
                              key={idx}
                              onClick={() => {
                                setSelectedReportDate(r.report_date);
                                setSelectedReportType(r.report_type);
                                fetchDailyReport(r.report_date, r.report_type);
                              }}
                              className={`flex items-center gap-3 px-3 py-2 rounded-xl text-left transition-all ${
                                isActive
                                  ? "bg-indigo-50 dark:bg-indigo-950/20 border border-indigo-200 dark:border-indigo-800"
                                  : "hover:bg-zinc-50 dark:hover:bg-zinc-900/40 border border-transparent"
                              }`}
                            >
                              <span className="text-base">{typeIcon}</span>
                              <div className="flex-1 min-w-0">
                                <div className="flex items-center gap-2">
                                  <span className="text-xs font-bold text-zinc-800 dark:text-zinc-200 truncate">{r.title}</span>
                                  <span className={`text-[10px] px-1.5 py-0.5 rounded-md font-bold ${
                                    r.report_type === "morning" ? "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400" :
                                    r.report_type === "evening" ? "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400" :
                                    "bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400"
                                  }`}>{typeLabel}</span>
                                </div>
                                <div className="text-[10px] text-zinc-400 mt-0.5">
                                  {r.report_date} · {r.quality_items} 条优质 · {new Date(r.generated_at).toLocaleTimeString()}
                                </div>
                              </div>
                            </button>
                          );
                        })}
                    </div>
                  </div>
                )}

                {dailyReport ? (
                  <Card className="border border-indigo-500/10 bg-white dark:bg-[#1c1c1e] shadow-md rounded-2xl overflow-hidden">
                    <CardHeader className="border-b border-black/5 dark:border-white/5 p-6 bg-indigo-50/20 dark:bg-indigo-950/10">
                      <CardTitle className="text-lg font-extrabold text-indigo-950 dark:text-indigo-200 flex items-center gap-2">
                        <span>
                          {dailyReport.report_type === "morning" ? "🌅" : dailyReport.report_type === "evening" ? "🌙" : "📰"}
                        </span>
                        {dailyReport.title}
                      </CardTitle>
                      <CardDescription className="text-xs text-indigo-600/80 dark:text-indigo-400/80 flex flex-wrap gap-x-4 gap-y-1 mt-1">
                        <span>生成时间: {new Date(dailyReport.generated_at).toLocaleString()}</span>
                        <span>使用模型: {dailyReport.model_used}</span>
                      </CardDescription>
                    </CardHeader>
                    <CardContent className="p-6 md:p-8 space-y-6">
                      <div className="grid grid-cols-3 gap-4 border-b border-black/5 dark:border-white/5 pb-6">
                        <div className="bg-zinc-50 dark:bg-zinc-900/40 p-3 rounded-xl">
                          <span className="text-[10px] text-zinc-400 dark:text-zinc-500 font-bold block uppercase">全天处理</span>
                          <span className="text-base font-black">{dailyReport.total_items} 条</span>
                        </div>
                        <div className="bg-zinc-50 dark:bg-zinc-900/40 p-3 rounded-xl">
                          <span className="text-[10px] text-zinc-400 dark:text-zinc-500 font-bold block uppercase">入选优质</span>
                          <span className="text-base font-black text-indigo-600 dark:text-indigo-400">{dailyReport.quality_items} 条</span>
                        </div>
                        <div className="bg-zinc-50 dark:bg-zinc-900/40 p-3 rounded-xl">
                          <span className="text-[10px] text-zinc-400 dark:text-zinc-500 font-bold block uppercase">质量占比</span>
                          <span className="text-base font-black">
                            {dailyReport.total_items > 0 
                              ? ((dailyReport.quality_items / dailyReport.total_items) * 100).toFixed(1) 
                              : 0}%
                          </span>
                        </div>
                      </div>
                      
                      {(() => {
                        const reportData = parseDailyReportContent(dailyReport?.content);
                        if (reportData) {
                          return <JsonDailyReportView reportData={reportData} />;
                        }

                        return (
                          <div 
                            className="prose dark:prose-invert max-w-none text-sm leading-relaxed text-zinc-800 dark:text-zinc-200"
                            dangerouslySetInnerHTML={{ __html: dailyReportHtml || `<pre className="whitespace-pre-wrap font-sans text-sm">${dailyReport.content}</pre>` }}
                          />
                        );
                      })()}
                    </CardContent>
                  </Card>
                ) : (
                  <div className="flex-1 flex flex-col items-center justify-center text-zinc-400 py-32 bg-white dark:bg-[#1c1c1e] rounded-2xl border border-black/5 dark:border-white/5">
                    <Calendar className="w-12 h-12 mb-2 stroke-1 text-zinc-300" />
                    <p className="text-sm font-medium text-zinc-600 dark:text-zinc-400">该日期暂无日报内容</p>
                    <p className="text-xs text-zinc-500 mt-1 text-center px-4">您可以选择该日期或今日，点击右上角 "生成日报" 进行生成</p>
                  </div>
                )}
              </div>
            </div>
  );
}
