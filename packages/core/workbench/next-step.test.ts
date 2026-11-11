// @vitest-environment node
import { expect, it } from "vitest";
import type { AcceptanceCriteria, Issue } from "../types";
import { workbenchNextStep } from "./next-step";

const criteria: AcceptanceCriteria = {
  v: 1, version: 2, next_item_no: 1, items: [], bounds: [],
  gate: { spec: null, accept: null },
};
const issue = (overrides: Partial<Issue> = {}) => ({
  id: "parent", project_id: "project", status: "todo", acceptance_criteria: criteria, ...overrides,
} as Issue);

it("routes an unconfirmed parent to the formal requirements form", () => {
  expect(workbenchNextStep(issue())).toEqual({ step: "requirements", destination: "requirements" });
});
it("keeps done but unaccepted legacy work in acceptance recovery", () => {
  expect(workbenchNextStep(issue({ status: "done", acceptance_criteria: undefined })))
    .toEqual({ step: "recovery", destination: "review" });
});
it("keeps blocked clarification work out of acceptance recovery", () => {
  expect(workbenchNextStep(issue({ status: "blocked" }))).toEqual({ step: "blocked", destination: "activity" });
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
