-- name: GetOneSpecWorkspace :one
SELECT * FROM onespec_workspace_config WHERE workspace_id = $1;

-- name: SaveOneSpecWorkspace :one
INSERT INTO onespec_workspace_config (workspace_id, default_runtime_id)
VALUES ($1, $2)
ON CONFLICT (workspace_id) DO UPDATE SET
default_runtime_id = EXCLUDED.default_runtime_id,
revision = onespec_workspace_config.revision + 1
RETURNING *;

-- name: GetOneSpecProject :one
SELECT * FROM onespec_project_config WHERE workspace_id = $1 AND project_id = $2;

-- name: SaveOneSpecProject :one
INSERT INTO onespec_project_config (workspace_id, project_id, enabled, runtime_id, overview_repository_id)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (workspace_id, project_id) DO UPDATE SET
enabled = EXCLUDED.enabled, runtime_id = EXCLUDED.runtime_id,
overview_repository_id = EXCLUDED.overview_repository_id,
revision = onespec_project_config.revision + 1
RETURNING *;

-- name: GetOneSpecFlow :one
SELECT * FROM onespec_flow WHERE workspace_id = $1 AND issue_id = $2;

-- name: LockOneSpecFlow :one
SELECT * FROM onespec_flow WHERE workspace_id = $1 AND issue_id = $2 FOR UPDATE;

-- name: CreateOneSpecFlow :one
INSERT INTO onespec_flow (workspace_id, issue_id, state) VALUES ($1, $2, $3)
ON CONFLICT (workspace_id, issue_id) DO NOTHING RETURNING *;

-- name: UpdateOneSpecFlow :one
UPDATE onespec_flow SET state = @state, revision = revision + 1, updated_at = now()
WHERE workspace_id = @workspace_id AND issue_id = @issue_id AND revision = @expected_revision
RETURNING *;

-- name: GetOneSpecAgent :one
SELECT * FROM agent WHERE workspace_id = $1 AND runtime_id = $2 AND system_key = $3
AND archived_at IS NULL ORDER BY created_at LIMIT 1;

-- name: DeleteOneSpecIssue :exec
DELETE FROM onespec_flow WHERE onespec_flow.workspace_id= @workspace_id AND onespec_flow.issue_id= @issue_id;

-- name: DeleteOneSpecProject :exec
WITH flows AS (
 DELETE FROM onespec_flow WHERE onespec_flow.workspace_id= @workspace_id AND onespec_flow.issue_id IN (SELECT issue.id FROM issue WHERE issue.workspace_id= @workspace_id AND issue.project_id= @project_id)
)
DELETE FROM onespec_project_config WHERE onespec_project_config.workspace_id= @workspace_id AND onespec_project_config.project_id= @project_id;

-- name: DeleteOneSpecWorkspace :exec
WITH flows AS (DELETE FROM onespec_flow WHERE onespec_flow.workspace_id=$1),
projects AS (DELETE FROM onespec_project_config WHERE onespec_project_config.workspace_id=$1)
DELETE FROM onespec_workspace_config WHERE onespec_workspace_config.workspace_id=$1;
