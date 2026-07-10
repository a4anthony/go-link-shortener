// Small formatting helpers shared across the console.

export function relativeTime(iso: string): string {
  const then = new Date(iso).getTime();
  const diff = Date.now() - then;
  const future = diff < 0;
  // Suffix "ago" for past times, prefix "in" for future ones.
  const frame = (unit: string) => (future ? `in ${unit}` : `${unit} ago`);
  const abs = Math.abs(diff);
  const sec = Math.round(abs / 1000);
  if (sec < 60) return frame(`${sec}s`);
  const min = Math.round(sec / 60);
  if (min < 60) return frame(`${min}m`);
  const hr = Math.round(min / 60);
  if (hr < 24) return frame(`${hr}h`);
  const day = Math.round(hr / 24);
  if (day < 30) return frame(`${day}d`);
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
