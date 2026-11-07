-- name: ListWorkbenchAuditPage :many
SELECT log.*, COALESCE(person.name, agent.name, '')::text AS actor_name
FROM activity_log log
LEFT JOIN "user" person ON log.actor_type = 'member' AND person.id = log.actor_id
LEFT JOIN agent ON log.actor_type = 'agent' AND agent.id = log.actor_id AND agent.workspace_id = log.workspace_id
WHERE log.workspace_id = @workspace_id AND log.action = ANY(@actions::text[])
  AND log.created_at >= @since_at AND log.created_at < @until_at
  AND (log.created_at, log.id) < (@before_at::timestamptz, @before_id::uuid)
ORDER BY log.created_at DESC, log.id DESC
LIMIT 1000;

-- name: CreateWorkbenchAuditExport :one
INSERT INTO workbench_audit_export (id,workspace_id,requester_id,filename,content,scope,range_start,range_end,expires_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING *;

-- name: GetWorkbenchAuditExport :one
SELECT * FROM workbench_audit_export WHERE workspace_id=$1 AND requester_id=$2 AND id=$3;

-- name: PruneWorkbenchAuditExports :execrows
WITH expired AS (
    SELECT id FROM workbench_audit_export WHERE expires_at<=now() ORDER BY expires_at LIMIT 100
)
DELETE FROM workbench_audit_export WHERE id IN (SELECT id FROM expired);
