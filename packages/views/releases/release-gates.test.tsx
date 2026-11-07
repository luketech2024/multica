import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { ReleaseOrderSchema } from "@multica/core/workbench/releases";
import { renderWithI18n } from "../test/i18n";
import { ReleaseGates } from "./release-gates";

const confirm = vi.hoisted(() => vi.fn());
vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    projectDetail: (id: string) => `/acme/projects/${id}`,
  }),
}));
vi.mock("../navigation", () => ({
  AppLink: ({
    href,
    children,
  }: {
    href: string;
    children: React.ReactNode;
  }) => <a href={href}>{children}</a>,
}));
vi.mock("@multica/core/api", () => ({ api: { confirmReleaseOrder: confirm } }));
afterEach(() => {
  cleanup();
  vi.resetAllMocks();
});

it("requires explicit confirmations and preserves them on a failed save", async () => {
  const id = "00000000-0000-4000-8000-000000000001";
  const order = ReleaseOrderSchema.parse({
    id,
    workspace_id: id,
    project_id: id,
    identifier: "REL-01",
    env: "prod",
    status: "draft",
    revision: 3,
    created_by: id,
    created_at: "now",
    updated_at: "now",
    manifest: { tasks: [], repositories: [] },
    gates: [
      {
        id: "security",
        passed: false,
        blockers: [{ code: "security_waiver" }],
      },
    ],
  });
  confirm
    .mockRejectedValueOnce(new Error("Release changed"))
    .mockResolvedValue(order);
  renderWithI18n(
    <QueryClientProvider client={new QueryClient()}>
      <ReleaseGates wsId={id} order={order} />
    </QueryClientProvider>,
  );
  const fields = screen.getAllByRole("checkbox");
  for (const field of fields) {
    expect(field).not.toBeChecked();
    fireEvent.click(field);
  }
  fireEvent.click(screen.getByRole("button", { name: "Save confirmations" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("Release changed");
  for (const field of fields) {
    expect(field).toBeChecked();
  }
  fireEvent.click(screen.getByRole("button", { name: "Save confirmations" }));
  await waitFor(() => expect(confirm).toHaveBeenCalledTimes(2));
  expect(confirm).toHaveBeenLastCalledWith(id, {
    expected_revision: 3,
    changes: true,
    migration: true,
    security_waiver: true,
  });
});

it("shows production-only gates as optional and omits confirmations for staging", () => {
  const id = "00000000-0000-4000-8000-000000000001";
  const order = ReleaseOrderSchema.parse({
    id, workspace_id: id, project_id: id, identifier: "REL-02", env: "staging",
    status: "draft", revision: 1, created_by: id, created_at: "now", updated_at: "now",
    manifest: { tasks: [], repositories: [] },
    gates: [
      { id: "security", passed: true, required: false, blockers: [] },
      { id: "changes", passed: true, required: false, blockers: [] },
      { id: "targets", passed: false, required: true, blockers: [{ code: "target_missing" }] },
    ],
  });
  renderWithI18n(<QueryClientProvider client={new QueryClient()}><ReleaseGates wsId={id} order={order} /></QueryClientProvider>);
  expect(screen.getByRole("heading", { name: "Security · Not required for staging" })).toBeVisible();
  expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Save confirmations" })).not.toBeInTheDocument();
  expect(screen.getByRole("link", { name: "Environment and deployment" })).toHaveAttribute("href", `/acme/projects/${id}?tab=deployment`);
});
