import { useEffect, useState, useCallback } from "react";
import { RefreshCw, ExternalLink, Heart, Repeat2, MessageCircle } from "lucide-react";
import { Button } from "@astryxdesign/core/Button";
import { Card } from "@astryxdesign/core/Card";
import { Table, proportional, pixel } from "@astryxdesign/core/Table";
import { VStack, HStack } from "@astryxdesign/core/Layout";
import { Badge } from "@astryxdesign/core/Badge";
import { api } from "@/lib/api";
import { useTranslation } from "@/lib/i18n";
import type { TweetCaptureRecord } from "@/types";

export function TwitterCapturesView() {
  const [records, setRecords] = useState<TweetCaptureRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [source, setSource] = useState("");
  const { t, language } = useTranslation();

  const SOURCES = [
    { key: "", label: language === "zh" ? "全部" : "All" },
    { key: "search", label: language === "zh" ? "搜索" : "Search" },
    { key: "timeline", label: language === "zh" ? "时间线" : "Timeline" },
    { key: "likes", label: language === "zh" ? "点赞" : "Likes" },
  ];

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

  const columns = [
    {
      key: "author",
      header: language === "zh" ? "作者" : "Author",
      width: pixel(140),
      renderCell: (t: TweetCaptureRecord) => (
        <VStack gap={0.5}>
          <div className="font-semibold truncate max-w-[120px]">{t.author_name || t.author}</div>
          <div className="text-[10px] text-zinc-400 truncate max-w-[120px]">@{t.author}</div>
        </VStack>
      )
    },
    {
      key: "text",
      header: language === "zh" ? "正文" : "Text",
      width: proportional(2),
      renderCell: (t: TweetCaptureRecord) => (
        <div className="line-clamp-3 max-w-lg text-zinc-700 dark:text-zinc-300">
          {t.text}
        </div>
      )
    },
    {
      key: "interactions",
      header: language === "zh" ? "互动" : "Interactions",
      width: pixel(130),
      renderCell: (t: TweetCaptureRecord) => (
        <HStack gap={2} vAlign="center" className="font-mono text-[10px] text-zinc-500">
          <span className="inline-flex items-center gap-0.5"><Heart className="w-3 h-3 text-rose-500" />{t.favorite_count}</span>
          <span className="inline-flex items-center gap-0.5"><Repeat2 className="w-3 h-3 text-green-500" />{t.retweet_count}</span>
          <span className="inline-flex items-center gap-0.5"><MessageCircle className="w-3 h-3 text-blue-500" />{t.reply_count}</span>
        </HStack>
      )
    },
    {
      key: "source",
      header: language === "zh" ? "来源" : "Source",
      width: pixel(80),
      align: "center" as const,
      renderCell: (t: TweetCaptureRecord) => <Badge variant="info" label={t.source} />
    },
    {
      key: "url",
      header: language === "zh" ? "打开" : "Open",
      width: pixel(60),
      align: "center" as const,
      renderCell: (t: TweetCaptureRecord) => t.url ? (
        <Button
          href={t.url}
          target="_blank"
          rel="noreferrer"
          variant="ghost"
          size="sm"
          isIconOnly
          label={language === "zh" ? "打开推文" : "Open Tweet"}
          icon={<ExternalLink className="w-3.5 h-3.5 text-zinc-400 hover:text-blue-500" />}
        />
      ) : "-"
    }
  ];

  return (
    <div className="flex-1 overflow-y-auto bg-body p-8">
      <VStack gap={6} className="max-w-5xl mx-auto pb-12">
        <HStack gap={3} vAlign="center" hAlign="between">
          <div>
            <h3 className="text-base font-bold text-primary">{t("title.twitter")}</h3>
            <p className="text-xs text-secondary mt-1">
              {language === "zh"
                ? `search / timeline / likes 抓到的推文按 ID 去重存档（共 ${total} 条）。`
                : `Deduplicated tweet archive captured via search, timeline, and likes (total ${total}).`}
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

        <HStack gap={1.5} vAlign="center" className="flex-wrap">
          {SOURCES.map((s) => (
            <Button
              key={s.key}
              onClick={() => setSource(s.key)}
              size="sm"
              variant={source === s.key ? "primary" : "secondary"}
              label={s.label}
            />
          ))}
        </HStack>

        <Card className="overflow-hidden">
          <Table<any>
            data={records}
            columns={columns}
            idKey="id"
            density="compact"
            dividers="rows"
            hasHover
          />
        </Card>
      </VStack>
    </div>
  );
}
