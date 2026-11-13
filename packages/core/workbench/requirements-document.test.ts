// @vitest-environment node
import { expect, it } from "vitest";
import { requirementSection, requirementChanges } from "./requirements-document";
import { RequirementsDocumentSchema } from "./onespec-schemas";
it("reads the named section including its subsections", () => {
  const content = "# PRD\n## Scope\nHomepage\n### Mobile\nResponsive\n## Acceptance\nVisible";
  expect(requirementSection(content, "Scope")).toBe("## Scope\nHomepage\n### Mobile\nResponsive");
  expect(requirementSection(content, "Unknown")).toBeUndefined();
});
it("requires readable document content and defaults missing verification to false", () => {
  const ref = { repository_id: "repo", path: "onespec/prd.md", commit: "commit", digest: "hash" };
  expect(RequirementsDocumentSchema.parse({ ref, content: "# PRD" }).verified).toBe(false);
  expect(RequirementsDocumentSchema.safeParse({ ref }).success).toBe(false);
});
it("shows the changed range without repeating surrounding sections", () => {
  expect(requirementChanges("# Scope\nOld\n# End", "# Scope\nNew\n# End")).toEqual({ before: "Old", after: "New" });
  expect(requirementChanges("# Scope\n# End", "# Scope\nNew\n# End")).toEqual({ before: "", after: "New" });
  expect(requirementChanges("same", "same")).toBeNull();
});
