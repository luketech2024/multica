import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { Issue } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { TaskPreview } from "./task-preview";

const setProperty = vi.fn();
const clearProperty = vi.fn();
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace" }));
vi.mock("@multica/core/permissions", () => ({
  useCurrentMember: () => ({ role: "member" }),
}));
vi.mock("@multica/core/api", () => ({
  api: {
    listProperties: async () => ({
      properties: [{ id: "preview-field", name: "preview_url", type: "url" }],
    }),
    listDeployTargets: async () => ({
      targets: [
        {
          id: "target",
          env: "preview",
          label: "Preview host",
          location: { domain: "preview.example.test" },
        },
      ],
    }),
    setIssueProperty: (...args: unknown[]) => setProperty(...args),
    unsetIssueProperty: (...args: unknown[]) => clearProperty(...args),
  },
}));
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

it("keeps a failed address draft and only offers an open link for the saved value", async () => {
  setProperty
    .mockRejectedValueOnce(new Error("Save unavailable"))
    .mockResolvedValueOnce({
      properties: {
        "preview-field": "https://preview.example.test/preview/project/WB-1/",
      },
    });
  clearProperty.mockResolvedValue({ properties: {} });
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const issue = {
    id: "task",
    project_id: "project",
    identifier: "WB-1",
    properties: {},
  } as Issue;
  renderWithI18n(
    <QueryClientProvider client={client}>
      <TaskPreview issue={issue} />
    </QueryClientProvider>,
  );
  await screen.findByRole("textbox", { name: "Preview address" });
  expect(
    screen.queryByRole("link", { name: "Open recorded preview" }),
  ).not.toBeInTheDocument();
  fireEvent.click(
    await screen.findByRole("button", { name: "Use Preview host domain" }),
  );
  const address = "https://preview.example.test/preview/project/WB-1/";
  expect(screen.getByRole("textbox", { name: "Preview address" })).toHaveValue(
    address,
  );
  fireEvent.click(screen.getByRole("button", { name: "Save address" }));
  await screen.findByText("Save unavailable");
  expect(screen.getByRole("textbox", { name: "Preview address" })).toHaveValue(
    address,
  );
  fireEvent.click(screen.getByRole("button", { name: "Save address" }));
  await waitFor(() => expect(setProperty).toHaveBeenCalledTimes(2));
  expect(setProperty.mock.calls[1]).toEqual(["task", "preview-field", address]);
});
