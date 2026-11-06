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
