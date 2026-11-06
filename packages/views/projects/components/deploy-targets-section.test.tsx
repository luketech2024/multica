// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { renderWithI18n } from "../../test/i18n";
import { DeployTargetsSection } from "./deploy-targets-section";

const save = vi.fn();
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace-1" }));
vi.mock("@multica/core/permissions", () => ({
  useCurrentMember: () => ({ role: "owner" }),
}));
vi.mock("@multica/core/api", () => ({
  api: {
    listDeployTargets: async () => ({ targets: [] }),
    listProjectResources: async () => ({
      resources: [
        { id: "repo-1", label: "Application", resource_type: "github_repo" },
      ],
    }),
    listWorkbenchCredentials: async () => ({
      credentials: [],
      encryption_configured: false,
    }),
    listRuntimes: async () => [],
    saveDeployTarget: (...args: unknown[]) => save(...args),
  },
}));
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

it("saves an incomplete target without inventing host details and retains rejected input", async () => {
  save
    .mockRejectedValueOnce(new Error("Already updated"))
    .mockResolvedValueOnce({ id: "target-1" });
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  renderWithI18n(
    <QueryClientProvider client={client}>
      <DeployTargetsSection projectId="project-1" />
    </QueryClientProvider>,
  );
  await screen.findByText("No deployment targets configured");
  await waitFor(() =>
    expect(
      client.getQueryData([
        "projects",
        "workspace-1",
        "detail",
        "project-1",
        "resources",
      ]),
    ).toBeDefined(),
  );
  fireEvent.click(screen.getByRole("button", { name: "Add" }));
  fireEvent.change(screen.getByRole("textbox", { name: "Name" }), {
    target: { value: "Future staging" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Save" }));
  await screen.findByText("Already updated");
  expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue(
    "Future staging",
  );
  expect(save).toHaveBeenCalledWith(
    "project-1",
    expect.objectContaining({
      label: "Future staging",
      location: {},
      runtime_id: null,
      credential_id: null,
    }),
    undefined,
  );
  fireEvent.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() =>
    expect(
      screen.queryByRole("textbox", { name: "Name" }),
    ).not.toBeInTheDocument(),
  );
  client.clear();
});
