// @vitest-environment node
import { expect, it } from "vitest";
import {
  OneSpecConfigurationSchema,
  OneSpecResponseSchema,
  oneSpecStages,
} from "./onespec-schemas";
import { parseWorkbenchResponse } from "./schemas";
it("accepts an older server's optional configuration defaults", () => {
  expect(OneSpecConfigurationSchema.parse({ revision: 0 })).toMatchObject({
    enabled: false,
    runtime_id: null,
    overview_repository_id: null,
  });
});
it("rejects a malformed confirmation response", () => {
  expect(() =>
    parseWorkbenchResponse(
      { flow: { phase: "plan" }, revision: 2 },
      OneSpecResponseSchema,
      "onespec",
    ),
  ).toThrow();
  expect(OneSpecResponseSchema.parse({ flow: null, revision: 0 })).toEqual({
    flow: null,
    revision: 0,
    can_confirm: false,
  });
});
it("separates archive approval and execution, and independent document integration", () => {
  expect(oneSpecStages("feature").slice(-3)).toEqual([
    "archive",
    "archiving",
    "acceptance",
  ]);
  expect(oneSpecStages("documentation")).toEqual([
    "requirements",
    "branch",
    "documentation",
    "testing",
    "merge",
    "merging",
    "acceptance",
  ]);
  expect(oneSpecStages("release")[1]).toBe("release_preparation");
});
