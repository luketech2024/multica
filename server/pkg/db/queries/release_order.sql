-- name: NextWorkbenchReleaseNumber :one
INSERT INTO workbench_release_counters (workspace_id,next_number) VALUES ($1,2)
ON CONFLICT (workspace_id) DO UPDATE SET next_number=workbench_release_counters.next_number+1
RETURNING (next_number-1)::bigint AS number;

-- name: CreateWorkbenchRelease :one
INSERT INTO workbench_release_orders (id,workspace_id,project_id,number,env,request_id,request_hash,manifest,created_by)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING *;

-- name: GetWorkbenchRelease :one
SELECT * FROM workbench_release_orders WHERE workspace_id=$1 AND id=$2;

-- name: ConfirmWorkbenchRelease :one
UPDATE workbench_release_orders SET confirmations=$3, revision=revision+1,updated_at=now()
WHERE workspace_id=$1 AND id=$2 AND revision=sqlc.arg(expected_revision)
  AND status IN ('draft','prechecking','ready') RETURNING *;

-- name: GetWorkbenchReleaseByRequest :one
SELECT * FROM workbench_release_orders WHERE workspace_id=$1 AND request_id=$2;

-- name: ListWorkbenchReleases :many
SELECT * FROM workbench_release_orders
WHERE workspace_id= @workspace_id AND id< @before_id
    AND (sqlc.narg('project_id')::uuid IS NULL OR project_id=sqlc.narg('project_id')::uuid)
    AND (@env_filter::text='' OR env= @env_filter)
ORDER BY id DESC LIMIT 100;

-- name: ReserveWorkbenchReleaseArtifact :exec
INSERT INTO workbench_release_reservations (workspace_id,release_id,repo_id,merge_commit,env)
VALUES ($1,$2,$3,$4,$5);

-- name: IssueHasWorkbenchRelease :one
SELECT EXISTS(SELECT 1 FROM workbench_release_orders
WHERE workspace_id= @workspace_id AND manifest->'tasks' @> jsonb_build_array(jsonb_build_object('id', @issue_id::text)));

-- name: ProjectHasWorkbenchReleases :one
SELECT EXISTS(SELECT 1 FROM workbench_release_orders WHERE workspace_id=$1 AND project_id=$2);

-- name: ListWorkbenchReleaseCandidates :many
SELECT * FROM issue i WHERE i.workspace_id= @workspace_id AND i.project_id= @project_id
    AND i.parent_issue_id IS NULL AND i.status='done' AND i.id< @before_id
    AND i.acceptance_criteria->'gate'->'accept' IS NOT NULL
ORDER BY i.id DESC LIMIT 100;

-- name: ListWorkbenchReleaseReservations :many
SELECT r.* FROM workbench_release_reservations r
JOIN workbench_release_orders o ON o.id=r.release_id AND o.workspace_id=r.workspace_id
WHERE r.workspace_id=$1 AND o.project_id=$2 AND r.env=$3;
