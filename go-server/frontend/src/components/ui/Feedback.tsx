import { useCallback, useMemo, useRef, useState } from "react";
import type { ReactNode } from "react";
import { AlertTriangle, Check, Info, X, XCircle } from "lucide-react";
import { FeedbackContext } from "./feedback-context";
import { useCopy } from "@/lib/copy";
import { uiCopy } from "./copy";
import type { ConfirmOptions, FeedbackContextValue, ToastTone } from "./feedback-context";

interface ToastItem {
  id: number;
  title: string;
  description?: string;
  tone: ToastTone;
}

const toneIcon: Record<ToastTone, ReactNode> = {
  success: <Check size={15} />,
  error: <XCircle size={15} />,
  info: <Info size={15} />,
};

const toneColor: Record<ToastTone, string> = {
  success: "var(--g-signal)",
  error: "var(--g-danger)",
  info: "var(--g-muted)",
};

export function FeedbackProvider({ children }: { children: ReactNode }) {
  const c = useCopy(uiCopy);
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const [confirmState, setConfirmState] = useState<ConfirmOptions | null>(null);
  const nextId = useRef(1);
  const resolver = useRef<((value: boolean) => void) | null>(null);

  const dismiss = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const toast = useCallback((title: string, description?: string, tone: ToastTone = "info") => {
    const id = nextId.current++;
    setToasts((prev) => [...prev.slice(-3), { id, title, description, tone }]);
    window.setTimeout(() => dismiss(id), 4600);
  }, [dismiss]);

  const confirm = useCallback((options: ConfirmOptions) => {
    setConfirmState(options);
    return new Promise<boolean>((resolve) => {
      resolver.current = resolve;
    });
  }, []);

  const settleConfirm = (value: boolean) => {
    setConfirmState(null);
    resolver.current?.(value);
    resolver.current = null;
  };

  const value = useMemo<FeedbackContextValue>(() => ({ toast, confirm }), [toast, confirm]);

  return (
    <FeedbackContext.Provider value={value}>
      {children}

      <div className="g-toast-viewport" role="status" aria-live="polite">
        {toasts.map((t) => (
          <div key={t.id} className="g-toast">
            <span
              className="flex items-center justify-center shrink-0 mt-0.5"
              style={{ color: toneColor[t.tone] }}
            >
              {toneIcon[t.tone]}
            </span>
            <div className="min-w-0 flex-1">
              <div className="g-toast-title">{t.title}</div>
              {t.description && <div className="g-toast-desc">{t.description}</div>}
            </div>
            <button className="g-icon-btn !w-5 !h-5 !-mt-1 !-mr-1" onClick={() => dismiss(t.id)} aria-label={c.close}>
              <X size={13} />
            </button>
          </div>
        ))}
      </div>

      {confirmState && (
        <div className="g-scrim" onClick={() => settleConfirm(false)}>
          <div className="g-modal" onClick={(e) => e.stopPropagation()} role="alertdialog" aria-modal="true">
            <div className="flex items-start gap-3">
              <span
                className="w-9 h-9 rounded-xl flex items-center justify-center shrink-0"
                style={{
                  background: confirmState.destructive ? "color-mix(in srgb, var(--g-danger) 12%, transparent)" : "var(--g-panel-2)",
                  color: confirmState.destructive ? "var(--g-danger)" : "var(--g-muted)",
                }}
              >
                <AlertTriangle size={17} />
              </span>
              <div className="min-w-0 flex-1 pt-0.5">
                <h3 className="text-[15px] font-semibold tracking-tight">{confirmState.title}</h3>
                {confirmState.description && (
                  <p className="mt-1.5 text-[12.5px] leading-relaxed" style={{ color: "var(--g-muted)" }}>
                    {confirmState.description}
                  </p>
                )}
              </div>
            </div>
            <div className="flex justify-end gap-2 mt-7">
              <button className="g-btn g-btn-secondary g-btn-sm" onClick={() => settleConfirm(false)}>
                {confirmState.cancelLabel ?? c.cancel}
              </button>
              <button
                className={`g-btn g-btn-sm ${confirmState.destructive ? "g-btn-danger" : "g-btn-primary"}`}
                onClick={() => settleConfirm(true)}
                autoFocus
              >
                {confirmState.confirmLabel ?? c.confirm}
              </button>
            </div>
          </div>
        </div>
      )}
    </FeedbackContext.Provider>
  );
}
