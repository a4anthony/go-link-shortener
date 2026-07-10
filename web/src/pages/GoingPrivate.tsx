import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { PageHeader } from '../components/Layout';
import { Panel, PanelHeader } from '../components/ui';
import { DEFAULT_API_KEY } from '../lib/api';
import { REPO_URL } from '../lib/site';

function Step({ n, title, children }: { n: number; title: string; children: ReactNode }) {
  return (
    <div className="flex gap-4">
      <span
        className="mt-0.5 grid h-7 w-7 shrink-0 place-items-center rounded-full font-mono text-sm font-bold text-[#1a1205]"
        style={{ background: 'var(--color-accent)' }}
        aria-hidden
      >
        {n}
      </span>
      <div className="min-w-0">
        <div className="font-medium text-text">{title}</div>
        <div className="mt-1 text-sm text-muted">{children}</div>
      </div>
    </div>
  );
}

function Code({ children }: { children: string }) {
  return (
    <pre className="mt-2 overflow-x-auto rounded-[var(--radius)] border border-border bg-bg p-3 font-mono text-xs leading-relaxed text-text">
      {children}
    </pre>
  );
}

export function GoingPrivate() {
  return (
    <>
      <PageHeader
        title="Going private"
        subtitle="How to stop sharing the public demo tenant and use a workspace only you can see."
      />

      <Panel className="max-w-2xl">
        <PanelHeader title="Why you're seeing shared links" />
        <div className="p-5 text-sm text-muted">
          This console ships as a keyless playground. With no key set, it authenticates with the
          seeded demo tenant's well-known key
          (<code className="font-mono text-faint">{DEFAULT_API_KEY}</code>), and{' '}
          <span className="text-text">every visitor uses that same tenant</span> — so everyone sees,
          creates, and deletes the same pool of links. Going private means authenticating as your
          own tenant instead. Your key lives only in this browser's local storage.
        </div>
      </Panel>

      <Panel className="mt-6 max-w-2xl">
        <PanelHeader title="If you already have an API key" />
        <div className="flex flex-col gap-5 p-5">
          <Step n={1} title="Open Settings">
            Go to{' '}
            <Link to="/settings" className="text-accent underline underline-offset-2">
              Settings
            </Link>{' '}
            and paste your key (starts with <code className="font-mono text-faint">sk_live_</code>)
            into the API key field.
          </Step>
          <Step n={2} title="Save and verify">
            Click <span className="text-text">Save</span>, then{' '}
            <span className="text-text">Test connection</span>. The shared-playground banner
            disappears once the console is on a non-demo key — you're now scoped to your own tenant.
          </Step>
        </div>
      </Panel>

      <Panel className="mt-6 max-w-2xl">
        <PanelHeader title="If you run the server (self-hosting)" />
        <div className="flex flex-col gap-5 p-5">
          <Step n={1} title="Get the code and deploy">
            The full source, Docker Compose stack, and deployment guide live on GitHub:{' '}
            <a
              href={REPO_URL}
              target="_blank"
              rel="noreferrer"
              className="break-all text-accent underline underline-offset-2"
            >
              {REPO_URL.replace('https://', '')}
            </a>
            . Clone it, copy <code className="font-mono text-faint">.env.prod.example</code> to{' '}
            <code className="font-mono text-faint">.env</code>, and run{' '}
            <code className="font-mono text-faint">make deploy-up</code> (or{' '}
            <code className="font-mono text-faint">bash scripts/deploy.sh</code> on a host like Ploi).
          </Step>
          <Step n={2} title="Turn off the demo playground">
            Set <code className="font-mono text-faint">SEED_DEMO_TENANT=false</code> in your{' '}
            <code className="font-mono text-faint">.env</code> and restart. The shared tenant is no
            longer seeded, and there is deliberately no public sign-up endpoint.
          </Step>
          <Step n={3} title="Provision your own tenant + key">
            Create a tenant and an API key with two inserts. The stored hash is plain SHA-256 hex of
            the full key, so keep the raw key somewhere safe — it's shown only here:
            <Code>{`INSERT INTO tenants (name) VALUES ('acme') RETURNING id;

INSERT INTO api_keys (tenant_id, name, prefix, key_hash)
VALUES ('<tenant-id>', 'default',
        left('sk_live_your-long-random-key', 16),
        encode(sha256('sk_live_your-long-random-key'), 'hex'));`}</Code>
          </Step>
          <Step n={4} title="Sign in with the raw key">
            Paste <code className="font-mono text-faint">sk_live_your-long-random-key</code> into{' '}
            <Link to="/settings" className="text-accent underline underline-offset-2">
              Settings
            </Link>
            . Only that tenant's links are visible from then on.
          </Step>
        </div>
      </Panel>
    </>
  );
}
