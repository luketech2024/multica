CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS project_deployment_identity ON project_deployment(workspace_id, project_id);
