CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_issue_workspace_completion
ON issue (workspace_id, completed_at) WHERE completed_at IS NOT NULL;
