import { RefreshCw, Trash2, Plus, Sparkles } from "lucide-react";
import { Button } from "@astryxdesign/core/Button";
import { TextInput } from "@astryxdesign/core/TextInput";
import { Switch } from "@astryxdesign/core/Switch";
import { Selector } from "@astryxdesign/core/Selector";
import { TextArea } from "@astryxdesign/core/TextArea";
import { TimeInput } from "@astryxdesign/core/TimeInput";
import { Card } from "@astryxdesign/core/Card";
import { Spinner } from "@astryxdesign/core/Spinner";
import { VStack, HStack } from "@astryxdesign/core/Layout";
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
  isAuthenticated: boolean;
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
  saveAISettings,
  isAuthenticated
}: AISettingsViewProps) {
  return (
    <div className="flex-1 overflow-y-auto bg-body p-8">
      <div className="max-w-4xl mx-auto pb-12">
        <Card className="overflow-hidden">
          <div className="border-b border-border p-6 bg-muted">
            <h3 className="text-base font-bold text-primary flex items-center gap-2">
              <Sparkles className="w-5 h-5 text-indigo-500" /> AI 评估与模型配置 (AI Settings)
            </h3>
            <p className="text-xs text-secondary mt-1">
              配置个人大语言模型（LLM）服务以进行文章自动打分、提取精炼摘要与生成智能日报。
            </p>
          </div>
          
          <div className="p-6">
            {isLoadingSettings ? (
              <HStack gap={2} hAlign="center" vAlign="center" className="py-12">
                <Spinner size="md" />
                <span className="text-sm text-zinc-400">正在加载配置...</span>
              </HStack>
            ) : (
              <VStack gap={6}>
                {isAuthenticated && (
                  <div className="flex items-center justify-between p-4 bg-indigo-50/30 dark:bg-indigo-950/10 rounded-xl border border-indigo-100 dark:border-indigo-900/30">
                    <div>
                      <h5 className="text-sm font-semibold text-indigo-950 dark:text-indigo-300">后台增量评测队列 (AI Evaluation Queue)</h5>
                      <p className="text-[11px] text-zinc-400 mt-0.5">可以手动触发对数据库中未被 AI 分析的文章进行增量评估。</p>
                    </div>
                    <Button
                      onClick={handleStartEvaluation}
                      size="sm"
                      variant="primary"
                      label="立即评测未评估内容"
                      icon={<Sparkles className="w-3.5 h-3.5" />}
                    />
                  </div>
                )}
                
                <div className="flex items-center justify-between p-4 bg-muted rounded-xl border border-border">
                  <div>
                    <h5 className="text-sm font-semibold">启用 AI 语义分析与评分</h5>
                    <p className="text-[11px] text-zinc-400 mt-0.5">关闭后，新抓取的文章将不再进行自动分类打分，也不会生成每日简报。</p>
                  </div>
                  <Switch
                    label="启用 AI"
                    isLabelHidden
                    value={aiEnabled}
                    onChange={setAiEnabled}
                  />
                </div>

                {aiEnabled && (
                  <VStack gap={6} className="animate-in fade-in duration-200">
                    <VStack gap={4} className="p-4 bg-muted rounded-xl border border-border">
                      <div className="flex flex-col md:flex-row md:items-end gap-3">
                        <div className="flex-1">
                          <Selector
                            label="服务商档案"
                            options={aiProfiles.map((profile) => ({
                              value: profile.id,
                              label: profile.name || "未命名服务商",
                            }))}
                            value={activeProfileId}
                            onChange={val => val && handleSelectAIProfile(val)}
                          />
                        </div>

                        {isAuthenticated && (
                          <HStack gap={2} className="pt-2">
                            <Button
                              type="button"
                              onClick={handleAddAIProfile}
                              variant="secondary"
                              label="添加档案"
                              icon={<Plus className="w-3.5 h-3.5" />}
                            />
                            <Button
                              type="button"
                              onClick={handleDeleteAIProfile}
                              variant="secondary"
                              isDisabled={aiProfiles.length <= 1}
                              label="删除"
                              icon={<Trash2 className="w-3.5 h-3.5 text-error" />}
                            />
                          </HStack>
                        )}
                      </div>

                      <Selector
                        label="多模型策略"
                        options={[
                          { value: "single", label: "单一模式 — 只用选中的档案" },
                          { value: "round-robin", label: "轮询模式 — 多个模型轮流使用" },
                          { value: "failover", label: "故障转移 — 主模型不可用时自动切换" }
                        ]}
                        value={aiStrategy}
                        onChange={val => val && setAiStrategy(val)}
                      />

                      <div className="space-y-2">
                        <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">
                          已配置的服务商列表
                          {aiStrategy === "failover" && "（数字越小优先级越高）"}
                        </label>
                        <VStack gap={2}>
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
                                {isAuthenticated && (
                                  <Switch
                                    label="启用服务商"
                                    isLabelHidden
                                    value={!profile.disabled}
                                    onChange={(checked) => {
                                      const next = aiProfiles.map((p) =>
                                        p.id === profile.id ? { ...p, disabled: !checked } : p
                                      );
                                      setAiProfiles(next);
                                    }}
                                  />
                                )}
                              </div>
                            ))}
                        </VStack>
                      </div>

                      <TextInput
                        label="档案名称"
                        value={aiProfileName}
                        onChange={handleProfileNameChange}
                        placeholder="如 LM Studio 本地、OpenAI、Gemini"
                      />
                    </VStack>

                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                      <Selector
                        label="AI 服务商 (AI Provider)"
                        options={[
                          { value: "gemini", label: "Google Gemini" },
                          { value: "openai", label: "OpenAI" },
                          { value: "custom", label: "自定义兼容 OpenAI (Custom)" },
                          { value: "lmstudio", label: "LM Studio (本地)" }
                        ]}
                        value={aiProvider}
                        onChange={val => val && setAiProvider(val)}
                      />

                      <Selector
                        label="评分阈值 (Quality Threshold)"
                        options={[
                          { value: "5", label: "5分及以上 (普通质量)" },
                          { value: "6", label: "6分及以上 (中等质量)" },
                          { value: "7", label: "7分及以上 (高分推荐 - 推荐)" },
                          { value: "8", label: "8分及以上 (极其优质)" },
                          { value: "9", label: "9分及以上 (行业特写)" }
                        ]}
                        value={String(aiQualityThreshold)}
                        onChange={val => val && setAiQualityThreshold(Number(val))}
                      />

                      <VStack gap={1}>
                        <TextInput
                          label="模型名称 (AI Model)"
                          value={aiModel}
                          onChange={setAiModel}
                          placeholder={aiProvider === "lmstudio" ? "如 gemma-4-12b" : "如 googleai/gemini-2.0-flash"}
                        />
                        {aiProvider === "lmstudio" ? (
                          <p className="text-[10px] text-zinc-400">LM Studio 中加载的模型名称，可在 LM Studio 界面查看。</p>
                        ) : (
                          <p className="text-[10px] text-zinc-400">对应 Genkit Go 模型格式，形式为 <code>provider/model-name</code>。</p>
                        )}
                      </VStack>

                      <TextInput
                        type="password"
                        label="API 密钥 (API Key)"
                        value={aiApiKey}
                        onChange={setAiApiKey}
                        placeholder="输入 API Key 密钥"
                      />

                      <VStack gap={1}>
                        <TextInput
                          type="text"
                          label="请求频率限制 (Requests/min)"
                          value={String(aiRequestsPerMinute)}
                          onChange={val => setAiRequestsPerMinute(Math.max(1, Number(val) || 10))}
                        />
                        <p className="text-[10px] text-zinc-400">每分钟最大请求数。本地模型可设高（如 100），云端 API 建议设低（如 5-10）。</p>
                      </VStack>
                    </div>

                    {(aiProvider === "custom" || aiProvider === "lmstudio") && (
                      <VStack gap={1} className="animate-in slide-in-from-top-2 duration-200">
                        <TextInput
                          label="接口 Base URL"
                          value={aiBaseUrl}
                          onChange={setAiBaseUrl}
                          placeholder={aiProvider === "lmstudio" ? "如 http://localhost:1234" : "如 https://api.deepseek.com/v1"}
                        />
                        {aiProvider === "lmstudio" && (
                          <p className="text-[10px] text-zinc-400">LM Studio 本地服务地址，默认 <code>http://localhost:1234</code>，无需 API 密钥。</p>
                        )}
                      </VStack>
                    )}

                    <TextArea
                      label="AI 资讯深度分析提示词 (System Prompt)"
                      description="自定义深度分析提示词。可用占位符：{{.Title}}, {{.OriginSource}}, {{.Summary}}, {{.Content}}"
                      value={aiSystemPrompt}
                      onChange={setAiSystemPrompt}
                      rows={8}
                    />

                    <TextArea
                      label="AI 智能日报生成提示词 (Daily Prompt)"
                      description="自定义简报生成提示词。可用占位符：{{.Count}}, {{.FeedText}}, {{.TotalItems}}, {{.QualityItems}}"
                      value={aiDailyPrompt}
                      onChange={setAiDailyPrompt}
                      rows={8}
                    />

                    <VStack gap={3} className="pt-4 border-t border-border">
                      <label className="text-xs font-semibold text-zinc-500 dark:text-zinc-400">定时早晚报设置</label>

                      <HStack gap={4} vAlign="center" className="p-3 bg-amber-50/50 dark:bg-amber-950/10 rounded-xl border border-amber-200/50 dark:border-amber-900/20">
                        <Switch
                          label="🌅 启用早报"
                          value={morningReportEnabled}
                          onChange={setMorningReportEnabled}
                        />
                        <HStack gap={2} vAlign="center">
                          <span className="text-[10px] text-zinc-500">发送时间</span>
                          <TimeInput
                            label="早报时间"
                            isLabelHidden
                            value={morningReportTime as any}
                            onChange={(val) => val && setMorningReportTime(val)}
                            width={120}
                          />
                        </HStack>
                        <p className="text-[10px] text-zinc-400 flex-1 text-right">覆盖最近 24 小时优质内容</p>
                      </HStack>

                      <HStack gap={4} vAlign="center" className="p-3 bg-blue-50/50 dark:bg-blue-950/10 rounded-xl border border-blue-200/50 dark:border-blue-900/20">
                        <Switch
                          label="🌙 启用晚报"
                          value={eveningReportEnabled}
                          onChange={setEveningReportEnabled}
                        />
                        <HStack gap={2} vAlign="center">
                          <span className="text-[10px] text-zinc-500">发送时间</span>
                          <TimeInput
                            label="晚报时间"
                            isLabelHidden
                            value={eveningReportTime as any}
                            onChange={(val) => val && setEveningReportTime(val)}
                            width={120}
                          />
                        </HStack>
                        <p className="text-[10px] text-zinc-400 flex-1 text-right">覆盖当日早报至晚报时段内容</p>
                      </HStack>
                    </VStack>

                    {aiTestResult && (
                      <div className={`p-4 rounded-xl border text-xs space-y-2.5 mt-4 animate-in fade-in duration-200 ${
                        aiTestResult.success 
                          ? "bg-emerald-50/50 dark:bg-emerald-950/10 border-emerald-200 dark:border-emerald-900/30 text-emerald-800 dark:text-emerald-300"
                          : "bg-rose-50/50 dark:bg-rose-950/10 border-rose-200 dark:border-rose-900/30 text-rose-800 dark:text-rose-300"
                      }`}>
                        <div className="font-bold flex items-center gap-1.5 text-sm">
                          <span className={aiTestResult.success ? "text-success" : "text-error"}>●</span>
                          {aiTestResult.success ? "AI 接口连接成功 (Success)" : "AI 接口连接失败 (Failed)"}
                        </div>
                        {aiTestResult.success ? (
                          <div className="space-y-1.5">
                            <p className="font-semibold text-zinc-700 dark:text-zinc-300">
                              测试文章标题: <span className="font-bold text-zinc-900 dark:text-white">{aiTestResult.title}</span>
                            </p>
                            <div className="grid grid-cols-2 gap-2 mt-2 pt-2 border-t border-border text-[11px]">
                              <div>
                                <span className="text-zinc-400">智能分类:</span> <span className="font-bold text-zinc-700 dark:text-zinc-300">{aiTestResult.analysis.ai_category} ({aiTestResult.analysis.ai_subcategory || "无"})</span>
                              </div>
                              <div>
                                <span className="text-zinc-400">质量评分:</span> <span className="font-bold text-indigo-600 dark:text-indigo-400">{aiTestResult.analysis.quality_score} / 10 分</span>
                              </div>
                            </div>
                            <div className="mt-2 pt-2 border-t border-border">
                              <span className="text-zinc-400 block mb-0.5">AI 极简摘要 (100字):</span>
                              <p className="text-zinc-600 dark:text-zinc-400 leading-relaxed">{aiTestResult.analysis.ai_summary}</p>
                            </div>
                            {aiTestResult.analysis.ai_comment && (
                              <div className="mt-2 pt-2 border-t border-border">
                                <span className="text-zinc-400 block mb-0.5">AI 推荐理由 / 避坑评价:</span>
                                <p className="text-zinc-600 dark:text-zinc-400 leading-relaxed">{aiTestResult.analysis.ai_comment}</p>
                              </div>
                            )}
                          </div>
                        ) : (
                          <p className="font-mono bg-white/50 dark:bg-black/20 p-2.5 rounded border border-border leading-relaxed break-all">
                            {aiTestResult.error}
                          </p>
                        )}
                      </div>
                    )}
                  </VStack>
                )}
              </VStack>
            )}
          </div>

          {!isLoadingSettings && isAuthenticated && (
            <div className="border-t border-border p-6 bg-muted flex justify-end gap-3">
              {aiEnabled && (
                <Button
                  onClick={handleTestAI}
                  isDisabled={isTestingAI || isSavingSettings}
                  isLoading={isTestingAI}
                  variant="secondary"
                  label="测试 AI 连通性"
                  icon={<RefreshCw className="w-3 h-3" />}
                />
              )}
              <Button
                onClick={saveAISettings}
                isDisabled={isSavingSettings}
                isLoading={isSavingSettings}
                variant="primary"
                label="保存 AI 配置"
              />
            </div>
          )}
        </Card>
      </div>
    </div>
  );
}
