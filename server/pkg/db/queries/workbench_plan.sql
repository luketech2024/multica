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

-- name: ListWorkbenchDispatchCandidates :many
SELECT child.* FROM issue child
JOIN issue parent ON parent.id = child.parent_issue_id AND parent.workspace_id = child.workspace_id
WHERE child.id > @after_id AND child.status = 'todo' AND child.assignee_type = 'agent'
  AND child.properties #>> '{workbench_handoff,v}' = '2'
  AND child.properties #>> '{workbench_handoff,superseded}' IS DISTINCT FROM 'true'
  AND parent.project_id = child.project_id
  AND parent.status IN ('todo', 'in_progress')
  AND parent.acceptance_criteria #>> '{gate,spec,passed_at}' IS NOT NULL
  AND COALESCE(parent.acceptance_criteria->'change', 'null'::jsonb) = 'null'::jsonb
  AND parent.acceptance_criteria->>'needs_resplit' IS DISTINCT FROM 'true'
  AND parent.acceptance_criteria #>> '{plan,version}' = child.properties #>> '{workbench_handoff,requirement_version}'
  AND (parent.acceptance_criteria #> '{plan,issues}') ? child.id::text
  AND NOT EXISTS (SELECT 1 FROM agent_task_queue run WHERE run.issue_id = child.id)
  AND NOT EXISTS (
      SELECT 1 FROM jsonb_array_elements_text(child.properties #> '{workbench_handoff,depends_on}') dependency(id)
      WHERE NOT EXISTS (SELECT 1 FROM issue prior
          WHERE prior.id::text = dependency.id AND prior.workspace_id = child.workspace_id
            AND prior.project_id = child.project_id AND prior.status = 'done'
            AND prior.properties #>> '{workbench_handoff,superseded}' IS DISTINCT FROM 'true'))
ORDER BY child.id LIMIT 100;

-- name: WorkbenchIssueHasRun :one
SELECT EXISTS (SELECT 1 FROM agent_task_queue WHERE issue_id = @issue_id);
