-- M4 — query execution history.
-- Records every Run attempt (success or failure) so the workspace's History
-- tab can show recent activity and let users re-run / inspect past queries.
-- Connection foreign key is SET NULL so deleting a connection doesn't lose
-- the user's historical record of what they ran against it.

CREATE TABLE IF NOT EXISTS query_history (
    id             TEXT PRIMARY KEY,
    user_id        TEXT NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
    connection_id  TEXT REFERENCES connections(id) ON DELETE SET NULL,
    keyspace       TEXT,
    cql            TEXT NOT NULL,
    statement_kind TEXT NOT NULL,
    success        INTEGER NOT NULL,                    -- 1/0
    error_code     TEXT,
    error_message  TEXT,
    row_count      INTEGER NOT NULL DEFAULT 0,
    duration_ms    INTEGER NOT NULL DEFAULT 0,
    executed_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_query_history_user_time ON query_history(user_id, executed_at DESC);
CREATE INDEX IF NOT EXISTS idx_query_history_conn      ON query_history(connection_id);
