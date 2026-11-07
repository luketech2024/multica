CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_workbench_audit_export_expiry ON workbench_audit_export(expires_at);
