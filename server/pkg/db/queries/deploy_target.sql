-- name: ListDeployTargets :many
SELECT * FROM deploy_targets WHERE workspace_id=$1 AND project_id=$2 ORDER BY env, label, id;

-- name: GetDeployTarget :one
SELECT * FROM deploy_targets WHERE workspace_id=$1 AND project_id=$2 AND id=$3;

-- name: CreateDeployTarget :one
INSERT INTO deploy_targets (workspace_id,project_id,repo_id,label,env,kind,channel,runtime_id,credential_id,location)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING *;

-- name: UpdateDeployTarget :one
UPDATE deploy_targets SET repo_id=$4,label=$5,env=$6,kind=$7,channel=$8,runtime_id=$9,credential_id=$10,location=$11,
    last_check=NULL, revision=revision+1, updated_at=now()
WHERE workspace_id=$1 AND project_id=$2 AND id=$3 AND revision=sqlc.arg(expected_revision)
RETURNING *;

-- name: DeleteDeployTarget :execrows
DELETE FROM deploy_targets WHERE workspace_id=$1 AND project_id=$2 AND id=$3 AND revision=$4;

-- name: DeleteProjectDeployTargets :exec
DELETE FROM deploy_targets WHERE workspace_id=$1 AND project_id=$2;

-- name: CountDeployTargetsForResource :one
SELECT count(*) FROM deploy_targets WHERE workspace_id=$1 AND repo_id=$2;

-- name: StartDeployTargetCheck :one
UPDATE deploy_targets SET last_check= @check_result, revision=revision+1, updated_at=now()
WHERE workspace_id= @workspace_id AND project_id= @project_id AND id= @id AND revision= @expected_revision
RETURNING *;

-- name: ClaimRuntimeDeployTargetCheck :one
WITH candidate AS (
    SELECT target.id FROM deploy_targets target
    JOIN agent_runtime runtime ON runtime.id=target.runtime_id AND runtime.workspace_id=target.workspace_id
    JOIN member requester ON requester.workspace_id=target.workspace_id AND requester.user_id::text=target.last_check->>'requested_by'
    WHERE runtime.id= @runtime_id AND runtime.owner_id IS NOT NULL AND target.channel='runtime' AND requester.role IN ('owner','admin')
      AND (runtime.owner_id=requester.user_id OR runtime.visibility='public' OR requester.user_id=ANY(runtime.shared_with))
      AND (target.last_check->>'at')::timestamptz>now()-interval '2 minutes'
      AND (target.last_check->>'status'='queued' OR (target.last_check->>'status'='running'
           AND (target.last_check->>'dispatched_at')::timestamptz<now()-interval '1 minute'))
    ORDER BY target.updated_at,target.id LIMIT 1 FOR UPDATE OF target SKIP LOCKED
)
UPDATE deploy_targets SET last_check=last_check || jsonb_build_object('status','running','dispatched_at',now())
WHERE id IN (SELECT id FROM candidate) RETURNING *;

-- name: GetRuntimeDeployTargetCheck :one
SELECT * FROM deploy_targets WHERE workspace_id=$1 AND runtime_id=$2 AND id=$3 FOR UPDATE;

-- name: FinishRuntimeDeployTargetCheck :one
UPDATE deploy_targets SET last_check= @check_result, revision=revision+1, updated_at=now()
WHERE workspace_id= @workspace_id AND id= @id AND last_check->>'id'= @check_id::text
RETURNING *;
