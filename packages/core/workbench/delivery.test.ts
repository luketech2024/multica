// @vitest-environment node
import { describe, expect, it } from "vitest";
import { DeliveryEvidenceSchema, DeliveryReviewSchema, ReleaseReadinessSchema } from "./delivery";
import { AcceptanceCriteriaSchema } from "../api/schemas";

const id = "00000000-0000-4000-8000-000000000001";
const evidence = {
  description_hash: "a".repeat(64),
  issue_id: id, issue_revision: 2, version: 1, role: "test", repo_id: id,
  commit: "a".repeat(40), passed: true, summary: "Verified report", changes: null,
  preview_url: "", reports: [{ name: "report", url: "https://example.test/report" }],
  recorded_at: "2026-11-07T00:00:00Z", recorded_by: id, actor_type: "member",
};
describe("delivery response boundary", () => {
  it("keeps version and actor attribution in the requirements projection", () => {
    const result = AcceptanceCriteriaSchema.parse({ v: 1, items: [], delivery: { [id]: evidence } });
    expect(result.delivery?.[id]).toEqual(evidence);
  });
  it.each([
    { ...evidence, passed: "true" },
    { ...evidence, issue_revision: -1 },
    { ...evidence, preview_url: "javascript:alert(1)" },
    { ...evidence, reports: [{ name: "report", url: "https://user:secret@example.test/report" }] },
    { ...evidence, role: "future" },
    { ...evidence, recorded_by: null },
  ])("does not expose malformed evidence as a valid report", (invalid) => {
    expect(DeliveryEvidenceSchema.safeParse(invalid).success).toBe(false);
    expect(AcceptanceCriteriaSchema.parse({ v: 1, items: [], delivery: { [id]: invalid } }).delivery).toBeUndefined();
  });
  it("allows older responses without delivery evidence", () => {
    expect(AcceptanceCriteriaSchema.parse({ v: 1, items: [] }).delivery).toBeUndefined();
  });
  it("rejects malformed readiness and keeps future blockers blocking", () => {
    expect(ReleaseReadinessSchema.safeParse({ revision: 1, merges: [], blockers: null }).success).toBe(false);
    expect(ReleaseReadinessSchema.parse({ revision: 1, merges: [], blockers: [{ code: "future" }] }).blockers[0]?.code).toBe("merge_missing");
    expect(DeliveryReviewSchema.safeParse({ revision: 1, blockers: null, checks: [] }).success).toBe(false);
    const state = DeliveryReviewSchema.parse({ revision: 1, blockers: [{ code: "future_gate" }], checks: [] });
    expect(state.blockers).toEqual([{ code: "plan" }]);
  });
  it("preserves attributed decisions and omits malformed decision records", () => {
    const review = { action: "reject", reason: "Fix export", checks: [], at: "2026-11-07T00:00:00Z", by: id };
    expect(AcceptanceCriteriaSchema.parse({ v: 1, items: [], review }).review).toEqual(review);
    expect(AcceptanceCriteriaSchema.parse({ v: 1, items: [], review: { ...review, action: "future" } }).review).toBeUndefined();
  });
});
