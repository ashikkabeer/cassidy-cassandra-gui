-- Cassidy — initial schema (M1: auth & users).
-- Subsequent migrations add the connections / query_history tables in M2 / M4.

CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    applied_at TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS app_users (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    email         TEXT,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'viewer',     -- 'admin' | 'editor' | 'viewer'
    is_active     INTEGER NOT NULL DEFAULT 1,
    must_reset_pw INTEGER NOT NULL DEFAULT 0,
    created_by    TEXT REFERENCES app_users(id) ON DELETE SET NULL,
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_app_users_active ON app_users(is_active);

CREATE TABLE IF NOT EXISTS app_sessions (
    id           TEXT PRIMARY KEY,                   -- opaque random token (cookie value)
    user_id      TEXT NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    last_seen_at TEXT NOT NULL DEFAULT (datetime('now')),
    expires_at   TEXT NOT NULL,
    ip_address   TEXT,
    user_agent   TEXT
);
CREATE INDEX IF NOT EXISTS idx_app_sessions_user   ON app_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_app_sessions_expiry ON app_sessions(expires_at);

CREATE TABLE IF NOT EXISTS app_settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
