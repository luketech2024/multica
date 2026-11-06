import { z } from "zod";

export const ExecutionPauseSchema = z.object({
  v: z.literal(1),
  phase: z.enum(["stopping", "unknown", "paused", "resuming"]),
  previous_status: z.string().min(1),
  requested_at: z.string().min(1),
  requested_by: z.string().uuid(),
  run_ids: z.array(z.string().uuid()),
  pending_runs: z.array(z.string().uuid()).default([]),
  resume_error: z.string().default(""),
});

export type ExecutionPause = z.infer<typeof ExecutionPauseSchema>;
export type ExecutionControlAction = "pause" | "reconcile" | "resume";
