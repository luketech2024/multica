import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  cleanup,
  fireEvent,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { renderWithI18n } from "../test/i18n";
import { ReleasesPage } from "./releases-page";

const mocks = vi.hoisted(() => ({
  create: vi.fn(),
  list: vi.fn(),
  candidates: vi.fn(),
  push: vi.fn(),
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace" }));
vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    releases: () => "/acme/releases",
    releaseDetail: (id: string) => `/acme/releases/${id}`,
  }),
}));
vi.mock("@multica/core/projects", () => ({
  projectListOptions: () => ({
    queryKey: ["projects"],
    queryFn: async () => [{ id: "project", title: "App" }],
  }),
}));
vi.mock("@multica/core/api", () => ({
  api: {
    listReleaseOrders: (...args: unknown[]) => mocks.list(...args),
    listReleaseCandidates: (...args: unknown[]) => mocks.candidates(...args),
    createReleaseOrder: (...args: unknown[]) => mocks.create(...args),
  },
}));
vi.mock("../navigation", () => ({
  useNavigation: () => ({
    searchParams: new URLSearchParams("project=project"),
    replace: vi.fn(),
    push: mocks.push,
  }),
  AppLink: ({
    href,
    children,
    className,
  }: {
    href: string;
    children: React.ReactNode;
    className?: string;
  }) => (
    <a href={href} className={className}>
      {children}
    </a>
  ),
}));
afterEach(() => {
  cleanup();
  vi.resetAllMocks();
});

it("preserves a failed selection and reuses the request identity on retry", async () => {
  mocks.list.mockResolvedValue({ releases: [], next_cursor: null });
  mocks.candidates.mockResolvedValue({
    candidates: [
      {
        id: "one",
        identifier: "TASK-1",
        title: "First task",
        revision: 3,
        merges: [],
      },
      {
        id: "two",
        identifier: "TASK-2",
        title: "Second task",
        revision: 5,
        merges: [],
      },
    ],
    next_cursor: null,
  });
  mocks.create
    .mockRejectedValueOnce(new Error("Connection lost"))
    .mockResolvedValue({ id: "release" });
  renderWithI18n(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <ReleasesPage />
    </QueryClientProvider>,
  );
  await screen.findByText("No release orders yet.");
  fireEvent.click(screen.getByRole("button", { name: "New release" }));
  const dialog = within(await screen.findByRole("dialog"));
  const first = await dialog.findByRole("checkbox", {
    name: "TASK-1 · First task",
  });
  expect(first).toBeChecked();
  fireEvent.click(
    dialog.getByRole("checkbox", { name: "TASK-2 · Second task" }),
  );
  fireEvent.click(dialog.getByRole("button", { name: "New release" }));
  expect(await dialog.findByRole("alert")).toHaveTextContent("Connection lost");
  expect(first).toBeChecked();
  fireEvent.click(dialog.getByRole("button", { name: "New release" }));
  await waitFor(() =>
    expect(mocks.push).toHaveBeenCalledWith("/acme/releases/release"),
  );
  const initial = mocks.create.mock.calls[0]![0];
  expect(initial.tasks).toEqual([{ id: "one", expected_revision: 3 }]);
  expect(mocks.create.mock.calls[1]![0].request_id).toBe(initial.request_id);
});
