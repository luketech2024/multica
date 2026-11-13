import type { AcceptanceCriteria, Issue } from "../types";
import { issueStatusCategory } from "../issues/status-category";

export type WorkbenchDestination = "requirements" | "review" | "execution" | "activity" | "children";
export type WorkbenchStep = "discussion" | "requirements" | "plan" | "planned" | "change" | "running" | "blocked" | "paused" | "review" | "recovery" | "accepted" | "closed" | "task";

export function requirementsReady(criteria: AcceptanceCriteria | undefined): boolean {
  return !!criteria?.items.length && (!criteria.phase || (criteria.phase === "ready"
    && (criteria.draft_revision ?? 0) > 0 && criteria.ready_revision === criteria.draft_revision
    && (criteria.covered_discussion_revision ?? 0) === (criteria.discussion_revision ?? 0)));
}

export function requirementsReviewReady(criteria: AcceptanceCriteria | undefined, flow?: {
  phase: string; status: string; artifacts: { stage: string; draft_revision?: number; document?: { verified: boolean; prototype_status?: string; prototype?: { verified: boolean } | null } | null }[];
} | null): boolean {
  if (!requirementsReady(criteria)) return false;
  if (!flow) return true;
  const artifact = flow.artifacts.findLast((entry) => entry.stage === "requirements");
  return flow.phase === "requirements" && flow.status === "awaiting_confirmation" && !!artifact
    && (!criteria?.phase || artifact.draft_revision === criteria.draft_revision)
    && (!artifact.document || (artifact.document.verified && !["preparing", "failed"].includes(artifact.document.prototype_status ?? "") && (!artifact.document.prototype || artifact.document.prototype.verified)));
}

export function requirementsDiscussion(issue: Issue): boolean {
  return !!issue.project_id && !issue.parent_issue_id && !!issue.acceptance_criteria
    && !issue.acceptance_criteria.gate.spec && !issue.acceptance_criteria.change
    && ["todo", "backlog", "unstarted"].includes(issueStatusCategory(issue) ?? "");
}

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
    return requirementsReady(criteria) ? { step: "requirements", destination: "requirements" } : { step: "discussion", destination: "activity" };
  }
  if (parent && criteria?.team && criteria.gate.spec && (!criteria.plan || criteria.plan.version !== criteria.version || criteria.needs_resplit)) {
    return { step: "plan", destination: "requirements" };
  }
  if (category === "started") return { step: "running", destination: parent ? "children" : "activity" };
  if (parent && criteria?.gate.spec && criteria.plan?.version === criteria.version) {
    return { step: "planned", destination: "children" };
  }
  return { step: "task", destination: "activity" };
}
