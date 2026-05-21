-- M2 — saved Cassandra connection configs (per-user).
-- Secrets (auth password, TLS client key) are stored AES-GCM-encrypted with the
-- app master key. NULL means "no secret of that kind".

CREATE TABLE IF NOT EXISTS connections (
    id                 TEXT PRIMARY KEY,
    owner_id           TEXT NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
    name               TEXT NOT NULL,
    hosts              TEXT NOT NULL,                    -- JSON array of strings
    port               INTEGER NOT NULL DEFAULT 9042,
    datacenter         TEXT,
    default_keyspace   TEXT,
    auth_username      TEXT,
    auth_password_enc  BLOB,                              -- AES-GCM(nonce||ct||tag)
    tls_enabled        INTEGER NOT NULL DEFAULT 0,
    tls_skip_verify    INTEGER NOT NULL DEFAULT 0,
    tls_ca_cert        TEXT,                              -- PEM (not secret)
    tls_client_cert    TEXT,                              -- PEM (not secret)
    tls_client_key_enc BLOB,                              -- AES-GCM(PEM key)
    read_only          INTEGER NOT NULL DEFAULT 0,
    consistency        TEXT NOT NULL DEFAULT 'LOCAL_QUORUM',
    connect_timeout_ms INTEGER NOT NULL DEFAULT 10000,
    request_timeout_ms INTEGER NOT NULL DEFAULT 15000,
    created_at         TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at         TEXT NOT NULL DEFAULT (datetime('now')),
    last_used_at       TEXT
);
CREATE INDEX IF NOT EXISTS idx_connections_owner ON connections(owner_id, name);
