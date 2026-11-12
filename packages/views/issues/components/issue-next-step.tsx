"use client";

import type { ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import type { AgentTask, Issue } from "@multica/core/types";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { useActorName } from "@multica/core/workspace/hooks";
import { useAgentPresenceDetail } from "@multica/core/agents";
import { useRetryIssueRun } from "@multica/core/issues/mutations";
import { workbenchNextStep, deliveryReviewOptions, releaseReadinessOptions, type WorkbenchDestination } from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";
import { AppLink } from "../../navigation";

export function IssueNextStep({ issue, onNavigate, blockedAction, statusLabel, latestRun, onShowRun, executionTasks = [], runsLoaded = false }: {
  issue: Issue;
  statusLabel: string;
  onNavigate: (destination: WorkbenchDestination) => void;
  blockedAction?: ReactNode;
  latestRun?: AgentTask;
  executionTasks?: Issue[];
  runsLoaded?: boolean;
  onShowRun?: (id: string) => void;
}) {
  const { t } = useT("issues");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const { getActorName } = useActorName();
  const { step, destination } = workbenchNextStep(issue);
  const start = useRetryIssueRun(issue.id);
  const plannedTasks = executionTasks.filter((task) => issue.acceptance_criteria?.plan?.issues.includes(task.id) && !task.workbench_handoff?.superseded);
  const singleTask = step === "planned" && issue.acceptance_criteria?.plan?.issues.length === 1 && plannedTasks.length === 1 ? plannedTasks[0] : undefined;
  const canStart = step === "task" && !!issue.workbench_handoff && !issue.workbench_handoff.superseded
    && ["todo", "backlog"].includes(issue.status) && issue.assignee_type === "agent" && !!issue.assignee_id && runsLoaded && !latestRun;
  const reviewing = step === "review" || step === "recovery";
  const review = useQuery({ ...deliveryReviewOptions(wsId, issue.id, issue.revision), enabled: reviewing });
  const release = useQuery({ ...releaseReadinessOptions(wsId, issue.id, issue.revision), enabled: step === "accepted" });
  const query = step === "accepted" ? release : review;
  const checking = reviewing || step === "accepted";
  const current = query.data?.revision === issue.revision;
  const blocked = current ? query.data?.blockers.length : undefined;
  return <section data-testid="issue-next-step" aria-label={t(($) => $.next_step.title)} className="shrink-0 border-b bg-background px-6 py-3">
    <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
      <div className="min-w-0 flex-1">
        <p role="status" className="text-label font-medium">{step === "task" ? statusLabel : t(($) => $.next_step.stages[step])}</p>
        {issue.assignee_id && issue.assignee_type && <p className="break-words text-caption text-muted-foreground">
          {t(($) => $.next_step.assignee, { name: getActorName(issue.assignee_type, issue.assignee_id) })}
        </p>}
      </div>
      {canStart ? <Button disabled={start.isPending || start.isSuccess} onClick={() => start.mutate(undefined)}>{t(($) => start.isPending || start.isSuccess ? $.next_step.starting : $.next_step.start_execution)}</Button>
        : singleTask ? <AppLink className="rounded-md bg-primary px-3 py-2 text-label text-primary-foreground" href={paths.issueDetail(singleTask.id)}>{t(($) => $.next_step.open_execution, { identifier: singleTask.identifier })}</AppLink>
        : step === "blocked" && blockedAction ? blockedAction : step === "accepted" && release.data?.revision === issue.revision && blocked === 0 && !release.isError && issue.project_id
        ? <AppLink className="rounded-md bg-primary px-3 py-2 text-label text-primary-foreground" href={`${paths.releases()}?project=${encodeURIComponent(issue.project_id)}`}>{t(($) => $.release_readiness.join)}</AppLink>
        : <Button onClick={() => onNavigate(destination)}>{t(($) => $.next_step.actions[step])}</Button>}
    </div>
    {step === "planned" && <p className="mt-1 text-caption text-muted-foreground">{t(($) => $.next_step.planned_hint, { count: issue.acceptance_criteria?.plan?.issues.length ?? 0 })}{singleTask && ` ${singleTask.title}`}</p>}
    {canStart && <p className="mt-1 text-caption text-muted-foreground">{t(($) => $.next_step.start_hint)}</p>}
    {start.isError && <p role="alert" className="mt-1 text-caption text-destructive">{start.error.message}</p>}
    {step === "blocked" && <p className="mt-1 text-caption text-muted-foreground">{t(($) => $.discussion_picker.hint)}</p>}
    {latestRun && step !== "planned" && <div className="mt-1 text-caption">
      <Button size="sm" variant="ghost" onClick={() => onShowRun?.(latestRun.id)}>{t(($) => $.next_step.latest_run)} · {t(($) => $.wakeups.run_states[latestRun.status])}</Button>
      {latestRun.status === "failed" && latestRun.error && <p className="line-clamp-2 break-words text-destructive">{latestRun.error}</p>}
    </div>}
    {checking && query.isPending && <p role="status" className="mt-1 text-caption text-muted-foreground">{t(($) => $.delivery_review.loading)}</p>}
    {checking && (query.isError || (!!query.data && !current)) && <div role="alert" className="mt-1 flex flex-wrap items-center gap-2 text-caption">
      <span>{t(($) => $.next_step.read_failed)}</span>
      <Button size="sm" variant="outline" onClick={() => void query.refetch()}>{t(($) => $.release_readiness.retry)}</Button>
    </div>}
    {checking && !query.isError && blocked !== undefined && blocked > 0 && <p className="mt-1 text-caption text-warning">{t(($) => $.next_step.blockers, { count: blocked })}</p>}
    {step === "accepted" && release.data && !release.isError && blocked === 0 && <p className="mt-1 text-caption text-success">{t(($) => $.release_readiness.matched)}</p>}
    {reviewing && current && review.data?.blockers[0] && !review.isError && <p className="text-caption text-muted-foreground">{t(($) => $.delivery_review.blockers[review.data!.blockers[0]!.code])}</p>}
    {issue.assignee_type === "agent" && issue.assignee_id && <AssigneeConnection agentId={issue.assignee_id} />}
  </section>;
}

function AssigneeConnection({ agentId }: { agentId: string }) {
  const { t } = useT("issues");
  const wsId = useWorkspaceId();
  const presence = useAgentPresenceDetail(wsId, agentId);
  if (presence === "loading" || presence.availability !== "offline") return null;
  return <p className="mt-1 text-caption text-warning">{t(($) => $.next_step.offline)}</p>;
}
