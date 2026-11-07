import { z } from "zod";

export const auditScopes = ["all", "configuration", "security"] as const;
export type AuditScope = (typeof auditScopes)[number];
export const AuditPageSchema = z.object({
  records: z.array(z.object({
    id: z.string().uuid(),
    created_at: z.string(),
    actor_type: z.enum(["member", "agent", "system"]).catch("system").nullable(),
    actor_name: z.string().default(""),
    actor_id: z.string().uuid().nullable(),
    action: z.string(),
    details: z.record(z.string(), z.unknown()).nullable(),
  })),
  next_cursor: z.string().default(""),
  range_start: z.string(),
  range_end: z.string(),
  scope: z.enum(auditScopes),
});
export const AuditExportSchema = z.object({
  id: z.string().uuid(),
  filename: z.string(),
  expires_at: z.string(),
  range_start: z.string(),
  range_end: z.string(),
  scope: z.enum(auditScopes),
  size: z.number().int().nonnegative(),
});
