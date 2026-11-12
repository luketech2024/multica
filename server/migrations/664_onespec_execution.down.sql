DROP TRIGGER IF EXISTS onespec_completion_gate ON issue;
DROP FUNCTION IF EXISTS guard_onespec_completion();
DROP TRIGGER IF EXISTS onespec_execution_gate ON agent_task_queue;
DROP FUNCTION IF EXISTS guard_onespec_execution();
