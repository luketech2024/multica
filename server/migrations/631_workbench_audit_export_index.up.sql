CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_activity_log_workspace_keyset
    ON activity_log (workspace_id, created_at DESC, id DESC);
