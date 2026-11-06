CREATE OR REPLACE FUNCTION stamp_workbench_retry_budget()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE handoff jsonb;
BEGIN
    SELECT properties->'workbench_handoff' INTO handoff FROM issue WHERE id = NEW.issue_id;
    IF handoff->>'v' = '2' THEN
        NEW.max_attempts := 1 + COALESCE((handoff->>'max_retry')::integer, 2);
        IF NEW.max_attempts < 1 OR NEW.max_attempts > 11 THEN
            RAISE EXCEPTION 'Workbench retry budget must be between zero and ten'
                USING ERRCODE = '23514', CONSTRAINT = 'workbench_execution_gate';
        END IF;
        NEW.context := COALESCE(NEW.context, '{}'::jsonb) || jsonb_build_object('workbench_max_attempts', NEW.max_attempts);
    END IF;
    RETURN NEW;
END $$;

CREATE TRIGGER workbench_retry_budget BEFORE INSERT ON agent_task_queue
FOR EACH ROW EXECUTE FUNCTION stamp_workbench_retry_budget();

UPDATE agent_task_queue run
SET max_attempts = 1 + COALESCE((owner.properties #>> '{workbench_handoff,max_retry}')::integer, 2),
    context = COALESCE(run.context, '{}'::jsonb) || jsonb_build_object('workbench_max_attempts',
        1 + COALESCE((owner.properties #>> '{workbench_handoff,max_retry}')::integer, 2))
FROM issue owner
WHERE owner.id = run.issue_id AND owner.properties #>> '{workbench_handoff,v}' = '2'
    AND run.status IN ('queued','deferred','dispatched','running','waiting_local_directory');

-- A refused retry must not roll back the failed attempt that caused it.
CREATE OR REPLACE FUNCTION workbench_retry_allowed(target_id uuid)
RETURNS boolean LANGUAGE plpgsql AS $$
DECLARE target issue; constraint_name text;
BEGIN
    IF target_id IS NULL THEN RETURN true; END IF;
    SELECT * INTO target FROM issue WHERE id = target_id FOR SHARE;
    IF target.acceptance_criteria->>'v' = '1' AND target.status IN ('paused','cancelled','done') THEN
        RETURN false;
    END IF;
    IF target.properties->'workbench_handoff' IS NULL THEN RETURN true; END IF;
    IF target.status NOT IN ('todo','in_progress','blocked','in_review') THEN RETURN false; END IF;
    PERFORM validate_workbench_handoff(target_id);
    RETURN true;
EXCEPTION WHEN check_violation THEN
    GET STACKED DIAGNOSTICS constraint_name = CONSTRAINT_NAME;
    IF constraint_name = 'workbench_execution_gate' THEN RETURN false; END IF;
    RAISE;
END $$;
