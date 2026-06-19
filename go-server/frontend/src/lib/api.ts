import type { SourceForm } from "@/types";

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const jsonFetch = async <T = any>(url: string, options?: RequestInit): Promise<T> => {
  const response = await fetch(url, options);
  return response.json();
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
  saveAISettings: (settings: Record<string, unknown>) => jsonFetch("/api/ai/settings", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(settings),
  }),
  testAI: () => jsonFetch("/api/ai/test", { method: "POST" }),
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

