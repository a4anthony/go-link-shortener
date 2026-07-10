import { useEffect, useState, type ReactNode } from 'react';
import { NavLink } from 'react-router-dom';
import { getBaseUrl } from '../lib/api';
import { REPO_URL } from '../lib/site';
import { DemoBanner } from './DemoBanner';

const nav = [
  { to: '/', label: 'Overview', end: true, glyph: '◇' },
  { to: '/links', label: 'Links', glyph: '⟿' },
  { to: '/webhooks', label: 'Webhooks', glyph: '⇄' },
  { to: '/settings', label: 'Settings', glyph: '⚙' },
];

function Brand() {
  return (
    <div className="leading-tight">
      <div className="font-display text-base font-bold text-text">Linkwire</div>
      <div className="font-mono text-[10px] uppercase tracking-widest text-faint">link console</div>
    </div>
  );
}

function ConnectionStatus() {
  const [online, setOnline] = useState<boolean | null>(null);
  useEffect(() => {
    let alive = true;
    const ping = () =>
      fetch(getBaseUrl() + '/healthz')
        .then((r) => alive && setOnline(r.ok))
        .catch(() => alive && setOnline(false));
    ping();
    const t = setInterval(ping, 10000);
    return () => {
      alive = false;
      clearInterval(t);
    };
  }, []);

  const color = online == null ? 'var(--color-faint)' : online ? 'var(--color-ok)' : 'var(--color-error)';
  const label = online == null ? 'connecting' : online ? 'api online' : 'api offline';
  return (
    <div className="flex items-center gap-2 font-mono text-xs text-muted">
      <span className="h-2 w-2 rounded-full" style={{ background: color }} aria-hidden />
      {label}
      <span className="text-faint">· {getBaseUrl() || 'same-origin'}</span>
    </div>
  );
}

function NavItems({ onNavigate }: { onNavigate?: () => void }) {
  return (
    <nav className="flex flex-col gap-1">
      {nav.map((item) => (
        <NavLink
          key={item.to}
          to={item.to}
          end={item.end}
          onClick={onNavigate}
          className={({ isActive }) =>
            `flex items-center gap-3 rounded-[var(--radius)] px-3 py-2 text-sm transition-colors ${
              isActive
                ? 'bg-raised text-text'
                : 'text-muted hover:bg-raised/50 hover:text-text'
            }`
          }
        >
          {({ isActive }) => (
            <>
              <span
                aria-hidden
                className="w-4 text-center font-mono"
                style={{ color: isActive ? 'var(--color-accent)' : 'inherit' }}
              >
                {item.glyph}
              </span>
              {item.label}
            </>
          )}
        </NavLink>
      ))}
    </nav>
  );
}

export function Layout({ children }: { children: ReactNode }) {
  return (
    <div className="min-h-screen md:grid md:grid-cols-[248px_1fr]">
      {/* Sidebar (md+) */}
      <aside className="sticky top-0 hidden h-screen flex-col border-r border-border bg-surface p-5 md:flex">
        <Brand />
        <div className="mt-8 flex-1">
          <div className="mb-2 px-3 font-mono text-[10px] uppercase tracking-widest text-faint">
            Console
          </div>
          <NavItems />
        </div>
        <a
          href={REPO_URL}
          target="_blank"
          rel="noreferrer"
          className="mb-3 flex items-center gap-2 font-mono text-xs text-muted transition-colors hover:text-text"
        >
          <span aria-hidden>{'</>'}</span>
          View source on GitHub
        </a>
        <ConnectionStatus />
      </aside>

      {/* Mobile top bar */}
      <header className="sticky top-0 z-40 flex items-center justify-between border-b border-border bg-surface px-4 py-3 md:hidden">
        <Brand />
        <ConnectionStatus />
      </header>
      <div className="border-b border-border bg-surface px-2 py-2 md:hidden">
        <NavItems />
      </div>

      <main className="console-grid min-h-screen">
        <DemoBanner />
        <div className="mx-auto max-w-6xl px-5 py-8 md:px-10">{children}</div>
      </main>
    </div>
  );
}

export function PageHeader({
  title,
  subtitle,
  action,
}: {
  title: string;
  subtitle?: string;
  action?: ReactNode;
}) {
  return (
    <div className="mb-7 flex flex-wrap items-end justify-between gap-4">
      <div>
        <h1 className="font-display text-2xl font-bold text-text">{title}</h1>
        {subtitle && <p className="mt-1 text-sm text-muted">{subtitle}</p>}
      </div>
      {action}
    </div>
  );
}
