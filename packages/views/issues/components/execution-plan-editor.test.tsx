import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { Issue } from "@multica/core/types";
import {
  projectRoleOrder,
  ProjectRolesSchema,
} from "@multica/core/workbench/configuration";
import { renderWithI18n } from "../../test/i18n";
import { ExecutionPlanEditor } from "./execution-plan-editor";

const save = vi.fn();
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace" }));
vi.mock("@multica/core/permissions", () => ({
  useCurrentMember: () => ({ role: "owner" }),
}));
vi.mock("@multica/core/api", () => ({
  api: {
    listProjectResources: async () => ({
      resources: [{ id: "repo", label: "API", resource_type: "github_repo" }],
    }),
    saveExecutionPlan: (...args: unknown[]) => save(...args),
  },
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
  project_id: "project",
  position: 1,
  stage: null,
  start_date: null,
  due_date: null,
  metadata: {},
  properties: {},
  created_at: "2026-11-06T00:00:00Z",
  updated_at: "2026-11-06T00:00:00Z",
  status: "todo",
  revision: 7,
  acceptance_criteria: {
    v: 1,
    version: 3,
    next_item_no: 2,
    bounds: [],
    items: [
      {
        id: "A-01",
        text: "Export report",
        origin: "user",
        covers: [],
        checked: { at: "now", by: "member" },
        note: null,
      },
    ],
    gate: { spec: { passed_at: "now", passed_by: "member" }, accept: null },
    team: {
      project_id: "project",
      revision: 1,
      config: {
        v: 1,
        roles: ProjectRolesSchema.parse({
          revision: 1,
          config: {
            v: 1,
            roles: Object.fromEntries(
              projectRoleOrder.map((role) => [role, { enabled: true }]),
            ),
          },
        }).config.roles,
      },
    },
  },
};
it("keeps the plan and its idempotency key when saving fails", async () => {
  save
    .mockRejectedValueOnce(new Error("Save unavailable"))
    .mockResolvedValueOnce(issue);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  renderWithI18n(
    <QueryClientProvider client={client}>
      <ExecutionPlanEditor issue={issue} />
    </QueryClientProvider>,
  );
  fireEvent.click(screen.getByRole("button", { name: "Plan subtasks" }));
  fireEvent.click(screen.getByRole("button", { name: "Add subtask" }));
  await screen.findByRole("option", { name: "API" });
  fireEvent.change(screen.getByRole("textbox", { name: "Subtask title" }), {
    target: { value: "Implement export" },
  });
  fireEvent.change(screen.getByRole("combobox", { name: "Repository" }), {
    target: { value: "repo" },
  });
  fireEvent.click(
    screen.getByRole("checkbox", { name: "A-01 · Export report" }),
  );
  fireEvent.click(screen.getByRole("button", { name: "Save plan" }));
  await screen.findByText("Save unavailable");
  expect(screen.getByRole("textbox", { name: "Subtask title" })).toHaveValue(
    "Implement export",
  );
  fireEvent.click(screen.getByRole("button", { name: "Save plan" }));
  await waitFor(() => expect(save).toHaveBeenCalledTimes(2));
  expect(save.mock.calls[0]).toEqual(save.mock.calls[1]);
  expect(save.mock.calls[0]?.[1]).toMatchObject({
    expected_revision: 7,
    version: 3,
    tasks: [{ title: "Implement export", covers: ["A-01"], repo_id: "repo" }],
  });
});
