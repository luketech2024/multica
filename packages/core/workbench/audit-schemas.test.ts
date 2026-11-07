// @vitest-environment node
import { expect, it } from "vitest";
import { AuditPageSchema, AuditExportSchema } from "./audit-schemas";

it("requires structured audit details and export identity", () => {
  const id = "00000000-0000-4000-8000-000000000001";
  const record = { id, created_at: "2026-11-07T00:00:00Z", actor_type: "future", actor_id: null, action: "security_accept", details: {} };
  const page = { records: [record], range_start: "2026-11-01T00:00:00Z", range_end: "2026-11-07T00:00:00Z", scope: "security" };
  expect(AuditPageSchema.parse(page).records[0]?.actor_type).toBe("system");
  expect(AuditPageSchema.safeParse({ ...page, records: [{ ...record, details: "e30=" }] }).success).toBe(false);
  expect(AuditExportSchema.safeParse({ filename: "audit.csv" }).success).toBe(false);
});
