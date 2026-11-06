CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_project_resource_identity
ON project_resource (project_id, resource_type,
    (resource_ref - ARRAY['stack', 'delivery', 'workbench_profile_revision']))
WHERE resource_type IN ('github_repo', 'local_directory');
