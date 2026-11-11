import { z } from "zod";

const evidenceURL = z.string().max(2048).refine((value) => {
  try {
    const url = new URL(value);
    return ["http:", "https:"].includes(url.protocol) && !url.username && !url.password;
  } catch {
    return false;
  }
});

export const DeliveryEvidenceSchema = z.object({
  description_hash: z.string().regex(/^[a-f0-9]{64}$/),
  issue_id: z.string().uuid(),
  issue_revision: z.number().int().positive(),
  version: z.number().int().positive(),
  role: z.enum(["sched", "product", "dev", "test", "sec", "ops"]),
  repo_id: z.string(),
  commit: z.string(),
  passed: z.boolean(),
  summary: z.string().min(1).max(2000),
  reports: z.array(z.object({ name: z.string().min(1), url: evidenceURL })).min(1),
  changes: z.object({ files: z.number().int().nonnegative(), additions: z.number().int().nonnegative(), deletions: z.number().int().nonnegative() }).nullable(),
  preview_url: z.union([z.literal(""), evidenceURL]),
  recorded_at: z.string().datetime({ offset: true }),
  recorded_by: z.string().uuid(),
  actor_type: z.enum(["member", "agent"]),
});

export type DeliveryEvidence = z.infer<typeof DeliveryEvidenceSchema>;
export const DeliveryReviewSchema = z.object({
  revision: z.number().int().positive(),
  blockers: z.array(z.object({
    code: z.enum(["plan", "running", "task_missing", "task_incomplete", "evidence_missing", "evidence_stale", "evidence_failed", "role_missing"]).catch("plan"),
    issue_id: z.string().optional(),
    title: z.string().optional(),
    role: z.enum(["dev", "test", "sec", "ops"]).optional(),
  })),
  checks: z.array(z.object({ id: z.string(), text: z.string(), check: z.string() })),
});
export type DeliveryReviewState = z.infer<typeof DeliveryReviewSchema>;

export const ReleaseReadinessSchema = z.object({
  revision: z.number().int().positive(),
  blockers: z.array(z.object({
    code: z.enum(["acceptance", "repository_missing", "merge_missing", "merge_ambiguous"]).catch("merge_missing"),
    repo_id: z.string().optional(),
  })),
  merges: z.array(z.object({
    repo_id: z.string().uuid(),
    pull_request_id: z.string().uuid(),
    url: evidenceURL,
    base_branch: z.string().min(1),
    source_commit: z.string().regex(/^[a-f0-9]{40}([a-f0-9]{24})?$/i),
    merge_commit: z.string().regex(/^[a-f0-9]{40}([a-f0-9]{24})?$/i),
  })),
});
export type ReleaseReadiness = z.infer<typeof ReleaseReadinessSchema>;
export type DeliveryReviewInput = {
  expected_revision: number;
  action: "accept" | "reject" | "undo" | "claim" | "recover";
  reason?: string;
  checks?: string[];
  recovery?: {
    items: string[];
    tasks: { issue_id: string; revision: number; role: string; repo_id: string; covers: string[] }[];
  };
};
export const DeliveryDecisionSchema = z.object({
  action: z.enum(["accept", "reject", "undo"]),
  reason: z.string(),
  checks: z.array(z.object({ id: z.string(), text: z.string(), check: z.string() })),
  at: z.string().datetime({ offset: true }),
  by: z.string().uuid(),
});
export type DeliveryDecision = z.infer<typeof DeliveryDecisionSchema>;
export type DeliveryEvidenceInput = Pick<DeliveryEvidence,
  "issue_id" | "issue_revision" | "commit" | "passed" | "summary" | "reports" | "changes" | "preview_url"
>;
