"use client";

import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { childIssuesOptions } from "@multica/core/issues/queries";
import { workbenchPipeline } from "@multica/core/workbench/configuration";
import type { AcceptanceCriteria, Issue } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { AppLink } from "../../navigation";
import { useT } from "../../i18n";

export function WorkbenchPipeline({ issue }: { issue: Issue }) {
  if (issue.parent_issue_id || !issue.acceptance_criteria?.team) return null;
  return <PipelineContent issue={issue} criteria={issue.acceptance_criteria} />;
}

function PipelineContent({ issue, criteria }: {
  issue: Issue;
  criteria: AcceptanceCriteria;
}) {
  const { t } = useT("issues");
  const { t: projectT } = useT("projects");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const children = useQuery({
    ...childIssuesOptions(wsId, issue.id),
    enabled: !!criteria.plan,
  });
  const loading = !!criteria.plan && children.isPending;
  const stages = workbenchPipeline(criteria, children.data ?? []);
  return (
    <section className="space-y-3 rounded-xl border bg-surface p-3" aria-label={t(($) => $.pipeline.title)}>
      <h3 className="text-label font-medium">{t(($) => $.pipeline.title)}</h3>
      <AppLink href={paths.issuePipeline(issue.id)} className="block text-caption text-primary hover:underline">
        {t(($) => $.pipeline_page.open)}
      </AppLink>
      {loading && <p role="status" className="text-caption">{t(($) => $.pipeline.loading)}</p>}
      {children.isError && (
        <div role="alert" className="space-y-2 text-caption">
          <p>{t(($) => $.pipeline.failed)}</p>
          <Button size="sm" variant="outline" onClick={() => void children.refetch()}>{t(($) => $.pipeline_page.retry)}</Button>
        </div>
      )}
      {!loading && !children.isError && <>
          {(!criteria.plan || criteria.needs_resplit || criteria.change) && <p className="text-caption text-muted-foreground">{t(($) => $.pipeline.unplanned)}</p>}
          <ol className="space-y-2">
            {stages.map((stage) => (
              <li key={stage.role} className="text-caption">
              <div className="flex items-center justify-between gap-2">
                <span className="font-medium">{projectT(($) => $.roles[stage.role])}</span>
                <span className={stage.state === "blocked" ? "text-destructive" : stage.state === "complete" ? "text-success" : "text-muted-foreground"}>
                  {t(($) => $.pipeline.states[stage.state])}
                </span>
              </div>
              {stage.tasks.length > 0 && (
                <ul className="mt-1 space-y-1">
                  {stage.tasks.map((task) => (
                    <li key={task.id}>
                      <AppLink href={paths.issueDetail(task.id)} className="block truncate text-primary hover:underline" title={task.title}>
                        {task.identifier} · {task.title}
                      </AppLink>
                    </li>
                  ))}
                </ul>
              )}
              </li>
            ))}
          </ol>
        </>}
    </section>
  );
}
