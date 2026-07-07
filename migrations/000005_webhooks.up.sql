-- Tenant-registered webhook endpoints. A webhook is disabled (dead-lettered)
-- after too many consecutive delivery failures.

CREATE TABLE webhooks (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    url           TEXT        NOT NULL,
    -- Shared secret used to sign payloads (HMAC-SHA256). Never returned by the API.
    secret        TEXT        NOT NULL,
    -- Subscribed event types, e.g. {'link.created','link.clicked'}.
    events        TEXT[]      NOT NULL,
    active        BOOLEAN     NOT NULL DEFAULT true,
    failure_count INT         NOT NULL DEFAULT 0,
    -- Set when the webhook is dead-lettered after exhausting retries repeatedly.
    disabled_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX webhooks_tenant_idx ON webhooks (tenant_id);
-- Dispatch looks up active subscribers per tenant.
CREATE INDEX webhooks_active_idx ON webhooks (tenant_id, active) WHERE active;
