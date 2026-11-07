ALTER TABLE issue ADD COLUMN completed_at timestamptz;

UPDATE issue i SET completed_at = (
    SELECT max(a.created_at) FROM activity_log a
    WHERE a.workspace_id=i.workspace_id AND a.issue_id=i.id AND a.action='status_changed'
      AND issue_effective_status(i.workspace_id, a.details->>'to')='done'
      AND issue_effective_status(i.workspace_id, a.details->>'from') IS DISTINCT FROM 'done'
)
WHERE issue_effective_status(i.workspace_id,i.status)='done';

CREATE FUNCTION maintain_issue_completion_time()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF issue_effective_status(NEW.workspace_id,NEW.status) IS DISTINCT FROM 'done' THEN
        NEW.completed_at := NULL;
    ELSIF TG_OP = 'INSERT' THEN
        NEW.completed_at := now();
    ELSIF issue_effective_status(OLD.workspace_id,OLD.status) IS DISTINCT FROM 'done' THEN
        NEW.completed_at := now();
    ELSE
        NEW.completed_at := OLD.completed_at;
    END IF;
    RETURN NEW;
END $$;

CREATE TRIGGER issue_completion_time
BEFORE INSERT OR UPDATE OF status, completed_at ON issue
FOR EACH ROW EXECUTE FUNCTION maintain_issue_completion_time();
