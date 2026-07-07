-- Tenants (organisations) and their hashed-at-rest API keys.

CREATE TABLE tenants (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE api_keys (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name         TEXT        NOT NULL,
    -- Cleartext display prefix (e.g. sk_live_ab12cd34) for listing/identifying.
    prefix       TEXT        NOT NULL,
    -- Hex SHA-256 of the full plaintext key; the secret itself is never stored.
    key_hash     TEXT        NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ
);

-- Authentication looks keys up by hash, so it must be unique and indexed.
CREATE UNIQUE INDEX api_keys_key_hash_idx ON api_keys (key_hash);
CREATE INDEX api_keys_tenant_id_idx ON api_keys (tenant_id);
