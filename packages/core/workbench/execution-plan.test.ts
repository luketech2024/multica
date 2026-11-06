// @vitest-environment node
import { expect, it } from "vitest";
import { parseWorkbenchHandoff } from "./execution-plan";
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
