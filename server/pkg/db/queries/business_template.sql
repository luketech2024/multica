-- name: ListWorkspaceBusinessTemplates :many
SELECT * FROM workbench_business_templates WHERE workspace_id=$1 ORDER BY content->>'name',id;

-- name: GetWorkspaceBusinessTemplateForUpdate :one
SELECT * FROM workbench_business_templates WHERE workspace_id=$1 AND id=$2 FOR UPDATE;

-- name: CreateWorkspaceBusinessTemplate :one
INSERT INTO workbench_business_templates (id,workspace_id,content) VALUES ($1,$2,$3) RETURNING *;

-- name: UpdateWorkspaceBusinessTemplate :one
UPDATE workbench_business_templates SET content=$3,revision=revision+1,updated_at=now()
WHERE workspace_id=$1 AND id=$2 AND revision=sqlc.arg(expected_revision) RETURNING *;
