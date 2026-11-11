// @vitest-environment jsdom
import { cleanup, fireEvent, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { renderWithI18n } from "../../test/i18n";
import { WorkbenchStatusBand } from "./workbench-status-band";

vi.mock("@tanstack/react-query", async (importOriginal) => ({
  ...await importOriginal<typeof import("@tanstack/react-query")>(),
  useQueries: () => [0, 0, 0, 2].map((total) => ({ data: { total }, isError: false })),
}));
afterEach(cleanup);

it("shows the view total when personal and status filters hide tasks and clears both", () => {
  const onSelect = vi.fn();
  renderWithI18n(<WorkbenchStatusBand
    workspaceId="ws"
    query={{ scope: { kind: "workspace" }, filters: {}, sort: { field: "updated_at", direction: "desc" } }}
    selection={{ band: "running", only_mine: true, start: "2026-11-08T00:00:00Z", end: "2026-11-09T00:00:00Z" }}
    onSelect={onSelect}
    enabled
  />);
  expect(screen.getByText("All tasks in this view: 2")).toBeInTheDocument();
  expect(screen.getByText("Assigned to me excludes tasks assigned to agents.")).toBeInTheDocument();
  expect(screen.getByText("Status filter: Running")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "All" }));
  expect(onSelect).toHaveBeenCalledWith("all", false);
});
