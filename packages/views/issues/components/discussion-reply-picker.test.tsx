import { cleanup, fireEvent, screen, within } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { TimelineEntry } from "@multica/core/types";
import { renderWithI18n } from "../../test/i18n";
import { DiscussionReplyPicker } from "./discussion-reply-picker";

vi.mock("@multica/core/workspace/hooks", () => ({ useActorName: () => ({ getActorName: () => "Test author" }) }));
afterEach(cleanup);
const comment = { id: "original", type: "comment", actor_type: "member", actor_id: "user", content: "Original discussion", created_at: "2026-11-11T00:00:00Z" } as TimelineEntry;

function mount(overrides: Partial<React.ComponentProps<typeof DiscussionReplyPicker>> = {}) {
  const props = { entries: [comment], loading: false, failed: false, onRetry: vi.fn(), onSelect: vi.fn(), label: "Choose discussion", onShowActivity: vi.fn(), ...overrides };
  renderWithI18n(<DiscussionReplyPicker {...props} />);
  fireEvent.click(screen.getByRole("button", { name: "Choose discussion" }));
  return props;
}

it("requires an explicit choice even for one discussion", async () => {
  const props = mount({ entries: [{ ...comment, content: "Original discussion [@Reviewer](mention://member/person)" }] });
  const dialog = await screen.findByRole("dialog");
  expect(within(dialog).getByText("Original discussion @Reviewer")).toBeVisible();
  expect(props.onSelect).not.toHaveBeenCalled();
});

it("shows loading and retry without offering stale choices", async () => {
  mount({ loading: true });
  expect(await screen.findByRole("status")).toHaveTextContent("Loading");
  expect(screen.queryByText("Original discussion")).not.toBeInTheDocument();
  cleanup();
  const props = mount({ failed: true });
  await screen.findByRole("alert");
  fireEvent.click(screen.getByRole("button", { name: "Retry" }));
  expect(props.onRetry).toHaveBeenCalledTimes(1);
  expect(screen.queryByText("Original discussion")).not.toBeInTheDocument();
});

it("offers activity when deleted and system entries leave no reply candidates", async () => {
  const props = mount({ entries: [{ ...comment, deleted_at: "2026-11-11T01:00:00Z" }, { ...comment, id: "system", actor_type: "system" }] });
  const dialog = await screen.findByRole("dialog");
  fireEvent.click(within(dialog).getByRole("button", { name: "View activity" }));
  expect(props.onShowActivity).toHaveBeenCalledTimes(1);
  expect(props.onSelect).not.toHaveBeenCalled();
});

it("includes nested replies and makes older discussion pages reachable", async () => {
  mount({ entries: Array.from({ length: 21 }, (_, index) => ({ ...comment, id: `reply-${index}`, parent_id: "thread-root", content: `Discussion ${index}`, created_at: new Date(Date.UTC(2026, 10, 11, index)).toISOString() })) });
  const dialog = await screen.findByRole("dialog");
  expect(within(dialog).getAllByRole("article")).toHaveLength(20);
  expect(within(dialog).queryByText("Discussion 0")).not.toBeInTheDocument();
  fireEvent.click(within(dialog).getByRole("button", { name: /more/i }));
  expect(within(dialog).getByText("Discussion 0")).toBeVisible();
  expect(within(dialog).getAllByRole("article")).toHaveLength(21);
});
