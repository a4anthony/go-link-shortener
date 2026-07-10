import { useState } from 'react';
import { PageHeader } from '../components/Layout';
import { Button, Field, Input, Panel, PanelHeader } from '../components/ui';
import { useToast } from '../components/Toast';
import { DEFAULT_API_KEY, api, getApiKey, getBaseUrl, setApiKey, setBaseUrl } from '../lib/api';

export function Settings() {
  const toast = useToast();
  const [key, setKey] = useState(getApiKey());
  const [base, setBase] = useState(getBaseUrl());
  const [testing, setTesting] = useState(false);

  function save() {
    setApiKey(key.trim());
    setBaseUrl(base.trim());
    toast.success('Settings saved');
  }

  async function testConnection() {
    setTesting(true);
    // Persist first so the test uses the entered values.
    setApiKey(key.trim());
    setBaseUrl(base.trim());
    try {
      const ov = await api.overview();
      toast.success(`Connected — ${ov.total_links} links, ${ov.total_clicks} clicks`);
    } catch {
      toast.error('Connection failed — check the base URL and API key');
    } finally {
      setTesting(false);
    }
  }

  return (
    <>
      <PageHeader title="Settings" subtitle="Point the console at an instance and authenticate." />

      <Panel className="max-w-2xl">
        <PanelHeader title="Connection" />
        <div className="flex flex-col gap-5 p-5">
          <Field
            label="API base URL"
            hint="Leave blank to use the same origin (the bundled dev/nginx proxy handles /api)."
          >
            <Input
              value={base}
              onChange={(e) => setBase(e.target.value)}
              placeholder="(same origin)"
              className="font-mono"
            />
          </Field>
          <Field label="API key" hint="Sent as a Bearer token on every request.">
            <Input
              value={key}
              onChange={(e) => setKey(e.target.value)}
              className="font-mono"
              type="password"
            />
          </Field>
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="primary" onClick={save}>
              Save
            </Button>
            <Button variant="subtle" onClick={testConnection} disabled={testing}>
              {testing ? 'Testing…' : 'Test connection'}
            </Button>
            <Button
              variant="ghost"
              onClick={() => {
                setKey(DEFAULT_API_KEY);
                setBase('');
              }}
            >
              Reset to demo defaults
            </Button>
          </div>
        </div>
      </Panel>

      <p className="mt-4 max-w-2xl text-xs text-muted">
        The console starts out using the shared demo tenant's key
        (<code className="font-mono text-faint">{DEFAULT_API_KEY}</code>), which works on any
        server with the demo playground enabled — including every dev server. Keys are stored
        only in this browser's local storage.
      </p>
    </>
  );
}
