import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { Issue } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { IssueNextStep } from "./issue-next-step";

const read = vi.fn();
const release = vi.fn();
const run = vi.fn();
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace" }));
vi.mock("@multica/core/workspace/hooks", () => ({ useActorName: () => ({ getActorName: () => "Reviewer" }) }));
vi.mock("@multica/core/paths", () => ({ useWorkspacePaths: () => ({ releases: () => "/test/releases", issueDetail: (id: string) => `/test/issues/${id}` }) }));
vi.mock("@multica/core/agents", () => ({ useAgentPresenceDetail: () => "loading" }));
vi.mock("../../navigation", () => ({ AppLink: ({ href, children }: { href: string; children: React.ReactNode }) => <a href={href}>{children}</a> }));
vi.mock("@multica/core/api", () => ({ api: { getDeliveryReview: () => read(), getReleaseReadiness: () => release(), rerunIssue: (...args: unknown[]) => run(...args) } }));
afterEach(() => { cleanup(); vi.resetAllMocks(); });

function mount(accepted = false) {
  const onNavigate = vi.fn();
  const issue = { id: "parent", project_id: "project", status: "done", revision: 7,
    assignee_type: "member", assignee_id: "person",
    acceptance_criteria: accepted ? { gate: { accept: { passed_by: "person" } } } : undefined,
  } as Issue;
  renderWithI18n(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
    <IssueNextStep issue={issue} statusLabel="Done" onNavigate={onNavigate} />
  </QueryClientProvider>);
  return onNavigate;
}
it("keeps legacy recovery reachable and shows the server's blocker", async () => {
  read.mockResolvedValue({ revision: 7, blockers: [{ code: "plan" }], checks: [] });
  const navigate = mount();
  fireEvent.click(screen.getByRole("button", { name: "Complete acceptance information" }));
  expect(navigate).toHaveBeenCalledWith("review");
  expect(await screen.findByText("1 conditions need attention")).toBeVisible();
  expect(screen.queryByRole("link")).not.toBeInTheDocument();
});
it("offers a retry after a read failure without claiming release readiness", async () => {
  release.mockRejectedValue(new Error("offline"));
  mount(true);
  expect(await screen.findByRole("alert")).toHaveTextContent("Unable to read current conditions.");
  expect(screen.queryByRole("link")).not.toBeInTheDocument();
  release.mockResolvedValue({ revision: 7, blockers: [], merges: [] });
  fireEvent.click(screen.getByRole("button", { name: "Retry" }));
  expect(await screen.findByRole("link")).toHaveAttribute("href", "/test/releases?project=project");
});
it("does not offer a release link for stale readiness data", async () => {
  release.mockResolvedValue({ revision: 6, blockers: [], merges: [] });
  mount(true);
  await screen.findByRole("button", { name: "View release conditions" });
  expect(screen.queryByRole("link")).not.toBeInTheDocument();
});

it("offers a direct entry to the only current planned child", () => {
  const issue = { id: "parent", project_id: "project", status: "todo", revision: 3,
    acceptance_criteria: { version: 1, gate: { spec: { passed_by: "person" } }, plan: { version: 1, issues: ["child"] } },
  } as Issue;
  renderWithI18n(<QueryClientProvider client={new QueryClient()}>
    <IssueNextStep issue={issue} statusLabel="Todo" onNavigate={vi.fn()}
      executionTasks={[{ id: "child", identifier: "TEST-12", title: "Deploy test service" } as Issue]} />
  </QueryClientProvider>);
  expect(screen.getByRole("link", { name: "Open TEST-12" })).toHaveAttribute("href", "/test/issues/child");
  expect(screen.getByText(/1 execution tasks have been planned/)).toHaveTextContent("Deploy test service");
});

it("starts an unstarted assigned execution task and presents a retryable failure", async () => {
  const issue = { id: "child", parent_issue_id: "parent", status: "todo", revision: 1,
    assignee_type: "agent", assignee_id: "agent", workbench_handoff: { superseded: false },
  } as Issue;
  run.mockRejectedValueOnce(new Error("Runtime offline"));
  const qc = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  renderWithI18n(<QueryClientProvider client={qc}>
    <IssueNextStep issue={issue} statusLabel="Todo" onNavigate={vi.fn()} runsLoaded />
  </QueryClientProvider>);
  fireEvent.click(screen.getByRole("button", { name: "Start execution" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("Runtime offline");
  expect(run).toHaveBeenCalledWith("child", undefined);
  expect(screen.getByRole("button", { name: "Start execution" })).toBeEnabled();
});
