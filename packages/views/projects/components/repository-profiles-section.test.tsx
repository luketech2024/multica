import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { renderWithI18n } from "../../test/i18n";
import { RepositoryProfilesSection } from "./repository-profiles-section";

const saveProfile = vi.fn();
const profile = {
  revision: 3,
  stack: {
    v: 3,
    template: "python-service",
    tags: [],
    detected: null,
    baseline: null,
    derived: {
      install: "uv sync",
      test: "uv run pytest -q",
      lint: [],
      scan: "",
    },
    overrides: {},
    runtime: "server",
    requires: [],
    checks: [],
  },
  delivery: ["none"],
};
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace" }));
vi.mock("@multica/core/permissions", () => ({
  useCurrentMember: () => ({ role: "member" }),
}));
vi.mock("@multica/core/api", () => ({
  api: {
    listProjectResources: async () => ({
      resources: [{ id: "repo", label: "API" }],
    }),
    getRepositoryProfile: async () => profile,
    listRepositoryStacks: async () => ({
      templates: [{ id: "python-service" }, { id: "java-backend" }],
      tags: [{ id: "java21" }],
      deliveries: ["none", "docker-image"],
    }),
    saveRepositoryProfile: (...args: unknown[]) => saveProfile(...args),
  },
}));
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});
it("preserves a stale draft for explicit retry and sends only editable inputs", async () => {
  saveProfile.mockRejectedValue(new Error("repository profile changed"));
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  renderWithI18n(
    <QueryClientProvider client={client}>
      <RepositoryProfilesSection projectId="project" />
    </QueryClientProvider>,
  );
  await screen.findByText("Not detected");
  expect(screen.getByText("Not scanned")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Edit profile" }));
  fireEvent.change(screen.getByRole("combobox", { name: "Stack template" }), {
    target: { value: "java-backend" },
  });
  fireEvent.click(screen.getByRole("checkbox", { name: "java21" }));
  fireEvent.click(screen.getByRole("checkbox", { name: "docker-image" }));
  expect(screen.getByRole("checkbox", { name: "none" })).not.toBeChecked();
  fireEvent.click(screen.getByText("Manual command overrides"));
  fireEvent.click(screen.getByRole("checkbox", { name: "build" }));
  fireEvent.change(screen.getByRole("textbox", { name: "build" }), {
    target: { value: "./scripts/build.sh" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Save profile" }));
  await screen.findByText("repository profile changed");
  expect(screen.getByRole("combobox", { name: "Stack template" })).toHaveValue(
    "java-backend",
  );
  await waitFor(() =>
    expect(saveProfile).toHaveBeenCalledWith("project", "repo", {
      expected_revision: 3,
      template: "java-backend",
      tags: ["java21"],
      delivery: ["docker-image"],
      overrides: { build: "./scripts/build.sh" },
    }),
  );
});
