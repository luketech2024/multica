-- name: GetProjectWorkbenchRoles :one
SELECT * FROM project_resource
WHERE workspace_id=$1 AND project_id=$2 AND resource_type='workbench_roles';

-- name: LockProjectWorkbenchRoles :one
SELECT id FROM project WHERE workspace_id=$1 AND id=$2 FOR UPDATE;
