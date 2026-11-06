import type { AcceptanceCriteria, Issue } from "../types";
import { issueStatusCategory } from "../issues/status-category";
import { projectRoleOrder } from "./schemas";

export type PipelineState = "waiting" | "running" | "review" | "complete" | "blocked" | "paused" | "stopping" | "unknown" | "resuming" | "cancelled" | "unavailable";

export function workbenchPipeline(criteria: AcceptanceCriteria, children: Issue[]) {
  if (!criteria.team) return [];
  const currentPlan = criteria.plan?.version === criteria.version && !criteria.change && !criteria.needs_resplit;
  const plannedIds = new Set(currentPlan ? criteria.plan?.issues : []);
  const planned = children.filter((child) => plannedIds.has(child.id)
    && child.workbench_handoff?.requirement_version === criteria.version && !child.workbench_handoff.superseded);
  const missing = plannedIds.size !== planned.length;
  return projectRoleOrder.filter((role) => criteria.team!.config.roles[role].enabled).map((role) => {
    const tasks = planned.filter((child) => child.workbench_handoff?.role === role);
    let state: PipelineState = "waiting";
    if (role === "product" && criteria.gate.spec) state = "complete";
    if (role === "sched" && currentPlan) state = "complete";
    if (tasks.length) {
      if (tasks.some((task) => task.status === "blocked")) state = "blocked";
      else if (tasks.some((task) => task.workbench_pause?.phase === "unknown")) state = "unknown";
      else if (tasks.some((task) => task.workbench_pause?.phase === "stopping")) state = "stopping";
      else if (tasks.some((task) => task.workbench_pause?.phase === "resuming")) state = "resuming";
      else if (tasks.some((task) => task.status === "paused")) state = "paused";
      else if (tasks.some((task) => task.status === "in_review")) state = "review";
      else if (tasks.some((task) => issueStatusCategory(task) === "started")) state = "running";
      else if (tasks.every((task) => issueStatusCategory(task) === "done")) state = "complete";
      else if (tasks.some((task) => issueStatusCategory(task) === "closed")) state = "cancelled";
    }
    // A missing planned task cannot count as completed evidence for any role.
    if (missing && role !== "product" && role !== "sched") state = "unavailable";
    return { role, state, tasks };
  });
}
