// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { renderWithI18n } from "../../test/i18n";
import { WorkbenchCredentials } from "./workbench-credentials";

const mock = vi.hoisted(() => ({
  configured: false,
  savedSecret: "",
  save: vi.fn(),
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace-1" }));
vi.mock("@multica/core/api", () => ({
  api: {
    listWorkbenchCredentials: async () => ({
      credentials: [],
      encryption_configured: mock.configured,
    }),
    listProjects: async () => ({ projects: [] }),
    saveWorkbenchCredential: (data: { secret: string }) => {
      mock.savedSecret = data.secret;
      return mock.save(data);
    },
  },
}));
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  mock.configured = false;
});

function mount() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  renderWithI18n(
    <QueryClientProvider client={client}>
      <WorkbenchCredentials />
    </QueryClientProvider>,
  );
  return client;
}

it("shows missing encryption configuration before accepting secrets", async () => {
  const client = mount();
  await screen.findByText(
    "Configure the server encryption key before saving credentials.",
  );
  expect(screen.getByRole("button", { name: "Add" })).toBeDisabled();
  expect(screen.queryByLabelText("Secret value")).not.toBeInTheDocument();
  client.clear();
});

it("clears the submitted secret from the mutation variables after saving", async () => {
  mock.configured = true;
  mock.save.mockResolvedValue({ id: "credential-1" });
  const client = mount();
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "Add" })).toBeEnabled(),
  );
  fireEvent.click(screen.getByRole("button", { name: "Add" }));
  fireEvent.change(screen.getByRole("textbox", { name: "Name" }), {
    target: { value: "Deployment token" },
  });
  fireEvent.change(screen.getByLabelText("Secret value"), {
    target: { value: "test-only-secret" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() =>
    expect(screen.queryByLabelText("Secret value")).not.toBeInTheDocument(),
  );
  expect(mock.savedSecret).toBe("test-only-secret");
  expect(mock.save.mock.calls[0]?.[0].secret).toBe("");
  client.clear();
});
