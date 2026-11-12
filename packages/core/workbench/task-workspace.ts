import type { AgentTask, Issue } from "../types";
import { workbenchNextStep } from "./next-step";
export function canControlWorkspaceExecution(issue: Issue, role?: string | null): boolean {
    return ["owner", "admin", "member"].includes(role ?? "") && !!issue.revision
        && !!(issue.acceptance_criteria || issue.workbench_handoff) && !issue.acceptance_criteria?.change
        && !(issue.acceptance_criteria?.resume_runs?.length)
        && !issue.workbench_handoff?.superseded && !["done", "cancelled"].includes(issue.status);
}
export function currentExecutionIssues(parent: Issue, children: readonly Issue[]): Issue[] {
    const criteria = parent.acceptance_criteria;
    if (!criteria?.plan)
        return children.filter((child) => !child.workbench_handoff?.superseded);
    if (!criteria.gate.spec || criteria.change || criteria.needs_resplit || criteria.plan.version !== criteria.version)
        return [];
    const planned = new Set(criteria.plan.issues);
    return children.filter((child) => planned.has(child.id)
        && child.workbench_handoff?.requirement_version === criteria.version
        && !child.workbench_handoff.superseded);
}
export function taskWorkspaceProgress(parent: Issue, children: readonly Issue[], runs: readonly AgentTask[]) {
    const active = runs.filter((run) => run.status === "running");
    const queued = runs.filter((run) => ["queued", "dispatched", "waiting_local_directory"].includes(run.status));
    const latest = new Map<string, AgentTask>();
    for (const run of runs) {
        const previous = latest.get(run.issue_id);
        if (!previous || run.created_at > previous.created_at)
            latest.set(run.issue_id, run);
    }
    const paused = children.filter((child) => !!child.workbench_pause);
    const attention = children.filter((child) => !child.workbench_pause && (child.status === "blocked"
        || latest.get(child.id)?.status === "failed"
        || (["todo", "backlog"].includes(child.status) && !child.assignee_id)));
    const startable = children.filter((child) => ["todo", "backlog"].includes(child.status)
        && child.assignee_type === "agent" && !!child.assignee_id
        && !child.workbench_pause && !latest.has(child.id));
    return { step: workbenchNextStep(parent).step, active, queued, paused, attention, startable };
}
export function conversationCommand(content: string): "pause" | "resume" | undefined {
    const text = content.trim().replace(/[。！!?.？]+$/, "").toLowerCase();
    if (/^(暂停|暂停任务|暂停部署|暂停执行|暂停一下|先暂停|先暂停一下|先停一下|停一下|停止任务|pause|pause task|stop)$/.test(text))
        return "pause";
    if (/^(继续|继续吧|继续任务|继续部署|继续执行|恢复任务|恢复执行|resume|resume task|continue)$/.test(text))
        return "resume";
    return undefined;
}
export function conversationDestination(parent: Issue, children: readonly Issue[], content: string): string | undefined {
    if (/修改需求|调整需求|需求变更|整体方案|总体方案|change (the )?requirements|overall plan/i.test(content))
        return parent.id;
    const named = children.filter((child) => child.identifier && content.toLowerCase().includes(child.identifier.toLowerCase()));
    if (named.length === 1)
        return named[0]!.id;
    if (named.length > 1)
        return undefined;
    if (children.length === 1)
        return children[0]!.id;
    return children.length ? undefined : parent.id;
}
