export interface ExecutionPlanTask {
  key: string;
  title: string;
  description: string;
  role: string;
  repo_id: string;
  covers: string[];
  depends_on: string[];
  serial_group: string;
  expected: string[];
}

export interface ExecutionPlanInput {
  expected_revision: number;
  version: number;
  idempotency_key: string;
  tasks: ExecutionPlanTask[];
}
