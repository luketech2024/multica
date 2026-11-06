-- Enforce confirmation gates at the storage boundary so batch updates and
-- agent/system writers obey the same prerequisites as the HTTP editor.
CREATE OR REPLACE FUNCTION guard_workbench_issue_execution()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    criteria jsonb;
    entry jsonb;
    covered_id text;
    previous_status text;
    target_status text;
    target_category text;
BEGIN
    criteria := NEW.acceptance_criteria;
    IF jsonb_typeof(criteria) IS DISTINCT FROM 'object' OR criteria->>'v' IS DISTINCT FROM '1' THEN
        RETURN NEW;
    END IF;
    target_status := issue_effective_status(NEW.workspace_id, NEW.status);
    SELECT category INTO target_category FROM issue_status
        WHERE workspace_id = NEW.workspace_id AND key = NEW.status;
    IF TG_OP = 'INSERT' THEN
        IF target_status NOT IN ('backlog', 'todo') AND target_category IS DISTINCT FROM 'unstarted' THEN
            RAISE EXCEPTION 'New requirements must start before execution'
                USING ERRCODE = '23514', CONSTRAINT = 'workbench_execution_gate';
        END IF;
        RETURN NEW;
    END IF;
    IF NEW.status IS NOT DISTINCT FROM OLD.status
       AND NEW.acceptance_criteria IS NOT DISTINCT FROM OLD.acceptance_criteria THEN RETURN NEW; END IF;
    previous_status := issue_effective_status(NEW.workspace_id, OLD.status);
    IF target_status = 'paused' AND criteria->'change' IS NULL THEN
        RAISE EXCEPTION 'Use the requirements change action to pause a workbench task'
            USING ERRCODE = '23514', CONSTRAINT = 'workbench_execution_gate';
    END IF;
    IF previous_status = 'paused' AND target_status NOT IN ('paused', 'cancelled')
       AND criteria->'change' IS NOT NULL THEN
        RAISE EXCEPTION 'Reconcile and confirm or cancel the requirements change before resuming'
            USING ERRCODE = '23514', CONSTRAINT = 'workbench_execution_gate';
    END IF;
    IF target_status = 'in_progress'
       OR (target_category = 'started' AND target_status NOT IN ('in_review', 'blocked', 'paused')) THEN
        IF criteria #>> '{gate,spec,passed_at}' IS NULL
           OR criteria #>> '{gate,spec,passed_by}' IS NULL
           OR NEW.assignee_id IS NULL OR NEW.assignee_type IS NULL
           OR jsonb_typeof(criteria->'items') IS DISTINCT FROM 'array'
           OR jsonb_array_length(criteria->'items') = 0 THEN
            RAISE EXCEPTION 'Confirm requirements and assign an executor before starting'
                USING ERRCODE = '23514', CONSTRAINT = 'workbench_execution_gate';
        END IF;
        FOR entry IN SELECT value FROM jsonb_array_elements(criteria->'items') LOOP
            IF entry->'checked' IS NULL OR entry->'checked' = 'null'::jsonb
               OR jsonb_typeof(entry->'covers') IS DISTINCT FROM 'array'
               OR jsonb_array_length(entry->'covers') = 0 THEN
                RAISE EXCEPTION 'Every requirement needs a confirmed check and subtask coverage'
                    USING ERRCODE = '23514', CONSTRAINT = 'workbench_execution_gate';
            END IF;
            FOR covered_id IN SELECT jsonb_array_elements_text(entry->'covers') LOOP
                IF NOT EXISTS (SELECT 1 FROM issue child
                    WHERE child.id::text = covered_id AND child.parent_issue_id = NEW.id
                      AND child.workspace_id = NEW.workspace_id
                      AND child.project_id IS NOT DISTINCT FROM NEW.project_id) THEN
                    RAISE EXCEPTION 'Requirement coverage must reference a subtask in the same project'
                        USING ERRCODE = '23514', CONSTRAINT = 'workbench_execution_gate';
                END IF;
            END LOOP;
        END LOOP;
    END IF;
    IF target_status = 'done' AND (
        previous_status <> 'in_review'
        OR criteria #>> '{gate,accept,passed_at}' IS NULL
        OR criteria #>> '{gate,accept,passed_by}' IS NULL
    ) THEN
        RAISE EXCEPTION 'Delivery acceptance is required before completing this task'
            USING ERRCODE = '23514', CONSTRAINT = 'workbench_execution_gate';
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS workbench_issue_execution_gate ON issue;
CREATE TRIGGER workbench_issue_execution_gate
BEFORE INSERT OR UPDATE OF status, acceptance_criteria ON issue
FOR EACH ROW EXECUTE FUNCTION guard_workbench_issue_execution();

CREATE OR REPLACE FUNCTION guard_workbench_paused_run()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE ancestor record;
BEGIN
    IF NEW.issue_id IS NULL OR NEW.status NOT IN ('queued', 'deferred', 'dispatched', 'running', 'waiting_local_directory') THEN RETURN NEW; END IF;
    FOR ancestor IN
        WITH RECURSIVE parents AS (
            SELECT id, parent_issue_id FROM issue WHERE id = NEW.issue_id
            UNION
            SELECT parent.id, parent.parent_issue_id FROM issue parent JOIN parents child ON child.parent_issue_id = parent.id
        )
        SELECT issue.status, issue.acceptance_criteria FROM issue JOIN parents ON parents.id = issue.id
        ORDER BY issue.id FOR SHARE OF issue
    LOOP
        IF ancestor.acceptance_criteria->>'v' = '1' AND ancestor.status = 'paused' THEN
            RAISE EXCEPTION 'This task or its parent is paused for a requirements change'
                USING ERRCODE = '23514', CONSTRAINT = 'workbench_execution_gate';
        END IF;
    END LOOP;
    RETURN NEW;
END $$;
CREATE TRIGGER workbench_paused_run_gate BEFORE INSERT OR UPDATE OF status ON agent_task_queue
FOR EACH ROW EXECUTE FUNCTION guard_workbench_paused_run();