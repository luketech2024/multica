// @vitest-environment node
import { describe, expect, it } from "vitest";
import { IssueSchema } from "./schemas";
import { parseWithFallback } from "./schema";
import { projectRoleOrder } from "../workbench/schemas";

const schema = IssueSchema.pick({ acceptance_criteria: true });
describe("requirements response compatibility", () => {
  it("preserves a change baseline and rejects malformed stop or resume data", () => {
    const change = {
      phase: "unknown",
      reason: "Adjust scope",
      requested_at: "2026-11-06T00:00:00Z",
      requested_by: "member-1",
      previous_status: "in_progress",
      baseline: {
        version: 1,
        items: [],
        bounds: ["No billing"],
        gate: { spec: null, accept: null },
      },
      run_ids: ["11111111-1111-4111-8111-111111111111"],
    };
    const criteria = {
      v: 1,
      items: [],
      change,
      needs_resplit: true,
      plan: {
        version: 1,
        key: "plan-1",
        digest: "digest",
        issues: ["11111111-1111-4111-8111-111111111111"],
      },
    };
    expect(
      schema.parse({ acceptance_criteria: criteria }).acceptance_criteria,
    ).toMatchObject(criteria);
    for (const invalid of [
      { ...criteria, change: { ...change, phase: "finished" } },
      { ...criteria, resume_runs: ["invalid"] },
      { ...criteria, needs_resplit: "yes" },
      { ...criteria, plan: { ...criteria.plan, issues: ["invalid"] } },
    ]) {
      expect(
        schema.parse({ acceptance_criteria: invalid }).acceptance_criteria,
      ).toBeUndefined();
    }
  });
  it("preserves a template snapshot and rejects malformed template data", () => {
    const template = {
      source_id: "blank",
      source_revision: 1,
      revision: 2,
      content: {
        v: 1,
        name: "Project rules",
        background: [],
        audience: [],
        scope_out: [],
        ac_skeleton: [],
        forbidden: [],
        dod: [],
        resources: [],
        terms: {},
      },
    };
    expect(
      schema.parse({ acceptance_criteria: { v: 1, items: [], template } })
        .acceptance_criteria?.template,
    ).toEqual(template);
    expect(
      schema.parse({
        acceptance_criteria: {
          v: 1,
          items: [],
          template: { ...template, revision: "bad" },
        },
      }).acceptance_criteria,
    ).toBeUndefined();
  });
  it("preserves a task's team snapshot and rejects incomplete team data", () => {
    const team = {
      project_id: "11111111-1111-4111-8111-111111111111",
      revision: 2,
      config: {
        v: 1,
        roles: Object.fromEntries(
          projectRoleOrder.map((role) => [role, { enabled: role !== "sec" }]),
        ),
      },
    };
    const criteria = { v: 1, items: [], team };
    expect(
      schema.parse({ acceptance_criteria: criteria }).acceptance_criteria?.team
        ?.config.roles.sec.enabled,
    ).toBe(false);
    expect(
      schema.parse({
        acceptance_criteria: {
          ...criteria,
          team: { ...team, revision: "bad" },
        },
      }).acceptance_criteria,
    ).toBeUndefined();
  });
  it.each([null, []])("accepts a legacy empty field %j", (value) => {
    const result = parseWithFallback(
      { acceptance_criteria: value },
      schema,
      {},
      { endpoint: "GET /api/issues/:id" },
    );
    expect(result).toMatchObject({
      acceptance_criteria: { v: 1, items: [], gate: { spec: null } },
    });
  });
  it.each([
    { v: 2 },
    { v: 1, items: "bad" },
    { v: 1, items: [], gate: { spec: true } },
  ])("does not expose malformed data as an editable empty draft", (value) => {
    expect(
      schema.parse({ acceptance_criteria: value }).acceptance_criteria,
    ).toBeUndefined();
  });
  it("keeps confirmed gates when optional extensions are absent", () => {
    const spec = { passed_at: "2026-11-06T00:00:00Z", passed_by: "user-1" };
    expect(
      schema.parse({
        acceptance_criteria: { v: 1, items: [], gate: { spec, accept: null } },
      }).acceptance_criteria?.gate.spec,
    ).toEqual(spec);
  });
  it("leaves the feature unavailable on a server without the field", () => {
    expect(schema.parse({}).acceptance_criteria).toBeUndefined();
  });
});
