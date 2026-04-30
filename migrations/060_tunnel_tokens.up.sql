CREATE TABLE IF NOT EXISTS tunnel_tokens (
    id          TEXT PRIMARY KEY,
    token_hash  TEXT NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ,
    revoked     BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX IF NOT EXISTS idx_tunnel_tokens_hash ON tunnel_tokens(token_hash);
