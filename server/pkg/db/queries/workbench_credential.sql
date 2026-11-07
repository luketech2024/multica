-- name: ListWorkbenchCredentials :many
SELECT id,workspace_id,project_id,label,kind,usage,last_four,expires_at,rotated_at,revision,created_at
FROM credentials WHERE workspace_id=$1 ORDER BY label,id;

-- name: GetWorkbenchCredentialForBinding :one
SELECT id,workspace_id,project_id,label,kind,usage,last_four,expires_at,rotated_at,revision,created_at
FROM credentials WHERE workspace_id=$1 AND id=$2 FOR SHARE;

-- name: GetWorkbenchCredentialForUpdate :one
SELECT id,workspace_id,project_id,label,kind,usage,last_four,expires_at,rotated_at,revision,created_at
FROM credentials WHERE workspace_id=$1 AND id=$2 FOR UPDATE;

-- name: CreateWorkbenchCredential :one
INSERT INTO credentials (id,workspace_id,project_id,label,kind,usage,secret_enc,wrapped_key,last_four,expires_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
RETURNING id,workspace_id,project_id,label,kind,usage,last_four,expires_at,rotated_at,revision,created_at;

-- name: RotateWorkbenchCredential :one
UPDATE credentials SET secret_enc=$3,wrapped_key=$4,last_four=$5,expires_at=$6,rotated_at=now(),revision=revision+1
WHERE workspace_id=$1 AND id=$2 AND revision=sqlc.arg(expected_revision)
RETURNING id,workspace_id,project_id,label,kind,usage,last_four,expires_at,rotated_at,revision,created_at;

-- name: DeleteWorkbenchCredential :execrows
DELETE FROM credentials WHERE credentials.workspace_id=$1 AND credentials.id=$2 AND credentials.revision=$3
AND NOT EXISTS (SELECT 1 FROM deploy_targets WHERE credential_id=credentials.id);

-- name: ListCredentialTargets :many
SELECT * FROM deploy_targets WHERE workspace_id=$1 AND credential_id=$2 ORDER BY project_id,label;

-- name: ListExpiringWorkbenchCredentials :many
SELECT id,workspace_id,project_id FROM credentials
WHERE expires_at > @now_at AND expires_at <= @until_at AND id > @after_id
ORDER BY id LIMIT 100;

-- name: HasCredentialExpiryNotice :one
SELECT EXISTS(SELECT 1 FROM inbox_item
WHERE workspace_id= @workspace_id AND recipient_id= @recipient_id
  AND details->>'source'='workbench_credential'
  AND details->>'credential_id'= @credential_id::text AND details->>'expires_at'= @expires_at::text);

-- name: ArchiveCredentialExpiryNotices :many
UPDATE inbox_item SET archived=true
WHERE workspace_id= @workspace_id AND details->>'source'='workbench_credential' AND details->>'credential_id'= @credential_id::text AND archived=false
RETURNING id,recipient_id;
