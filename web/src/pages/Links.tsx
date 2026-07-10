import { useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { PageHeader } from '../components/Layout';
import { CopyButton, RedirectFlow } from '../components/data';
import { Button, EmptyState, ErrorNote, Field, Input, Panel, PanelHeader, Select, Spinner } from '../components/ui';
import { useToast } from '../components/Toast';
import { useConfirm } from '../components/Confirm';
import { useAsync } from '../hooks/useAsync';
import { ApiError, api } from '../lib/api';
import { formatNumber, relativeTime } from '../lib/format';
import type { CreateLinkInput } from '../lib/types';

function CreateLinkForm({ onCreated }: { onCreated: () => void }) {
  const toast = useToast();
  const [url, setUrl] = useState('');
  const [alias, setAlias] = useState('');
  const [redirectType, setRedirectType] = useState('302');
  const [expiresAt, setExpiresAt] = useState('');
  const [maxClicks, setMaxClicks] = useState('');
  const [submitting, setSubmitting] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    const input: CreateLinkInput = { url, redirect_type: Number(redirectType) };
    if (alias.trim()) input.custom_alias = alias.trim();
    if (expiresAt) input.expires_at = new Date(expiresAt).toISOString();
    if (maxClicks) input.max_clicks = Number(maxClicks);

    try {
      const link = await api.createLink(input);
      toast.success(`Created ${link.code}`);
      setUrl('');
      setAlias('');
      setExpiresAt('');
      setMaxClicks('');
      onCreated();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Could not create link');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Panel className="mb-6">
      <PanelHeader title="New link" />
      <form onSubmit={submit} className="grid grid-cols-1 gap-4 p-5 md:grid-cols-2">
        <div className="md:col-span-2">
          <Field label="Destination URL" hint="Where the short link sends visitors.">
            <Input
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="https://example.com/page"
              type="url"
              required
            />
          </Field>
        </div>
        <Field label="Custom alias" hint="Optional. 3–64 characters, else one is generated.">
          <Input
            value={alias}
            onChange={(e) => setAlias(e.target.value)}
            placeholder="promo-2026"
            className="font-mono"
          />
        </Field>
        <Field label="Redirect type">
          <Select value={redirectType} onChange={(e) => setRedirectType(e.target.value)}>
            <option value="302">302 — Found (temporary, keeps analytics)</option>
            <option value="301">301 — Moved Permanently (cacheable)</option>
          </Select>
        </Field>
        <Field label="Expires at" hint="Optional. After this, the link returns 410 Gone.">
          <Input type="datetime-local" value={expiresAt} onChange={(e) => setExpiresAt(e.target.value)} />
        </Field>
        <Field label="Max clicks" hint="Optional. After the limit, the link returns 410 Gone.">
          <Input
            type="number"
            min={1}
            value={maxClicks}
            onChange={(e) => setMaxClicks(e.target.value)}
            placeholder="unlimited"
          />
        </Field>
        <div className="md:col-span-2">
          <Button variant="primary" type="submit" disabled={submitting}>
            {submitting ? 'Creating…' : 'Create link'}
          </Button>
        </div>
      </form>
    </Panel>
  );
}

export function Links() {
  const toast = useToast();
  const confirm = useConfirm();
  const [showForm, setShowForm] = useState(false);
  const { data, error, loading, refetch } = useAsync(() => api.listLinks(), []);
  const links = data?.links ?? [];

  async function remove(id: string, code: string) {
    const ok = await confirm({
      title: `Delete link ${code}?`,
      message: 'This frees the code for reuse and cannot be undone.',
      confirmLabel: 'Delete',
    });
    if (!ok) return;
    try {
      await api.deleteLink(id);
      toast.success(`Deleted ${code}`);
      refetch();
    } catch {
      toast.error('Could not delete link');
    }
  }

  return (
    <>
      <PageHeader
        title="Links"
        subtitle="Create, inspect, and retire short links."
        action={
          <Button variant={showForm ? 'ghost' : 'primary'} onClick={() => setShowForm((s) => !s)}>
            {showForm ? 'Close' : 'New link'}
          </Button>
        }
      />

      {showForm && <CreateLinkForm onCreated={() => { refetch(); }} />}

      {loading && <Spinner label="Loading links…" />}
      {error && <ErrorNote message={error} />}

      {data && (
        <Panel>
          <PanelHeader title={<span>Links <span className="text-faint">· {links.length}</span></span>} />
          {links.length === 0 ? (
            <EmptyState
              title="No links yet"
              action={
                <Button variant="primary" onClick={() => setShowForm(true)}>
                  Create your first link
                </Button>
              }
            >
              Short links you create will appear here with live click counts.
            </EmptyState>
          ) : (
            <div className="divide-y divide-border">
              {links.map((l) => (
                <div key={l.id} className="flex flex-wrap items-center gap-4 px-5 py-4">
                  <div className="min-w-0 flex-1">
                    <RedirectFlow code={l.code} target={l.target_url} status={l.redirect_type} />
                    <div className="mt-1.5 font-mono text-xs text-faint">
                      {formatNumber(l.click_count)} clicks
                      {l.max_clicks ? ` / ${formatNumber(l.max_clicks)} max` : ''} · created{' '}
                      {relativeTime(l.created_at)}
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <CopyButton value={l.short_url} label="Copy URL" />
                    <Link to={`/links/${l.id}`}>
                      <Button variant="subtle" className="text-xs">
                        Stats
                      </Button>
                    </Link>
                    <Button variant="danger" className="text-xs" onClick={() => remove(l.id, l.code)}>
                      Delete
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </Panel>
      )}
    </>
  );
}
