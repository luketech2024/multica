import type { ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, screen, within } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { Issue } from "@multica/core/types";
import { projectRoleOrder, ProjectRolesSchema } from "@multica/core/workbench/configuration";
import { renderWithI18n } from "../../test/i18n";
import { IssuePipelinePage } from "./issue-pipeline-page";

const list = vi.fn();
const replace = vi.fn();
const navigation = { replace, searchParams: new URLSearchParams("stage=test&task=second") };
let resolved: { issue?: Issue; canonicalId: string | null; isResolving: boolean; notFound: boolean };
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace" }));
vi.mock("@multica/core/issues/canonical-id", () => ({ useCanonicalIssue: () => resolved }));
vi.mock("@multica/core/paths", () => ({ useWorkspacePaths: () => ({
  issueDetail: (id: string) => `/workspace/issues/${id}`,
  issuePipeline: (id: string) => `/workspace/issues/${id}/pipeline`,
  taskList: () => "/workspace/issues?layout=table",
}) }));
vi.mock("@multica/core/api", () => ({ api: { listChildIssues: (...args: unknown[]) => list(...args) } }));
vi.mock("../../navigation", () => ({
  useNavigation: () => navigation,
  AppLink: ({ href, children, ...props }: { href: string; children: ReactNode }) => <a href={href} {...props}>{children}</a>,
}));
vi.mock("../utils/status-label", () => ({ useStatusLabel: () => (status: string) => status }));
vi.mock("./execution-plan-editor", () => ({ ExecutionPlanEditor: () => <div>Plan editor</div> }));
vi.mock("./workbench-handoff-panel", () => ({ WorkbenchHandoffPanel: ({ issue }: { issue: Issue }) => <div>Handoff {issue.identifier}</div> }));
vi.mock("./delivery-evidence", () => ({ DeliveryEvidence: ({ taskIds }: { taskIds: string[] }) => <div>Reports {taskIds.join(",")}</div> }));
vi.mock("./execution-log-section", () => ({ ExecutionLogSection: () => <div>Execution records</div> }));
vi.mock("./pull-requests-section", () => ({ PullRequestsSection: () => <div>Pull requests</div> }));
afterEach(() => { cleanup(); vi.clearAllMocks(); });

function mount(configure?: (issue: Issue) => void) {
  resolved = {
    canonicalId: "parent", isResolving: false, notFound: false,
    issue: {
      id: "parent", identifier: "WB-1", title: "Export", status: "in_progress",
      workspace_id: "workspace", number: 1, description: null, priority: "none",
      assignee_type: null, assignee_id: null, creator_type: "member", creator_id: "member",
      parent_issue_id: null, project_id: "project", position: 0, stage: null,
      start_date: null, due_date: null, metadata: {}, properties: {},
      created_at: "2026-11-07T00:00:00Z", updated_at: "2026-11-07T00:00:00Z",
      acceptance_criteria: {
        v: 1, version: 1, next_item_no: 1, items: [], bounds: [], gate: { spec: { passed_at: "now", passed_by: "member" }, accept: null },
        plan: { version: 1, key: "plan", digest: "digest", issues: ["first", "second"] },
        team: { project_id: "project", ...ProjectRolesSchema.parse({ revision: 1, config: { v: 1, roles: Object.fromEntries(projectRoleOrder.map((role) => [role, { enabled: role !== "sec" }])) } }) },
      },
    },
  };
  if (configure) configure(resolved.issue!);
  return renderWithI18n(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
    <IssuePipelinePage routeId="parent" />
  </QueryClientProvider>);
}

it("restores the selected stage and task while hiding disabled and retired stages", async () => {
  list.mockResolvedValue({ issues: [
    { id: "first", identifier: "WB-2", title: "First tests", status: "done", workbench_handoff: { role: "test", requirement_version: 1 } },
    { id: "second", identifier: "WB-3", title: "Second tests", status: "in_progress", workbench_handoff: { role: "test", requirement_version: 1 } },
    { id: "old", identifier: "WB-0", title: "Old tests", status: "done", workbench_handoff: { role: "sec", requirement_version: 0, superseded: true } },
  ] });
  mount();
  expect(await screen.findByText("Reports second")).toBeVisible();
  const stages = screen.getByRole("navigation", { name: "Stages" });
  expect(within(stages).queryByRole("button", { name: /Security/ })).not.toBeInTheDocument();
  expect(screen.queryByText("Old tests")).not.toBeInTheDocument();
  expect(screen.getByText("Handoff WB-3")).toBeVisible();
  fireEvent.click(within(stages).getByRole("button", { name: /Development/ }));
  expect(replace).toHaveBeenCalledWith("/workspace/issues/parent/pipeline?stage=dev");
});

it("shows a retryable error when planned children cannot be loaded", async () => {
  list.mockRejectedValue(new Error("offline"));
  mount();
  expect(await screen.findByRole("alert")).toHaveTextContent("Task progress could not be loaded.");
  expect(screen.getByRole("alert")).toHaveTextContent("Execution continues.");
  expect(screen.queryByText("Execution records")).not.toBeInTheDocument();
  list.mockResolvedValue({ issues: [] });
  fireEvent.click(screen.getByRole("button", { name: "Retry" }));
  expect(await screen.findByText("No subtasks yet")).toBeVisible();
});

it("offers planning from an empty pipeline", async () => {
  list.mockResolvedValue({ issues: [] });
  mount();
  expect(await screen.findByText("No subtasks yet")).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "Plan subtasks" }));
  expect(replace).toHaveBeenCalledWith("/workspace/issues/parent/pipeline?stage=sched");
});

it("explains why an unconfirmed task cannot be planned", async () => {
  navigation.searchParams = new URLSearchParams("stage=sched");
  mount((issue) => {
    issue.acceptance_criteria!.gate.spec = null;
    issue.acceptance_criteria!.plan = undefined;
  });
  expect(screen.getByText("No tasks ready to plan")).toBeVisible();
  expect(screen.getByRole("link", { name: "Go to tasks" })).toHaveAttribute("href", "/workspace/issues?layout=table");
  navigation.searchParams = new URLSearchParams("stage=test&task=second");
});
