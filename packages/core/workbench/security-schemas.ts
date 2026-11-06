import { z } from "zod";

export const securityStatuses = ["unresolved", "pending_recheck", "fixed", "accepted"] as const;
export type SecurityStatus = (typeof securityStatuses)[number];
export const SecurityCheckSchema = z.object({
  id: z.string().uuid(),
  identifier: z.string(),
  project_id: z.string().uuid(),
  repo_id: z.string().uuid(),
  issue_id: z.string().uuid(),
  severity: z.enum(["high", "medium", "low"]).catch("high"),
  category: z.enum(["secrets", "deps", "authz", "injection"]).catch("authz"),
  title: z.string(),
  cwe: z.string().default(""),
  location: z.string(),
  recommendation: z.string(),
  status: z.enum(securityStatuses).catch("unresolved"),
  effective_status: z.enum(securityStatuses).catch("unresolved"),
  artifact_version: z.string(),
  last_scan_id: z.string().uuid(),
  fixed_scan_id: z.string().uuid().nullable().default(null),
  fix_version: z.string().default(""),
  accepted_until: z.string().nullable().default(null),
  acceptance_reason: z.string().default(""),
  revision: z.number().int().positive(),
});
export type SecurityCheck = z.infer<typeof SecurityCheckSchema>;
export const SecurityScanSchema = z.object({
  id: z.string().uuid(),
  run_id: z.string(),
  repo_id: z.string().uuid(),
  artifact_version: z.string(),
  source_commit: z.string(),
  status: z.enum(["completed", "failed"]).catch("failed"),
  scanned_at: z.string(),
  report: z.object({ report_url: z.string().url().refine((url) => /^https?:\/\//i.test(url)) }),
});
export const SecurityChecksSchema = z.object({
  checks: z.array(SecurityCheckSchema),
  counts: z.array(z.object({
    effective_status: z.enum(securityStatuses).catch("unresolved"),
    severity: z.enum(["high", "medium", "low"]).catch("high"),
    total: z.number().int().nonnegative(),
  })).nullable().transform((value) => value ?? []),
  next_cursor: z.string().default(""),
});
export type SecurityDecision =
  | { action: "accept"; expected_revision: number; reason: string; days: number }
  | { action: "request_recheck"; expected_revision: number; artifact_version: string }
  | { action: "recheck"; expected_revision: number; scan_id: string };

export interface SecurityFindingInput {
  fingerprint: string;
  severity: "high" | "medium" | "low";
  category: "secrets" | "deps" | "authz" | "injection";
  title: string;
  cwe: string;
  location: string;
  recommendation: string;
}

export interface SecurityScanInput {
  issue_id: string;
  issue_revision: number;
  repo_id: string;
  run_id: string;
  artifact_version: string;
  commit: string;
  scanned_at: string;
  status: "completed" | "failed";
  report_url: string;
  categories: string[];
  findings: SecurityFindingInput[];
}
