-- Initial migration: enable extensions used across the schema.
-- pgcrypto provides gen_random_uuid() for primary keys.
CREATE EXTENSION IF NOT EXISTS pgcrypto;
