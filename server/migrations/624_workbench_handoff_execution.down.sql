DROP TRIGGER IF EXISTS workbench_handoff_run_gate ON agent_task_queue;
DROP TRIGGER IF EXISTS workbench_handoff_issue_gate ON issue;
DROP FUNCTION IF EXISTS guard_workbench_handoff_run();
DROP FUNCTION IF EXISTS guard_workbench_handoff_issue();
DROP FUNCTION IF EXISTS validate_workbench_handoff(uuid);
