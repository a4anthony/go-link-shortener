-- Click events, one row per served redirect. High-volume, insert-only table
-- written in batches by the analytics pipeline. tenant_id is denormalised so
-- tenant-wide aggregate queries never need to join links.

CREATE TABLE clicks (
    id          BIGSERIAL PRIMARY KEY,
    link_id     UUID        NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    tenant_id   UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    occurred_at TIMESTAMPTZ NOT NULL,
    referrer    TEXT        NOT NULL DEFAULT '',
    user_agent  TEXT        NOT NULL DEFAULT '',
    browser     TEXT        NOT NULL DEFAULT '',
    os          TEXT        NOT NULL DEFAULT '',
    device      TEXT        NOT NULL DEFAULT '',
    country     TEXT        NOT NULL DEFAULT '',
    -- Salted hash of the truncated client IP; the raw IP is never stored.
    ip_hash     TEXT        NOT NULL DEFAULT ''
);

-- Per-link time-range scans (stats endpoints) and tenant-wide rollups.
CREATE INDEX clicks_link_time_idx ON clicks (link_id, occurred_at DESC);
CREATE INDEX clicks_tenant_time_idx ON clicks (tenant_id, occurred_at DESC);
