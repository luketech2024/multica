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
const listResources = vi.fn(async () => ({
  resources: [{ id: "repo", label: "API", resource_type: "github_repo" }],
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace" }));
vi.mock("@multica/core/permissions", () => ({
  useCurrentMember: () => ({ role: "owner" }),
}));
vi.mock("@multica/core/api", () => ({
  api: {
    listProjectResources: () => listResources(),
    saveExecutionPlan: (...args: unknown[]) => save(...args),
  },
}));
vi.mock("../../modals/issue-picker-modal", () => ({
  IssuePickerModal: ({
    isSelectable,
    onSelect,
  }: {
    isSelectable: (candidate: Issue) => boolean;
    onSelect: (candidate: Issue) => void;
  }) => {
    const candidates = [
      {
        ...issue,
        id: "prior",
        title: "Existing prerequisite",
        identifier: "WB-2",
      },
      {
        ...issue,
        id: "foreign",
        title: "Foreign prerequisite",
        project_id: "other",
      },
      { ...issue, id: "child", title: "Old child", parent_issue_id: issue.id },
      {
        ...issue,
        id: "cancelled",
        title: "Cancelled prerequisite",
        status: "cancelled" as const,
      },
    ];
    return (
      <div>
        {candidates.filter(isSelectable).map((candidate) => (
          <button key={candidate.id} onClick={() => onSelect(candidate)}>
            {candidate.title}
          </button>
        ))}
      </div>
    );
  },
}));
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

it("shows loading, explains repository failures and retains the draft on retry", async () => {
  let fail: (reason: Error) => void = () => {};
  listResources.mockImplementationOnce(() => new Promise((_, reject) => { fail = reject; }));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  renderWithI18n(<QueryClientProvider client={client}><ExecutionPlanEditor issue={issue} /></QueryClientProvider>);
  fireEvent.click(screen.getByRole("button", { name: "Plan subtasks" }));
  expect(screen.getByRole("status", { name: "Loading repositories…" })).toBeVisible();
  fireEvent.click(screen.getByRole("button", { name: "Add subtask" }));
  fireEvent.change(screen.getByRole("textbox", { name: "Subtask title" }), { target: { value: "Retained draft" } });
  fail(new Error("offline"));
  expect(await screen.findByRole("alert")).toHaveTextContent("Repository coverage cannot be verified");
  expect(screen.getByRole("button", { name: "Save plan" })).toBeDisabled();
  fireEvent.click(screen.getByRole("button", { name: "Retry repositories" }));
  await screen.findByRole("option", { name: "API" });
  expect(screen.getByRole("textbox", { name: "Subtask title" })).toHaveValue("Retained draft");
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
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
  expect(screen.getByRole("button", { name: "Save plan" })).toBeDisabled();
  expect(screen.getByText("Add coverage for A-01")).toBeInTheDocument();
  fireEvent.change(screen.getByRole("textbox", { name: "Subtask title" }), {
    target: { value: "Implement export" },
  });
  fireEvent.change(screen.getByRole("combobox", { name: "Repository" }), {
    target: { value: "repo" },
  });
  expect(screen.getByRole("button", { name: "Save plan" })).toBeDisabled();
  fireEvent.click(
    screen.getByRole("checkbox", { name: "A-01 · Export report" }),
  );
  expect(
    screen.getByText("1 / 1 acceptance items covered"),
  ).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Save plan" })).toBeEnabled();
  fireEvent.click(
    screen.getByRole("button", { name: "Add existing dependency" }),
  );
  expect(
    screen.queryByRole("button", { name: "Foreign prerequisite" }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Old child" }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Cancelled prerequisite" }),
  ).not.toBeInTheDocument();
  fireEvent.click(
    screen.getByRole("button", { name: "Existing prerequisite" }),
  );
  expect(screen.getByText("WB-2 · Existing prerequisite")).toBeInTheDocument();
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
    tasks: [
      {
        title: "Implement export",
        covers: ["A-01"],
        repo_id: "repo",
        existing_dependencies: ["prior"],
      },
    ],
  });
});

it("blocks cyclic dependencies and recalculates after removing a subtask", async () => {
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
  fireEvent.click(screen.getByRole("button", { name: "Add subtask" }));
  screen
    .getAllByRole("textbox", { name: "Subtask title" })
    .forEach((input, i) => {
      fireEvent.change(input, {
        target: { value: i === 0 ? "First" : "Second" },
      });
    });
  screen.getAllByRole("combobox", { name: "Repository" }).forEach((input) => {
    fireEvent.change(input, { target: { value: "repo" } });
  });
  screen
    .getAllByRole("checkbox", { name: "A-01 · Export report" })
    .forEach((input) => {
      fireEvent.click(input);
    });
  expect(screen.getByRole("button", { name: "Save plan" })).toBeEnabled();
  fireEvent.click(screen.getByRole("checkbox", { name: / · First$/ }));
  fireEvent.click(screen.getByRole("checkbox", { name: / · Second$/ }));
  expect(screen.getByRole("alert")).toHaveTextContent("Dependency cycle");
  expect(screen.getByRole("alert")).toHaveTextContent("→");
  expect(screen.getByRole("button", { name: "Save plan" })).toBeDisabled();
  fireEvent.click(screen.getAllByRole("button", { name: "Remove" })[0]!);
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Save plan" })).toBeEnabled();
});
