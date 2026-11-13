-- name: CreateTaskUserInput :one
INSERT INTO task_user_input(task_id, workspace_id, issue_id, request_key, request)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (task_id, request_key) DO UPDATE SET request_key = EXCLUDED.request_key
WHERE task_user_input.request = EXCLUDED.request
RETURNING *;

-- name: GetTaskUserInput :one
SELECT * FROM task_user_input WHERE id = $1 AND task_id = $2;

-- name: GetIssueUserInput :one
SELECT * FROM task_user_input WHERE id = $1 AND workspace_id = $2 AND issue_id = $3;

-- name: ListTaskUserInputs :many
SELECT i.* FROM task_user_input i JOIN agent_task_queue t ON t.id = i.task_id
WHERE i.workspace_id = $1 AND i.issue_id = $2
ORDER BY i.created_at, i.id;

-- name: AnswerTaskUserInput :one
UPDATE task_user_input SET response = $2, answered_by = $3, status = 'answered', updated_at = now()
WHERE id = $1 AND status = 'pending' RETURNING *;

-- name: ResolveTaskUserInput :exec
UPDATE task_user_input SET status = sqlc.arg(status), updated_at = now()
WHERE id = $1 AND task_id = $2 AND status IN ('pending', 'answered');

-- name: SettleTaskUserInputs :exec
UPDATE task_user_input SET status = 'cancelled', updated_at = now()
WHERE task_id = ANY($1::uuid[]) AND status IN ('pending', 'answered')
AND NOT (COALESCE(request->>'origin', '') = 'platform' AND EXISTS (
    SELECT 1 FROM agent_task_queue t WHERE t.id=task_user_input.task_id AND t.status='completed'
));
