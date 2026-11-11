// @vitest-environment node
import { expect, it } from "vitest";
import {
  ReleaseCandidatesSchema,
  ReleaseOrderSchema,
  ReleaseGateSchema,
} from "./release-schemas";

it("keeps old candidate responses compatible and exposes blocked tasks safely", () => {
  expect(ReleaseCandidatesSchema.parse({ candidates: [] }).blocked).toEqual([]);
  const blocked = { id: "00000000-0000-4000-8000-000000000001", identifier: "TEST-1", title: "Completed", blockers: [{ code: "future_gate" }] };
  expect(ReleaseCandidatesSchema.parse({ candidates: [], blocked: [blocked] }).blocked[0]?.blockers[0]?.code).toBe("acceptance");
  expect(ReleaseCandidatesSchema.safeParse({ candidates: [], blocked: [{ ...blocked, blockers: null }] }).success).toBe(false);
});

it("rejects missing release snapshots and malformed merge evidence", () => {
  expect(
    ReleaseOrderSchema.safeParse({ id: "bad", status: "ready" }).success,
  ).toBe(false);
  expect(ReleaseCandidatesSchema.safeParse({ candidates: null }).success).toBe(
    false,
  );
  const id = "00000000-0000-4000-8000-000000000001";
  const candidate = {
    id,
    identifier: "TASK-1",
    title: "Accepted task",
    revision: 1,
    merges: [
      {
        repo_id: id,
        pull_request_id: id,
        url: "https://example.test/pr/1",
        base_branch: "main",
        source_commit: "a".repeat(40),
        merge_commit: "short",
      },
    ],
  };
  expect(
    ReleaseCandidatesSchema.safeParse({ candidates: [candidate] }).success,
  ).toBe(false);
  candidate.merges[0]!.merge_commit = "b".repeat(40);
  expect(
    ReleaseCandidatesSchema.parse({ candidates: [candidate] }).candidates[0]
      ?.merges[0]?.merge_commit,
  ).toBe("b".repeat(40));
});

it("rejects malformed gate states and preserves unknown blocker codes", () => {
  const gate = { id: "security", passed: true, blockers: [] };
  expect(ReleaseGateSchema.parse(gate).required).toBe(true);
  expect(ReleaseGateSchema.parse({ ...gate, required: false }).required).toBe(false);
  expect(ReleaseGateSchema.safeParse({ ...gate, required: "false" }).success).toBe(false);
  expect(
    ReleaseGateSchema.safeParse({
      id: "security",
      passed: "true",
      blockers: [],
    }).success,
  ).toBe(false);
  expect(
    ReleaseGateSchema.safeParse({ id: "security", passed: true }).success,
  ).toBe(false);
  expect(
    ReleaseGateSchema.parse({
      id: "security",
      passed: false,
      blockers: [{ code: "new_server_check" }],
    }).blockers[0]?.code,
  ).toBe("new_server_check");
});
