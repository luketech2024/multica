-- name: ListWorkbenchPlanChildren :many
SELECT * FROM issue
WHERE parent_issue_id = @parent_id AND workspace_id = @workspace_id
ORDER BY id FOR UPDATE;

-- name: SupersedeWorkbenchPlanChild :one
UPDATE issue SET
    properties = jsonb_set(properties, '{workbench_handoff}',
        COALESCE(properties->'workbench_handoff', '{}'::jsonb) || '{"superseded":true}'::jsonb),
    status = CASE WHEN status IN ('done', 'cancelled') THEN status ELSE 'cancelled' END,
    updated_at = now(), revision = revision + 1
WHERE id = @id AND workspace_id = @workspace_id
RETURNING *;

-- name: WorkbenchPlanHasActiveRuns :one
WITH RECURSIVE family AS (
    SELECT root.id FROM issue root WHERE root.id = @parent_id AND root.workspace_id = @workspace_id
    UNION
    SELECT child.id FROM issue child JOIN family ON child.parent_issue_id = family.id
    WHERE child.workspace_id = @workspace_id
)
SELECT EXISTS (SELECT 1 FROM agent_task_queue
    WHERE issue_id IN (SELECT id FROM family)
    AND status IN ('queued','deferred','dispatched','running','waiting_local_directory'));
