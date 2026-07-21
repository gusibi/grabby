import { useEffect, useState, useCallback } from "react";
import { RefreshCw, ExternalLink } from "lucide-react";
import { Button } from "@astryxdesign/core/Button";
import { Card } from "@astryxdesign/core/Card";
import { Table, proportional, pixel } from "@astryxdesign/core/Table";
import { VStack, HStack } from "@astryxdesign/core/Layout";
import { api } from "@/lib/api";
import { useTranslation } from "@/lib/i18n";
import type { ExtractCaptureRecord } from "@/types";

export function ExtractCapturesView() {
  const [records, setRecords] = useState<ExtractCaptureRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const { t, language } = useTranslation();

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

  const columns = [
    {
      key: "title",
      header: language === "zh" ? "标题 / URL" : "Title / URL",
      width: proportional(2),
      renderCell: (r: ExtractCaptureRecord) => (
        <VStack gap={0.5}>
          <div className="font-semibold truncate max-w-lg">{r.title || (language === "zh" ? "(无标题)" : "(No Title)")}</div>
          <div className="text-[10px] text-zinc-400 font-mono truncate max-w-lg">{r.url}</div>
        </VStack>
      )
    },
    {
      key: "chars",
      header: language === "zh" ? "字数" : "Word Count",
      width: pixel(90),
      align: "center" as const,
      renderCell: (r: ExtractCaptureRecord) => <span className="font-mono">{r.chars.toLocaleString()}</span>
    },
    {
      key: "updated_at",
      header: language === "zh" ? "更新时间" : "Updated At",
      width: pixel(160),
      renderCell: (r: ExtractCaptureRecord) => <span className="font-mono">{new Date(r.updated_at).toLocaleString()}</span>
    },
    {
      key: "url",
      header: language === "zh" ? "打开" : "Open",
      width: pixel(60),
      align: "center" as const,
      renderCell: (r: ExtractCaptureRecord) => (
        <Button
          href={r.url}
          target="_blank"
          rel="noreferrer"
          variant="ghost"
          size="sm"
          isIconOnly
          label={t("btn.openLink")}
          icon={<ExternalLink className="w-3.5 h-3.5 text-zinc-400 hover:text-blue-500" />}
        />
      )
    }
  ];

  return (
    <div className="flex-1 overflow-y-auto bg-body p-8">
      <VStack gap={6} className="max-w-5xl mx-auto pb-12">
        <HStack gap={3} vAlign="center" hAlign="between">
          <div>
            <h3 className="text-base font-bold text-primary">{t("title.extract")}</h3>
            <p className="text-xs text-secondary mt-1">
              {language === "zh" 
                ? `通过 /api/extract 抓取并缓存的网页（按 URL 去重，共 ${total} 条）。再次请求同一 URL 将直接命中缓存。`
                : `Webpages crawled and cached via /api/extract (deduplicated by URL, total ${total}). Future requests to the same URL hit the cache directly.`}
            </p>
          </div>
          <Button 
            onClick={load} 
            size="sm" 
            variant="secondary" 
            isDisabled={loading}
            isLoading={loading}
            label={t("btn.refresh")}
            icon={<RefreshCw className="w-3.5 h-3.5" />}
          />
        </HStack>

        <Card className="overflow-hidden">
          <Table<any>
            data={records}
            columns={columns}
            idKey="url"
            density="compact"
            dividers="rows"
            hasHover
          />
        </Card>
      </VStack>
    </div>
  );
}
