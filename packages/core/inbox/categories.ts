import type { InboxItem } from "../types";
import { deduplicateInboxItems } from "./queries";

export const inboxCategories = ["all", "pending", "mentions", "unread"] as const;
export type InboxCategory = (typeof inboxCategories)[number];

export function inboxCategory(value: string | null): InboxCategory {
  return inboxCategories.find((category) => category === value) ?? "all";
}

export function inboxCategoryItems(items: InboxItem[], category: InboxCategory): InboxItem[] {
  if (category === "mentions") {
    return deduplicateInboxItems(items.filter((item) => item.type === "mentioned"));
  }
  const grouped = deduplicateInboxItems(items);
  if (category === "pending") {
    return grouped.filter((item) => item.workbench && ["requirements", "delivery", "blocked"].includes(item.workbench_action ?? ""));
  }
  return category === "unread" ? grouped.filter((item) => !item.read) : grouped;
}
