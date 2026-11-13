import type { AgentTask, Issue, TimelineEntry } from "@multica/core/types";
import { buildCommentRunView, orderTimelineWithRuns, type CommentRun } from "./comment-runs";

export type ConversationSource = {
  issue: Issue;
  timeline: readonly TimelineEntry[];
  runs: readonly AgentTask[];
};

export type ConversationMessage = {
  source: Issue;
  id: string;
  time: string;
  entry?: TimelineEntry;
  run?: CommentRun;
  runs: CommentRun[];
  replies: TimelineEntry[];
};

export function conversationRows(sources: ConversationSource[]): ConversationMessage[] {
  return sources.flatMap(({ issue, timeline, runs }) => {
    const view = buildCommentRunView(runs, timeline);
    const byId = new Map(view.timeline.map((entry) => [entry.id, entry]));
    const entries = view.timeline.filter((entry) => entry.type === "comment"
      || (entry.type === "activity" && entry.action === "execution_control"));
    const turns = new Map([...view.runs.values()].flat().concat(view.standaloneRuns)
      .map((run) => [run.task.id, run]));
    return orderTimelineWithRuns(entries, [...turns.values()], byId).map((item): ConversationMessage => {
      const run = "task" in item ? item : undefined;
      const entry = "task" in item ? (item.hasReply ? byId.get(item.commentId ?? "") : undefined) : item;
      return {
        source: issue,
        id: entry?.id ?? run!.task.id,
        time: entry?.created_at ?? run!.task.created_at,
        entry, run,
        runs: run ? [run] : [],
        replies: [],
      };
    });
  }).sort((left, right) => Date.parse(left.time) - Date.parse(right.time) || left.id.localeCompare(right.id));
}
