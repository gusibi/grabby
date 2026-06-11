import { RefreshCw, Trash2, Plus, Loader2, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { AIProviderProfile } from "@/types";

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
  aiModel: string;
  setAiModel: (value: string) => void;
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
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  aiTestResult: any | null;
  handleTestAI: () => void;
  isTestingAI: boolean;
  isSavingSettings: boolean;
  saveAISettings: () => void;
}

export function AISettingsView({
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
  aiModel,
  setAiModel,
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
  saveAISettings
}: AISettingsViewProps) {
  return (
            <div className="flex-1 overflow-y-auto bg-zinc-50/50 dark:bg-[#121212] p-8">
              <div className="max-w-4xl mx-auto space-y-8 pb-12">
                <Card className="border border-black/5 dark:border-white/5 bg-white dark:bg-[#1c1c1e] shadow-sm rounded-2xl overflow-hidden">
                  <CardHeader className="border-b border-black/5 p-6 bg-zinc-50/50 dark:bg-zinc-900/50">
                    <CardTitle className="text-base font-bold flex items-center gap-2">
                      <Sparkles className="w-5 h-5 text-indigo-500" /> AI 评估与模型配置 (AI Settings)
                    </CardTitle>
                    <CardDescription className="text-xs">
                      配置个人大语言模型（LLM）服务以进行文章自动打分、提取精炼摘要与生成智能日报。
                    </CardDescription>
                  </CardHeader>
                  <CardContent className="p-6 space-y-6">
                    {isLoadingSettings ? (
                      <div className="flex items-center justify-center py-12">
                        <Loader2 className="w-6 h-6 animate-spin text-zinc-400" />
                        <span className="text-sm text-zinc-400 ml-2">正在加载配置...</span>
                      </div>
                    ) : (
                      <>
                        <div className="flex items-center justify-between p-4 bg-indigo-50/30 dark:bg-indigo-950/10 rounded-xl border border-indigo-100 dark:border-indigo-900/30">
                          <div>
                            <h5 className="text-sm font-semibold text-indigo-950 dark:text-indigo-300">后台增量评测队列 (AI Evaluation Queue)</h5>
                            <p className="text-[11px] text-zinc-400 mt-0.5">可以手动触发对数据库中未被 AI 分析的文章进行增量评估。</p>
                          </div>
                          <Button
                            onClick={handleStartEvaluation}
                            size="sm"
                            className="bg-indigo-600 hover:bg-indigo-700 text-white rounded-lg px-4 h-8 text-xs gap-1.5 font-sans"
                          >
                            <Sparkles className="w-3.5 h-3.5" />
                            立即评测未评估内容
                          </Button>
                        </div>
                        <div className="flex items-center justify-between p-4 bg-zinc-50 dark:bg-zinc-900/40 rounded-xl border border-black/5 dark:border-white/5">
                          <div>
                            <h5 className="text-sm font-semibold">启用 AI 语义分析与评分</h5>
                            <p className="text-[11px] text-zinc-400 mt-0.5">关闭后，新抓取的文章将不再进行自动分类打分，也不会生成每日简报。</p>
                          </div>
                          <Switch
                            checked={aiEnabled}
                            onCheckedChange={setAiEnabled}
                          />
                        </div>

                        {aiEnabled && (
                          <div className="space-y-6 animate-in fade-in duration-200">
                            <div className="space-y-4 p-4 bg-zinc-50 dark:bg-zinc-900/40 rounded-xl border border-black/5 dark:border-white/5">
                              <div className="flex flex-col md:flex-row md:items-end gap-3">
                                <div className="space-y-2 flex-1">
                                  <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">服务商档案</label>
                                  <Select value={activeProfileId} onValueChange={handleSelectAIProfile}>
                                    <SelectTrigger className="w-full h-9 border border-zinc-200 dark:border-zinc-800 text-sm">
                                      <SelectValue placeholder="选择一个服务商档案" />
                                    </SelectTrigger>
                                    <SelectContent>
                                      {aiProfiles.map((profile) => (
                                        <SelectItem key={profile.id} value={profile.id}>
                                          {profile.name || "未命名服务商"}
                                        </SelectItem>
                                      ))}
                                    </SelectContent>
                                  </Select>
                                </div>

                                <Button
                                  type="button"
                                  onClick={handleAddAIProfile}
                                  variant="outline"
                                  className="h-9 text-xs gap-1.5 border-zinc-200 dark:border-zinc-800"
                                >
                                  <Plus className="w-3.5 h-3.5" />
                                  添加档案
                                </Button>
                                <Button
                                  type="button"
                                  onClick={handleDeleteAIProfile}
                                  variant="outline"
                                  disabled={aiProfiles.length <= 1}
                                  className="h-9 text-xs gap-1.5 border-zinc-200 dark:border-zinc-800 text-rose-600 hover:text-rose-700"
                                >
                                  <Trash2 className="w-3.5 h-3.5" />
                                  删除
                                </Button>
                              </div>

                              <div className="space-y-2">
                                <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">多模型策略</label>
                                <Select value={aiStrategy} onValueChange={setAiStrategy}>
                                  <SelectTrigger className="w-full h-9 border border-zinc-200 dark:border-zinc-800 text-sm">
                                    <SelectValue placeholder="选择策略" />
                                  </SelectTrigger>
                                  <SelectContent>
                                    <SelectItem value="single">单一模式 — 只用选中的档案</SelectItem>
                                    <SelectItem value="round-robin">轮询模式 — 多个模型轮流使用</SelectItem>
                                    <SelectItem value="failover">故障转移 — 主模型不可用时自动切换</SelectItem>
                                  </SelectContent>
                                </Select>
                              </div>

                              <div className="space-y-2">
                                <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">
                                  已配置的服务商列表
                                  {aiStrategy === "failover" && "（数字越小优先级越高）"}
                                </label>
                                <div className="space-y-1.5">
                                  {aiProfiles
                                    .slice()
                                    .sort((a, b) => (a.priority || 999) - (b.priority || 999))
                                    .map((profile, idx) => (
                                    <div
                                      key={profile.id}
                                      className={`flex items-center gap-2 px-3 py-2 rounded-lg border text-sm transition-colors ${
                                        profile.id === activeProfileId
                                          ? "border-indigo-300 dark:border-indigo-700 bg-indigo-50/40 dark:bg-indigo-950/20"
                                          : profile.disabled
                                            ? "border-zinc-200 dark:border-zinc-800 opacity-50 bg-zinc-100 dark:bg-zinc-900"
                                            : "border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900/60"
                                      }`}
                                    >
                                      {aiStrategy === "failover" && (
                                        <span className="flex-shrink-0 w-5 h-5 flex items-center justify-center rounded-full bg-zinc-200 dark:bg-zinc-700 text-[10px] font-bold text-zinc-600 dark:text-zinc-300">
                                          {idx + 1}
                                        </span>
                                      )}
                                      <span className="flex-1 truncate font-medium">{profile.name || "未命名"}</span>
                                      {profile.id === activeProfileId && (
                                        <span className="flex-shrink-0 text-[10px] font-semibold px-1.5 py-0.5 rounded bg-indigo-100 dark:bg-indigo-900/40 text-indigo-600 dark:text-indigo-400">
                                          默认
                                        </span>
                                      )}
                                      <span className="text-[10px] text-zinc-400">{profile.provider}</span>
                                      <span className="text-[10px] text-zinc-400">{profile.requests_per_minute || 10}/min</span>
                                      <button
                                        type="button"
                                        onClick={() => {
                                          const next = aiProfiles.map((p: AIProviderProfile) =>
                                            p.id === profile.id ? { ...p, disabled: !p.disabled } : p
                                          );
                                          setAiProfiles(next);
                                        }}
                                        className={`relative w-9 h-5 rounded-full transition-colors flex-shrink-0 ${
                                          profile.disabled
                                            ? "bg-zinc-300 dark:bg-zinc-700"
                                            : "bg-emerald-500"
                                        }`}
                                      >
                                        <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white shadow transition-transform ${
                                          profile.disabled ? "" : "translate-x-4"
                                        }`} />
                                      </button>
                                    </div>
                                  ))}
                                </div>
                              </div>

                              <div className="space-y-2">
                                <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">档案名称</label>
                                <Input
                                  value={aiProfileName}
                                  onChange={(e) => handleProfileNameChange(e.target.value)}
                                  placeholder="如 LM Studio 本地、OpenAI、Gemini"
                                  className="h-9 text-sm bg-white dark:bg-zinc-900/50"
                                />
                              </div>
                            </div>

                            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                              <div className="space-y-2">
                                <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">AI 服务商 (AI Provider)</label>
                                <Select value={aiProvider} onValueChange={setAiProvider}>
                                  <SelectTrigger className="w-full h-9 border border-zinc-200 dark:border-zinc-800 text-sm">
                                    <SelectValue placeholder="选择服务商" />
                                  </SelectTrigger>
                                  <SelectContent>
                                    <SelectItem value="gemini">Google Gemini</SelectItem>
                                    <SelectItem value="openai">OpenAI</SelectItem>
                                    <SelectItem value="custom">自定义兼容 OpenAI (Custom)</SelectItem>
                                    <SelectItem value="lmstudio">LM Studio (本地)</SelectItem>
                                  </SelectContent>
                                </Select>
                              </div>

                              <div className="space-y-2">
                                <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">评分阈值 (Quality Threshold)</label>
                                <Select value={String(aiQualityThreshold)} onValueChange={(v) => setAiQualityThreshold(Number(v))}>
                                  <SelectTrigger className="w-full h-9 border border-zinc-200 dark:border-zinc-800 text-sm">
                                    <SelectValue placeholder="选择评分阈值" />
                                  </SelectTrigger>
                                  <SelectContent>
                                    <SelectItem value="5">5分及以上 (普通质量)</SelectItem>
                                    <SelectItem value="6">6分及以上 (中等质量)</SelectItem>
                                    <SelectItem value="7">7分及以上 (高分推荐 - 推荐)</SelectItem>
                                    <SelectItem value="8">8分及以上 (极其优质)</SelectItem>
                                    <SelectItem value="9">9分及以上 (行业特写)</SelectItem>
                                  </SelectContent>
                                </Select>
                              </div>

                              <div className="space-y-2">
                                <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">模型名称 (AI Model)</label>
                                <Input
                                  value={aiModel}
                                  onChange={(e) => setAiModel(e.target.value)}
                                  placeholder={aiProvider === "lmstudio" ? "如 gemma-4-12b 或 qwen2.5-7b" : "如 googleai/gemini-2.0-flash 或 openai/gpt-4o-mini"}
                                  className="h-9 text-sm"
                                />
                                {aiProvider === "lmstudio" ? (
                                  <p className="text-[10px] text-zinc-400">LM Studio 中加载的模型名称，可在 LM Studio 界面查看。</p>
                                ) : (
                                  <p className="text-[10px] text-zinc-400">对应 Genkit Go 模型格式，形式为 <code>provider/model-name</code>。</p>
                                )}
                              </div>

                              <div className="space-y-2">
                                <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">API 密钥 (API Key)</label>
                                <Input
                                  type="password"
                                  value={aiApiKey}
                                  onChange={(e) => setAiApiKey(e.target.value)}
                                  placeholder="输入 API Key 密钥"
                                  className="h-9 text-sm"
                                />
                              </div>

                              <div className="space-y-2">
                                <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">请求频率限制 (Requests/min)</label>
                                <Input
                                  type="number"
                                  min={1}
                                  max={1000}
                                  value={aiRequestsPerMinute}
                                  onChange={(e) => setAiRequestsPerMinute(Math.max(1, Number(e.target.value) || 10))}
                                  className="h-9 text-sm"
                                />
                                <p className="text-[10px] text-zinc-400">每分钟最大请求数，不同服务商可分别设置。本地模型可设高（如 100），云端 API 建议设低（如 5-10）。</p>
                              </div>
                            </div>

                            {(aiProvider === "custom" || aiProvider === "lmstudio") && (
                              <div className="space-y-2 animate-in slide-in-from-top-2 duration-200">
                                <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">接口 Base URL</label>
                                <Input
                                  value={aiBaseUrl}
                                  onChange={(e) => setAiBaseUrl(e.target.value)}
                                  placeholder={aiProvider === "lmstudio" ? "如 http://localhost:1234" : "如 https://api.moonshot.cn/v1 或 https://api.deepseek.com/v1"}
                                  className="h-9 text-sm"
                                />
                                {aiProvider === "lmstudio" && (
                                  <p className="text-[10px] text-zinc-400">LM Studio 本地服务地址，默认 <code>http://localhost:1234</code>，无需 API 密钥。</p>
                                )}
                                {aiProvider === "custom" && (
                                  <p className="text-[10px] text-zinc-400">只有在 AI 服务商选择"自定义兼容 OpenAI"时该配置项才生效。</p>
                                )}
                              </div>
                            )}

                            <div className="space-y-2">
                              <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">AI 资讯深度分析提示词 (System Prompt)</label>
                              <p className="text-[10px] text-zinc-400 mb-1">
                                自定义深度分析提示词。可用占位符：
                                <code className="mx-1 bg-zinc-100 dark:bg-zinc-800 px-1 py-0.5 rounded text-[10px]">{`{{.Title}}`}</code>, 
                                <code className="mx-1 bg-zinc-100 dark:bg-zinc-800 px-1 py-0.5 rounded text-[10px]">{`{{.OriginSource}}`}</code>, 
                                <code className="mx-1 bg-zinc-100 dark:bg-zinc-800 px-1 py-0.5 rounded text-[10px]">{`{{.Summary}}`}</code>, 
                                <code className="mx-1 bg-zinc-100 dark:bg-zinc-800 px-1 py-0.5 rounded text-[10px]">{`{{.Content}}`}</code>
                              </p>
                              <textarea
                                value={aiSystemPrompt}
                                onChange={(e) => setAiSystemPrompt(e.target.value)}
                                rows={8}
                                className="flex w-full rounded-xl border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900/50 px-3 py-2 text-xs focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-indigo-500 font-mono resize-y"
                                placeholder="输入系统分析提示词..."
                              />
                            </div>

                            <div className="space-y-2">
                              <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">AI 智能日报生成提示词 (Daily Prompt)</label>
                              <p className="text-[10px] text-zinc-400 mb-1">
                                自定义简报生成提示词。可用占位符：
                                <code className="mx-1 bg-zinc-100 dark:bg-zinc-800 px-1 py-0.5 rounded text-[10px]">{`{{.Count}}`}</code>, 
                                <code className="mx-1 bg-zinc-100 dark:bg-zinc-800 px-1 py-0.5 rounded text-[10px]">{`{{.FeedText}}`}</code>, 
                                <code className="mx-1 bg-zinc-100 dark:bg-zinc-800 px-1 py-0.5 rounded text-[10px]">{`{{.TotalItems}}`}</code>, 
                                <code className="mx-1 bg-zinc-100 dark:bg-zinc-800 px-1 py-0.5 rounded text-[10px]">{`{{.QualityItems}}`}</code>
                              </p>
                              <textarea
                                value={aiDailyPrompt}
                                onChange={(e) => setAiDailyPrompt(e.target.value)}
                                rows={8}
                                className="flex w-full rounded-xl border border-zinc-200 dark:border-zinc-800 bg-white dark:bg-zinc-900/50 px-3 py-2 text-xs focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-indigo-500 font-mono resize-y"
                                placeholder="输入日报生成提示词..."
                              />
                            </div>

                            <div className="space-y-4 pt-2 border-t border-zinc-200 dark:border-zinc-800">
                              <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">定时早晚报设置</label>

                              <div className="flex items-center gap-4 p-3 bg-amber-50/50 dark:bg-amber-950/10 rounded-xl border border-amber-200/50 dark:border-amber-900/20">
                                <label className="flex items-center gap-2 cursor-pointer">
                                  <input
                                    type="checkbox"
                                    checked={morningReportEnabled}
                                    onChange={(e) => setMorningReportEnabled(e.target.checked)}
                                    className="rounded border-zinc-300 text-amber-600 focus:ring-amber-500"
                                  />
                                  <span className="text-xs font-semibold text-amber-700 dark:text-amber-400">🌅 启用早报</span>
                                </label>
                                <div className="flex items-center gap-2">
                                  <label className="text-[10px] text-zinc-500">时间</label>
                                  <Input
                                    type="time"
                                    value={morningReportTime}
                                    onChange={(e) => setMorningReportTime(e.target.value)}
                                    className="h-7 text-xs w-24"
                                  />
                                </div>
                                <p className="text-[10px] text-zinc-400 flex-1">覆盖最近 24 小时优质内容</p>
                              </div>

                              <div className="flex items-center gap-4 p-3 bg-blue-50/50 dark:bg-blue-950/10 rounded-xl border border-blue-200/50 dark:border-blue-900/20">
                                <label className="flex items-center gap-2 cursor-pointer">
                                  <input
                                    type="checkbox"
                                    checked={eveningReportEnabled}
                                    onChange={(e) => setEveningReportEnabled(e.target.checked)}
                                    className="rounded border-zinc-300 text-blue-600 focus:ring-blue-500"
                                  />
                                  <span className="text-xs font-semibold text-blue-700 dark:text-blue-400">🌙 启用晚报</span>
                                </label>
                                <div className="flex items-center gap-2">
                                  <label className="text-[10px] text-zinc-500">时间</label>
                                  <Input
                                    type="time"
                                    value={eveningReportTime}
                                    onChange={(e) => setEveningReportTime(e.target.value)}
                                    className="h-7 text-xs w-24"
                                  />
                                </div>
                                <p className="text-[10px] text-zinc-400 flex-1">覆盖当日早报至晚报时段内容</p>
                              </div>
                            </div>

                            {aiTestResult && (
                              <div className={`p-4 rounded-xl border text-xs space-y-2.5 mt-4 animate-in fade-in duration-200 ${
                                aiTestResult.success 
                                  ? "bg-emerald-50/50 dark:bg-emerald-950/10 border-emerald-200 dark:border-emerald-900/30 text-emerald-800 dark:text-emerald-300"
                                  : "bg-rose-50/50 dark:bg-rose-950/10 border-rose-200 dark:border-rose-900/30 text-rose-800 dark:text-rose-300"
                              }`}>
                                <div className="font-bold flex items-center gap-1.5 text-sm">
                                  {aiTestResult.success ? (
                                    <>
                                      <span className="text-emerald-500">●</span> AI 接口连接成功 (Success)
                                    </>
                                  ) : (
                                    <>
                                      <span className="text-rose-500">●</span> AI 接口连接失败 (Failed)
                                    </>
                                  )}
                                </div>
                                {aiTestResult.success ? (
                                  <div className="space-y-1.5 font-sans">
                                    <p className="font-semibold text-zinc-700 dark:text-zinc-300">
                                      测试文章标题: <span className="font-bold text-zinc-900 dark:text-white">{aiTestResult.title}</span>
                                    </p>
                                    <div className="grid grid-cols-2 gap-2 mt-2 pt-2 border-t border-black/5 dark:border-white/5 text-[11px]">
                                      <div>
                                        <span className="text-zinc-400">智能分类:</span> <span className="font-bold text-zinc-700 dark:text-zinc-300">{aiTestResult.analysis.ai_category} ({aiTestResult.analysis.ai_subcategory || "无"})</span>
                                      </div>
                                      <div>
                                        <span className="text-zinc-400">质量评分:</span> <span className="font-bold text-indigo-600 dark:text-indigo-400">{aiTestResult.analysis.quality_score} / 10 分</span>
                                      </div>
                                    </div>
                                    <div className="mt-2 pt-2 border-t border-black/5 dark:border-white/5">
                                      <span className="text-zinc-400 block mb-0.5">AI 极简摘要 (100字):</span>
                                      <p className="text-zinc-600 dark:text-zinc-400 leading-relaxed font-sans">{aiTestResult.analysis.ai_summary}</p>
                                    </div>
                                    {aiTestResult.analysis.ai_comment && (
                                      <div className="mt-2 pt-2 border-t border-black/5 dark:border-white/5">
                                        <span className="text-zinc-400 block mb-0.5">AI 推荐理由 / 避坑评价:</span>
                                        <p className="text-zinc-600 dark:text-zinc-400 leading-relaxed font-sans">{aiTestResult.analysis.ai_comment}</p>
                                      </div>
                                    )}
                                  </div>
                                ) : (
                                  <p className="font-mono bg-white/50 dark:bg-black/20 p-2.5 rounded border border-black/5 dark:border-white/5 leading-relaxed break-all font-sans">
                                    {aiTestResult.error}
                                  </p>
                                )}
                              </div>
                            )}
                          </div>
                        )}
                      </>
                    )}
                  </CardContent>
                  {!isLoadingSettings && (
                    <CardFooter className="border-t border-black/5 p-6 bg-zinc-50/50 dark:bg-zinc-900/50 flex justify-end">
                      {aiEnabled && (
                        <Button
                          onClick={handleTestAI}
                          disabled={isTestingAI || isSavingSettings}
                          variant="outline"
                          className="mr-3 border-zinc-200 dark:border-zinc-800 rounded-xl h-9 text-xs gap-1.5"
                        >
                          {isTestingAI ? (
                            <>
                              <Loader2 className="w-3.5 h-3.5 animate-spin" />
                              正在测试...
                            </>
                          ) : (
                            <>
                              <RefreshCw className="w-3 h-3" />
                              测试 AI 连通性
                            </>
                          )}
                        </Button>
                      )}
                      <Button
                        onClick={saveAISettings}
                        disabled={isSavingSettings}
                        className="bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl shadow-md px-5 h-9 text-sm"
                      >
                        {isSavingSettings ? (
                          <>
                            <Loader2 className="w-4 h-4 animate-spin mr-2" />
                            正在保存...
                          </>
                        ) : (
                          "保存 AI 配置"
                        )}
                      </Button>
                    </CardFooter>
                  )}
                </Card>
              </div>
            </div>
  );
}
