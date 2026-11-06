CREATE OR REPLACE FUNCTION guard_workbench_delivery_acceptance()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    previous_status text;
    target_status text;
BEGIN
    IF NEW.acceptance_criteria->>'v' IS DISTINCT FROM '1' OR NEW.status IS NOT DISTINCT FROM OLD.status THEN
        RETURN NEW;
    END IF;
    previous_status := issue_effective_status(NEW.workspace_id, OLD.status);
    target_status := issue_effective_status(NEW.workspace_id, NEW.status);
    IF target_status = 'done' AND previous_status <> 'done'
       AND NEW.acceptance_criteria #>> '{gate,accept,passed_at}' IS NOT DISTINCT FROM OLD.acceptance_criteria #>> '{gate,accept,passed_at}' THEN
        RAISE EXCEPTION 'A fresh delivery acceptance is required before completing this task'
            USING ERRCODE = '23514', CONSTRAINT = 'workbench_execution_gate';
    END IF;
    IF previous_status = 'done' AND target_status <> 'done' THEN
        NEW.acceptance_criteria := jsonb_set(NEW.acceptance_criteria, '{gate,accept}', 'null'::jsonb);
    END IF;
    RETURN NEW;
END $$;

CREATE TRIGGER workbench_delivery_acceptance_gate
BEFORE UPDATE OF status ON issue
FOR EACH ROW EXECUTE FUNCTION guard_workbench_delivery_acceptance();
