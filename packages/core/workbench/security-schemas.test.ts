// @vitest-environment node
import { expect, it } from "vitest";
import { SecurityCheckSchema, SecurityChecksSchema, SecurityScanSchema } from "./security-schemas";
import { parseWorkbenchResponse } from "./schemas";

const id = "00000000-0000-4000-8000-000000000001";
const check = { id, identifier: "SEC-01", project_id: id, repo_id: id, issue_id: id, severity: "high", category: "injection",
  title: "Unsafe query", location: "query.go:1", recommendation: "Bind parameters", status: "unresolved", effective_status: "unresolved",
  artifact_version: "v1", last_scan_id: id, revision: 1 };

it("rejects missing risk identity, invalid revisions and incomplete pages", () => {
  for (const input of [{ ...check, id: "wrong" }, { ...check, revision: 0 }, { ...check, last_scan_id: null }]) {
    expect(() => parseWorkbenchResponse(input, SecurityCheckSchema, "security")).toThrow();
  }
  expect(SecurityChecksSchema.safeParse({ counts: [] }).success).toBe(false);
  expect(SecurityChecksSchema.parse({ checks: [], counts: null }).counts).toEqual([]);
});

it("keeps unknown severity and status conservative and defaults optional evidence", () => {
  const parsed = SecurityCheckSchema.parse({ ...check, severity: "critical", status: "new_state", effective_status: "new_state" });
  expect(parsed.severity).toBe("high");
  expect(parsed.effective_status).toBe("unresolved");
  expect(parsed.fixed_scan_id).toBeNull();
});

it("rejects executable scan URLs and treats unknown scan outcomes as failed", () => {
  const scan = { id, run_id: "run-1", repo_id: id, artifact_version: "v1", source_commit: "a".repeat(40), status: "unknown", scanned_at: "2026-11-07T00:00:00Z", report: { report_url: "https://example.test/report" } };
  expect(SecurityScanSchema.parse(scan).status).toBe("failed");
  expect(SecurityScanSchema.safeParse({ ...scan, report: { report_url: "javascript:alert(1)" } }).success).toBe(false);
});
