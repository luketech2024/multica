import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { Issue } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { ReleaseReadiness } from "./release-readiness";

const read = vi.fn();
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace" }));
vi.mock("@multica/core/paths", () => ({ useWorkspacePaths: () => ({ releases: () => "/acme/releases" }) }));
vi.mock("@multica/core/api", () => ({ api: { getReleaseReadiness: (...args: unknown[]) => read(...args) } }));
afterEach(() => { cleanup(); vi.resetAllMocks(); });

function mount() {
  renderWithI18n(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
    <ReleaseReadiness issue={{ id: "parent", revision: 7 } as Issue} />
  </QueryClientProvider>);
}

it("shows the confirmed repository merge", async () => {
  read.mockResolvedValue({ revision: 7, blockers: [], merges: [{ repo_id: "repo", url: "https://github.com/acme/app/pull/1", base_branch: "main", merge_commit: "a".repeat(40) }] });
  mount();
  expect(await screen.findByText("Accepted source commits match the repository merges.")).toBeVisible();
  expect(screen.getByRole("link")).toHaveAttribute("href", "https://github.com/acme/app/pull/1");
  expect(screen.getByTitle("a".repeat(40))).toHaveTextContent("main");
});

it("retries a failed read and shows a blocker", async () => {
  read.mockRejectedValueOnce(new Error("offline")).mockResolvedValue({ revision: 7, blockers: [{ code: "merge_missing" }], merges: [] });
  mount();
  expect(await screen.findByRole("alert")).toHaveTextContent("Could not load merge evidence.");
  fireEvent.click(screen.getByRole("button", { name: "Retry" }));
  expect(await screen.findByText("A matching merge for the accepted source commit is missing.")).toBeVisible();
  expect(screen.queryByText("Accepted source commits match the repository merges.")).not.toBeInTheDocument();
});
