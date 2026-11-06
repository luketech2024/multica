-- Pin configured role runtimes across enqueue, retry and claim transitions.
CREATE OR REPLACE FUNCTION guard_workbench_role_runtime()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    target issue%ROWTYPE;
    roles jsonb;
    binding jsonb;
    role_name text;
BEGIN
    IF NEW.issue_id IS NULL OR NEW.status NOT IN ('queued', 'deferred', 'dispatched', 'running', 'waiting_local_directory') THEN
        RETURN NEW;
    END IF;
    IF TG_OP = 'UPDATE' AND NEW.status IS NOT DISTINCT FROM OLD.status
       AND NEW.agent_id IS NOT DISTINCT FROM OLD.agent_id
       AND NEW.runtime_id IS NOT DISTINCT FROM OLD.runtime_id THEN
        RETURN NEW;
    END IF;
    SELECT * INTO target FROM issue WHERE id = NEW.issue_id;
    IF target.properties #>> '{workbench_handoff,v}' = '2' THEN
        SELECT acceptance_criteria #> '{team,config,roles}' INTO roles
        FROM issue WHERE id = target.parent_issue_id AND workspace_id = target.workspace_id;
        role_name := target.properties #>> '{workbench_handoff,role}';
        binding := roles->role_name;
        IF binding->>'runtime_id' IS NOT NULL AND binding->>'runtime_id' IS DISTINCT FROM NEW.runtime_id::text THEN
            RAISE EXCEPTION 'Role runtime no longer matches the executing agent; update its binding before dispatch'
                USING ERRCODE = '23514', CONSTRAINT = 'workbench_execution_gate';
        END IF;
    ELSE
        roles := target.acceptance_criteria #> '{team,config,roles}';
        IF jsonb_typeof(roles) = 'object' THEN
            FOR binding IN SELECT value FROM jsonb_each(roles) LOOP
                IF binding->>'enabled' = 'true' AND binding->>'agent_id' = NEW.agent_id::text
                   AND binding->>'runtime_id' IS NOT NULL AND binding->>'runtime_id' IS DISTINCT FROM NEW.runtime_id::text THEN
                    RAISE EXCEPTION 'Role runtime no longer matches the executing agent; update its binding before dispatch'
                        USING ERRCODE = '23514', CONSTRAINT = 'workbench_execution_gate';
                END IF;
            END LOOP;
        END IF;
    END IF;
    RETURN NEW;
END $$;

CREATE TRIGGER workbench_role_runtime_gate
BEFORE INSERT OR UPDATE OF status, agent_id, runtime_id ON agent_task_queue
FOR EACH ROW EXECUTE FUNCTION guard_workbench_role_runtime();
