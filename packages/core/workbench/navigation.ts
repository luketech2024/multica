"use client";

import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";
import { defaultStorage } from "../platform/storage";
import {
  createWorkspaceAwareStorage,
  registerForWorkspaceRehydration,
} from "../platform/workspace-storage";

export const WORKBENCH_NAVIGATION = [
  { id: "inbox", group: "personal" },
  { id: "my_issues", group: "personal" },
  { id: "issues", group: "work" },
  { id: "chat", group: "personal" },
  { id: "projects", group: "work" },
  { id: "board", group: "work" },
  { id: "releases", group: "work" },
  { id: "agents", group: "team" },
  { id: "squads", group: "team" },
  { id: "skills", group: "team" },
  { id: "runtimes", group: "team" },
  { id: "security", group: "system" },
  { id: "usage", group: "system" },
  { id: "settings", group: "system" },
] as const;
export type WorkbenchNavID = (typeof WORKBENCH_NAVIGATION)[number]["id"];
const DEFAULT_PINS: WorkbenchNavID[] = ["inbox", "my_issues", "issues"];

interface WorkbenchNavigationState {
  pinned: WorkbenchNavID[];
  togglePin: (id: WorkbenchNavID) => void;
}

export const useWorkbenchNavigationStore = create<WorkbenchNavigationState>()(
  persist(
    (set) => ({
      pinned: DEFAULT_PINS,
      togglePin: (id) =>
        set((state) => ({
          pinned: state.pinned.includes(id)
            ? state.pinned.filter((value) => value !== id)
            : state.pinned.length < 3
              ? [...state.pinned, id]
              : state.pinned,
        })),
    }),
    {
      name: "multica_workbench_navigation",
      storage: createJSONStorage(() =>
        createWorkspaceAwareStorage(defaultStorage),
      ),
      partialize: (state) => ({
        pinned: state.pinned,
      }),
      merge: (persisted, current) => {
        const saved = persisted as
          | Partial<WorkbenchNavigationState>
          | undefined;
        const pinned = Array.isArray(saved?.pinned)
          ? [...new Set(saved.pinned)]
              .filter((id) =>
                WORKBENCH_NAVIGATION.some((entry) => entry.id === id),
              )
              .slice(0, 3)
          : DEFAULT_PINS;
        return {
          ...current,
          pinned,
        };
      },
    },
  ),
);
registerForWorkspaceRehydration(() =>
  useWorkbenchNavigationStore.persist.rehydrate(),
);
