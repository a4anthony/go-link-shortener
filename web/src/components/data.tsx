import { useState } from 'react';
import type { LabelCount } from '../lib/types';
import { hostOf, truncate } from '../lib/format';
import { useToast } from './Toast';

const statusColor = { ok: 'var(--color-ok)', redirect: 'var(--color-redirect)', error: 'var(--color-error)' };

/** CodeChip renders a short code as a first-class monospace object — the brand. */
export function CodeChip({ code }: { code: string }) {
  return (
    <span
      className="inline-flex items-center rounded border px-2 py-0.5 font-mono text-sm text-accent"
      style={{ borderColor: 'color-mix(in srgb, var(--color-accent) 35%, transparent)', background: 'color-mix(in srgb, var(--color-accent) 8%, transparent)' }}
    >
      {code}
    </span>
  );
}

/** StatusChip shows an HTTP status, colored by class (2xx/3xx/4xx). */
export function StatusChip({ status, label }: { status: number; label?: string }) {
  const kind = status >= 400 ? 'error' : status >= 300 ? 'redirect' : 'ok';
  return (
    <span
      className="inline-flex items-center gap-1.5 rounded border px-2 py-0.5 font-mono text-xs"
      style={{
        color: statusColor[kind],
        borderColor: `color-mix(in srgb, ${statusColor[kind]} 40%, transparent)`,
        background: `color-mix(in srgb, ${statusColor[kind]} 8%, transparent)`,
      }}
    >
      <span aria-hidden className="h-1.5 w-1.5 rounded-full" style={{ background: statusColor[kind] }} />
      {label ?? status}
    </span>
  );
}

/**
 * RedirectFlow is the signature element: the link rendered as a wire —
 * code ──▶ destination — with the redirect status colored by its HTTP class.
 */
export function RedirectFlow({ code, target, status }: { code: string; target: string; status: number }) {
  return (
    <div className="flex min-w-0 items-center gap-2.5 font-mono text-sm">
      <CodeChip code={code} />
      <span aria-hidden className="text-faint">
        ──▶
      </span>
      <a
        href={target}
        target="_blank"
        rel="noreferrer noopener"
        className="min-w-0 truncate text-muted hover:text-text hover:underline"
        title={target}
      >
        {truncate(hostOf(target) + new URL(target, 'http://x').pathname, 40)}
      </a>
      <StatusChip status={status} />
    </div>
  );
}

export function CopyButton({ value, label = 'Copy' }: { value: string; label?: string }) {
  const toast = useToast();
  const [copied, setCopied] = useState(false);
  return (
    <button
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(value);
          setCopied(true);
          toast.success('Copied to clipboard');
          setTimeout(() => setCopied(false), 1500);
        } catch {
          toast.error('Could not copy');
        }
      }}
      className="rounded border border-border px-2 py-1 font-mono text-xs text-muted transition-colors hover:border-border-strong hover:text-text"
      aria-label={label}
    >
      {copied ? '✓ copied' : label}
    </button>
  );
}

/** Bars renders a labelled breakdown (referrers, countries, devices) as
 *  max-normalized horizontal bars. */
export function Bars({ data, empty }: { data: LabelCount[] | null; empty: string }) {
  if (!data || data.length === 0) {
    return <p className="px-1 py-6 text-center text-sm text-faint">{empty}</p>;
  }
  const max = Math.max(...data.map((d) => d.count), 1);
  return (
    <ul className="flex flex-col gap-2.5">
      {data.map((d) => (
        <li key={d.label} className="grid grid-cols-[1fr_auto] items-center gap-3">
          <div className="min-w-0">
            <div className="mb-1 flex items-center justify-between gap-2">
              <span className="truncate font-mono text-xs text-muted" title={d.label}>
                {d.label}
              </span>
              <span className="font-mono text-xs text-text">{d.count}</span>
            </div>
            <div className="h-1.5 w-full overflow-hidden rounded-full bg-bg">
              <div
                className="h-full rounded-full"
                style={{
                  width: `${(d.count / max) * 100}%`,
                  background: 'linear-gradient(90deg, var(--color-accent-dim), var(--color-accent))',
                }}
              />
            </div>
          </div>
        </li>
      ))}
    </ul>
  );
}

/** Sparkline draws a tiny inline area chart from a series of counts. */
export function Sparkline({ points, width = 96, height = 28 }: { points: number[]; width?: number; height?: number }) {
  if (points.length === 0) return <span className="text-xs text-faint">—</span>;
  const max = Math.max(...points, 1);
  const step = points.length > 1 ? width / (points.length - 1) : width;
  const coords = points.map((p, i) => [i * step, height - (p / max) * (height - 4) - 2] as const);
  const line = coords.map(([x, y]) => `${x.toFixed(1)},${y.toFixed(1)}`).join(' ');
  const area = `0,${height} ${line} ${width},${height}`;
  return (
    <svg width={width} height={height} className="overflow-visible" aria-hidden>
      <polygon points={area} fill="color-mix(in srgb, var(--color-accent) 14%, transparent)" />
      <polyline points={line} fill="none" stroke="var(--color-accent)" strokeWidth="1.5" />
    </svg>
  );
}
