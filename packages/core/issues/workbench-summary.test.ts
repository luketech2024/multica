// @vitest-environment node
import { describe, expect, it } from "vitest";
import { parseWorkbenchSummary } from "./workbench-summary";

describe("workbench summary markers", () => {
  it.each(["", "结论：finished", "<!-- wb-summary v2 -->结论：done<!-- /wb-summary -->", "<!-- wb-summary v1 -->结论：done", "<!-- wb-summary v1 --><!-- wb-summary v1 -->结论：done<!-- /wb-summary -->", "<!-- wb-summary v1 -->free text<!-- /wb-summary -->"])("preserves the full timeline fallback for %s", (text) => {
    expect(parseWorkbenchSummary(text)).toBeNull();
  });
  it.each(["v1", ""])("reads supported %s blocks and missing sections", (version) => {
    expect(parseWorkbenchSummary(`prefix\n<!-- wb-summary ${version} -->\n结论：Done\n验证：\n- Tests passed\n<!-- /wb-summary -->suffix`)).toEqual({ conclusion: "Done", changes: [], validation: ["Tests passed"], risks: [] });
  });
  it("uses only the first paired block in one message", () => {
    expect(parseWorkbenchSummary("<!-- wb-summary v1 -->结论：first<!-- /wb-summary --><!-- wb-summary v1 -->结论：second<!-- /wb-summary -->")?.conclusion).toBe("first");
  });
  it("limits presentation without changing the full source", () => {
    const text = `<!-- wb-summary v1 -->\n结论：${"字".repeat(100)}\n改动：\n${Array.from({ length: 7 }, () => "- " + "项".repeat(120)).join("\n")}\n<!-- /wb-summary -->`;
    const parsed = parseWorkbenchSummary(text)!;
    expect(parsed.conclusion).toHaveLength(80);
    expect(parsed.changes).toHaveLength(5);
    expect(parsed.changes[0]).toHaveLength(100);
    expect(text).toContain("字".repeat(100));
  });
});
