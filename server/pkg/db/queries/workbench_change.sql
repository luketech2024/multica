-- name: CancelWorkbenchChangeRuns :many
WITH RECURSIVE family AS (
    SELECT root.id FROM issue root WHERE root.id = @issue_id AND root.workspace_id = @workspace_id
    UNION
    SELECT child.id FROM issue child JOIN family parent ON child.parent_issue_id = parent.id
    WHERE child.workspace_id = @workspace_id
)
UPDATE agent_task_queue task
SET result = COALESCE(task.result, '{}'::jsonb) || jsonb_build_object(
        'workbench_stop_state', CASE WHEN task.status IN ('queued', 'deferred') THEN 'stopped' ELSE 'stopping' END),
    status = 'cancelled', completed_at = now(), prepare_lease_expires_at = NULL,
    cancelled_by_type = 'member', cancelled_by_id = @actor_id,
    cancelled_by_name = NULL
WHERE task.issue_id IN (SELECT id FROM family)
    AND task.status IN ('queued', 'deferred', 'dispatched', 'running', 'waiting_local_directory')
RETURNING task.*;

-- name: WorkbenchRunsStopped :one
SELECT NOT EXISTS (
    SELECT 1 FROM unnest(@run_ids::uuid[]) requested(id)
    LEFT JOIN agent_task_queue task ON task.id = requested.id
    LEFT JOIN issue owner ON owner.id = task.issue_id AND owner.workspace_id = @workspace_id
    WHERE owner.id IS NULL OR task.result->>'workbench_stop_state' IS DISTINCT FROM 'stopped'
);

-- name: AcknowledgeWorkbenchStop :exec
UPDATE agent_task_queue
SET result = COALESCE(result, '{}'::jsonb) || jsonb_build_object(
    'workbench_stop_state', CASE WHEN @confirmed::bool THEN 'stopped' ELSE 'unknown' END)
WHERE id = @id AND status IN ('cancelled', 'failed') AND result ? 'workbench_stop_state';

-- name: WorkbenchRunsUnconfirmed :one
SELECT EXISTS (
    SELECT 1 FROM agent_task_queue task
    JOIN issue owner ON owner.id = task.issue_id AND owner.workspace_id = @workspace_id
    LEFT JOIN agent_runtime runtime ON runtime.id = task.runtime_id
    WHERE task.id = ANY(@run_ids::uuid[])
      AND task.result->>'workbench_stop_state' IS DISTINCT FROM 'stopped'
      AND (task.result->>'workbench_stop_state' = 'unknown' OR task.failure_reason = 'cleanup_unconfirmed'
        OR runtime.status = 'offline' OR task.completed_at < now() - interval '30 seconds')
);

-- name: LockWorkbenchResume :exec
SELECT pg_advisory_xact_lock(hashtextextended('workbench-resume:' || @issue_id::text, 0));

-- name: FindWorkbenchResumedRun :one
SELECT resumed.id FROM agent_task_queue resumed
JOIN agent_task_queue source ON source.id = resumed.rerun_of_task_id
WHERE source.id = @source_id AND resumed.created_at >= source.completed_at
ORDER BY resumed.created_at DESC, resumed.id DESC LIMIT 1;
