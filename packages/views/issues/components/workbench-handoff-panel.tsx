"use client";

import { useQueries, useQuery } from "@tanstack/react-query";
import type { Issue } from "@multica/core/types";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { issueDetailOptions } from "@multica/core/issues/queries";
import { projectResourcesOptions } from "@multica/core/projects";
import type { WorkbenchHandoff } from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { AppLink } from "../../navigation";
import { useT } from "../../i18n";

export function WorkbenchHandoffPanel({ issue }: { issue: Issue }) {
  const { t } = useT("issues");
  const handoff = issue.workbench_handoff;
  return (
    <section
      className="space-y-3 rounded-xl border bg-surface p-3"
      aria-label={t(($) => $.handoff.title)}
    >
      <h3 className="text-label font-medium">{t(($) => $.handoff.title)}</h3>
      {handoff ? (
        <HandoffContent issue={issue} handoff={handoff} />
      ) : (
        <p role="status" className="text-caption text-muted-foreground">
          {t(($) => $.handoff.unavailable)}
        </p>
      )}
    </section>
  );
}

function HandoffContent({
  issue,
  handoff,
}: {
  issue: Issue;
  handoff: WorkbenchHandoff;
}) {
  const { t } = useT("issues");
  const { t: projectT } = useT("projects");
  const paths = useWorkspacePaths();
  const wsId = useWorkspaceId();
  const resources = useQuery({
    ...projectResourcesOptions(wsId, issue.project_id ?? ""),
    enabled: !!issue.project_id && !!handoff.repo_id,
  });
  const parent = useQuery({
    ...issueDetailOptions(wsId, issue.parent_issue_id ?? ""),
    enabled: !!issue.parent_issue_id,
  });
  const dependencies = useQueries({
    queries: handoff.depends_on.map((id) => issueDetailOptions(wsId, id)),
  });
  const repository = resources.data?.find(
    (repo) => repo.id === handoff.repo_id,
  );
  const samePlan =
    parent.data?.acceptance_criteria?.plan?.version ===
      handoff.requirement_version &&
    !parent.data.acceptance_criteria.change &&
    !parent.data.acceptance_criteria.needs_resplit;
  const blockedBy = dependencies.filter(
    ({ data }) =>
      data?.status !== "done" ||
      data.project_id !== issue.project_id ||
      data.workbench_handoff?.superseded,
  );
  const loading =
    parent.isLoading ||
    resources.isLoading ||
    dependencies.some((query) => query.isLoading);
  const failed =
    parent.isError ||
    resources.isError ||
    dependencies.some((query) => query.isError);
  return (
    <div className="space-y-3 text-caption">
      <p>
        {projectT(($) => $.roles[handoff.role])} ·{" "}
        {t(($) => $.requirements.version, {
          version: handoff.requirement_version,
        })}
      </p>
      {handoff.superseded && (
        <p role="status" className="text-muted-foreground">
          {t(($) => $.handoff.retired)}
        </p>
      )}
      {loading && <p role="status">{t(($) => $.handoff.loading)}</p>}
      {failed && (
        <Button
          size="sm"
          variant="outline"
          onClick={() => {
            if (parent.isError) void parent.refetch();
            if (resources.isError) void resources.refetch();
            dependencies.forEach((query) => {
              if (query.isError) void query.refetch();
            });
          }}
        >
          {t(($) => $.handoff.retry)}
        </Button>
      )}
      <div>
        <h4 className="font-medium">{t(($) => $.execution_plan.repo)}</h4>
        {!resources.isLoading && (
          <p>
            {!handoff.repo_id
              ? t(($) => $.handoff.no_repo)
              : repository?.label || t(($) => $.handoff.repo_unavailable)}
          </p>
        )}
      </div>
      <div>
        <h4 className="font-medium">{t(($) => $.execution_plan.covers)}</h4>
        <ul className="space-y-1">
          {handoff.covers.map((id) => {
            const item = samePlan
              ? parent.data?.acceptance_criteria?.items.find(
                  (entry) => entry.id === id,
                )
              : undefined;
            return (
              <li key={id}>
                {id}
                {item ? ` · ${item.text}` : ""}
              </li>
            );
          })}
        </ul>
      </div>
      <div>
        <h4 className="font-medium">
          {t(($) => $.execution_plan.dependencies)}
        </h4>
        {handoff.depends_on.length === 0 ? (
          <p>{t(($) => $.handoff.no_dependencies)}</p>
        ) : (
          <>
            {!loading && !failed && !handoff.superseded && (
              <p>
                {blockedBy.length
                  ? t(($) => $.handoff.waiting, { count: blockedBy.length })
                  : t(($) => $.handoff.dependencies_ready)}
              </p>
            )}
            {!dependencies.some((query) => query.isLoading) && (
              <ul>
                {handoff.depends_on.map((id, index) => {
                  const dependency = dependencies[index]?.data;
                  return (
                    <li key={id}>
                      {dependency?.project_id === issue.project_id &&
                      !dependency.workbench_handoff?.superseded ? (
                        <AppLink
                          href={paths.issueDetail(dependency.id)}
                          className="text-foreground underline-offset-4 hover:underline"
                        >
                          {dependency.identifier} · {dependency.title}
                        </AppLink>
                      ) : (
                        t(($) => $.handoff.dependency_unavailable)
                      )}
                    </li>
                  );
                })}
              </ul>
            )}
          </>
        )}
      </div>
      {handoff.serial_group && (
        <p>{t(($) => $.handoff.serial, { group: handoff.serial_group })}</p>
      )}
      <div>
        <h4 className="font-medium">{t(($) => $.handoff.deliverables)}</h4>
        <ul className="space-y-1">
          {handoff.expected.map((entry, index) => (
            <li key={`${entry}-${index}`}>{entry}</li>
          ))}
        </ul>
      </div>
      <p className="text-muted-foreground">
        {t(($) => $.handoff.retries, { count: handoff.max_retry })}
      </p>
    </div>
  );
}
