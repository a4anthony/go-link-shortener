import { useState, type FormEvent } from 'react';
import { PageHeader } from '../components/Layout';
import { CopyButton } from '../components/data';
import { Button, EmptyState, ErrorNote, Field, Input, Panel, PanelHeader, Spinner } from '../components/ui';
import { useToast } from '../components/Toast';
import { useAsync } from '../hooks/useAsync';
import { ApiError, api } from '../lib/api';
import { relativeTime } from '../lib/format';
import type { WebhookEvent } from '../lib/types';

const ALL_EVENTS: WebhookEvent[] = ['link.created', 'link.clicked'];

export function Webhooks() {
  const toast = useToast();
  const { data, error, loading, refetch } = useAsync(() => api.listWebhooks(), []);
  const hooks = data?.webhooks ?? [];

  const [url, setUrl] = useState('');
  const [events, setEvents] = useState<WebhookEvent[]>(['link.created']);
  const [submitting, setSubmitting] = useState(false);
  const [revealedSecret, setRevealedSecret] = useState<string | null>(null);

  function toggleEvent(e: WebhookEvent) {
    setEvents((cur) => (cur.includes(e) ? cur.filter((x) => x !== e) : [...cur, e]));
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (events.length === 0) {
      toast.error('Select at least one event');
      return;
    }
    setSubmitting(true);
    try {
      const wh = await api.createWebhook({ url, events });
      toast.success('Webhook created');
      setRevealedSecret(wh.secret ?? null);
      setUrl('');
      refetch();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Could not create webhook');
    } finally {
      setSubmitting(false);
    }
  }

  async function remove(id: string) {
    if (!confirm('Delete this webhook?')) return;
    try {
      await api.deleteWebhook(id);
      toast.success('Webhook deleted');
      refetch();
    } catch {
      toast.error('Could not delete webhook');
    }
  }

  return (
    <>
      <PageHeader title="Webhooks" subtitle="Get notified of link.created and link.clicked events." />

      {revealedSecret && (
        <Panel className="mb-6" >
          <div
            className="rounded-t-[var(--radius)] px-5 py-3 text-sm font-semibold"
            style={{ background: 'color-mix(in srgb, var(--color-accent) 14%, transparent)', color: 'var(--color-accent)' }}
          >
            Signing secret — shown once
          </div>
          <div className="flex flex-wrap items-center gap-3 p-5">
            <code className="min-w-0 flex-1 break-all rounded border border-border bg-bg px-3 py-2 font-mono text-sm text-text">
              {revealedSecret}
            </code>
            <CopyButton value={revealedSecret} label="Copy secret" />
            <Button variant="ghost" onClick={() => setRevealedSecret(null)}>
              I saved it
            </Button>
          </div>
          <p className="px-5 pb-5 text-xs text-muted">
            Use it to verify the <code className="font-mono">X-Webhook-Signature</code> header
            (HMAC-SHA256 of the raw body). It cannot be retrieved again.
          </p>
        </Panel>
      )}

      <Panel className="mb-6">
        <PanelHeader title="New webhook" />
        <form onSubmit={submit} className="grid grid-cols-1 gap-4 p-5">
          <Field label="Endpoint URL" hint="We POST signed JSON payloads here.">
            <Input
              type="url"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="https://hooks.example.com/ingest"
              required
            />
          </Field>
          <div>
            <span className="mb-2 block text-xs font-medium uppercase tracking-wide text-muted">Events</span>
            <div className="flex flex-wrap gap-2">
              {ALL_EVENTS.map((e) => {
                const on = events.includes(e);
                return (
                  <button
                    type="button"
                    key={e}
                    onClick={() => toggleEvent(e)}
                    className={`rounded-[var(--radius)] border px-3 py-1.5 font-mono text-xs transition-colors ${
                      on
                        ? 'border-accent text-accent'
                        : 'border-border text-muted hover:border-border-strong hover:text-text'
                    }`}
                    style={on ? { background: 'color-mix(in srgb, var(--color-accent) 10%, transparent)' } : undefined}
                    aria-pressed={on}
                  >
                    {on ? '✓ ' : ''}
                    {e}
                  </button>
                );
              })}
            </div>
          </div>
          <div>
            <Button variant="primary" type="submit" disabled={submitting}>
              {submitting ? 'Creating…' : 'Create webhook'}
            </Button>
          </div>
        </form>
      </Panel>

      {loading && <Spinner label="Loading webhooks…" />}
      {error && <ErrorNote message={error} />}

      {data && (
        <Panel>
          <PanelHeader title={<span>Webhooks <span className="text-faint">· {hooks.length}</span></span>} />
          {hooks.length === 0 ? (
            <EmptyState title="No webhooks">
              Register an endpoint above to receive signed event deliveries.
            </EmptyState>
          ) : (
            <div className="divide-y divide-border">
              {hooks.map((w) => (
                <div key={w.id} className="flex flex-wrap items-center gap-4 px-5 py-4">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span
                        className="h-2 w-2 rounded-full"
                        style={{ background: w.active ? 'var(--color-ok)' : 'var(--color-error)' }}
                        title={w.active ? 'active' : 'disabled (dead-lettered)'}
                        aria-hidden
                      />
                      <span className="min-w-0 truncate font-mono text-sm text-text" title={w.url}>
                        {w.url}
                      </span>
                    </div>
                    <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
                      {w.events.map((e) => (
                        <span
                          key={e}
                          className="rounded border border-border px-1.5 py-0.5 font-mono text-[11px] text-muted"
                        >
                          {e}
                        </span>
                      ))}
                      <span className="font-mono text-[11px] text-faint">· {relativeTime(w.created_at)}</span>
                    </div>
                  </div>
                  <Button variant="danger" className="text-xs" onClick={() => remove(w.id)}>
                    Delete
                  </Button>
                </div>
              ))}
            </div>
          )}
        </Panel>
      )}
    </>
  );
}
