// @vitest-environment node
import { expect, it } from "vitest";
import {
  analyzeExecutionPlan,
  parseWorkbenchHandoff,
  type ExecutionPlanTask,
} from "./execution-plan";
import { IssueSchema } from "../api/schemas";

const handoff = {
  v: 2,
  requirement_version: 3,
  role: "dev",
  repo_id: "11111111-1111-4111-8111-111111111111",
  covers: ["A-01"],
  depends_on: [],
  serial_group: "api",
  max_retry: 2,
  expected: ["diff"],
};

it("requires complete coverage and available development repositories before saving", () => {
  const task: ExecutionPlanTask = {
    key: "T-01",
    title: "Export",
    description: "",
    role: "dev",
    repo_id: "api",
    covers: ["A-01"],
    depends_on: [],
    serial_group: "",
    expected: ["diff"],
  };
  expect(analyzeExecutionPlan([task], ["A-01", "A-02"], ["api"])).toMatchObject(
    {
      coveredCount: 1,
      uncovered: ["A-02"],
      canSave: false,
    },
  );
  expect(
    analyzeExecutionPlan([task], ["A-01"], []).missingRepositories,
  ).toEqual(["T-01"]);
  expect(analyzeExecutionPlan([task], ["A-01"], ["api"]).canSave).toBe(true);
  expect(
    analyzeExecutionPlan(
      [{ ...task, role: "product", repo_id: "" }],
      ["A-01"],
      [],
    ).canSave,
  ).toBe(true);
  expect(
    analyzeExecutionPlan([{ ...task, title: " " }], ["A-01"], ["api"]).canSave,
  ).toBe(false);
});

it("reports the cyclic segment and allows shared acyclic prerequisites", () => {
  const makeTask = (key: string, depends_on: string[]): ExecutionPlanTask => ({
    key,
    depends_on,
    title: key,
    description: "",
    role: "product",
    repo_id: "",
    covers: ["A-01"],
    serial_group: "",
    expected: ["spec"],
  });
  const tasks = [
    makeTask("T-01", ["T-02"]),
    makeTask("T-02", ["T-03"]),
    makeTask("T-03", ["T-02"]),
  ];
  expect(analyzeExecutionPlan(tasks, ["A-01"], [])).toMatchObject({
    cycle: ["T-02", "T-03", "T-02"],
    canSave: false,
  });
  tasks[2]!.depends_on = [];
  tasks[0]!.depends_on.push("T-03");
  expect(analyzeExecutionPlan(tasks, ["A-01"], [])).toMatchObject({
    cycle: [],
    canSave: true,
  });
});
it("retains versioned handoffs and safe defaults", () => {
  expect(parseWorkbenchHandoff(handoff)).toEqual({
    ...handoff,
    superseded: false,
  });
  expect(
    parseWorkbenchHandoff({ ...handoff, superseded: true })?.superseded,
  ).toBe(true);
});

it("keeps the typed API projection separate from scalar custom fields", () => {
  const schema = IssueSchema.pick({
    properties: true,
    workbench_handoff: true,
  });
  const result = schema.parse({
    properties: { text: "value", workbench_handoff: handoff },
    workbench_handoff: handoff,
  });
  expect(result.properties).toEqual({ text: "value" });
  expect(result.workbench_handoff?.covers).toEqual(["A-01"]);
  expect(
    schema.parse({ workbench_handoff: { v: 99 } }).workbench_handoff,
  ).toBeNull();
  expect(schema.parse({}).workbench_handoff).toBeUndefined();
});
it.each([
  { ...handoff, v: 1 },
  { ...handoff, role: "unknown" },
  { ...handoff, covers: [] },
  { ...handoff, depends_on: ["broken"] },
  { ...handoff, repo_id: "foreign" },
  { ...handoff, requirement_version: 0 },
  { superseded: true },
  null,
])("does not expose invalid handoff data as executable", (value) => {
  expect(parseWorkbenchHandoff(value)).toBeNull();
});
