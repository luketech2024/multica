// @vitest-environment node
import { expect, it } from "vitest";
import {
  WORKBENCH_NAVIGATION,
  useWorkbenchNavigationStore,
} from "./navigation";

it("restores workspace defaults and normalizes saved navigation preferences", () => {
  const merge = useWorkbenchNavigationStore.persist.getOptions().merge!;
  const current = useWorkbenchNavigationStore.getState();
  const restored = merge(
    {
      pinned: ["issues", "issues", "unknown", "projects", "chat", "agents"],
      lastProject: 12,
    },
    current,
  );
  expect(restored.pinned).toEqual(["issues", "projects", "chat"]);
  const nextWorkspace = merge(undefined, restored);
  expect(nextWorkspace.pinned).toEqual(["inbox", "my_issues", "issues"]);
  expect(new Set(WORKBENCH_NAVIGATION.map((entry) => entry.id)).size).toBe(14);
});
