"use client";

import {
  createContext,
  useContext,
  type ReactNode,
} from "react";

import type { AgentTask } from "@multica/core/types";

export type DiscussionReplyTarget = {
  parentId: string;
  draftThreadId?: string;
  name: string;
  onEditAnnotation?: (id: string) => boolean;
  steerByDefault?: (task: AgentTask) => boolean;
};

export type CurrentIssueRenderContextValue = Readonly<{
  id: string;
  identifier: string;
  status?: string;
  composeReply?: (target: DiscussionReplyTarget) => void;
}>;

/**
 * The issue identity that owns the content currently being rendered.
 *
 * It intentionally defaults to null: content renderers are shared by issue
 * detail, inbox, chat, and other surfaces, and only an explicit issue-detail
 * owner may opt into current-issue semantics.
 */
const CurrentIssueRenderContext =
  createContext<CurrentIssueRenderContextValue | null>(null);

export function CurrentIssueRenderContextProvider({
  value,
  children,
}: {
  value: CurrentIssueRenderContextValue | null;
  children: ReactNode;
}) {
  return (
    <CurrentIssueRenderContext.Provider value={value}>
      {children}
    </CurrentIssueRenderContext.Provider>
  );
}

export function useCurrentIssueRenderContext(): CurrentIssueRenderContextValue | null {
  return useContext(CurrentIssueRenderContext);
}
