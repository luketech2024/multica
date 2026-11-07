"use client";

import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { Issue, AcceptanceCriteria } from "@multica/core/types";
import { useWorkspaceId } from "@multica/core/hooks";
import { useCanonicalIssue } from "@multica/core/issues/canonical-id";
import { childIssuesOptions, issueKeys } from "@multica/core/issues/queries";
import { useWorkspacePaths } from "@multica/core/paths";
import { workbenchPipeline } from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { Badge } from "@multica/ui/components/ui/badge";
import { AppLink, useNavigation } from "../../navigation";
import { useT } from "../../i18n";
import { useStatusLabel } from "../utils/status-label";
import { WorkbenchHandoffPanel } from "./workbench-handoff-panel";
import { ExecutionPlanEditor } from "./execution-plan-editor";
import { DeliveryEvidence } from "./delivery-evidence";
import { ExecutionLogSection } from "./execution-log-section";
import { PullRequestsSection } from "./pull-requests-section";
import { ScanReportForm } from "../../security/scan-report-form";

export function IssuePipelinePage({ routeId }: { routeId: string }) {
  const { t } = useT("issues");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const qc = useQueryClient();
  const { issue, canonicalId, isResolving, notFound } = useCanonicalIssue(wsId, routeId);
  const detailState = qc.getQueryState(issueKeys.detail(wsId, canonicalId ?? routeId));
  if (isResolving || (!issue && detailState?.status === "pending")) {
    return <div className="space-y-4 p-6" role="status" aria-label={t(($) => $.pipeline.loading)}>
      <div className="h-10 animate-pulse rounded-xl bg-muted" />
      <div className="h-64 animate-pulse rounded-xl bg-muted" />
    </div>;
  }
  if (notFound || !issue) {
    return <div role="alert" className="space-y-3 p-6">
      <p>{t(($) => $.pipeline.failed)}</p>
      <p className="text-caption text-muted-foreground">{t(($) => $.pipeline_page.error_impact)}</p>
      <Button variant="outline" onClick={() => void qc.refetchQueries({ queryKey: issueKeys.detail(wsId, canonicalId ?? routeId) })}>
        {t(($) => $.pipeline_page.retry)}
      </Button>
    </div>;
  }
  if (!issue.acceptance_criteria?.team || issue.parent_issue_id) {
    return <div className="space-y-3 p-6">
      <p>{t(($) => $.pipeline_page.unavailable)}</p>
      <AppLink href={paths.issueDetail(issue.parent_issue_id ?? issue.id)} className="text-primary hover:underline">
        {t(($) => $.pipeline_page.open_task)}
      </AppLink>
    </div>;
  }
  return <PipelineWorkspace key={issue.id} issue={issue} criteria={issue.acceptance_criteria} />;
}

function PipelineWorkspace({ issue, criteria }: { issue: Issue; criteria: AcceptanceCriteria }) {
  const { t } = useT("issues");
  const { t: projectT } = useT("projects");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const statusLabel = useStatusLabel(wsId);
  const children = useQuery({ ...childIssuesOptions(wsId, issue.id), enabled: !!criteria.plan });
  const stages = workbenchPipeline(criteria, children.data ?? []);
  const plannedChildren = stages.flatMap((entry) => entry.tasks);
  const stage = stages.find((entry) => entry.role === navigation.searchParams.get("stage"))
    ?? stages.find((entry) => ["running", "blocked", "review"].includes(entry.state))
    ?? stages[0];
  const selectedTask = stage?.tasks.find((child) => child.id === navigation.searchParams.get("task")) ?? stage?.tasks[0];
  const select = (role: string, taskId?: string) => {
    const params = new URLSearchParams({ stage: role });
    if (taskId) params.set("task", taskId);
    navigation.replace(`${paths.issuePipeline(issue.id)}?${params}`);
  };
  const loading = !!criteria.plan && children.isPending;

  return <main className="min-h-0 flex-1 overflow-y-auto p-4 sm:p-6">
    <header className="mb-6 space-y-3">
      <AppLink href={paths.issueDetail(issue.id)} className="text-caption text-primary hover:underline">
        {t(($) => $.pipeline_page.open_task)}
      </AppLink>
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="min-w-0 break-words text-display-sm font-medium">{issue.identifier} · {issue.title}</h1>
        <Badge variant="neutral">{statusLabel(issue.status)}</Badge>
      </div>
      <div className="flex gap-1" aria-label={t(($) => $.pipeline.title)}>
        {stages.map((entry) => <span key={entry.role} title={projectT(($) => $.roles[entry.role])}
          className={`h-1.5 min-w-0 flex-1 rounded-full ${entry.state === "complete" ? "bg-success" : entry.state === "blocked" ? "bg-destructive" : entry.state === "running" ? "bg-primary" : "bg-muted"}`} />)}
      </div>
    </header>
    <div className="grid min-w-0 grid-cols-1 items-start gap-4 lg:grid-cols-[230px_minmax(0,1fr)] xl:grid-cols-[230px_minmax(0,1fr)_292px]">
      <nav className="relative space-y-2 rounded-xl border border-surface-border bg-surface p-3 shadow-[var(--surface-shadow)]" aria-label={t(($) => $.pipeline_page.stages)}>
        {stages.map((entry) => <Button key={entry.role} variant={stage?.role === entry.role ? "brandSubtle" : "ghost"}
          className="h-auto w-full justify-between gap-2 whitespace-normal py-3" aria-current={stage?.role === entry.role ? "step" : undefined}
          onClick={() => select(entry.role)}>
          <span>{projectT(($) => $.roles[entry.role])}</span>
          <span className="text-caption">{t(($) => $.pipeline.states[entry.state])}</span>
        </Button>)}
      </nav>
      <section className="min-w-0 space-y-4" aria-label={t(($) => $.pipeline_page.outputs)}>
        {loading && <div role="status" aria-label={t(($) => $.pipeline.loading)} className="h-64 animate-pulse rounded-xl bg-muted" />}
        {children.isError && <div role="alert" className="space-y-3 rounded-xl border p-4">
          <p>{t(($) => $.pipeline.failed)}</p>
          <p className="text-caption text-muted-foreground">{t(($) => $.pipeline_page.error_impact)}</p>
          <Button variant="outline" onClick={() => void children.refetch()}>{t(($) => $.pipeline_page.retry)}</Button>
        </div>}
        {!loading && !children.isError && stage && <>
          {plannedChildren.length === 0 && stage.role !== "sched" && <div className="space-y-3 rounded-xl border bg-surface p-4">
            <h2 className="text-title-sm font-medium">{t(($) => $.pipeline_page.empty_title)}</h2>
            <p className="text-caption text-muted-foreground">{t(($) => $.pipeline_page.empty_description)}</p>
            <Button variant="outline" onClick={() => select("sched")}>{t(($) => $.pipeline_page.go_plan)}</Button>
          </div>}
          <h2 className="text-title-sm font-medium">{projectT(($) => $.roles[stage.role])}</h2>
          {(!criteria.plan || criteria.change || criteria.needs_resplit) && <p className="text-caption text-muted-foreground">{t(($) => $.pipeline.unplanned)}</p>}
          {stage.role === "product" && <RequirementOutput criteria={criteria} tasks={plannedChildren} />}
          {stage.role === "sched" && <>
            {!criteria.gate.spec && <div className="space-y-3 rounded-xl border bg-surface p-4">
              <h3 className="text-title-sm font-medium">{t(($) => $.execution_plan.empty_title)}</h3>
              <p className="text-caption text-muted-foreground">{t(($) => $.execution_plan.empty_description)}</p>
              <AppLink href={paths.taskList()} className="text-caption text-primary hover:underline">{t(($) => $.execution_plan.open_tasks)}</AppLink>
            </div>}
            <ExecutionPlanEditor issue={issue} />
            <RequirementOutput criteria={criteria} tasks={plannedChildren} />
            {criteria.plan && <details className="rounded-xl border bg-surface p-3 text-caption">
              <summary className="cursor-pointer">{t(($) => $.pipeline_page.handoff_json)}</summary>
              <pre className="mt-3 max-h-96 overflow-auto whitespace-pre-wrap break-all">{JSON.stringify(plannedChildren.map((child) => ({ task_id: child.id, ...child.workbench_handoff })), null, 2)}</pre>
            </details>}
          </>}
          {stage.tasks.length > 0 ? <>
            <div className="flex flex-wrap gap-2">
              {stage.tasks.map((child) => <Button key={child.id} variant={selectedTask?.id === child.id ? "brandSubtle" : "outline"}
                size="sm" aria-pressed={selectedTask?.id === child.id} onClick={() => select(stage.role, child.id)}>
                {child.identifier}
              </Button>)}
            </div>
            {selectedTask && <TaskOutput key={selectedTask.id} parent={issue} task={selectedTask} />}
          </> : plannedChildren.length > 0 && <p className="rounded-xl border bg-surface p-4 text-caption text-muted-foreground">{t(($) => $.pipeline_page.no_task)}</p>}
        </>}
      </section>
      <aside className="min-w-0 space-y-4 lg:col-start-2 xl:col-start-auto" aria-label={t(($) => $.handoff.title)}>
        {!loading && !children.isError && selectedTask && <WorkbenchHandoffPanel issue={selectedTask} />}
        <div className="rounded-xl border bg-surface p-3 text-caption">
          <h3 className="mb-2 font-medium">{t(($) => $.pipeline_page.next)}</h3>
          <AppLink href={paths.issueDetail(selectedTask?.id ?? issue.id)} className="text-primary hover:underline">
            {t(($) => $.pipeline_page.open_task)}
          </AppLink>
        </div>
      </aside>
    </div>
  </main>;
}

function RequirementOutput({ criteria, tasks }: { criteria: AcceptanceCriteria; tasks: Issue[] }) {
  const { t } = useT("issues");
  const paths = useWorkspacePaths();
  return <div className="space-y-3 rounded-xl border bg-surface p-4 text-caption">
    <h3 className="font-medium">{t(($) => $.requirements.title)}</h3>
    {criteria.items.length === 0 && <p className="text-muted-foreground">{t(($) => $.requirements.empty)}</p>}
    <ul className="space-y-2">
      {criteria.items.map((item) => <li key={item.id} className="break-words">
        <span className="font-medium">{item.id}</span> · {item.text}
        {item.covers.length > 0 && <p className="text-muted-foreground">{t(($) => $.pipeline_page.coverage, { count: item.covers.length })}</p>}
        <div className="flex flex-wrap gap-2">
          {tasks.filter((child) => item.covers.includes(child.id)).map((child) => (
            <AppLink key={child.id} href={paths.issueDetail(child.id)} className="text-primary hover:underline">{child.identifier}</AppLink>
          ))}
        </div>
      </li>)}
    </ul>
    {criteria.bounds.length > 0 && <>
      <h3 className="font-medium">{t(($) => $.pipeline_page.bounds)}</h3>
      <ul className="list-inside list-disc space-y-1">{criteria.bounds.map((bound, index) => <li key={index}>{bound}</li>)}</ul>
    </>}
  </div>;
}

function TaskOutput({ parent, task }: { parent: Issue; task: Issue }) {
  const [pullRequestsOpen, setPullRequestsOpen] = useState(true);
  return <div className="space-y-4">
    <h3 className="break-words text-label font-medium">{task.title}</h3>
    <DeliveryEvidence issue={parent} taskIds={[task.id]} />
    {task.workbench_handoff?.role === "sec" && <ScanReportForm task={task} />}
    {task.workbench_handoff?.role === "dev" && (
      <PullRequestsSection issueId={task.id} identifier={task.identifier} open={pullRequestsOpen} onOpenChange={setPullRequestsOpen} />
    )}
    <ExecutionLogSection issueId={task.id} identifier={task.identifier} issueTitle={task.title} />
  </div>;
}
