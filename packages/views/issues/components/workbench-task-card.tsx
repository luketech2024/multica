"use client";

import { useQuery } from "@tanstack/react-query";
import type { Issue, Project } from "@multica/core/types";
import { useWorkspaceId } from "@multica/core/hooks";
import { projectResourcesOptions } from "@multica/core/projects";
import { useActorName } from "@multica/core/workspace/hooks";
import { cn } from "@multica/ui/lib/utils";
import { ActorAvatar } from "../../common/actor-avatar";
import { useT, useTimeAgo } from "../../i18n";
import { StatusBadge } from "./status-heading";

export function WorkbenchTaskCardContent({ issue, project, compact = false }: {
  issue: Issue;
  project?: Project;
  compact?: boolean;
}) {
  const { t } = useT("issues");
  const timeAgo = useTimeAgo();
  const { getActorName } = useActorName();
  const wsId = useWorkspaceId();
  const repoId = issue.workbench_handoff?.repo_id;
  const resources = useQuery({
    ...projectResourcesOptions(wsId, issue.project_id ?? ""),
    enabled: !compact && !!issue.project_id && !!repoId,
  });
  const repository = resources.data?.find((resource) => resource.id === repoId);
  const ref = repository?.resource_ref;
  const repositoryName = repository?.label
    || (ref && "url" in ref && typeof ref.url === "string" ? ref.url.replace(/\.git$/, "").split("/").pop() : undefined)
    || (ref && "local_path" in ref && typeof ref.local_path === "string" ? ref.local_path.split(/[\\/]/).filter(Boolean).pop() : undefined);
  const context = [project?.title, repositoryName].filter(Boolean).join(" / ");
  const assignee = issue.assignee_type && issue.assignee_id
    ? getActorName(issue.assignee_type, issue.assignee_id)
    : t(($) => $.pickers.assignee.trigger_unassigned);

  return (
    <div
      data-workbench-card={compact ? "compact" : "flow"}
      title={[issue.identifier, issue.external_ticket_no, context].filter(Boolean).join(" · ")}
      className={cn(
        "flex min-w-0 flex-col gap-0.5 overflow-hidden rounded-xl border border-surface-border bg-surface px-3 py-2 shadow-[var(--surface-shadow)] transition-[box-shadow,transform] motion-safe:group-hover/card:-translate-y-px group-hover/card:shadow-[var(--surface-shadow-hover)] group-focus-visible/card:ring-2 group-focus-visible/card:ring-ring group-data-[peeked]/card:ring-2 group-data-[peeked]/card:ring-brand",
        compact ? "h-[72px]" : "h-[92px]",
      )}
    >
      <p title={issue.title} className={cn("shrink-0 text-body font-medium leading-[18px]", compact ? "h-[18px] truncate" : "h-[36px] line-clamp-2")}>
        {issue.title}
      </p>
      {!compact && <p className="truncate text-micro leading-[12px] text-muted-foreground" title={[issue.identifier, issue.external_ticket_no, context].filter(Boolean).join(" · ")}>
        {issue.identifier}
        {issue.external_ticket_no && ` · #${issue.external_ticket_no}`}
        {context && ` · ${context}`}
      </p>}
      <div className="mt-auto flex min-w-0 items-center gap-1.5 text-micro">
        <span className="min-w-0 max-w-[40%] truncate [&>[data-slot=badge]]:max-w-full [&>[data-slot=badge]]:overflow-hidden">
          <StatusBadge status={issue.status} />
        </span>
        <span className="flex min-w-0 flex-1 items-center gap-1" title={assignee}>
          {!compact && issue.assignee_type && issue.assignee_id && (
            <ActorAvatar actorType={issue.assignee_type} actorId={issue.assignee_id} size="xs" profileLink={false} />
          )}
          <span className="truncate text-muted-foreground">{assignee}</span>
        </span>
        {issue.status === "in_review" ? (
          <span className="shrink-0 rounded-full bg-brand/12 px-2 text-brand-text">{t(($) => $.workbench_card.review)}</span>
        ) : (
          <time dateTime={issue.updated_at} title={issue.updated_at} className="max-w-[30%] shrink-0 truncate text-micro text-muted-foreground">
            {timeAgo(issue.updated_at)}
          </time>
        )}
      </div>
    </div>
  );
}
