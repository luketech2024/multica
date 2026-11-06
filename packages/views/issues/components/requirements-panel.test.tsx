// @vitest-environment jsdom
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider, useQuery } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Issue, UpdateIssueRequest } from "@multica/core/types";
import { issueKeys } from "@multica/core/issues/queries";
import { renderWithI18n } from "../../test/i18n";
import { RequirementsPanel } from "./requirements-panel";

const updateIssue = vi.fn<(id: string, data: UpdateIssueRequest) => Promise<Issue>>();
vi.mock("@multica/core/api", async (importOriginal) => ({
  ...await importOriginal<typeof import("@multica/core/api")>(),
  api: { updateIssue: (id: string, data: UpdateIssueRequest) => updateIssue(id, data) },
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));
afterEach(() => { cleanup(); vi.clearAllMocks(); });

const initial: Issue = {
  id: "task-1", workspace_id: "ws-1", number: 1, identifier: "SAPT-1", title: "Task",
  description: null, status: "todo", priority: "none", assignee_type: null,
  assignee_id: null, creator_type: "member", creator_id: "user-1", parent_issue_id: null,
  project_id: null, position: 1, stage: null, start_date: null, due_date: null,
  metadata: {}, properties: {}, created_at: "2026-11-06T00:00:00Z", updated_at: "2026-11-06T00:00:00Z", revision: 3,
  acceptance_criteria: { v: 1, version: 1, next_item_no: 2, items: [{ id: "A-01", text: "Export report", origin: "user", covers: [], checked: null, note: null }], bounds: ["No billing"], gate: { spec: null, accept: null } },
};
function mount(issue = initial) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  client.setQueryData(issueKeys.detail("ws-1", issue.id), structuredClone(issue));
  function Detail() {
    const { data } = useQuery<Issue>({ queryKey: issueKeys.detail("ws-1", issue.id), queryFn: async () => issue, enabled: false });
    return data ? <RequirementsPanel issue={data} /> : null;
  }
  renderWithI18n(<QueryClientProvider client={client}><Detail /></QueryClientProvider>);
  return client;
}
describe("requirements panel", () => {
  it("waits for the persisted check and locks only after successful confirmation", async () => {
    updateIssue.mockImplementation(async (_id, data) => ({ ...initial, revision: (data.expected_revision ?? 3) + 1,
      acceptance_criteria: { ...data.acceptance_criteria!, gate: { spec: data.confirm_requirements ? { passed_at: "now", passed_by: "user-1" } : null, accept: null } },
    }));
    mount();
    expect(screen.getByRole("button", { name: "Confirm requirements" })).toBeDisabled();
    fireEvent.click(screen.getByRole("checkbox", { name: "Check A-01" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Confirm requirements" })).toBeEnabled());
    expect(updateIssue.mock.calls[0]?.[1].expected_revision).toBe(3);
    fireEvent.click(screen.getByRole("button", { name: "Confirm requirements" }));
    await screen.findByText("Requirements confirmed and locked");
    expect(screen.getByRole("textbox", { name: "Criterion A-01" })).toHaveAttribute("readonly");
    expect(screen.getByRole("checkbox")).toBeDisabled();
  });
  it("preserves the draft and its original revision after a concurrent update is rejected", async () => {
    updateIssue.mockRejectedValue(new Error("Already updated"));
    mount();
    fireEvent.change(screen.getByRole("textbox", { name: "Criterion A-01" }), { target: { value: "Edited report" } });
    fireEvent.click(screen.getByRole("button", { name: "Save changes" }));
    await screen.findByRole("alert");
    expect(screen.getByRole("textbox", { name: "Criterion A-01" })).toHaveValue("Edited report");
    expect(updateIssue.mock.calls[0]?.[1]).toMatchObject({ expected_revision: 3, acceptance_criteria: { items: [{ id: "A-01", text: "Edited report", checked: null }] } });
  });
  it("does not enable editing when criteria could not be validated", () => {
    mount({ ...initial, acceptance_criteria: undefined });
    expect(screen.queryByRole("button", { name: "Add criterion" })).not.toBeInTheDocument();
  });
});
