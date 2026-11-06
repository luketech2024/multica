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
  serial_group: string;
  expected: string[];
}

export interface ExecutionPlanInput {
  expected_revision: number;
  version: number;
  idempotency_key: string;
  tasks: ExecutionPlanTask[];
}
