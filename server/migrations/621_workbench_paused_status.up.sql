DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM issue_status WHERE key = 'paused' AND NOT is_system) THEN
        RAISE EXCEPTION 'Rename the custom paused status key before adding the workbench paused status';
    END IF;
END $$;

ALTER TABLE issue_status DROP CONSTRAINT IF EXISTS issue_status_system_is_canonical;
ALTER TABLE issue_status ADD CONSTRAINT issue_status_system_is_canonical CHECK (
    NOT is_system OR (key, category) IN (
        ('backlog', 'unstarted'), ('todo', 'unstarted'),
        ('in_progress', 'started'), ('in_review', 'started'), ('blocked', 'started'),
        ('paused', 'started'), ('done', 'done'), ('cancelled', 'closed')
    )
);
INSERT INTO issue_status (workspace_id, key, name, description, category, color, is_system, position)
SELECT id, 'paused', 'Paused', 'Execution is paused while requirements change.', 'started', '#6b7280', TRUE, 0
FROM workspace ON CONFLICT DO NOTHING;
