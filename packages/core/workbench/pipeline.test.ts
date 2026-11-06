// @vitest-environment node
import { expect, it } from "vitest";
import type { AcceptanceCriteria, Issue } from "../types";
import { projectRoleOrder } from "./schemas";
import { workbenchPipeline } from "./pipeline";

function criteria(): AcceptanceCriteria {
  return {
    v: 1, version: 2, next_item_no: 2, items: [], bounds: [],
    gate: { spec: { passed_at: "2026-11-07", passed_by: "member" }, accept: null },
    team: { project_id: "project", revision: 1, config: { v: 1, roles: Object.fromEntries(
      projectRoleOrder.map((role) => [role, { enabled: role !== "sec", agent_id: null, runtime_id: null }]),
    ) as NonNullable<AcceptanceCriteria["team"]>["config"]["roles"] } },
    plan: { version: 2, key: "plan", digest: "digest", issues: ["dev"] },
  };
}
function child(status: string, overrides: Partial<Issue> = {}): Issue {
  return {
    id: "dev", status, status_category: undefined,
    workbench_handoff: { v: 2, requirement_version: 2, role: "dev", repo_id: "repo", covers: ["A-01"], depends_on: [], serial_group: "", max_retry: 2, expected: ["diff"], superseded: false },
    ...overrides,
  } as Issue;
}
it("uses the task team snapshot and counts only current planned subtasks", () => {
  const result = workbenchPipeline(criteria(), [child("done"), child("blocked", { id: "old" })]);
  expect(result.map((stage) => stage.role)).toEqual(["sched", "product", "dev", "test", "ops"]);
  expect(result.find((stage) => stage.role === "dev")).toMatchObject({ state: "complete", tasks: [{ id: "dev" }] });
  expect(result.find((stage) => stage.role === "test")?.state).toBe("waiting");
});
it.each(["missing", "superseded", "old-version"])("does not declare completion with %s task evidence", (mode) => {
  const task = child("done");
  if (mode === "superseded") task.workbench_handoff!.superseded = true;
  if (mode === "old-version") task.workbench_handoff!.requirement_version = 1;
  const result = workbenchPipeline(criteria(), mode === "missing" ? [] : [task]);
  expect(result.find((stage) => stage.role === "dev")?.state).toBe("unavailable");
});
it.each(["stopping", "unknown", "resuming", "paused"] as const)("preserves the %s execution phase", (phase) => {
  const task = child("paused", { workbench_pause: { v: 1, phase, previous_status: "in_progress", requested_at: "now", requested_by: "member", run_ids: [], pending_runs: [], resume_error: "" } });
  expect(workbenchPipeline(criteria(), [task]).find((stage) => stage.role === "dev")?.state).toBe(phase);
});
it("retires old progress while requirements await a replacement plan", () => {
  expect(workbenchPipeline({ ...criteria(), needs_resplit: true }, [child("done")])
    .find((stage) => stage.role === "dev")).toMatchObject({ state: "waiting", tasks: [] });
});
it("uses custom lifecycle categories while preserving acceptance review", () => {
  expect(workbenchPipeline(criteria(), [child("custom_active", { status_category: "started" })])
    .find((stage) => stage.role === "dev")?.state).toBe("running");
  expect(workbenchPipeline(criteria(), [child("in_review", { status_category: "started" })])
    .find((stage) => stage.role === "dev")?.state).toBe("review");
});
