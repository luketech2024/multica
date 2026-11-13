import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { api } from "@multica/core/api";
import { OneSpecConfigurationSchema } from "@multica/core/workbench/configuration";
import { renderWithI18n } from "../../test/i18n";
import { OneSpecConfigurationSection } from "./onespec-configuration-section";

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace" }));
vi.mock("@multica/core/permissions", () => ({ useCurrentMember: () => ({ role: "owner" }) }));
vi.mock("@multica/core/api", () => ({ api: {
  getOneSpecConfiguration: vi.fn(), saveOneSpecConfiguration: vi.fn(),
  provisionOneSpecWorkspace: vi.fn(), provisionOneSpecTeam: vi.fn(),
  listRuntimes: async () => [], listProjectResources: async () => [],
} }));

it("saves explicit OneSpec selection without requiring a runtime or manual provisioning", async () => {
  const settings = OneSpecConfigurationSchema.parse({ revision: 0 });
  vi.mocked(api.getOneSpecConfiguration).mockResolvedValue(settings);
  vi.mocked(api.saveOneSpecConfiguration).mockResolvedValue({ ...settings, enabled: true, revision: 1 });
  renderWithI18n(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
    <OneSpecConfigurationSection projectId="project" />
  </QueryClientProvider>);
  const toggle = await screen.findByRole("switch");
  expect(toggle).not.toBeChecked();
  fireEvent.click(toggle);
  fireEvent.click(screen.getByRole("button", { name: "Save configuration" }));
  await waitFor(() => expect(api.saveOneSpecConfiguration).toHaveBeenCalledWith("workspace", expect.objectContaining({ enabled: true, runtime_id: null }), "project"));
  expect(api.provisionOneSpecTeam).not.toHaveBeenCalled();
});
