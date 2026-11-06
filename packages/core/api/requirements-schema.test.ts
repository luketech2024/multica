// @vitest-environment node
import { describe, expect, it } from "vitest";
import { IssueSchema } from "./schemas";
import { parseWithFallback } from "./schema";

const schema = IssueSchema.pick({ acceptance_criteria: true });
describe("requirements response compatibility", () => {
  it.each([null, []])("accepts a legacy empty field %j", (value) => {
    const result = parseWithFallback({ acceptance_criteria: value }, schema, {}, { endpoint: "GET /api/issues/:id" });
    expect(result).toMatchObject({ acceptance_criteria: { v: 1, items: [], gate: { spec: null } } });
  });
  it.each([{ v: 2 }, { v: 1, items: "bad" }, { v: 1, items: [], gate: { spec: true } }])("does not expose malformed data as an editable empty draft", (value) => {
    expect(schema.parse({ acceptance_criteria: value }).acceptance_criteria).toBeUndefined();
  });
  it("keeps confirmed gates when optional extensions are absent", () => {
    const spec = { passed_at: "2026-11-06T00:00:00Z", passed_by: "user-1" };
    expect(schema.parse({ acceptance_criteria: { v: 1, items: [], gate: { spec, accept: null } } }).acceptance_criteria?.gate.spec).toEqual(spec);
  });
  it("leaves the feature unavailable on a server without the field", () => {
    expect(schema.parse({}).acceptance_criteria).toBeUndefined();
  });
});
