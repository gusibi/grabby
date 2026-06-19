import { useEffect, useState, useCallback } from "react";
import { RefreshCw, ExternalLink, Heart, Repeat2, MessageCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { api } from "@/lib/api";
import type { TweetCaptureRecord } from "@/types";

const SOURCES = [
  { key: "", label: "全部" },
  { key: "search", label: "搜索" },
  { key: "timeline", label: "时间线" },
  { key: "likes", label: "点赞" },
];

export function TwitterCapturesView() {
  const [records, setRecords] = useState<TweetCaptureRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [source, setSource] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api.getTwitterCaptures(source, 200, 0);
      setRecords(res.items || []);
      setTotal(res.total || 0);
    } catch {
      setRecords([]);
    } finally {
      setLoading(false);
    }
  }, [source]);

  useEffect(() => { load(); }, [load]);

  return (
    <div className="flex-1 overflow-y-auto bg-zinc-50/50 dark:bg-[#121212] p-8">
      <div className="max-w-5xl mx-auto space-y-6 pb-12">
        <div className="flex justify-between items-center">
          <div>
            <h3 className="text-base font-bold">推文归档 Twitter Captures</h3>
            <p className="text-xs text-zinc-500">search / timeline / likes 抓到的推文按 ID 去重存档（共 {total} 条）。</p>
          </div>
          <Button onClick={load} size="sm" variant="outline" className="h-8 gap-1 text-xs" disabled={loading}>
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? "animate-spin" : ""}`} /> 刷新
          </Button>
        </div>

        <div className="flex gap-1.5">
          {SOURCES.map((s) => (
            <button
              key={s.key}
              onClick={() => setSource(s.key)}
              className={`px-3 py-1 rounded-lg text-xs font-medium transition-all ${
                source === s.key
                  ? "bg-blue-600 text-white shadow-sm"
                  : "bg-black/5 dark:bg-white/5 text-zinc-600 dark:text-zinc-400 hover:bg-black/10 dark:hover:bg-white/10"
              }`}
            >
              {s.label}
            </button>
          ))}
        </div>

        <Card className="border border-black/5 dark:border-white/5 bg-white dark:bg-[#1c1c1e] shadow-sm rounded-2xl overflow-hidden">
          <Table>
            <TableHeader className="bg-zinc-50/50 dark:bg-zinc-900/50">
              <TableRow>
                <TableHead className="text-xs font-bold w-[140px]">作者</TableHead>
                <TableHead className="text-xs font-bold">正文</TableHead>
                <TableHead className="text-xs font-bold w-[130px]">互动</TableHead>
                <TableHead className="text-xs font-bold text-center w-[70px]">来源</TableHead>
                <TableHead className="text-xs font-bold text-center w-[50px]">打开</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {records.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="text-center text-xs text-zinc-500 py-10">
                    {loading ? "加载中…" : "暂无推文归档。"}
                  </TableCell>
                </TableRow>
              ) : (
                records.map((t) => (
                  <TableRow key={t.id}>
                    <TableCell className="text-xs align-top">
                      <div className="font-semibold truncate max-w-[120px]">{t.author_name || t.author}</div>
                      <div className="text-[10px] text-zinc-400 truncate max-w-[120px]">@{t.author}</div>
                    </TableCell>
                    <TableCell className="text-xs text-zinc-700 dark:text-zinc-300">
                      <div className="line-clamp-3 max-w-md">{t.text}</div>
                    </TableCell>
                    <TableCell className="text-[10px] text-zinc-500 font-mono">
                      <div className="flex gap-2">
                        <span className="inline-flex items-center gap-0.5"><Heart className="w-3 h-3" />{t.favorite_count}</span>
                        <span className="inline-flex items-center gap-0.5"><Repeat2 className="w-3 h-3" />{t.retweet_count}</span>
                        <span className="inline-flex items-center gap-0.5"><MessageCircle className="w-3 h-3" />{t.reply_count}</span>
                      </div>
                    </TableCell>
                    <TableCell className="text-center">
                      <span className="text-[9px] px-1.5 py-0.5 rounded-full bg-blue-500/10 text-blue-600 font-bold">{t.source}</span>
                    </TableCell>
                    <TableCell className="text-center">
                      {t.url ? (
                        <a href={t.url} target="_blank" rel="noreferrer" className="inline-flex text-zinc-400 hover:text-blue-500">
                          <ExternalLink className="w-3.5 h-3.5" />
                        </a>
                      ) : "-"}
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
