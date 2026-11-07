// @vitest-environment node
import { expect, it } from "vitest";
import { workbenchBandQuery, workbenchBandSelection, workbenchDayRange } from "./status-band";
import type { IssueTableQuerySpec } from "../types";

it("preserves explicit all-members selection and validates URL bands", () => {
  const now = new Date(2026, 10, 7, 16);
  expect(workbenchBandSelection(new URLSearchParams(), true, now).only_mine).toBe(true);
  expect(workbenchBandSelection(new URLSearchParams("wb_mine=0&wb_band=review"), true, now)).toMatchObject({ band: "review", only_mine: false });
  expect(workbenchBandSelection(new URLSearchParams("wb_band=unknown"), false, now).band).toBe("all");
});

it("uses local calendar midnights and preserves the existing query", () => {
  const range = workbenchDayRange(new Date(2026, 10, 7, 16));
  expect(range.start).toBe(new Date(2026, 10, 7).toISOString());
  expect(range.end).toBe(new Date(2026, 10, 8).toISOString());
  const query: IssueTableQuerySpec = { scope: { kind: "workspace" }, filters: { priorities: ["high"] }, sort: { field: "created_at", direction: "desc" } };
  const selection = { ...range, band: "today" as const, only_mine: true };
  expect(workbenchBandQuery(query, selection).filters).toEqual({ priorities: ["high"], workbench: selection });
  expect(query.filters.workbench).toBeUndefined();
  expect(workbenchBandQuery(query)).toBe(query);
});
