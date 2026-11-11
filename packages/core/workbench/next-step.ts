import type { Issue } from "../types";
import { issueStatusCategory } from "../issues/status-category";

export type WorkbenchDestination = "requirements" | "review" | "execution" | "activity" | "children";
export type WorkbenchStep = "requirements" | "plan" | "change" | "running" | "blocked" | "paused" | "review" | "recovery" | "accepted" | "closed" | "task";

/** Business gates take precedence over the outcome of any single agent run. */
export function workbenchNextStep(issue: Issue): { step: WorkbenchStep; destination: WorkbenchDestination } {
  const category = issueStatusCategory(issue);
  const criteria = issue.acceptance_criteria;
  const root = !issue.parent_issue_id && !issue.workbench_handoff;
  const parent = !!issue.project_id && root;
  if (category === "closed") return { step: "closed", destination: "activity" };
  if (criteria?.change) return { step: "change", destination: "requirements" };
  if (issue.workbench_pause || (issue.status === "paused" && (criteria || issue.workbench_handoff))) return { step: "paused", destination: "execution" };
  if (parent && criteria?.gate.accept) return { step: "accepted", destination: "review" };
  if (parent && ["in_review", "done"].includes(issue.status) && !criteria?.plan) {
    return { step: "recovery", destination: "review" };
  }
  if (parent && ["in_review", "done"].includes(issue.status)) return { step: "review", destination: "review" };
  if (issue.status === "blocked") return { step: "blocked", destination: "activity" };
  if (root && criteria && !criteria.gate.spec && ["todo", "backlog", "unstarted"].includes(category ?? "")) {
    return { step: "requirements", destination: "requirements" };
  }
  if (parent && criteria?.team && criteria.gate.spec && (!criteria.plan || criteria.plan.version !== criteria.version || criteria.needs_resplit)) {
    return { step: "plan", destination: "requirements" };
  }
  if (category === "started") return { step: "running", destination: parent ? "children" : "activity" };
  return { step: "task", destination: "activity" };
}
