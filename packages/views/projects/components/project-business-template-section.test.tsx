import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { renderWithI18n } from "../../test/i18n";
import { ProjectBusinessTemplateSection } from "./project-business-template-section";

const save = vi.fn();
const template = {
  source_id: "blank",
  source_revision: 1,
  revision: 3,
  content: {
    v: 1,
    name: "Project template",
    background: [],
    audience: [],
    scope_out: [],
    ac_skeleton: ["Deliver {feature}"],
    forbidden: [{ rule: "No credentials", check: "Scan changes" }],
    dod: [],
    resources: [],
    terms: { acceptance: "Reviewed result" },
  },
};
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace" }));
vi.mock("@multica/core/permissions", () => ({
  useCurrentMember: () => ({ role: "owner" }),
}));
vi.mock("@multica/core/api", () => ({
  api: {
    getProjectBusinessTemplate: async () => ({ template }),
    listBusinessTemplates: async () => ({ templates: [] }),
    saveProjectBusinessTemplate: (...args: unknown[]) => save(...args),
  },
}));
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

it("retains template edits on failure and updates the saved revision on retry", async () => {
  save.mockRejectedValueOnce(new Error("Save failed"));
  save.mockImplementationOnce(async (_project, content) => ({
    template: { ...template, revision: 4, content },
  }));
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  renderWithI18n(
    <QueryClientProvider client={client}>
      <ProjectBusinessTemplateSection projectId="project" />
    </QueryClientProvider>,
  );
  fireEvent.click(await screen.findByRole("button", { name: "Edit" }));
  fireEvent.change(screen.getByRole("textbox", { name: "Template name" }), {
    target: { value: "Local template" },
  });
  fireEvent.change(screen.getByRole("textbox", { name: "Check method" }), {
    target: { value: "Inspect pipeline" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Save" }));
  await screen.findByText("Save failed");
  expect(screen.getByRole("textbox", { name: "Template name" })).toHaveValue(
    "Local template",
  );
  fireEvent.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() =>
    expect(
      screen.queryByRole("textbox", { name: "Template name" }),
    ).not.toBeInTheDocument(),
  );
  expect(save).toHaveBeenLastCalledWith(
    "project",
    expect.objectContaining({
      name: "Local template",
      forbidden: [{ rule: "No credentials", check: "Inspect pipeline" }],
    }),
    3,
  );
  fireEvent.click(screen.getByRole("button", { name: "Edit" }));
  fireEvent.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() =>
    expect(save).toHaveBeenLastCalledWith("project", expect.anything(), 4),
  );
});
