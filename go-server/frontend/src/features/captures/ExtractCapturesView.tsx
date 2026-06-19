import { useEffect, useState, useCallback } from "react";
import { RefreshCw, ExternalLink } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { api } from "@/lib/api";
import type { ExtractCaptureRecord } from "@/types";

export function ExtractCapturesView() {
  const [records, setRecords] = useState<ExtractCaptureRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api.getExtractCaptures(200, 0);
      setRecords(res.items || []);
      setTotal(res.total || 0);
    } catch {
      setRecords([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  return (
    <div className="flex-1 overflow-y-auto bg-zinc-50/50 dark:bg-[#121212] p-8">
      <div className="max-w-5xl mx-auto space-y-6 pb-12">
        <div className="flex justify-between items-center">
          <div>
            <h3 className="text-base font-bold">网页提取记录 Extract Captures</h3>
            <p className="text-xs text-zinc-500">通过 /api/extract 抓取并缓存的网页（按 URL 去重，共 {total} 条）。再次请求同一 URL 将直接命中缓存。</p>
          </div>
          <Button onClick={load} size="sm" variant="outline" className="h-8 gap-1 text-xs" disabled={loading}>
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? "animate-spin" : ""}`} /> 刷新
          </Button>
        </div>

        <Card className="border border-black/5 dark:border-white/5 bg-white dark:bg-[#1c1c1e] shadow-sm rounded-2xl overflow-hidden">
          <Table>
            <TableHeader className="bg-zinc-50/50 dark:bg-zinc-900/50">
              <TableRow>
                <TableHead className="text-xs font-bold">标题 / URL</TableHead>
                <TableHead className="text-xs font-bold text-center w-[90px]">字数</TableHead>
                <TableHead className="text-xs font-bold w-[150px]">更新时间</TableHead>
                <TableHead className="text-xs font-bold text-center w-[60px]">打开</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {records.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={4} className="text-center text-xs text-zinc-500 py-10">
                    {loading ? "加载中…" : "暂无网页提取记录。"}
                  </TableCell>
                </TableRow>
              ) : (
                records.map((r) => (
                  <TableRow key={r.url}>
                    <TableCell className="text-xs">
                      <div className="font-semibold truncate max-w-md">{r.title || "(无标题)"}</div>
                      <div className="text-[10px] text-zinc-400 font-mono truncate max-w-md">{r.url}</div>
                    </TableCell>
                    <TableCell className="text-xs text-center font-mono">{r.chars.toLocaleString()}</TableCell>
                    <TableCell className="text-xs text-zinc-500 font-mono">{new Date(r.updated_at).toLocaleString()}</TableCell>
                    <TableCell className="text-center">
                      <a href={r.url} target="_blank" rel="noreferrer" className="inline-flex text-zinc-400 hover:text-blue-500">
                        <ExternalLink className="w-3.5 h-3.5" />
                      </a>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </Card>
      </div>
    </div>
  );
}
