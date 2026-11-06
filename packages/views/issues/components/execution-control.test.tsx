import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { Issue } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { ExecutionControl } from "./execution-control";

const control = vi.fn();
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace" }));
vi.mock("@multica/core/permissions", () => ({
  useCurrentMember: () => ({ role: "owner" }),
}));
vi.mock("@multica/core/api", () => ({
  api: { controlIssueExecution: (...args: unknown[]) => control(...args) },
}));
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});
const issue: Issue = {
  id: "task",
  workspace_id: "workspace",
  number: 1,
  identifier: "WB-1",
  title: "Task",
  description: null,
  priority: "none",
  assignee_type: null,
  assignee_id: null,
  creator_type: "member",
  creator_id: "member",
  parent_issue_id: null,
  project_id: null,
  position: 1,
  stage: null,
  start_date: null,
  due_date: null,
  metadata: {},
  properties: {},
  created_at: "2026-11-07T00:00:00Z",
  updated_at: "2026-11-07T00:00:00Z",
  status: "in_progress",
  revision: 7,
  acceptance_criteria: {
    v: 1,
    version: 1,
    next_item_no: 2,
    items: [],
    bounds: [],
    gate: { spec: null, accept: null },
  },
};
function mount(value: Issue) {
  renderWithI18n(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { mutations: { retry: false } } })
      }
    >
      <ExecutionControl issue={value} />
    </QueryClientProvider>,
  );
}
it("sends the revision and keeps the pause action available after failure", async () => {
  control.mockRejectedValueOnce(new Error("Already updated"));
  mount(issue);
  fireEvent.click(screen.getByRole("button", { name: "Pause execution" }));
  await screen.findByText("Already updated");
  expect(control).toHaveBeenCalledWith("task", "pause", 7);
  expect(screen.getByRole("button", { name: "Pause execution" })).toBeEnabled();
});
it("withholds resumption until stopping is acknowledged", async () => {
  control.mockResolvedValueOnce(issue);
  mount({
    ...issue,
    status: "paused",
    workbench_pause: {
      v: 1,
      phase: "unknown",
      previous_status: "in_progress",
      requested_at: "now",
      requested_by: "member",
      run_ids: [],
      pending_runs: [],
      resume_error: "",
    },
  });
  expect(screen.getByRole("status")).toHaveTextContent("Stop result unknown");
  expect(
    screen.queryByRole("button", { name: "Resume execution" }),
  ).not.toBeInTheDocument();
  fireEvent.click(
    screen.getByRole("button", { name: "Check stop acknowledgements" }),
  );
  await waitFor(() =>
    expect(control).toHaveBeenCalledWith("task", "reconcile", 7),
  );
});
