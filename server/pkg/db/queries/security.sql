-- name: GetSecurityScanByRun :one
SELECT * FROM security_scans WHERE workspace_id=$1 AND repo_id=$2 AND run_id=$3;

-- name: GetSecurityScan :one
SELECT * FROM security_scans WHERE workspace_id=$1 AND id=$2;

-- name: CreateSecurityScan :one
INSERT INTO security_scans (workspace_id,project_id,repo_id,issue_id,run_id,artifact_version,source_commit,status,scanned_at,actor_type,actor_id,report,digest)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING *;

-- name: UpsertSecurityCheck :exec
INSERT INTO security_checks (workspace_id,project_id,repo_id,issue_id,fingerprint,severity,category,title,cwe,location,recommendation,artifact_version,first_scan_id,last_scan_id,last_seen_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$13,$14)
ON CONFLICT (workspace_id,repo_id,fingerprint) DO UPDATE SET
    issue_id=EXCLUDED.issue_id,severity=EXCLUDED.severity,category=EXCLUDED.category,title=EXCLUDED.title,cwe=EXCLUDED.cwe,
    location=EXCLUDED.location,recommendation=EXCLUDED.recommendation,artifact_version=EXCLUDED.artifact_version,
    last_scan_id=EXCLUDED.last_scan_id,last_seen_at=EXCLUDED.last_seen_at,fixed_scan_id=NULL,fix_version='',fix_requested_at=NULL,
    status=CASE WHEN security_checks.status='accepted' AND EXCLUDED.severity='low' AND security_checks.accepted_until>now()
                     AND security_checks.artifact_version=EXCLUDED.artifact_version THEN 'accepted' ELSE 'unresolved' END,
    accepted_until=CASE WHEN security_checks.status='accepted' AND EXCLUDED.severity='low' AND security_checks.accepted_until>now()
                     AND security_checks.artifact_version=EXCLUDED.artifact_version THEN security_checks.accepted_until ELSE NULL END,
    acceptance_reason=CASE WHEN security_checks.status='accepted' AND EXCLUDED.severity='low' AND security_checks.accepted_until>now()
                     AND security_checks.artifact_version=EXCLUDED.artifact_version THEN security_checks.acceptance_reason ELSE '' END,
    revision=security_checks.revision+1,updated_at=now()
WHERE security_checks.last_seen_at <= EXCLUDED.last_seen_at;

-- name: GetSecurityCheck :one
SELECT * FROM security_checks WHERE workspace_id=$1 AND id=$2;

-- name: LockSecurityCheck :one
SELECT * FROM security_checks WHERE workspace_id=$1 AND id=$2 FOR UPDATE;

-- name: UpdateSecurityCheck :one
UPDATE security_checks SET status= @status,fix_version= @fix_version,fix_requested_at= @fix_requested_at,
    fixed_scan_id= @fixed_scan_id,accepted_until= @accepted_until,acceptance_reason= @acceptance_reason,
    revision=revision+1,updated_at=now()
WHERE workspace_id= @workspace_id AND id= @id AND revision= @expected_revision RETURNING *;

-- name: ListSecurityChecks :many
SELECT * FROM security_checks
WHERE workspace_id= @workspace_id AND id < @before_id
  AND (sqlc.narg('project_id')::uuid IS NULL OR project_id=sqlc.narg('project_id')::uuid)
  AND (@status_filter::text = '' OR (CASE WHEN status='accepted' AND accepted_until<=now() THEN 'unresolved' ELSE status END) = @status_filter)
ORDER BY id DESC LIMIT 100;

-- name: SecurityCheckCounts :many
SELECT CASE WHEN status='accepted' AND accepted_until<=now() THEN 'unresolved' ELSE status END::text AS effective_status,
       severity,count(*)::bigint AS total
FROM security_checks
WHERE workspace_id= @workspace_id AND (sqlc.narg('project_id')::uuid IS NULL OR project_id=sqlc.narg('project_id')::uuid)
GROUP BY effective_status,severity;

-- name: ListSecurityAcceptanceCandidates :many
SELECT * FROM security_checks
WHERE status='accepted' AND accepted_until<= @until_at AND id> @after_id
ORDER BY id LIMIT 100;

-- name: HasSecurityAcceptanceNotice :one
SELECT EXISTS(SELECT 1 FROM inbox_item
WHERE workspace_id= @workspace_id AND recipient_id= @recipient_id
  AND details->>'source'='workbench_security'
  AND details->>'check_id'= @check_id::text AND details->>'accepted_until'= @accepted_until::text);

-- name: LatestSecurityScanForArtifact :one
SELECT * FROM security_scans
WHERE workspace_id=$1 AND repo_id=$2 AND artifact_version=$3
ORDER BY scanned_at DESC,recorded_at DESC,id DESC LIMIT 1;

-- name: CountBlockingSecurityChecks :one
SELECT count(*) FROM security_checks
WHERE workspace_id=$1 AND repo_id=ANY(@repo_ids::uuid[])
  AND severity IN ('high','medium') AND status<>'fixed';

-- name: CountSecurityScansForRepository :one
SELECT count(*) FROM security_scans WHERE workspace_id=$1 AND repo_id=$2;

-- name: DeleteProjectSecurity :exec
WITH deleted_notices AS (
    DELETE FROM inbox_item WHERE inbox_item.workspace_id=$1 AND details->>'source'='workbench_security'
      AND details->>'check_id' IN (SELECT id::text FROM security_checks WHERE workspace_id=$1 AND project_id=$2)
), deleted_checks AS (
    DELETE FROM security_checks WHERE security_checks.workspace_id=$1 AND security_checks.project_id=$2
)
DELETE FROM security_scans WHERE security_scans.workspace_id=$1 AND security_scans.project_id=$2;
