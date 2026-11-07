import type { IssueTableQuerySpec, WorkbenchBandFilter } from "../types";

export function workbenchDayRange(now: Date) {
  const start = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const end = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1);
  return { start: start.toISOString(), end: end.toISOString() };
}

export function workbenchBandSelection(params: URLSearchParams, defaultMine: boolean, now: Date): WorkbenchBandFilter {
  const selected = params.get("wb_band");
  const band = selected === "running" || selected === "review" || selected === "today" ? selected : "all";
  return { band, only_mine: params.has("wb_mine") ? params.get("wb_mine") === "1" : defaultMine, ...workbenchDayRange(now) };
}

export function workbenchBandQuery(query: IssueTableQuerySpec, selection?: WorkbenchBandFilter): IssueTableQuerySpec {
  return selection ? { ...query, filters: { ...query.filters, workbench: selection } } : query;
}
