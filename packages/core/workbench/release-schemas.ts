import { z } from "zod";
import { ReleaseReadinessSchema } from "./delivery";

export const ReleaseOrderSchema = z.object({
  id: z.string().uuid(),
  workspace_id: z.string().uuid(),
  project_id: z.string().uuid(),
  identifier: z.string(),
  env: z.enum(["staging", "prod"]),
  status: z
    .enum([
      "draft",
      "prechecking",
      "ready",
      "deploying",
      "verifying",
      "live",
      "failed",
      "partial",
      "retrying",
    ])
    .catch("draft"),
  revision: z.number().int().positive(),
  created_by: z.string().uuid(),
  created_at: z.string(),
  updated_at: z.string(),
  manifest: z.object({
    tasks: z.array(
      z.object({
        id: z.string().uuid(),
        identifier: z.string(),
        title: z.string(),
        revision: z.number().int().positive(),
      }),
    ),
    repositories: z.array(
      z.object({
        repo_id: z.string().uuid(),
        tag: z.string(),
        merges: ReleaseReadinessSchema.shape.merges,
      }),
    ),
  }),
});
export const ReleaseOrdersSchema = z.object({
  releases: z.array(ReleaseOrderSchema),
  next_cursor: z.string().nullable().default(null),
});
export const ReleaseCandidatesSchema = z.object({
  candidates: z.array(
    z.object({
      id: z.string().uuid(),
      identifier: z.string(),
      title: z.string(),
      revision: z.number().int().positive(),
      merges: ReleaseReadinessSchema.shape.merges,
    }),
  ),
  next_cursor: z.string().nullable().default(null),
});
export type ReleaseOrder = z.infer<typeof ReleaseOrderSchema>;
export type ReleaseOrders = z.infer<typeof ReleaseOrdersSchema>;
export type ReleaseCandidates = z.infer<typeof ReleaseCandidatesSchema>;
export type ReleaseOrderInput = {
  project_id: string;
  env: "staging" | "prod";
  request_id: string;
  tasks: { id: string; expected_revision: number }[];
  tags?: Record<string, string>;
};
