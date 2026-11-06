-- name: GetProjectWorkbenchRoles :one
SELECT * FROM project_resource
WHERE workspace_id=$1 AND project_id=$2 AND resource_type='workbench_roles';

-- name: LockProjectWorkbenchRoles :one
SELECT id FROM project WHERE workspace_id=$1 AND id=$2 FOR UPDATE;

-- name: LockProjectNames :exec
SELECT pg_advisory_xact_lock(hashtextextended('project-names:' || sqlc.arg(workspace_id)::uuid::text, 0));

-- name: ProjectTitleExists :one
SELECT EXISTS(SELECT 1 FROM project
WHERE workspace_id=sqlc.arg(workspace_id)
  AND lower(btrim(title))=lower(btrim(sqlc.arg(title)::text))
  AND (sqlc.narg(exclude_id)::uuid IS NULL OR id<>sqlc.narg(exclude_id)::uuid));
