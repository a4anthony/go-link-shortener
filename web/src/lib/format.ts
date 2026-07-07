// Small formatting helpers shared across the console.

export function relativeTime(iso: string): string {
  const then = new Date(iso).getTime();
  const diff = Date.now() - then;
  const sec = Math.round(diff / 1000);
  if (sec < 60) return `${sec}s ago`;
  const min = Math.round(sec / 60);
  if (min < 60) return `${min}m ago`;
  const hr = Math.round(min / 60);
  if (hr < 24) return `${hr}h ago`;
  const day = Math.round(hr / 24);
  if (day < 30) return `${day}d ago`;
  return new Date(iso).toLocaleDateString();
}

export function formatNumber(n: number): string {
  return new Intl.NumberFormat().format(n);
}

/** statusClass maps an HTTP status to a semantic token used across the UI. */
export function statusClass(status: number): 'ok' | 'redirect' | 'error' {
  if (status >= 300 && status < 400) return 'redirect';
  if (status >= 400) return 'error';
  return 'ok';
}

/** hostOf returns just the host of a URL, for compact display. */
export function hostOf(url: string): string {
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
}

export function truncate(s: string, max = 48): string {
  return s.length > max ? s.slice(0, max - 1) + '…' : s;
}
