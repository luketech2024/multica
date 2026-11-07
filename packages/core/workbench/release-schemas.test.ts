// @vitest-environment node
import { expect, it } from "vitest";
import { ReleaseCandidatesSchema, ReleaseOrderSchema } from "./release-schemas";

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
