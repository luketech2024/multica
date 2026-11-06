import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { projectRoleOrder } from "@multica/core/workbench/configuration";
import { renderWithI18n } from "../../test/i18n";
import { ProjectRolesSection } from "./project-roles-section";

const save = vi.fn();
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace" }));
vi.mock("@multica/core/permissions", () => ({
  useCurrentMember: () => ({ role: "owner" }),
}));
vi.mock("@multica/core/api", () => ({
  api: {
    getProjectWorkbenchRoles: async () => ({
      config: {
        v: 1,
        roles: Object.fromEntries(
          projectRoleOrder.map((role) => [
            role,
            { enabled: true, agent_id: null, runtime_id: null },
          ]),
        ),
      },
      revision: 1,
    }),
    listAgents: async () => [],
    listRuntimes: async () => [],
    saveProjectWorkbenchRoles: (...args: unknown[]) => save(...args),
  },
}));
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

it("requires scheduling and product while retaining an optional-role edit on conflict", async () => {
  save.mockRejectedValueOnce(new Error("Already updated"));
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  renderWithI18n(
    <QueryClientProvider client={client}>
      <ProjectRolesSection projectId="project" />
    </QueryClientProvider>,
  );
  expect(
    await screen.findByRole("switch", { name: "Scheduling" }),
  ).toHaveAttribute("aria-disabled", "true");
  expect(screen.getByRole("switch", { name: "Product" })).toHaveAttribute(
    "aria-disabled",
    "true",
  );
  fireEvent.click(screen.getByRole("switch", { name: "Scheduling" }));
  expect(screen.getByRole("switch", { name: "Scheduling" })).toBeChecked();
  fireEvent.click(screen.getByRole("switch", { name: "Security" }));
  fireEvent.click(screen.getByRole("button", { name: "Save" }));
  await screen.findByText("Already updated");
  expect(screen.getByRole("switch", { name: "Security" })).not.toBeChecked();
  await waitFor(() =>
    expect(save).toHaveBeenCalledWith(
      "project",
      expect.objectContaining({
        roles: expect.objectContaining({
          sec: expect.objectContaining({ enabled: false }),
        }),
      }),
      1,
    ),
  );
});
