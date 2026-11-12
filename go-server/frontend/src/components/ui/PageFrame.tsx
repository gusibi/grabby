import type { ReactNode } from "react";

export function PageFrame({
  kicker,
  title,
  description,
  actions,
  children,
}: {
  kicker: string;
  title: string;
  description?: string;
  actions?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="g-scroll flex-1 overflow-y-auto" style={{ background: "var(--g-paper)" }}>
      <div className="max-w-[1180px] mx-auto px-9 pt-8 pb-16">
        <header className="g-rise flex flex-wrap items-end justify-between gap-5">
          <div>
            <div className="g-eyebrow">{kicker}</div>
            <h1 className="g-display mt-2.5 text-[clamp(2rem,3.2vw,2.75rem)]">{title}</h1>
            {description && (
              <p className="mt-2.5 text-[12.5px] leading-relaxed max-w-2xl" style={{ color: "var(--g-muted)" }}>
                {description}
              </p>
            )}
          </div>
          {actions && <div className="flex items-center gap-2 pb-1">{actions}</div>}
        </header>
        <div className="mt-7 g-rule" />
        <div className="mt-7">{children}</div>
      </div>
    </div>
  );
}

export function DataTable({ head, children }: { head: string[]; children: ReactNode }) {
  return (
    <div className="g-panel-flat overflow-hidden">
      <div className="g-scroll overflow-x-auto">
        <table className="w-full text-left border-collapse">
          <thead>
            <tr style={{ borderBottom: "1px solid var(--g-line)", background: "var(--g-panel-2)" }}>
              {head.map((h) => (
                <th key={h} className="g-eyebrow !text-[9.5px] px-4 py-3 font-medium whitespace-nowrap">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>{children}</tbody>
        </table>
      </div>
    </div>
  );
}

export function DataRow({ children }: { children: ReactNode }) {
  return (
    <tr className="transition-colors" style={{ borderBottom: "1px solid var(--g-line)" }} onMouseEnter={(e) => { e.currentTarget.style.background = "var(--g-panel-2)"; }} onMouseLeave={(e) => { e.currentTarget.style.background = ""; }}>
      {children}
    </tr>
  );
}

export function DataCell({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <td className={`px-4 py-3 text-[12px] align-middle ${className}`}>{children}</td>;
}

export function EmptyRow({ colSpan, children }: { colSpan: number; children: ReactNode }) {
  return (
    <tr>
      <td colSpan={colSpan} className="px-4 py-16 text-center">
        <div className="text-[12.5px]" style={{ color: "var(--g-faint)" }}>{children}</div>
      </td>
    </tr>
  );
}
