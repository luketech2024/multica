import { z } from "zod";
import { parseWithFallback } from "../api/schema";
import { projectRoleOrder } from "./schemas";

export const WorkbenchHandoffSchema = z.object({
  v: z.literal(2),
  requirement_version: z.number().int().positive(),
  role: z.enum(projectRoleOrder),
  repo_id: z.union([z.literal(""), z.string().uuid()]),
  covers: z.array(z.string().regex(/^A-[0-9]+$/)).min(1),
  depends_on: z.array(z.string().uuid()),
  serial_group: z.string(),
  max_retry: z.number().int().nonnegative(),
  expected: z.array(z.string().min(1)).min(1),
  superseded: z.boolean().default(false),
});
export type WorkbenchHandoff = z.infer<typeof WorkbenchHandoffSchema>;
export function parseWorkbenchHandoff(value: unknown): WorkbenchHandoff | null {
  return parseWithFallback<WorkbenchHandoff | null>(
    value,
    WorkbenchHandoffSchema,
    null,
    { endpoint: "issue.properties.workbench_handoff" },
  );
}

export interface ExecutionPlanTask {
  key: string;
  title: string;
  description: string;
  role: string;
  repo_id: string;
  covers: string[];
  depends_on: string[];
  existing_dependencies?: string[];
  serial_group: string;
  expected: string[];
}

export interface ExecutionPlanInput {
  expected_revision: number;
  version: number;
  idempotency_key: string;
  tasks: ExecutionPlanTask[];
}

export function analyzeExecutionPlan(
  tasks: ExecutionPlanTask[],
  itemIds: string[],
  repositoryIds: string[],
) {
  const covered = new Set(tasks.flatMap((task) => task.covers));
  const uncovered = itemIds.filter((id) => !covered.has(id));
  const missingRepositories = tasks
    .filter((task) =>
      task.repo_id
        ? !repositoryIds.includes(task.repo_id)
        : task.role === "dev",
    )
    .map((task) => task.key);
  const graph = new Map(tasks.map((task) => [task.key, task.depends_on]));
  const visited = new Set<string>();
  const active = new Set<string>();
  const path: string[] = [];
  let cycle: string[] = [];
  function visit(key: string): boolean {
    if (active.has(key)) {
      cycle = [...path.slice(path.indexOf(key)), key];
      return true;
    }
    if (visited.has(key)) return false;
    visited.add(key);
    active.add(key);
    path.push(key);
    for (const dependency of graph.get(key) ?? []) {
      if (visit(dependency)) return true;
    }
    path.pop();
    active.delete(key);
    return false;
  }
  tasks.some((task) => visit(task.key));
  return {
    coveredCount: itemIds.length - uncovered.length,
    uncovered,
    missingRepositories,
    cycle,
    canSave:
      tasks.length > 0 &&
      uncovered.length === 0 &&
      missingRepositories.length === 0 &&
      cycle.length === 0 &&
      tasks.every(
        (task) =>
          task.title.trim() &&
          task.covers.length > 0 &&
          task.expected.length > 0 &&
          task.expected.every((value) => value.trim()),
      ),
  };
}
