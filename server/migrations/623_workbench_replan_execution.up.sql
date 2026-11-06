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
        IF ancestor.acceptance_criteria->>'v' = '1' AND (ancestor.status = 'paused' OR ancestor.acceptance_criteria->>'needs_resplit' = 'true') THEN
            RAISE EXCEPTION 'This task or its parent is paused or awaiting a new execution plan'
                USING ERRCODE = '23514', CONSTRAINT = 'workbench_execution_gate';
        END IF;
    END LOOP;
    RETURN NEW;
END $$;
