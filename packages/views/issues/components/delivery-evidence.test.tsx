import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, screen, within, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { Issue } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { DeliveryEvidence } from "./delivery-evidence";

const save = vi.fn();
const children = vi.fn();
const read = vi.fn();
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace" }));
vi.mock("@multica/core/permissions", () => ({ useCurrentMember: () => ({ role: "member" }) }));
vi.mock("@multica/core/workspace/hooks", () => ({ useActorName: () => ({ getActorName: () => "Reviewer" }) }));
vi.mock("@multica/core/api", () => ({ api: {
  listChildIssues: (...args: unknown[]) => children(...args),
  saveDeliveryEvidence: (...args: unknown[]) => save(...args),
  getDeliveryReview: (...args: unknown[]) => read(...args),
} }));
afterEach(() => { cleanup(); vi.resetAllMocks(); });
const child = { id: "child", revision: 5, identifier: "WB-2", title: "Verify export", status: "done", workbench_handoff: { role: "test", expected: ["test report"], requirement_version: 1, superseded: false, repo_id: "" } };
function mount(delivery = {}, blockers: { code: string; issue_id: string }[] = []) {
  read.mockResolvedValue({ revision: 7, blockers, checks: [] });
  children.mockResolvedValue({ issues: [child] });
  const issue = { id: "parent", revision: 7, status: "in_review", acceptance_criteria: { version: 1, plan: { version: 1, issues: ["child"] }, delivery } } as unknown as Issue;
  renderWithI18n(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><DeliveryEvidence issue={issue} /></QueryClientProvider>);
}
it("retains entered evidence on conflict and sends both revisions", async () => {
  save.mockRejectedValueOnce(new Error("Task was updated"));
  mount();
  fireEvent.click(await screen.findByRole("button", { name: "Record evidence" }));
  const dialog = screen.getByRole("dialog");
  fireEvent.change(within(dialog).getByRole("textbox", { name: "Conclusion" }), { target: { value: "Export verified" } });
  fireEvent.change(within(dialog).getByRole("textbox", { name: "test report" }), { target: { value: "https://example.test/report" } });
  fireEvent.click(within(dialog).getByRole("checkbox"));
  fireEvent.submit(within(dialog).getByRole("button", { name: "Save evidence" }).closest("form")!);
  expect(await screen.findByRole("alert")).toHaveTextContent("Task was updated");
  expect(within(dialog).getByRole("textbox", { name: "Conclusion" })).toHaveValue("Export verified");
  expect(save).toHaveBeenCalledWith("parent", 7, expect.objectContaining({ issue_id: "child", issue_revision: 5, passed: true, summary: "Export verified" }));
  save.mockResolvedValueOnce({});
  fireEvent.submit(within(dialog).getByRole("button", { name: "Save evidence" }).closest("form")!);
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
});
it("marks reports from an earlier child revision as out of date", async () => {
  mount({ child: { issue_revision: 4, version: 1, passed: true, summary: "Old result", recorded_by: "member", recorded_at: "yesterday", reports: [] } }, [{ code: "evidence_stale", issue_id: "child" }]);
  const result = await screen.findByText("Delivery result needs to be rechecked");
  expect(result).toBeVisible();
  expect(result).not.toHaveClass("text-destructive");
  expect(screen.queryByText("Reported as passed")).not.toBeInTheDocument();
});
it("uses server verification when only the conversation revision changed", async () => {
  mount({ child: { issue_revision: 4, version: 1, passed: true, summary: "Verified result", recorded_by: "member", recorded_at: "yesterday", reports: [] } });
  expect(await screen.findByText("Reported as passed")).toBeVisible();
  expect(screen.queryByText("Delivery result needs to be rechecked")).not.toBeInTheDocument();
});
