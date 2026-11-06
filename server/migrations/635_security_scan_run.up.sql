CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_security_scans_run ON security_scans (workspace_id, repo_id, run_id);
