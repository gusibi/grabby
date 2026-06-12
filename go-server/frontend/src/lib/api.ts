import type { SourceForm } from "@/types";

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const jsonFetch = async <T = any>(url: string, options?: RequestInit): Promise<T> => {
  const response = await fetch(url, options);
  return response.json();
};

export const api = {
  getHealth: () => jsonFetch("/api/health"),
  getStats: () => jsonFetch("/api/stats"),
  getSources: () => jsonFetch("/api/sources"),
  getLogs: () => jsonFetch("/api/logs"),
  getAICategories: () => jsonFetch("/api/ai/categories"),
  getDailyReport: (date: string, reportType: string) => (
    jsonFetch(`/api/ai/daily?date=${date}&type=${reportType}`)
  ),
  getReportList: (limit = 30) => jsonFetch(`/api/ai/daily/list?limit=${limit}`),
  generateDailyReport: (date: string, reportType: string) => jsonFetch("/api/ai/daily/generate", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ date, report_type: reportType }),
  }),
  getItems: (url: string) => jsonFetch(url),
  getItemDetail: (id: number) => jsonFetch(`/api/items/${id}`),
  setItemStarred: (id: number, starred: number) => fetch(`/api/items/${id}/star`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ starred }),
  }),
  setItemReadStatus: (id: number, read_status: number) => fetch(`/api/items/${id}/read`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ read_status }),
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
};
