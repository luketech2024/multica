// @vitest-environment node
import { expect, it } from "vitest";
import { previewPath, previewURL, validPreviewURL } from "./preview";

it("builds the agreed path from the configured origin and stable task identity", () => {
  expect(previewURL("preview.example.test", "project-1", "WB-2")).toBe(
    "https://preview.example.test/preview/project-1/WB-2/",
  );
  expect(previewURL("http://localhost:8000", "project-1", "WB-2")).toBe(
    "http://localhost:8000/preview/project-1/WB-2/",
  );
  expect(previewPath("project/a", "WB 2")).toBe("/preview/project%2Fa/WB%202/");
});
it.each([
  "",
  "https://name:secret@example.test",
  "https://example.test/path",
  "https://example.test?token=secret",
  "file:///tmp/example",
  "https://example.test/#path",
])("does not generate an address from an invalid origin %s", (origin) => {
  expect(previewURL(origin, "project", "WB-1")).toBeNull();
});
it("only opens HTTP previews without embedded credentials", () => {
  expect(validPreviewURL("https://example.test/preview/p/WB-1/")).toBe(true);
  expect(validPreviewURL("javascript:alert(1)")).toBe(false);
  expect(validPreviewURL("https://name:secret@example.test/preview/")).toBe(
    false,
  );
});
