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
