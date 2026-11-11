-- name: GetProjectDeployment :one
SELECT * FROM project_deployment WHERE workspace_id=$1 AND project_id=$2;

-- name: ListApplicationDeploymentResults :many
SELECT DISTINCT ON (change->>'application_id', release.env, result->>'node_id')
  COALESCE(change->>'application_id', '')::text AS application_id,
  release.env, release.id AS release_id,
  COALESCE(result->>'node_id', '')::text AS node_id,
  COALESCE(NULLIF(change->>'resolved_version', ''), change->>'version', '')::text AS version,
  COALESCE(result->>'status', '')::text AS status
FROM workbench_release_orders release
CROSS JOIN LATERAL jsonb_array_elements(COALESCE(release.manifest->'delivery'->'changes', '[]'::jsonb)) change
CROSS JOIN LATERAL jsonb_array_elements(COALESCE(release.manifest->'delivery'->'results', '[]'::jsonb)) result
WHERE release.workspace_id=$1 AND release.project_id=$2
  AND change->>'type'='app' AND result->>'id'=change->>'id'
ORDER BY change->>'application_id', release.env, result->>'node_id', release.created_at DESC, release.id DESC;

-- name: SaveProjectDeployment :one
INSERT INTO project_deployment(workspace_id,project_id,config)
SELECT $1,$2,$3 WHERE sqlc.arg(expected_revision)::bigint=0
ON CONFLICT (workspace_id,project_id) DO NOTHING RETURNING *;

-- name: UpdateProjectDeployment :one
UPDATE project_deployment SET config=$3,revision=revision+1,updated_at=now()
WHERE workspace_id=$1 AND project_id=$2 AND revision=sqlc.arg(expected_revision) RETURNING *;

-- name: ListDeploymentCertificates :many
SELECT id,name,domains,not_before,not_after,fingerprint,created_at FROM deployment_certificate
WHERE workspace_id=$1 AND project_id=$2 ORDER BY created_at DESC;

-- name: GetDeploymentCertificate :one
SELECT * FROM deployment_certificate WHERE workspace_id=$1 AND project_id=$2 AND id=$3;

-- name: CreateDeploymentCertificate :one
INSERT INTO deployment_certificate(id,workspace_id,project_id,name,domains,not_before,not_after,fingerprint,secret_enc,wrapped_key)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
RETURNING id,name,domains,not_before,not_after,fingerprint,created_at;

-- name: UpdateReleaseManifest :one
UPDATE workbench_release_orders SET manifest=$3,confirmations='{}',revision=revision+1,updated_at=now()
WHERE workspace_id=$1 AND id=$2 AND revision=sqlc.arg(expected_revision) AND status='draft' RETURNING *;

-- name: UpdateReleaseExecution :one
UPDATE workbench_release_orders SET manifest=$3,status=$4,revision=revision+1,updated_at=now()
WHERE workspace_id=$1 AND id=$2 AND revision=sqlc.arg(expected_revision) RETURNING *;

-- name: StartApplicationRelease :one
UPDATE workbench_release_orders SET manifest=$3,status='deploying',lease_until=now()+interval '30 minutes',revision=revision+1,updated_at=now()
WHERE workspace_id=$1 AND id=$2 AND revision=sqlc.arg(expected_revision)
 AND (status IN ('draft','failed','partial') OR (status='deploying' AND lease_until<now())) RETURNING *;

-- name: ProjectReleaseExecuting :one
SELECT EXISTS(SELECT 1 FROM workbench_release_orders WHERE workspace_id=$1 AND project_id=$2 AND env=$3 AND status='deploying' AND lease_until>now());

-- name: DeleteProjectDeployment :exec
DELETE FROM project_deployment WHERE workspace_id=$1 AND project_id=$2;

-- name: DeleteProjectCertificates :exec
DELETE FROM deployment_certificate WHERE workspace_id=$1 AND project_id=$2;
