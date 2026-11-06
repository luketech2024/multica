DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM issue WHERE status = 'paused') THEN
        RAISE EXCEPTION 'Resolve paused issues before removing the paused status';
    END IF;
END $$;
DELETE FROM issue_status WHERE key = 'paused' AND is_system;
ALTER TABLE issue_status DROP CONSTRAINT IF EXISTS issue_status_system_is_canonical;
ALTER TABLE issue_status ADD CONSTRAINT issue_status_system_is_canonical CHECK (
    NOT is_system OR (key, category) IN (
        ('backlog', 'unstarted'), ('todo', 'unstarted'),
        ('in_progress', 'started'), ('in_review', 'started'),
        ('blocked', 'started'), ('done', 'done'), ('cancelled', 'closed')
    )
);
