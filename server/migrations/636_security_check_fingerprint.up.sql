CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_security_checks_fingerprint ON security_checks (workspace_id, repo_id, fingerprint);
