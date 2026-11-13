// @vitest-environment node
import { expect, it } from "vitest";
import type { AgentTask, Issue } from "../types";
import { currentExecutionIssues, taskWorkspaceProgress, conversationCommand, conversationDestination } from "./task-workspace";
const parent = { id: "parent", project_id: "project", status: "todo", acceptance_criteria: {
        v: 1, version: 2, next_item_no: 1, items: [], bounds: [], gate: { spec: { passed_at: "now", passed_by: "human" }, accept: null },
        plan: { version: 2, issues: ["child"], key: "plan", digest: "digest" },
    } } as unknown as Issue;
const child = { id: "child", parent_issue_id: parent.id, status: "todo", assignee_type: "agent", assignee_id: "ops",
    workbench_handoff: { requirement_version: 2, superseded: false } } as Issue;
const run = (status: AgentTask["status"], extra: Partial<AgentTask> = {}) => ({
    id: "run", issue_id: child.id, status, created_at: "2026-11-11T12:00:00Z", ...extra,
} as AgentTask);
it("uses only the current plan's non-superseded children", () => {
    expect(currentExecutionIssues(parent, [child, { ...child, id: "other" }])).toEqual([child]);
    expect(currentExecutionIssues(parent, [{ ...child, workbench_handoff: { ...child.workbench_handoff!, superseded: true } }])).toEqual([]);
    expect(currentExecutionIssues(parent, [{ ...child, workbench_handoff: { ...child.workbench_handoff!, requirement_version: 1 } }])).toEqual([]);
});
it("excludes a stale plan while requirements are being changed", () => {
    expect(currentExecutionIssues({ ...parent, acceptance_criteria: { ...parent.acceptance_criteria!, needs_resplit: true } }, [child])).toEqual([]);
});
it("shows an actual running child even when its parent remains todo", () => {
    const result = taskWorkspaceProgress(parent, [child], [run("running")]);
    expect(result.active).toHaveLength(1);
    expect(result.startable).toEqual([]);
});
it("separates waiting runs from running and prevents duplicate starts", () => {
    const result = taskWorkspaceProgress(parent, [child], [run("queued")]);
    expect(result.active).toEqual([]);
    expect(result.queued).toHaveLength(1);
    expect(result.startable).toEqual([]);
});
it("does not present an older failed run as the current blocker", () => {
    const result = taskWorkspaceProgress(parent, [child], [run("failed"), run("running", { id: "new", created_at: "2026-11-11T13:00:00Z" })]);
    expect(result.attention).toEqual([]);
});
it("keeps unassigned work actionable and starts only assigned tasks without runs", () => {
    expect(taskWorkspaceProgress(parent, [child], []).startable).toEqual([child]);
    const unassigned = { ...child, assignee_id: null };
    const result = taskWorkspaceProgress(parent, [unassigned], []);
    expect(result.startable).toEqual([]);
    expect(result.attention).toEqual([unassigned]);
});
it("keeps structured pause separate from failure and prevents retrying it", () => {
    const paused = { ...child, status: "blocked", workbench_pause: { phase: "paused" } } as Issue;
    const result = taskWorkspaceProgress(parent, [paused], [run("failed")]);
    expect(result.paused).toEqual([paused]);
    expect(result.attention).toEqual([]);
    expect(result.startable).toEqual([]);
});
it("recognizes explicit task controls without interpreting service operations or quoted discussion", () => {
    expect(conversationCommand("先暂停一下。")).toBe("pause");
    expect(conversationCommand("继续吧")).toBe("resume");
    expect(conversationCommand("暂停部署")).toBe("pause");
    expect(conversationCommand("停止 JumpServer 容器")).toBeUndefined();
    expect(conversationCommand("不要暂停")).toBeUndefined();
    expect(conversationCommand("解释一下暂停功能")).toBeUndefined();
});
it("routes coordination to the parent and asks when execution destinations are ambiguous", () => {
    const one = { ...child, identifier: "TEST-12" };
    const two = { ...child, id: "other", identifier: "TEST-13" };
    expect(conversationDestination(parent, [one], "修改需求")).toBe(parent.id);
    expect(conversationDestination(parent, [one], "检查日志")).toBe(one.id);
    expect(conversationDestination(parent, [one, two], "检查日志")).toBeUndefined();
    expect(conversationDestination(parent, [one, two], "查看TEST-13日志")).toBe(two.id);
    expect(conversationDestination(parent, [one, two], "TEST-12和TEST-13怎么样")).toBeUndefined();
});
