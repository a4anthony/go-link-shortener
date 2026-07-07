import { useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { PageHeader } from '../components/Layout';
import { Stat } from '../components/Stat';
import { Bars, CodeChip, CopyButton, StatusChip } from '../components/data';
import { Button, ErrorNote, Panel, PanelHeader, Spinner } from '../components/ui';
import { useToast } from '../components/Toast';
import { useAsync } from '../hooks/useAsync';
import { api } from '../lib/api';
import { formatNumber, relativeTime } from '../lib/format';
import type { TimePoint } from '../lib/types';

type Bucket = 'hour' | 'day';

function seriesData(series: TimePoint[] | null, bucket: Bucket) {
  return (series ?? []).map((p) => {
    const d = new Date(p.time);
    const label =
      bucket === 'hour'
        ? `${d.getHours().toString().padStart(2, '0')}:00`
        : `${d.getMonth() + 1}/${d.getDate()}`;
    return { label, count: p.count };
  });
}

export function LinkDetail() {
  const { id = '' } = useParams();
  const navigate = useNavigate();
  const toast = useToast();
  const [bucket, setBucket] = useState<Bucket>('hour');

  const link = useAsync(() => api.getLink(id), [id]);
  const stats = useAsync(() => api.linkStats(id, bucket), [id, bucket]);

  async function remove() {
    if (!link.data) return;
    if (!confirm(`Delete link ${link.data.code}?`)) return;
    try {
      await api.deleteLink(id);
      toast.success(`Deleted ${link.data.code}`);
      navigate('/links');
    } catch {
      toast.error('Could not delete link');
    }
  }

  const chart = seriesData(stats.data?.series ?? null, bucket);

  return (
    <>
      <div className="mb-4">
        <Link to="/links" className="font-mono text-xs text-muted hover:text-text">
          ← all links
        </Link>
      </div>

      {link.loading && <Spinner label="Loading link…" />}
      {link.error && <ErrorNote message={link.error} />}

      {link.data && (
        <>
          <PageHeader
            title={link.data.code}
            action={
              <div className="flex items-center gap-2">
                <CopyButton value={link.data.short_url} label="Copy URL" />
                <Button variant="danger" onClick={remove}>
                  Delete
                </Button>
              </div>
            }
          />

          <Panel className="mb-6">
            <div className="flex flex-wrap items-center gap-3 p-5 font-mono text-sm">
              <CodeChip code={link.data.code} />
              <span aria-hidden className="text-faint">
                ──▶
              </span>
              <a
                href={link.data.target_url}
                target="_blank"
                rel="noreferrer noopener"
                className="min-w-0 break-all text-muted hover:text-text hover:underline"
              >
                {link.data.target_url}
              </a>
              <StatusChip status={link.data.redirect_type} />
            </div>
          </Panel>

          <div className="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-3">
            <Stat label="Total clicks" value={formatNumber(link.data.click_count)} accent />
            <Stat
              label="Redirect"
              value={<span className="text-2xl">{link.data.redirect_type}</span>}
              hint={link.data.redirect_type === 301 ? 'Permanent' : 'Temporary'}
            />
            <Stat
              label="Limit"
              value={
                <span className="text-2xl">
                  {link.data.max_clicks ? formatNumber(link.data.max_clicks) : '∞'}
                </span>
              }
              hint={link.data.expires_at ? `expires ${relativeTime(link.data.expires_at)}` : 'no expiry'}
            />
          </div>

          <Panel className="mb-6">
            <PanelHeader
              title="Clicks over time"
              action={
                <div className="flex items-center gap-1 rounded-[var(--radius)] border border-border p-0.5">
                  {(['hour', 'day'] as Bucket[]).map((b) => (
                    <button
                      key={b}
                      onClick={() => setBucket(b)}
                      className={`rounded px-2.5 py-1 font-mono text-xs transition-colors ${
                        bucket === b ? 'bg-raised text-accent' : 'text-muted hover:text-text'
                      }`}
                    >
                      {b}
                    </button>
                  ))}
                </div>
              }
            />
            <div className="p-5">
              {stats.loading && <Spinner label="Loading stats…" />}
              {stats.error && <ErrorNote message={stats.error} />}
              {stats.data && chart.length === 0 && (
                <p className="py-10 text-center text-sm text-faint">
                  No clicks in this window yet. Visit the short link to generate one.
                </p>
              )}
              {stats.data && chart.length > 0 && (
                <div className="h-64 w-full">
                  <ResponsiveContainer width="100%" height="100%">
                    <AreaChart data={chart} margin={{ top: 8, right: 8, left: -16, bottom: 0 }}>
                      <defs>
                        <linearGradient id="clk" x1="0" y1="0" x2="0" y2="1">
                          <stop offset="0%" stopColor="var(--color-accent)" stopOpacity={0.35} />
                          <stop offset="100%" stopColor="var(--color-accent)" stopOpacity={0} />
                        </linearGradient>
                      </defs>
                      <CartesianGrid stroke="var(--color-border)" vertical={false} />
                      <XAxis
                        dataKey="label"
                        stroke="var(--color-faint)"
                        tick={{ fontSize: 11, fontFamily: 'var(--font-mono)' }}
                        tickLine={false}
                      />
                      <YAxis
                        allowDecimals={false}
                        stroke="var(--color-faint)"
                        tick={{ fontSize: 11, fontFamily: 'var(--font-mono)' }}
                        tickLine={false}
                        width={40}
                      />
                      <Tooltip
                        cursor={{ stroke: 'var(--color-border-strong)' }}
                        contentStyle={{
                          background: 'var(--color-raised)',
                          border: '1px solid var(--color-border-strong)',
                          borderRadius: 8,
                          fontFamily: 'var(--font-mono)',
                          fontSize: 12,
                          color: 'var(--color-text)',
                        }}
                        labelStyle={{ color: 'var(--color-muted)' }}
                      />
                      <Area
                        type="monotone"
                        dataKey="count"
                        stroke="var(--color-accent)"
                        strokeWidth={2}
                        fill="url(#clk)"
                      />
                    </AreaChart>
                  </ResponsiveContainer>
                </div>
              )}
            </div>
          </Panel>

          {stats.data && (
            <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
              <Panel>
                <PanelHeader title="Top referrers" />
                <div className="p-5">
                  <Bars data={stats.data.top_referrers} empty="No referrers yet." />
                </div>
              </Panel>
              <Panel>
                <PanelHeader title="Countries" />
                <div className="p-5">
                  <Bars data={stats.data.top_countries} empty="No geo data (no-op resolver)." />
                </div>
              </Panel>
              <Panel>
                <PanelHeader title="Devices" />
                <div className="p-5">
                  <Bars data={stats.data.devices} empty="No device data yet." />
                </div>
              </Panel>
            </div>
          )}
        </>
      )}
    </>
  );
}
