// @vitest-environment node
import { expect, it } from "vitest";
import { mergeBusinessTemplateFields } from "./template-merge";
import { BusinessTemplateAssetSchema, parseWorkbenchResponse } from "./schemas";

const current = { v: 1 as const, name: "Project rules", background: [], audience: [], scope_out: ["Local scope"], ac_skeleton: [], forbidden: [], dod: ["Local check"], resources: [], terms: {} };
it("replaces only selected fields and preserves both inputs", () => {
  const incoming = { ...current, name: "Shared rules", scope_out: ["Shared scope"], dod: ["New check"] };
  const merged = mergeBusinessTemplateFields(current, incoming, ["dod"]);
  expect(merged).toEqual({ ...current, dod: ["New check"] });
  expect(current.dod).toEqual(["Local check"]);
  expect(mergeBusinessTemplateFields(current, incoming, [])).toEqual(current);
});
it("defaults old catalog entries to built-in and rejects malformed publication responses", () => {
  const asset = { id: "blank", revision: 1, content: current, disabled_roles: [] };
  expect(BusinessTemplateAssetSchema.parse(asset).builtin).toBe(true);
  expect(BusinessTemplateAssetSchema.parse({ ...asset, builtin: false }).builtin).toBe(false);
  expect(() => parseWorkbenchResponse({ ...asset, revision: "2" }, BusinessTemplateAssetSchema, "business-template")).toThrow();
  expect(BusinessTemplateAssetSchema.safeParse({ ...asset, builtin: "false" }).success).toBe(false);
});
