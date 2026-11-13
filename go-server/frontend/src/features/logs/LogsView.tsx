import { RefreshCw } from "lucide-react";
import { useCopy } from "@/lib/copy";
import { logsCopy } from "./copy";
import { PageFrame, DataTable, DataRow, DataCell, EmptyRow } from "@/components/ui/PageFrame";
import type { FetchLog } from "@/types";

interface LogsViewProps {
  fetchLogs: () => void;
  logs: FetchLog[];
}

const statusStyle: Record<string, { bg: string; color: string }> = {
  success: { bg: "var(--g-signal-soft)", color: "var(--g-signal)" },
  running: { bg: "var(--g-panel-2)", color: "var(--g-muted)" },
  skipped: { bg: "var(--g-panel-2)", color: "var(--g-faint)" },
  error: { bg: "color-mix(in srgb, var(--g-danger) 10%, transparent)", color: "var(--g-danger)" },
};

export function LogsView({ fetchLogs, logs }: LogsViewProps) {
  const c = useCopy(logsCopy);

  return (
    <PageFrame
      kicker={c.kicker}
      title={c.title}
      description={c.description}
      actions={
        <button className="g-btn g-btn-secondary g-btn-sm" onClick={fetchLogs}>
          <RefreshCw size={12} /> {c.refresh}
        </button>
      }
    >
      <DataTable head={[
        c.colStarted,
        c.colSource,
        c.colStatus,
        c.colFound,
        c.colAdded,
        c.colDiagnostic,
      ]}>
        {logs.length === 0 ? (
          <EmptyRow colSpan={6}>{c.empty}</EmptyRow>
        ) : (
          logs.map((log) => {
            const style = statusStyle[log.status] || statusStyle.error;
            return (
              <DataRow key={log.id}>
                <DataCell className="g-mono !text-[10.5px]" >{new Date(log.started_at).toLocaleString()}</DataCell>
                <DataCell className="font-semibold">{log.source_id}</DataCell>
                <DataCell>
                  <span className="g-chip !h-[20px] !text-[9.5px]" style={{ background: style.bg, color: style.color, borderColor: "transparent" }}>
                    {log.status}
                  </span>
                </DataCell>
                <DataCell className="g-mono">{log.items_found}</DataCell>
                <DataCell className="g-mono">{log.items_added}</DataCell>
                <DataCell className="g-mono !text-[10.5px] max-w-[280px] truncate" >
                  <span style={{ color: "var(--g-muted)" }}>{log.error_message || "—"}</span>
                </DataCell>
              </DataRow>
            );
          })
        )}
      </DataTable>
    </PageFrame>
  );
}
