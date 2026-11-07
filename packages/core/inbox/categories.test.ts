// @vitest-environment node
import { expect, it } from "vitest";
import type { InboxItem } from "../types";
import { inboxCategory, inboxCategoryItems } from "./categories";

function item(overrides: Partial<InboxItem>): InboxItem {
  return {
    id: "notification", workspace_id: "workspace", recipient_type: "member", recipient_id: "member",
    actor_type: "system", actor_id: null, type: "review_requested", severity: "action_required",
    issue_id: "issue", title: "Review", body: null, issue_status: "in_review", read: false, archived: false,
    created_at: "2026-11-07T00:00:00Z", details: null, workbench: true, workbench_action: "delivery", ...overrides,
  };
}

it("keeps read pending work until the server clears its action", () => {
  const pending = item({ read: true });
  expect(inboxCategoryItems([pending], "pending")).toEqual([pending]);
  expect(inboxCategoryItems([pending], "unread")).toEqual([]);
  expect(inboxCategoryItems([{ ...pending, workbench_action: "" }], "pending")).toEqual([]);
  expect(inboxCategoryItems([item({ workbench_action: "future" })], "pending")).toEqual([]);
  expect(inboxCategory("unknown")).toBe("all");
  for (const workbench_action of ["credential", "security"]) {
    expect(inboxCategoryItems([item({ issue_id: null, read: true, workbench_action })], "pending")).toHaveLength(1);
  }
});

it("retains older mentions when a newer notification exists on the same task", () => {
  const mention = item({ id: "mention", type: "mentioned", details: { comment_id: "comment" } });
  const latest = item({ id: "latest", read: true, created_at: "2026-11-07T00:01:00Z" });
  expect(inboxCategoryItems([mention, latest], "mentions")).toEqual([mention]);
  expect(inboxCategoryItems([mention, latest], "all")).toHaveLength(1);
  expect(inboxCategoryItems([mention, latest], "unread")).toEqual([]);
});
