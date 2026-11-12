CREATE INDEX CONCURRENTLY task_user_input_issue_idx ON task_user_input(workspace_id, issue_id, created_at);
