import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { Issue } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { RequirementsChangeControls } from "./requirements-change-controls";

const change = vi.fn();
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace" }));
vi.mock("@multica/core/permissions", () => ({
  useCurrentMember: () => ({ role: "owner" }),
}));
vi.mock("@multica/core/api", () => ({
  api: { changeIssueRequirements: (...args: unknown[]) => change(...args) },
}));
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});
const issue: Issue = {
  id: "task",
  workspace_id: "workspace", number: 1, identifier: "WB-1", title: "Task",
  description: null, priority: "none", assignee_type: null, assignee_id: null,
  creator_type: "member", creator_id: "member", parent_issue_id: null, project_id: null,
  position: 1, stage: null, start_date: null, due_date: null, metadata: {}, properties: {},
  created_at: "2026-11-06T00:00:00Z", updated_at: "2026-11-06T00:00:00Z",
  status: "in_progress",
  revision: 7,
  acceptance_criteria: {
    v: 1,
    version: 1,
    next_item_no: 2,
    items: [],
    bounds: [],
    gate: { spec: { passed_at: "before", passed_by: "member" }, accept: null },
  },
};
function mount(value: Issue) {
  const client = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  });
  renderWithI18n(
    <QueryClientProvider client={client}>
      <RequirementsChangeControls issue={value} />
    </QueryClientProvider>,
  );
}
it("requires a reason and preserves it when the revision is rejected", async () => {
  change.mockRejectedValueOnce(new Error("Already updated"));
  mount(issue);
  fireEvent.click(screen.getByRole("button", { name: "Change requirements" }));
  expect(screen.getByRole("button", { name: "Request stop" })).toBeDisabled();
  fireEvent.change(screen.getByRole("textbox", { name: "Reason for change" }), {
    target: { value: "Export format changed" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Request stop" }));
  await screen.findByText("Already updated");
  expect(screen.getByRole("textbox")).toHaveValue("Export format changed");
  await waitFor(() =>
    expect(change).toHaveBeenCalledWith(
      "task",
      "start",
      7,
      "Export format changed",
    ),
  );
});
it("offers stop reconciliation while withholding editing and resumption", () => {
  mount({
    ...issue,
    status: "paused",
    acceptance_criteria: {
      ...issue.acceptance_criteria!,
      change: {
        phase: "stopping",
        reason: "Changed scope",
        requested_at: "now",
        requested_by: "member",
        previous_status: "in_progress",
        run_ids: [],
        baseline: {
          version: 1,
          items: [],
          bounds: [],
          gate: issue.acceptance_criteria!.gate,
        },
      },
    },
  });
  expect(screen.getByRole("status")).toHaveTextContent("Stopping");
  expect(
    screen.getByRole("button", { name: "Check stop acknowledgements" }),
  ).toBeEnabled();
  expect(
    screen.queryByRole("button", { name: "Update requirements" }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Cancel change" }),
  ).not.toBeInTheDocument();
});
