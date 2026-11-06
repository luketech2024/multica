CREATE OR REPLACE FUNCTION validate_workbench_handoff(target_id uuid)
RETURNS void LANGUAGE plpgsql AS $$
DECLARE
    target issue;
    parent issue;
    handoff jsonb;
    dependency text;
BEGIN
    SELECT * INTO target FROM issue WHERE id = target_id;
    handoff := target.properties->'workbench_handoff';
    IF handoff IS NULL THEN RETURN; END IF;
    IF handoff->>'superseded' = 'true' THEN
        RAISE EXCEPTION 'This subtask belongs to a retired execution plan'
            USING ERRCODE = '23514', CONSTRAINT = 'workbench_execution_gate';
    END IF;
    SELECT * INTO parent FROM issue WHERE id = target.parent_issue_id AND workspace_id = target.workspace_id FOR SHARE;
    IF parent.id IS NULL OR parent.project_id IS DISTINCT FROM target.project_id
       OR parent.status IN ('paused','cancelled','done')
       OR parent.acceptance_criteria->>'needs_resplit' = 'true'
       OR parent.acceptance_criteria #>> '{plan,version}' IS DISTINCT FROM handoff->>'requirement_version'
       OR NOT COALESCE((parent.acceptance_criteria #> '{plan,issues}') ? target.id::text, false) THEN
        RAISE EXCEPTION 'This subtask is not in the current executable plan'
            USING ERRCODE = '23514', CONSTRAINT = 'workbench_execution_gate';
    END IF;
    FOR dependency IN SELECT jsonb_array_elements_text(handoff->'depends_on') LOOP
        IF NOT EXISTS (SELECT 1 FROM issue i WHERE i.id::text = dependency
            AND i.workspace_id = target.workspace_id AND i.project_id = target.project_id
            AND i.status = 'done' AND i.properties #>> '{workbench_handoff,superseded}' IS DISTINCT FROM 'true') THEN
            RAISE EXCEPTION 'Complete this subtask dependency before starting execution'
                USING ERRCODE = '23514', CONSTRAINT = 'workbench_execution_gate';
        END IF;
    END LOOP;
    IF COALESCE(handoff->>'serial_group','') <> '' THEN
        PERFORM pg_advisory_xact_lock(hashtextextended('workbench-serial:' || target.project_id::text || ':' || (handoff->>'role') || ':' || (handoff->>'serial_group'), 0));
        IF EXISTS (SELECT 1 FROM issue other WHERE other.id <> target.id
            AND other.workspace_id = target.workspace_id AND other.project_id = target.project_id
            AND other.properties #>> '{workbench_handoff,role}' = handoff->>'role'
            AND other.properties #>> '{workbench_handoff,serial_group}' = handoff->>'serial_group'
            AND (other.status = 'in_progress' OR EXISTS (SELECT 1 FROM agent_task_queue run
                WHERE run.issue_id = other.id AND run.status IN ('dispatched','running','waiting_local_directory')))) THEN
            RAISE EXCEPTION 'Another subtask in this serial group is executing'
                USING ERRCODE = '23514', CONSTRAINT = 'workbench_execution_gate';
        END IF;
    END IF;
END $$;

