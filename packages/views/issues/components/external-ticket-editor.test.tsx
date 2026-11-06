import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { Issue } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { ExternalTicketEditor } from "./external-ticket-editor";

const update = vi.fn();
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace" }));
vi.mock("@multica/core/issues/mutations", () => ({
  useUpdateIssue: () => ({ mutateAsync: update, isPending: false }),
}));
vi.mock("@multica/core/api", () => ({
  api: { countExternalTicketDuplicates: async () => ({ count: 1 }) },
}));
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

it("keeps a rejected edit and original revision, permits duplicate hints, and saves clearing", async () => {
  update
    .mockRejectedValueOnce(new Error("Revision changed"))
    .mockResolvedValueOnce({});
  const issue = {
    id: "task",
    identifier: "TEST-42",
    external_ticket_no: "XQ-3482",
    revision: 7,
  } as Issue;
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  renderWithI18n(
    <QueryClientProvider client={client}>
      <ExternalTicketEditor issue={issue} />
    </QueryClientProvider>,
  );
  fireEvent.click(
    screen.getByRole("button", { name: "Edit external ticket number" }),
  );
  expect(screen.getByLabelText("System number")).toHaveAttribute("readonly");
  const input = screen.getByLabelText("External ticket number (optional)");
  fireEvent.change(input, { target: { value: "OTHER-7" } });
  fireEvent.blur(input);
  await screen.findByText("Another issue uses this ticket number");
  fireEvent.click(screen.getByRole("button", { name: "Save changes" }));
  await screen.findByText("Revision changed");
  expect(input).toHaveValue("OTHER-7");
  expect(update).toHaveBeenLastCalledWith({
    id: "task",
    expected_revision: 7,
    external_ticket_no: "OTHER-7",
  });
  fireEvent.change(input, { target: { value: "" } });
  fireEvent.click(screen.getByRole("button", { name: "Save changes" }));
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
  expect(update).toHaveBeenLastCalledWith({
    id: "task",
    expected_revision: 7,
    external_ticket_no: "",
  });
});
