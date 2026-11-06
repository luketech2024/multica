import type { ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { Issue } from "@multica/core/types";
import { projectRoleOrder } from "@multica/core/workbench/configuration";
import { renderWithI18n } from "../../test/i18n";
import { WorkbenchPipeline } from "./workbench-pipeline";

const listChildren = vi.fn();
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace" }));
vi.mock("@multica/core/paths", () => ({ useWorkspacePaths: () => ({ issueDetail: (id: string) => `/workspace/issues/${id}`, issuePipeline: (id: string) => `/workspace/issues/${id}/pipeline` }) }));
vi.mock("../../navigation", () => ({ AppLink: ({ children, href }: { children: ReactNode; href: string }) => <a href={href}>{children}</a> }));
vi.mock("@multica/core/api", () => ({ api: { listChildIssues: (...args: unknown[]) => listChildren(...args) } }));
afterEach(() => { cleanup(); vi.resetAllMocks(); });
function mount(plan = true) {
  const issue = {
    id: "parent", parent_issue_id: null,
    acceptance_criteria: {
      version: 1, gate: { spec: { passed_at: "now", passed_by: "member" }, accept: null },
      team: { config: { roles: Object.fromEntries(projectRoleOrder.map((role) => [role, { enabled: role !== "sec" }])) } },
      plan: plan ? { version: 1, issues: ["child"] } : undefined,
    },
  } as Issue;
  renderWithI18n(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
    <WorkbenchPipeline issue={issue} />
  </QueryClientProvider>);
}
it("shows loading and retry before linking the real planned child", async () => {
  let reject: (error: Error) => void = () => {};
  listChildren.mockImplementationOnce(() => new Promise((_, fail) => { reject = fail; }));
  mount();
  expect(screen.getByText("Loading task progress…")).toBeVisible();
  reject(new Error("offline"));
  await screen.findByText("Task progress could not be loaded.");
  listChildren.mockResolvedValue({ issues: [{ id: "child", identifier: "WB-2", title: "Export report", status: "in_progress", workbench_handoff: { role: "dev", requirement_version: 1, superseded: false } }] });
  fireEvent.click(screen.getByRole("button"));
  expect(await screen.findByRole("link", { name: "WB-2 · Export report" })).toHaveAttribute("href", "/workspace/issues/child");
  expect(screen.getByText("In progress")).toBeVisible();
  expect(screen.queryByText("Security")).not.toBeInTheDocument();
});
it("shows the unplanned state without fetching invented children", () => {
  mount(false);
  expect(screen.getByText("A confirmed execution plan is needed for this requirement version.")).toBeVisible();
  expect(listChildren).not.toHaveBeenCalled();
});
