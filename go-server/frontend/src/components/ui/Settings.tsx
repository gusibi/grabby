import type { ReactNode } from "react";
import { Loader2 } from "lucide-react";
import { useCopy } from "@/lib/copy";
import { uiCopy } from "./copy";

export function Mark({ label, color, size = "md" }: { label: string; color: string; size?: "sm" | "md" | "lg" }) {
  return (
    <span
      className={`g-mark ${size === "sm" ? "g-mark-sm" : size === "lg" ? "g-mark-lg" : ""}`}
      style={{ background: color }}
      aria-hidden
    >
      {label}
    </span>
  );
}

export function SettingSection({
  id,
  index,
  title,
  description,
  aside,
  children,
}: {
  id?: string;
  index?: string;
  title: string;
  description?: string;
  aside?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section id={id} className="g-panel scroll-mt-6">
      <div className="flex items-start justify-between gap-4 px-6 py-5 border-b" style={{ borderColor: "var(--g-line)" }}>
        <div className="flex items-start gap-3.5 min-w-0">
          {index && (
            <span className="g-mono text-[10px] pt-1.5 w-6 shrink-0" style={{ color: "var(--g-faint)" }}>
              {index}
            </span>
          )}
          <div className="min-w-0">
            <h2 className="g-section-title">{title}</h2>
            {description && (
              <p className="mt-1.5 text-[12px] leading-relaxed" style={{ color: "var(--g-muted)" }}>
                {description}
              </p>
            )}
          </div>
        </div>
        {aside && <div className="shrink-0 pt-0.5">{aside}</div>}
      </div>
      <div className="px-6 py-6">{children}</div>
    </section>
  );
}

export function SettingRow({
  label,
  description,
  children,
  stacked = false,
}: {
  label: string;
  description?: string;
  children: ReactNode;
  stacked?: boolean;
}) {
  return (
    <div className={`flex gap-6 ${stacked ? "flex-col" : "items-center justify-between"}`}>
      <div className={stacked ? "" : "min-w-0 flex-1"}>
        <div className="g-label">{label}</div>
        {description && <div className="g-hint mt-1 max-w-md">{description}</div>}
      </div>
      <div className={stacked ? "" : "shrink-0"}>{children}</div>
    </div>
  );
}

export function Toggle({ checked, onChange, label }: { checked: boolean; onChange: (v: boolean) => void; label: string }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      onClick={() => onChange(!checked)}
      className="relative inline-flex h-[22px] w-[40px] shrink-0 items-center rounded-full transition-colors duration-200"
      style={{ background: checked ? "var(--g-signal)" : "var(--g-line-strong)" }}
    >
      <span
        className="absolute h-[16px] w-[16px] rounded-full bg-white shadow-sm transition-transform duration-300"
        style={{
          transform: checked ? "translateX(21px)" : "translateX(3px)",
          transitionTimingFunction: "cubic-bezier(0.16, 1, 0.3, 1)",
        }}
      />
    </button>
  );
}

export interface SegmentOption<T extends string> {
  value: T;
  label: string;
  description?: string;
}

export function Segmented<T extends string>({
  value,
  options,
  onChange,
}: {
  value: T;
  options: SegmentOption<T>[];
  onChange: (v: T) => void;
}) {
  return (
    <div
      className="inline-flex p-1 rounded-xl gap-1"
      style={{ background: "var(--g-panel-2)", border: "1px solid var(--g-line)" }}
      role="tablist"
    >
      {options.map((opt) => {
        const active = opt.value === value;
        return (
          <button
            key={opt.value}
            role="tab"
            aria-selected={active}
            onClick={() => onChange(opt.value)}
            className="px-3 py-2 rounded-lg text-[12px] font-medium transition-all duration-200 text-left"
            style={{
              background: active ? "var(--g-panel)" : "transparent",
              color: active ? "var(--g-ink)" : "var(--g-muted)",
              boxShadow: active ? "var(--g-shadow)" : "none",
            }}
          >
            {opt.label}
            {opt.description && (
              <span className="block text-[10px] font-normal mt-0.5" style={{ color: "var(--g-faint)" }}>
                {opt.description}
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}

export function Spinner({ size = 14 }: { size?: number }) {
  return <Loader2 size={size} className="g-spin" />;
}

export function ScoreRing({ score }: { score: number }) {
  const c = useCopy(uiCopy);
  const value = Math.max(0, Math.min(10, score || 0));
  return (
    <div className="g-score" style={{ ["--score" as string]: value }} aria-label={c.scoreLabel(value)}>
      <span className="relative">{value}</span>
    </div>
  );
}
