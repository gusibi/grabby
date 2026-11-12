/* eslint-disable react-hooks/set-state-in-effect -- legacy data views load on mount */
import { useEffect, useState, useCallback } from "react";
import { ExternalLink, Heart, MessageCircle, RefreshCw, Repeat2 } from "lucide-react";
import { api } from "@/lib/api";
import { useCopy } from "@/lib/copy";
import { capturesCopy } from "./copy";
import { PageFrame, DataTable, DataRow, DataCell, EmptyRow } from "@/components/ui/PageFrame";
import type { TweetCaptureRecord } from "@/types";

export function TwitterCapturesView() {
  const [records, setRecords] = useState<TweetCaptureRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [source, setSource] = useState("");
  const c = useCopy(capturesCopy);

  const sources = [
    { key: "", label: c.all },
    { key: "search", label: c.srcSearch },
    { key: "timeline", label: c.srcTimeline },
    { key: "likes", label: c.srcLikes },
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

  return (
    <PageFrame
      kicker={c.tweetKicker}
      title={c.tweetTitle}
      description={c.tweetDescription(total)}
      actions={
        <button className="g-btn g-btn-secondary g-btn-sm" onClick={load} disabled={loading}>
          <RefreshCw size={12} className={loading ? "g-spin" : ""} /> {c.refresh}
        </button>
      }
    >
      <div className="flex items-center gap-1.5 mb-4">
        {sources.map((s) => (
          <button
            key={s.key}
            onClick={() => setSource(s.key)}
            className="g-chip !h-[26px] !px-3 !text-[10.5px]"
            style={{
              background: source === s.key ? "var(--g-signal)" : "var(--g-panel)",
              color: source === s.key ? "var(--g-on-signal)" : "var(--g-muted)",
              borderColor: source === s.key ? "var(--g-signal)" : "var(--g-line)",
            }}
          >
            {s.label}
          </button>
        ))}
      </div>

      <DataTable head={[c.tweetColAuthor, c.tweetColText, c.tweetColEngagement, c.tweetColSource, ""]}>
        {records.length === 0 ? (
          <EmptyRow colSpan={5}>{c.tweetEmpty}</EmptyRow>
        ) : (
          records.map((r) => (
            <DataRow key={r.id}>
              <DataCell>
                <div className="font-semibold truncate max-w-[130px]">{r.author_name || r.author}</div>
                <div className="g-mono text-[10px] mt-1 truncate max-w-[130px]" style={{ color: "var(--g-faint)" }}>@{r.author}</div>
              </DataCell>
              <DataCell>
                <div className="line-clamp-3 max-w-[420px] leading-relaxed" style={{ color: "var(--g-muted)" }}>{r.text}</div>
              </DataCell>
              <DataCell>
                <div className="flex items-center gap-2.5 g-mono !text-[10px]" style={{ color: "var(--g-faint)" }}>
                  <span className="inline-flex items-center gap-1"><Heart size={11} style={{ color: "#e26d78" }} />{r.favorite_count}</span>
                  <span className="inline-flex items-center gap-1"><Repeat2 size={11} style={{ color: "#6aa87a" }} />{r.retweet_count}</span>
                  <span className="inline-flex items-center gap-1"><MessageCircle size={11} style={{ color: "#6b8fc4" }} />{r.reply_count}</span>
                </div>
              </DataCell>
              <DataCell><span className="g-chip !h-[19px] !text-[9.5px]">{r.source}</span></DataCell>
              <DataCell className="!text-right">
                {r.url ? (
                  <a className="g-icon-btn inline-flex" href={r.url} target="_blank" rel="noreferrer" title={c.tweetOpen}>
                    <ExternalLink size={13} />
                  </a>
                ) : "—"}
              </DataCell>
            </DataRow>
          ))
        )}
      </DataTable>
    </PageFrame>
  );
}
