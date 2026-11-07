"use client";

import { useQueries, useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import type { Project, IssueTableFilters } from "@multica/core/types";
import { useIssueStatuses } from "@multica/core/issue-statuses/hooks";
import { issueTableFacetsOptions } from "@multica/core/issues/queries";
import { projectRolesOptions } from "@multica/core/workbench/configuration";
import { workbenchDayRange } from "@multica/core/workbench/status-band";
import { useT } from "../../i18n";

export function ProjectWorkbenchMetrics({ project, expanded = false }: {
  project: Project;
  expanded?: boolean;
}) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const catalog = useIssueStatuses(wsId);
  const roles = useQuery({ ...projectRolesOptions(wsId, project.id), enabled: !expanded });
  const now = new Date();
  const day = workbenchDayRange(now);
  const weekStart = new Date(now.getFullYear(), now.getMonth(), now.getDate() - (now.getDay() + 6) % 7);
  const weekEnd = new Date(weekStart.getFullYear(), weekStart.getMonth(), weekStart.getDate() + 7);
  const unfinished = catalog.statuses.filter((status) => !["done", "closed"].includes(status.category)).map((status) => status.key);
  const filters: IssueTableFilters[] = [
    { workbench: { band: "running", only_mine: false, ...day } },
    { workbench: { band: "review", only_mine: true, ...day } },
    { statuses: unfinished },
    { date: { field: "completed_at", start: weekStart.toISOString(), end: weekEnd.toISOString() } },
  ];
  const counts = useQueries({
    queries: filters.map((filter, index) => ({
      ...issueTableFacetsOptions(wsId, {
        query: {
          scope: { kind: "project", project_id: project.id },
          filters: filter,
          sort: { field: "updated_at", direction: "desc" },
        },
        facets: [{ kind: "status" }],
        include_total: true,
      }),
      enabled: index < 2 || (expanded && catalog.isLoaded),
      refetchInterval: 30000,
    })),
  });
  const count = (index: number) => counts[index]?.isError ? "—" : counts[index]?.data?.total ?? "—";
  const values = [
    { label: t(($) => $.overview.running), value: count(0), color: "text-brand-text" },
    { label: t(($) => $.overview.review), value: count(1), color: "text-warning" },
    ...(expanded ? [
      { label: t(($) => $.overview.unfinished), value: catalog.isError ? "—" : count(2), color: "text-foreground" },
      { label: t(($) => $.overview.weekly), value: count(3), color: "text-success" },
    ] : [
      { label: t(($) => $.overview.roles), value: roles.data ? Object.values(roles.data.config.roles).filter((role) => role.enabled).length : "—", color: "text-foreground" },
    ]),
  ];
  return (
    <dl className={expanded ? "grid grid-cols-4 gap-4" : "flex flex-wrap items-center gap-x-3 gap-y-1 text-caption"}>
      {values.map(({ label, value, color }) => (
        <div key={label} className={expanded ? "flex flex-col gap-1" : "flex items-baseline gap-1"}>
          <dd className={`${expanded ? "text-title-lg" : "text-caption"} font-medium tabular-nums ${color}`}>{value}</dd>
          <dt className="text-caption text-muted-foreground">{label}</dt>
        </div>
      ))}
    </dl>
  );
}
