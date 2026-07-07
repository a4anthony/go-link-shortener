-- Short links owned by a tenant. Codes are globally unique among live links so
-- the public GET /:code redirect can resolve without a tenant context.

CREATE TABLE links (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    code          TEXT        NOT NULL,
    target_url    TEXT        NOT NULL,
    -- HTTP redirect status to emit: 301 (permanent) or 302 (found).
    redirect_type SMALLINT    NOT NULL DEFAULT 302,
    expires_at    TIMESTAMPTZ,
    -- NULL means unlimited clicks.
    max_clicks    BIGINT,
    click_count   BIGINT      NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Soft delete: a non-NULL value hides the link from all reads and frees the code.
    deleted_at    TIMESTAMPTZ
);

-- A code is unique only among live (non-deleted) links, so deleting a link frees
-- its code for reuse.
CREATE UNIQUE INDEX links_code_live_idx ON links (code) WHERE deleted_at IS NULL;

-- Tenant-scoped listing of live links, newest first.
CREATE INDEX links_tenant_live_idx ON links (tenant_id, created_at DESC) WHERE deleted_at IS NULL;
