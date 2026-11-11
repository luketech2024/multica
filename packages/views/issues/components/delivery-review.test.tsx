import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { Issue } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { DeliveryReview } from "./delivery-review";

const read = vi.fn();
const review = vi.fn();
const success = vi.fn();
vi.mock("sonner", () => ({ toast: { success: (...args: unknown[]) => success(...args), error: vi.fn() } }));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace" }));
vi.mock("@multica/core/permissions", () => ({ useCurrentMember: () => ({ role: "member", userId: "member" }) }));
vi.mock("@multica/core/workspace/hooks", () => ({ useActorName: () => ({ getActorName: () => "Reviewer" }) }));
vi.mock("@multica/core/api", () => ({ api: {
  getDeliveryReview: (...args: unknown[]) => read(...args),
  reviewDelivery: (...args: unknown[]) => review(...args),
} }));
afterEach(() => { cleanup(); vi.resetAllMocks(); });

function mount(assignee = "member", overrides: Partial<Issue> = {}) {
  const issue = {
    id: "parent", project_id: "project", revision: 7, status: "in_review", assignee_type: "member", assignee_id: assignee,
    acceptance_criteria: { plan: { issues: [] }, gate: { accept: null } },
    ...overrides,
  } as unknown as Issue;
  renderWithI18n(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
    <DeliveryReview issue={issue} />
  </QueryClientProvider>);
}

it("requires project checks before accepting and offers revision-safe undo", async () => {
  read.mockResolvedValue({ revision: 7, blockers: [], checks: [{ id: "dod-0", text: "Regression verified", check: "" }] });
  review.mockResolvedValue({ revision: 8 });
  mount();
  const accept = await screen.findByRole("button", { name: "Accept delivery" });
  expect(accept).toBeDisabled();
  fireEvent.click(screen.getByRole("checkbox", { name: "Regression verified" }));
  fireEvent.click(accept);
  await waitFor(() => expect(review).toHaveBeenCalledWith("parent", {
    expected_revision: 7, action: "accept", reason: undefined, checks: ["dod-0"],
  }));
  await waitFor(() => expect(success).toHaveBeenCalled());
  const options = success.mock.calls[0]![1] as { duration: number; action: { onClick: () => void } };
  expect(options.duration).toBe(5000);
  options.action.onClick();
  await waitFor(() => expect(review).toHaveBeenLastCalledWith("parent", expect.objectContaining({ action: "undo", expected_revision: 8 })));
});

it("keeps acceptance visible for legacy completed tasks without a plan", async () => {
  read.mockResolvedValue({ revision: 7, blockers: [{ code: "plan" }], checks: [] });
  mount("member", { status: "done", acceptance_criteria: undefined });
  expect(screen.getByRole("button", { name: "Complete acceptance data" })).toBeVisible();
  await waitFor(() => expect(read).toHaveBeenCalledWith("parent"));
  await waitFor(() => expect(screen.queryByRole("status")).not.toBeInTheDocument());
});

it("lets a member claim a review assigned to an agent", async () => {
  read.mockResolvedValue({ revision: 7, blockers: [], checks: [] });
  review.mockResolvedValue({ revision: 8 });
  mount("agent", { assignee_type: "agent" });
  fireEvent.click(await screen.findByRole("button", { name: "Take over acceptance" }));
  await waitFor(() => expect(review).toHaveBeenCalledWith("parent", expect.objectContaining({ action: "claim", expected_revision: 7 })));
});

it("shows named missing evidence and retains a required rejection reason on conflict", async () => {
  read.mockResolvedValue({ revision: 7, blockers: [{ code: "evidence_missing", title: "Export tests" }], checks: [] });
  review.mockRejectedValue(new Error("Task was updated"));
  mount();
  expect(await screen.findByRole("button", { name: "Accept delivery" })).toBeDisabled();
  expect(screen.getByText("Export tests")).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "Request changes" }));
  const send = screen.getByRole("button", { name: "Send back for changes" });
  expect(send).toBeDisabled();
  fireEvent.change(screen.getByRole("textbox", { name: "Reason for changes" }), { target: { value: "Verify empty exports" } });
  fireEvent.click(send);
  expect(await screen.findByRole("alert")).toHaveTextContent("Task was updated");
  expect(screen.getByRole("textbox", { name: "Reason for changes" })).toHaveValue("Verify empty exports");
});

it("keeps review actions with the assigned member", async () => {
  read.mockResolvedValue({ revision: 7, blockers: [], checks: [] });
  mount("other");
  expect(await screen.findByText("Waiting for the assigned member to review.")).toBeVisible();
  expect(screen.queryByRole("button", { name: "Accept delivery" })).not.toBeInTheDocument();
});
