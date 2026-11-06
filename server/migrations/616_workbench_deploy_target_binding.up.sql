CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_deploy_targets_binding
ON deploy_targets (workspace_id, project_id, repo_id, env, label);
