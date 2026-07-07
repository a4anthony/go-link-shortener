import type { ReactNode } from 'react';

/** Stat is a compact metric tile: a big mono value with a small label. */
export function Stat({
  label,
  value,
  hint,
  accent,
}: {
  label: string;
  value: ReactNode;
  hint?: ReactNode;
  accent?: boolean;
}) {
  return (
    <div className="rounded-[var(--radius)] border border-border bg-surface p-5">
      <div className="font-mono text-[11px] uppercase tracking-widest text-muted">{label}</div>
      <div
        className="mt-2 font-mono text-3xl font-semibold tabular-nums"
        style={{ color: accent ? 'var(--color-accent)' : 'var(--color-text)' }}
      >
        {value}
      </div>
      {hint && <div className="mt-1 text-xs text-faint">{hint}</div>}
    </div>
  );
}
