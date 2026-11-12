/* eslint-disable react-hooks/set-state-in-effect -- legacy data views load on mount */
import { useEffect, useState, useCallback } from "react";
import { ExternalLink, RefreshCw } from "lucide-react";
import { api } from "@/lib/api";
import { useCopy } from "@/lib/copy";
import { capturesCopy } from "./copy";
import { PageFrame, DataTable, DataRow, DataCell, EmptyRow } from "@/components/ui/PageFrame";
import type { ExtractCaptureRecord } from "@/types";

export function ExtractCapturesView() {
  const [records, setRecords] = useState<ExtractCaptureRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const c = useCopy(capturesCopy);

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
    <PageFrame
      kicker={c.extractKicker}
      title={c.extractTitle}
      description={c.extractDescription(total)}
      actions={
        <button className="g-btn g-btn-secondary g-btn-sm" onClick={load} disabled={loading}>
          <RefreshCw size={12} className={loading ? "g-spin" : ""} /> {c.refresh}
        </button>
      }
    >
      <DataTable head={[c.extractColTitle, c.extractColChars, c.extractColUpdated, ""]}>
        {records.length === 0 ? (
          <EmptyRow colSpan={4}>{c.extractEmpty}</EmptyRow>
        ) : (
          records.map((r) => (
            <DataRow key={r.url}>
              <DataCell>
                <div className="font-semibold truncate max-w-[420px]">{r.title || c.extractNoTitle}</div>
                <div className="g-mono text-[10px] mt-1 truncate max-w-[420px]" style={{ color: "var(--g-faint)" }}>{r.url}</div>
              </DataCell>
              <DataCell className="g-mono">{r.chars.toLocaleString()}</DataCell>
              <DataCell className="g-mono !text-[10.5px]" >{new Date(r.updated_at).toLocaleString()}</DataCell>
              <DataCell className="!text-right">
                <a className="g-icon-btn inline-flex" href={r.url} target="_blank" rel="noreferrer" title={c.open}>
                  <ExternalLink size={13} />
                </a>
              </DataCell>
            </DataRow>
          ))
        )}
      </DataTable>
    </PageFrame>
  );
}
