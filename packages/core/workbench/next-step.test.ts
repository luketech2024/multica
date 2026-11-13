// @vitest-environment node
import { expect, it } from "vitest";
import type { AcceptanceCriteria, Issue } from "../types";
import { requirementsReady, workbenchNextStep } from "./next-step";

const criteria: AcceptanceCriteria = {
  v: 1, version: 2, next_item_no: 1, items: [], bounds: [],
  gate: { spec: null, accept: null },
};
const issue = (overrides: Partial<Issue> = {}) => ({
  id: "parent", project_id: "project", status: "todo", acceptance_criteria: criteria, ...overrides,
} as Issue);

it("starts an empty parent in discussion", () => {
  expect(workbenchNextStep(issue())).toEqual({ step: "discussion", destination: "activity" });
});
it("keeps done but unaccepted legacy work in acceptance recovery", () => {
  expect(workbenchNextStep(issue({ status: "done", acceptance_criteria: undefined })))
    .toEqual({ step: "recovery", destination: "review" });
});
it("keeps blocked clarification work out of acceptance recovery", () => {
  expect(workbenchNextStep(issue({ status: "blocked" }))).toEqual({ step: "blocked", destination: "activity" });
});
it("directs a planned parent still in todo to its execution tasks", () => {
  expect(workbenchNextStep(issue({ acceptance_criteria: {
    ...criteria,
    gate: { spec: { passed_at: "now", passed_by: "human" }, accept: null },
    plan: { version: 2, key: "plan", digest: "digest", issues: ["child"] },
  } }))).toEqual({ step: "planned", destination: "children" });
});
it("requires delivery review after a planned parent completes", () => {
  expect(workbenchNextStep(issue({ status: "done", acceptance_criteria: { ...criteria, plan: { version: 2, key: "p", digest: "d", issues: [] } } }))).toEqual({ step: "review", destination: "review" });
});
it("shows release conditions only after formal acceptance", () => {
  expect(workbenchNextStep(issue({ status: "done", acceptance_criteria: { ...criteria, gate: { spec: null, accept: { passed_at: "now", passed_by: "human" } } } }))).toEqual({ step: "accepted", destination: "review" });
});
it("keeps completed children and ordinary tasks out of parent acceptance", () => {
  expect(workbenchNextStep(issue({ status: "done", parent_issue_id: "parent" })).destination).toBe("activity");
  expect(workbenchNextStep(issue({ project_id: null, status: "done" })).destination).toBe("activity");
});
it("supports custom lifecycle statuses and retains pause controls", () => {
  expect(workbenchNextStep(issue({ status: "custom", status_category: "started" })).step).toBe("running");
  expect(workbenchNextStep(issue({ status: "paused" })).destination).toBe("execution");
  expect(workbenchNextStep(issue({ status: "cancelled" })).step).toBe("closed");
});

it("requires explicit submission and current discussion coverage", () => {
  const draft: AcceptanceCriteria = { ...criteria, phase: "draft", draft_revision: 2, discussion_revision: 3,
    items: [{ id: "A-01", text: "Open homepage", covers: [], checked: null, note: null, origin: "product" }] };
  expect(requirementsReady(draft)).toBe(false);
  const ready = { ...draft, phase: "ready" as const, ready_revision: 2, covered_discussion_revision: 3 };
  expect(workbenchNextStep(issue({ acceptance_criteria: ready })).step).toBe("requirements");
  expect(requirementsReady({ ...ready, discussion_revision: 4 })).toBe(false);
  expect(requirementsReady({ ...ready, draft_revision: 3 })).toBe(false);
  expect(requirementsReady({ ...draft, phase: undefined })).toBe(true);
});
