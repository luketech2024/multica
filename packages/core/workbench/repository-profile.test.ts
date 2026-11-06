// @vitest-environment node
import { expect, it } from "vitest";
import { RepositoryProfileSchema, parseWorkbenchResponse } from "./schemas";

const profile = {
  revision: 1,
  stack: {
    v: 3,
    template: "python-service",
    tags: ["python312"],
    detected: null,
    baseline: null,
    derived: {
      install: "uv sync",
      test: "uv run pytest -q",
      lint: ["uv run ruff check ."],
      scan: "uv run pip-audit",
    },
    overrides: { test: { value: "./verify.sh", by: "manual" } },
    runtime: "server",
    requires: ["python312"],
    checks: [],
  },
  delivery: ["service-process"],
};
it("retains manual overrides and unverified repository evidence", () => {
  expect(
    parseWorkbenchResponse(
      profile,
      RepositoryProfileSchema,
      "repository-profile",
    ),
  ).toEqual(profile);
});
it.each([
  { ...profile, revision: -1 },
  { ...profile, stack: { ...profile.stack, v: 99 } },
  {
    ...profile,
    stack: { ...profile.stack, overrides: { test: "raw command" } },
  },
])("refuses malformed editable repository profiles", (value) => {
  expect(() =>
    parseWorkbenchResponse(
      value,
      RepositoryProfileSchema,
      "repository-profile",
    ),
  ).toThrow("Invalid workbench configuration response");
});
