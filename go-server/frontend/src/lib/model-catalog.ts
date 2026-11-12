/**
 * Model catalog
 * -------------
 * A curated, versioned snapshot of the open `models.dev` catalog schema
 * (https://models.dev/api.json — provider / model / pricing / capability data),
 * narrowed to endpoints that expose an OpenAI-compatible or Genkit-supported
 * API and therefore work with Grabby's Go AI engine.
 *
 * Keeping this as a standalone registry means the UI only ever deals with
 * friendly ids (`deepseek-chat`, `gpt-4o`) while `toModelRef()` produces the
 * exact Genkit model reference the backend expects. Users never have to know
 * about `custom/` or `googleai/` prefixes again.
 */

export type BackendProvider = "gemini" | "openai" | "custom" | "lmstudio";

export type ProviderKind = "cloud" | "local" | "aggregator";

export interface ModelInfo {
  /** id sent to the provider API, without any Genkit namespace prefix */
  id: string;
  name: string;
  context?: string;
  input?: string;
  output?: string;
  capabilities?: string[];
  recommended?: boolean;
}

export interface ProviderPreset {
  id: string;
  name: string;
  legalName: string;
  mark: string;
  color: string;
  kind: ProviderKind;
  backend: BackendProvider;
  baseUrl: string;
  apiKeyHint: string;
  apiKeyUrl: string;
  docsUrl: string;
  /** Localised one-liner shown under the provider mark. */
  description: { zh: string; en: string };
  /** English name override for UI labels (brand names stay as-is). */
  nameEn?: string;
  requiresKey: boolean;
  models: ModelInfo[];
}

const C = (cap: string) => cap;

export const providerPresets: ProviderPreset[] = [
  {
    id: "openai",
    name: "OpenAI",
    legalName: "OpenAI Platform",
    mark: "OA",
    color: "#111827",
    kind: "cloud",
    backend: "openai",
    baseUrl: "",
    apiKeyHint: "sk-...",
    apiKeyUrl: "https://platform.openai.com/api-keys",
    docsUrl: "https://platform.openai.com/docs/models",
    description: { zh: "官方 GPT 与 o 系列推理模型", en: "Official GPT and o-series reasoning models" },
    requiresKey: true,
    models: [
      { id: "gpt-4.1", name: "GPT-4.1", context: "1M", input: "2.00", output: "8.00", capabilities: [C("旗舰"), C("视觉")], recommended: true },
      { id: "gpt-4.1-mini", name: "GPT-4.1 mini", context: "1M", input: "0.40", output: "1.60", capabilities: [C("高性价比")] },
      { id: "gpt-4.1-nano", name: "GPT-4.1 nano", context: "1M", input: "0.10", output: "0.40", capabilities: [C("极速")] },
      { id: "gpt-4o", name: "GPT-4o", context: "128K", input: "2.50", output: "10.00", capabilities: [C("视觉"), C("多模态")] },
      { id: "gpt-4o-mini", name: "GPT-4o mini", context: "128K", input: "0.15", output: "0.60", capabilities: [C("轻量")] },
      { id: "o4-mini", name: "o4-mini", context: "200K", input: "1.10", output: "4.40", capabilities: [C("推理")] },
      { id: "o3-mini", name: "o3-mini", context: "200K", input: "1.10", output: "4.40", capabilities: [C("推理")] },
    ],
  },
  {
    id: "gemini",
    name: "Google Gemini",
    legalName: "Google AI Studio",
    mark: "G",
    color: "#3B6FD4",
    kind: "cloud",
    backend: "gemini",
    baseUrl: "",
    apiKeyHint: "AIza...",
    apiKeyUrl: "https://aistudio.google.com/app/apikey",
    docsUrl: "https://ai.google.dev/gemini-api/docs/models",
    description: { zh: "原生 Google Genkit 接入，长上下文", en: "Native Google Genkit integration, long context" },
    requiresKey: true,
    models: [
      { id: "gemini-2.5-pro", name: "Gemini 2.5 Pro", context: "1M", input: "1.25", output: "10.00", capabilities: [C("推理"), C("旗舰")], recommended: true },
      { id: "gemini-2.5-flash", name: "Gemini 2.5 Flash", context: "1M", input: "0.30", output: "2.50", capabilities: [C("高性价比"), C("推理")] },
      { id: "gemini-2.0-flash", name: "Gemini 2.0 Flash", context: "1M", input: "0.10", output: "0.40", capabilities: [C("极速")] },
      { id: "gemini-2.0-flash-lite", name: "Gemini 2.0 Flash Lite", context: "1M", input: "0.075", output: "0.30", capabilities: [C("轻量")] },
    ],
  },
  {
    id: "deepseek",
    name: "DeepSeek",
    legalName: "深度求索",
    mark: "DS",
    color: "#4D6BFE",
    kind: "cloud",
    backend: "custom",
    baseUrl: "https://api.deepseek.com/v1",
    apiKeyHint: "sk-...",
    apiKeyUrl: "https://platform.deepseek.com/api_keys",
    docsUrl: "https://api-docs.deepseek.com/",
    description: { zh: "中文表现优秀，推理与性价比突出", en: "Strong Chinese, excellent reasoning value" },
    requiresKey: true,
    models: [
      { id: "deepseek-chat", name: "DeepSeek V3 (Chat)", context: "64K", input: "0.27", output: "1.10", capabilities: [C("通用"), C("推荐")], recommended: true },
      { id: "deepseek-reasoner", name: "DeepSeek R1 (Reasoner)", context: "64K", input: "0.55", output: "2.19", capabilities: [C("推理")] },
    ],
  },
  {
    id: "qwen",
    name: "通义千问",
    legalName: "阿里云百炼",
    mark: "QW",
    color: "#FF6A00",
    kind: "cloud",
    backend: "custom",
    baseUrl: "https://dashscope.aliyuncs.com/compatible-mode/v1",
    apiKeyHint: "sk-...",
    apiKeyUrl: "https://bailian.console.aliyun.com/?tab=model#/api-key",
    docsUrl: "https://help.aliyun.com/zh/model-studio/getting-started/",
    description: { zh: "阿里百炼 OpenAI 兼容模式", en: "Alibaba Model Studio, OpenAI-compatible" },
    requiresKey: true,
    models: [
      { id: "qwen-plus", name: "Qwen Plus", context: "128K", capabilities: [C("均衡")], recommended: true },
      { id: "qwen-max", name: "Qwen Max", context: "32K", capabilities: [C("旗舰")] },
      { id: "qwen-turbo", name: "Qwen Turbo", context: "1M", capabilities: [C("极速")] },
      { id: "qwen3-235b-a22b", name: "Qwen3 235B", context: "128K", capabilities: [C("推理")] },
      { id: "qwen-vl-max", name: "Qwen VL Max", context: "32K", capabilities: [C("视觉")] },
    ],
  },
  {
    id: "moonshot",
    name: "Moonshot / Kimi",
    legalName: "月之暗面",
    mark: "K",
    color: "#1B1B1F",
    kind: "cloud",
    backend: "custom",
    baseUrl: "https://api.moonshot.cn/v1",
    apiKeyHint: "sk-...",
    apiKeyUrl: "https://platform.moonshot.cn/console/api-keys",
    docsUrl: "https://platform.moonshot.cn/docs/",
    description: { zh: "长文本与中文写作见长", en: "Long context and strong Chinese writing" },
    requiresKey: true,
    models: [
      { id: "kimi-k2-0905-preview", name: "Kimi K2", context: "256K", capabilities: [C("旗舰"), C("推理")], recommended: true },
      { id: "moonshot-v1-128k", name: "Moonshot v1 128K", context: "128K", capabilities: [C("长文本")] },
      { id: "moonshot-v1-32k", name: "Moonshot v1 32K", context: "32K", capabilities: [C("均衡")] },
      { id: "moonshot-v1-8k", name: "Moonshot v1 8K", context: "8K", capabilities: [C("极速")] },
    ],
  },
  {
    id: "zhipu",
    name: "智谱 GLM",
    legalName: "智谱 AI 开放平台",
    mark: "GL",
    color: "#3859FF",
    kind: "cloud",
    backend: "custom",
    baseUrl: "https://open.bigmodel.cn/api/paas/v4",
    apiKeyHint: "xxxxx.xxxxx",
    apiKeyUrl: "https://bigmodel.cn/usercenter/proj-mgmt/apikeys",
    docsUrl: "https://docs.bigmodel.cn/",
    description: { zh: "GLM 系列，国内访问稳定", en: "GLM family, reliable access in China" },
    requiresKey: true,
    models: [
      { id: "glm-4.6", name: "GLM-4.6", context: "200K", capabilities: [C("旗舰"), C("推理")], recommended: true },
      { id: "glm-4-air", name: "GLM-4-Air", context: "128K", capabilities: [C("均衡")] },
      { id: "glm-4-flash", name: "GLM-4-Flash", context: "128K", capabilities: [C("免费"), C("极速")] },
    ],
  },
  {
    id: "openrouter",
    name: "OpenRouter",
    legalName: "OpenRouter, Inc.",
    mark: "OR",
    color: "#6366F1",
    kind: "aggregator",
    backend: "custom",
    baseUrl: "https://openrouter.ai/api/v1",
    apiKeyHint: "sk-or-v1-...",
    apiKeyUrl: "https://openrouter.ai/keys",
    docsUrl: "https://openrouter.ai/docs/models",
    description: { zh: "一个 Key 接入 300+ 主流模型", en: "One key for 300+ leading models" },
    requiresKey: true,
    models: [
      { id: "anthropic/claude-sonnet-4.5", name: "Claude Sonnet 4.5", context: "200K", capabilities: [C("写作"), C("推理")], recommended: true },
      { id: "anthropic/claude-haiku-4.5", name: "Claude Haiku 4.5", context: "200K", capabilities: [C("极速")] },
      { id: "google/gemini-2.5-pro", name: "Gemini 2.5 Pro", context: "1M", capabilities: [C("长文本")] },
      { id: "openai/gpt-4.1", name: "GPT-4.1", context: "1M", capabilities: [C("旗舰")] },
      { id: "deepseek/deepseek-chat", name: "DeepSeek V3", context: "64K", capabilities: [C("高性价比")] },
      { id: "meta-llama/llama-3.3-70b-instruct", name: "Llama 3.3 70B", context: "128K", capabilities: [C("开源")] },
    ],
  },
  {
    id: "siliconflow",
    name: "硅基流动",
    legalName: "SiliconFlow",
    mark: "SF",
    color: "#7C3AED",
    kind: "aggregator",
    backend: "custom",
    baseUrl: "https://api.siliconflow.cn/v1",
    apiKeyHint: "sk-...",
    apiKeyUrl: "https://cloud.siliconflow.cn/account/ak",
    docsUrl: "https://docs.siliconflow.cn/",
    description: { zh: "国产开源模型高速推理平台", en: "Fast inference for open Chinese models" },
    requiresKey: true,
    models: [
      { id: "deepseek-ai/DeepSeek-V3", name: "DeepSeek V3", context: "64K", capabilities: [C("推荐")], recommended: true },
      { id: "deepseek-ai/DeepSeek-R1", name: "DeepSeek R1", context: "64K", capabilities: [C("推理")] },
      { id: "Qwen/Qwen3-8B", name: "Qwen3 8B", context: "32K", capabilities: [C("轻量")] },
      { id: "Qwen/Qwen3-235B-A22B", name: "Qwen3 235B", context: "128K", capabilities: [C("旗舰")] },
    ],
  },
  {
    id: "qiniu",
    name: "七牛云 AI",
    legalName: "Qiniu AI Inference",
    mark: "Q7",
    color: "#00AAE6",
    kind: "aggregator",
    backend: "custom",
    baseUrl: "https://api.qnaigc.com/v1",
    apiKeyHint: "sk-...",
    apiKeyUrl: "https://portal.qiniu.com/ai-inference/api-key",
    docsUrl: "https://developer.qiniu.com/aitokenapi",
    description: { zh: "聚合多家主流模型的推理服务", en: "Aggregated inference across leading models" },
    requiresKey: true,
    models: [
      { id: "deepseek/deepseek-v4.1-flash", name: "DeepSeek V4.1 Flash", context: "128K", capabilities: [C("极速")], recommended: true },
      { id: "deepseek/deepseek-v3.2", name: "DeepSeek V3.2", context: "128K", capabilities: [C("均衡")] },
      { id: "qwen/qwen3-235b", name: "Qwen3 235B", context: "128K", capabilities: [C("旗舰")] },
      { id: "moonshot/kimi-k2", name: "Kimi K2", context: "256K", capabilities: [C("长文本")] },
    ],
  },
  {
    id: "groq",
    name: "Groq",
    legalName: "Groq LPU Inference",
    mark: "GQ",
    color: "#F55036",
    kind: "cloud",
    backend: "custom",
    baseUrl: "https://api.groq.com/openai/v1",
    apiKeyHint: "gsk_...",
    apiKeyUrl: "https://console.groq.com/keys",
    docsUrl: "https://console.groq.com/docs/models",
    description: { zh: "LPU 极速推理，适合大批量评估", en: "LPU-fast inference, great for bulk scoring" },
    requiresKey: true,
    models: [
      { id: "llama-3.3-70b-versatile", name: "Llama 3.3 70B", context: "128K", capabilities: [C("极速")], recommended: true },
      { id: "llama-3.1-8b-instant", name: "Llama 3.1 8B", context: "128K", capabilities: [C("轻量")] },
      { id: "openai/gpt-oss-120b", name: "GPT-OSS 120B", context: "128K", capabilities: [C("开源")] },
      { id: "meta-llama/llama-4-scout-17b-16e-instruct", name: "Llama 4 Scout", context: "128K", capabilities: [C("视觉")] },
    ],
  },
  {
    id: "mistral",
    name: "Mistral AI",
    legalName: "Mistral AI",
    mark: "M",
    color: "#FA520F",
    kind: "cloud",
    backend: "custom",
    baseUrl: "https://api.mistral.ai/v1",
    apiKeyHint: "...",
    apiKeyUrl: "https://console.mistral.ai/api-keys/",
    docsUrl: "https://docs.mistral.ai/getting-started/models/models_overview/",
    description: { zh: "欧洲模型，结构化输出稳定", en: "European models with reliable structured output" },
    requiresKey: true,
    models: [
      { id: "mistral-large-latest", name: "Mistral Large", context: "128K", capabilities: [C("旗舰")], recommended: true },
      { id: "mistral-small-latest", name: "Mistral Small", context: "32K", capabilities: [C("均衡")] },
      { id: "ministral-8b-latest", name: "Ministral 8B", context: "128K", capabilities: [C("轻量")] },
    ],
  },
  {
    id: "xai",
    name: "xAI Grok",
    legalName: "xAI",
    mark: "X",
    color: "#1D1D1F",
    kind: "cloud",
    backend: "custom",
    baseUrl: "https://api.x.ai/v1",
    apiKeyHint: "xai-...",
    apiKeyUrl: "https://console.x.ai/",
    docsUrl: "https://docs.x.ai/docs/models",
    description: { zh: "Grok 系列，实时信息能力强", en: "Grok family, strong at current events" },
    requiresKey: true,
    models: [
      { id: "grok-3", name: "Grok 3", context: "128K", capabilities: [C("旗舰")], recommended: true },
      { id: "grok-3-mini", name: "Grok 3 Mini", context: "128K", capabilities: [C("推理"), C("轻量")] },
      { id: "grok-3-fast", name: "Grok 3 Fast", context: "128K", capabilities: [C("极速")] },
    ],
  },
  {
    id: "together",
    name: "Together AI",
    legalName: "Together AI",
    mark: "TG",
    color: "#2563EB",
    kind: "aggregator",
    backend: "custom",
    baseUrl: "https://api.together.xyz/v1",
    apiKeyHint: "...",
    apiKeyUrl: "https://api.together.ai/settings/api-keys",
    docsUrl: "https://docs.together.ai/docs/inference-models",
    description: { zh: "开源模型托管与微调推理", en: "Hosted open models with fine-tuning" },
    requiresKey: true,
    models: [
      { id: "meta-llama/Llama-3.3-70B-Instruct-Turbo", name: "Llama 3.3 70B Turbo", context: "128K", capabilities: [C("推荐")], recommended: true },
      { id: "deepseek-ai/DeepSeek-V3", name: "DeepSeek V3", context: "128K", capabilities: [C("旗舰")] },
      { id: "Qwen/Qwen2.5-7B-Instruct-Turbo", name: "Qwen2.5 7B Turbo", context: "32K", capabilities: [C("轻量")] },
    ],
  },
  {
    id: "lmstudio",
    name: "LM Studio",
    legalName: "LM Studio (Local)",
    mark: "LM",
    color: "#5B4BE1",
    kind: "local",
    backend: "lmstudio",
    baseUrl: "http://localhost:1234/v1",
    apiKeyHint: "无需密钥",
    apiKeyUrl: "https://lmstudio.ai/docs/app/api/endpoints/openai",
    docsUrl: "https://lmstudio.ai/docs/app/api/endpoints/openai",
    description: { zh: "本地桌面模型服务，数据不出本机", en: "Local desktop models, nothing leaves your machine" },
    requiresKey: false,
    models: [
      { id: "local-model", name: "当前加载的模型", capabilities: [C("本地")], recommended: true },
    ],
  },
  {
    id: "ollama",
    name: "Ollama",
    legalName: "Ollama (Local)",
    mark: "OL",
    color: "#2C2C2C",
    kind: "local",
    backend: "custom",
    baseUrl: "http://localhost:11434/v1",
    apiKeyHint: "无需密钥",
    apiKeyUrl: "https://docs.ollama.com/api/openai-compatibility",
    docsUrl: "https://docs.ollama.com/api/openai-compatibility",
    description: { zh: "本地运行开源模型，OpenAI 兼容", en: "Self-hosted open models, OpenAI-compatible" },
    requiresKey: false,
    models: [
      { id: "qwen3", name: "Qwen 3", capabilities: [C("本地"), C("推理")], recommended: true },
      { id: "llama3.2", name: "Llama 3.2", capabilities: [C("本地")] },
      { id: "deepseek-r1", name: "DeepSeek R1", capabilities: [C("推理")] },
      { id: "gemma3", name: "Gemma 3", capabilities: [C("视觉")] },
    ],
  },
  {
    id: "custom",
    name: "自定义端点",
    nameEn: "Custom endpoint",
    legalName: "OpenAI-compatible endpoint",
    mark: "+",
    color: "#6B7280",
    kind: "cloud",
    backend: "custom",
    baseUrl: "",
    apiKeyHint: "按需填写",
    apiKeyUrl: "",
    docsUrl: "https://platform.openai.com/docs/api-reference/chat",
    description: { zh: "任意 OpenAI 兼容服务，手动填写配置", en: "Any OpenAI-compatible service, manual setup" },
    requiresKey: true,
    models: [],
  },
];

export const presetById = (id: string): ProviderPreset =>
  providerPresets.find((p) => p.id === id) ?? providerPresets[providerPresets.length - 1];

const normalizeBase = (url: string) => url.trim().replace(/\/+$/, "").toLowerCase();

/** Infer the preset from a saved profile so legacy configurations still render correctly. */
export function resolvePreset(provider: string, baseUrl: string): ProviderPreset {
  const normalized = normalizeBase(baseUrl);
  const candidates = providerPresets.filter((p) => p.backend === provider && p.id !== "custom");
  const match = candidates.find((p) => p.baseUrl && normalizeBase(p.baseUrl) === normalized);
  if (match) return match;

  if (!normalized) {
    const official = candidates.find((p) => !p.baseUrl);
    if (official) return official;
  }
  return presetById("custom");
}

/** Convert a friendly model id into the exact reference Genkit expects. */
export function toModelRef(backend: BackendProvider, modelId: string): string {
  const clean = modelId.trim().replace(/^\/+/, "");
  switch (backend) {
    case "gemini":
      return `googleai/${clean}`;
    case "openai":
      return `openai/${clean}`;
    case "lmstudio":
      return clean;
    case "custom":
    default:
      return clean.startsWith("custom/") ? clean : `custom/${clean}`;
  }
}

/** Strip the Genkit namespace so the model can be shown / matched in the catalog. */
export function fromModelRef(ref: string): string {
  const clean = (ref || "").trim();
  return clean.replace(/^(googleai|openai|custom)\//i, "");
}

export function defaultModelRef(preset: ProviderPreset): string {
  const model = preset.models.find((m) => m.recommended) ?? preset.models[0];
  return model ? toModelRef(preset.backend, model.id) : "";
}

export function findModelInfo(preset: ProviderPreset, ref: string): ModelInfo | undefined {
  const short = fromModelRef(ref);
  return preset.models.find((m) => m.id === short);
}

export const kindLabel: Record<ProviderKind, { zh: string; en: string }> = {
  cloud: { zh: "云端", en: "Cloud" },
  local: { zh: "本地", en: "Local" },
  aggregator: { zh: "聚合", en: "Aggregator" },
};

/** Provider display name in the active UI language. */
export const providerName = (p: ProviderPreset, language: "zh" | "en") =>
  language === "en" && p.nameEn ? p.nameEn : p.name;

/** Provider description in the active UI language. */
export const providerDescription = (p: ProviderPreset, language: "zh" | "en") =>
  p.description[language];
