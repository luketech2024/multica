DROP TRIGGER IF EXISTS workbench_retry_budget ON agent_task_queue;
DROP FUNCTION IF EXISTS stamp_workbench_retry_budget();
DROP FUNCTION IF EXISTS workbench_retry_allowed(uuid);
