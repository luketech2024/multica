import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { Issue } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { WorkbenchHandoffPanel } from "./workbench-handoff-panel";

const repoId = "11111111-1111-4111-8111-111111111111";
const dependencyId = "22222222-2222-4222-8222-222222222222";
const parent = {
  acceptance_criteria: {
    plan: { version: 1 },
    items: [{ id: "A-01", text: "Export report" }],
    needs_resplit: false,
  },
};
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace" }));
vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    issueDetail: (id: string) => `/workspace/issues/${id}`,
  }),
}));
vi.mock("../../navigation", () => ({
  AppLink: ({
    children,
    href,
  }: {
    children: React.ReactNode;
    href: string;
  }) => <a href={href}>{children}</a>,
}));
vi.mock("@multica/core/api", () => ({
  api: {
    listProjectResources: async () => ({
      resources: [
        { id: repoId, label: "API" },
        { id: "other", label: "Other repository" },
      ],
    }),
    getIssue: async () => parent,
    listChildIssues: async () => ({
      issues: [
        {
          id: dependencyId,
          identifier: "WB-2",
          title: "Prepare export",
          status: "todo",
        },
      ],
    }),
  },
}));
const issue: Issue = {
  id: "task",
  workspace_id: "workspace",
  number: 3,
  identifier: "WB-3",
  title: "Task",
  description: null,
  status: "todo",
  priority: "none",
  assignee_type: null,
  assignee_id: null,
  creator_type: "member",
  creator_id: "member",
  parent_issue_id: "parent",
  project_id: "project",
  position: 1,
  stage: null,
  start_date: null,
  due_date: null,
  metadata: {},
  created_at: "2026-11-06T00:00:00Z",
  updated_at: "2026-11-06T00:00:00Z",
  properties: {},
  workbench_handoff: {
    v: 2,
    requirement_version: 1,
    role: "dev",
    repo_id: repoId,
    covers: ["A-01"],
    depends_on: [dependencyId],
    serial_group: "api",
    max_retry: 2,
    expected: ["diff"],
    superseded: false,
  },
};
afterEach(() => {
  cleanup();
  parent.acceptance_criteria.needs_resplit = false;
});
function mount() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  renderWithI18n(
    <QueryClientProvider client={client}>
      <WorkbenchHandoffPanel issue={issue} />
    </QueryClientProvider>,
  );
}
it("shows the selected repository, coverage and actionable dependency link", async () => {
  mount();
  await screen.findByText("API");
  expect(screen.queryByText("Other repository")).not.toBeInTheDocument();
  expect(await screen.findByText("A-01 · Export report")).toBeInTheDocument();
  expect(
    await screen.findByRole("link", { name: "WB-2 · Prepare export" }),
  ).toHaveAttribute("href", `/workspace/issues/${dependencyId}`);
  expect(
    await screen.findByText("Waiting for 1 dependencies"),
  ).toBeInTheDocument();
});
it("does not present updated requirement text as the old execution baseline", async () => {
  parent.acceptance_criteria.needs_resplit = true;
  mount();
  await screen.findByText("API");
  expect(screen.queryByText("A-01 · Export report")).not.toBeInTheDocument();
  expect(screen.getByText("A-01")).toBeInTheDocument();
});
