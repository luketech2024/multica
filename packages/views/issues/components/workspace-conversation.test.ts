// @vitest-environment node
import { expect, it } from "vitest";
import type { AgentTask, Issue, TimelineEntry } from "@multica/core/types";
import { conversationRows } from "./workspace-conversation";

function comment(id: string, created_at: string, fields: Partial<TimelineEntry> = {}): TimelineEntry {
  return { id, created_at, type: "comment", actor_type: "member", content: id, ...fields } as TimelineEntry;
}

it("places late replies at their own time across parent and child conversations", () => {
  const prompt = comment("prompt", "2026-11-13T10:00:00+08:00");
  const intervening = comment("intervening", "2026-11-13T02:01:00Z");
  const reply = comment("reply", "2026-11-13T10:02:00+08:00", {
    parent_id: prompt.id, actor_type: "agent", source_task_id: "run",
  });
  const followup = comment("followup", "2026-11-13T10:03:00+08:00", { parent_id: prompt.id });
  const run = { id: "run", issue_id: "child", status: "completed", created_at: prompt.created_at,
    trigger_comment_id: prompt.id, delivered_comment_ids: [prompt.id], completed_at: reply.created_at } as AgentTask;
  const rows = conversationRows([
    { issue: { id: "parent" } as Issue, timeline: [intervening], runs: [] },
    { issue: { id: "child" } as Issue, timeline: [prompt, reply, followup], runs: [run] },
  ]);
  expect(rows.map((row) => row.id)).toEqual(["prompt", "intervening", "reply", "followup"]);
  expect(rows.every((row) => row.replies.length === 0)).toBe(true);
  expect(rows[2]?.run?.task.id).toBe("run");
  expect(rows[0]?.runs).toEqual([]);
});

it("keeps a running turn after its input without rendering the input twice", () => {
  const prompt = comment("prompt", "2026-11-13T10:00:00+08:00");
  const run = { id: "run", issue_id: "child", status: "running", created_at: "2026-11-13T10:00:01+08:00",
    trigger_comment_id: prompt.id, delivered_comment_ids: [prompt.id] } as AgentTask;
  const rows = conversationRows([{ issue: { id: "child" } as Issue, timeline: [prompt], runs: [run] }]);
  expect(rows.map((row) => row.id)).toEqual(["prompt", "run"]);
  expect(rows[1]?.entry).toBeUndefined();
});

it("renders an unprompted run's published response only once", () => {
  const reply = comment("reply", "2026-11-13T10:02:00+08:00", { actor_type: "agent", source_task_id: "run" });
  const run = { id: "run", issue_id: "child", status: "completed", created_at: "2026-11-13T10:00:00+08:00",
    completed_at: reply.created_at } as AgentTask;
  const rows = conversationRows([{ issue: { id: "child" } as Issue, timeline: [reply], runs: [run] }]);
  expect(rows.map((row) => row.id)).toEqual(["reply"]);
});
