import type { SourceForm } from "@/types";

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const jsonFetch = async <T = any>(url: string, options?: RequestInit): Promise<T> => {
  const response = await fetch(url, options);
  const text = await response.text();
  try {
    return JSON.parse(text) as T;
  } catch {
    // Some legacy endpoints return plain-text errors / empty bodies.
    return { success: false, error: text || `请求失败 (${response.status})` } as T;
  }
};

export const api = {
  getAuthSession: () => jsonFetch("/api/auth/session"),
  login: (key: string) => jsonFetch("/api/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ key }),
  }),
  logout: () => jsonFetch("/api/auth/logout", { method: "POST" }),
  getHealth: () => jsonFetch("/open/api/health"),
  getStats: () => jsonFetch("/open/api/stats"),
  getSources: () => jsonFetch("/api/sources"),
  getLogs: () => jsonFetch("/api/logs"),
  getAICategories: () => jsonFetch("/open/api/ai/categories"),
  getDailyReport: (date: string, reportType: string) => (
    jsonFetch(`/open/api/ai/daily?date=${date}&type=${reportType}`)
  ),
  getReportList: (limit = 30) => jsonFetch(`/open/api/ai/daily/list?limit=${limit}`),
  generateDailyReport: (date: string, reportType: string) => jsonFetch("/api/ai/daily/generate", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ date, report_type: reportType }),
  }),
  getItems: (url: string) => jsonFetch(url),
  getItemDetail: (id: number) => jsonFetch(`/open/api/items/${id}`),
  setItemStarred: (id: number, starred: number) => fetch(`/api/items/${id}/star`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ starred }),
  }),
  toggleSource: (id: string, enabled: number) => fetch(`/api/sources/${id}/toggle`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ enabled }),
  }),
  runSource: (id: string) => jsonFetch(`/api/sources/${id}/run`, { method: "POST" }),
  deleteSource: (id: string) => fetch(`/api/sources/${id}`, { method: "DELETE" }),
  saveSource: (sourceForm: SourceForm, editingSourceId?: string) => fetch(editingSourceId ? `/api/sources/${editingSourceId}` : "/api/sources", {
    method: editingSourceId ? "PUT" : "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(sourceForm),
  }),
  getAISettings: () => jsonFetch("/api/ai/settings"),
  getAICatalog: (refresh = false) => jsonFetch(`/api/ai/catalog${refresh ? "?refresh=1" : ""}`),
  discoverAIModels: (payload: { provider: string; base_url?: string; api_key?: string; profile_id?: string }) => jsonFetch("/api/ai/models/discover", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  }),
  importAIConfig: (source = "auto", apply = true) => jsonFetch("/api/ai/import", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ source, apply }),
  }),
  getAIUsage: (days = 7) => jsonFetch(`/api/ai/usage?days=${days}`),
  saveAISettings: (settings: Record<string, unknown>) => jsonFetch("/api/ai/settings", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(settings),
  }),
  testAI: (draft?: { profile: unknown; system_prompt: string }) => jsonFetch("/api/ai/test", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(draft ?? {}),
  }),
  exportConfig: async () => {
    const response = await fetch("/api/config/export");
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || `请求失败 (${response.status})`);
    }
    const disposition = response.headers.get("Content-Disposition") || "";
    const match = /filename="?([^";]+)"?/.exec(disposition);
    return {
      blob: await response.blob(),
      filename: match?.[1] || "grabby-config.json",
    };
  },
  importConfig: (data: unknown) => jsonFetch("/api/config/import", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ data }),
  }),
  startEvaluation: () => jsonFetch("/api/ai/start_eval", { method: "POST" }),
  getExtractCaptures: (limit = 100, offset = 0) => jsonFetch(`/api/captures/extract?limit=${limit}&offset=${offset}`),
  getTwitterCaptures: (source = "", limit = 100, offset = 0) => jsonFetch(
    `/api/captures/twitter?limit=${limit}&offset=${offset}${source ? `&source=${encodeURIComponent(source)}` : ""}`
  ),
  getBrowsers: () => jsonFetch("/api/browsers"),
  registerBrowser: (connectId: string, name: string) => jsonFetch("/api/browsers/register", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ connect_id: connectId, name }),
  }),
  kickBrowser: (connId: string) => jsonFetch("/api/browsers/kick", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ conn_id: connId }),
  }),
  banBrowser: (connId: string) => jsonFetch("/api/browsers/ban", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ conn_id: connId }),
  }),
  unbanBrowser: (connId: string) => jsonFetch("/api/browsers/unban", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ conn_id: connId }),
  }),
};

