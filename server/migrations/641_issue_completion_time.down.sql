DROP TRIGGER IF EXISTS issue_completion_time ON issue;
DROP FUNCTION IF EXISTS maintain_issue_completion_time();
ALTER TABLE issue DROP COLUMN IF EXISTS completed_at;
