"use client";

import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { useActorName } from "@multica/core/workspace/hooks";
import { projectRolesOptions, projectRoleOrder } from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { AppLink } from "../../navigation";
import { useT } from "../../i18n";

export function ProjectTeamPanel({ projectId, onConfigure }: {
  projectId: string;
  onConfigure: () => void;
}) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const { getActorName } = useActorName();
  const roles = useQuery(projectRolesOptions(wsId, projectId));
  const enabled = projectRoleOrder.filter((role) => roles.data?.config.roles[role].enabled);
  return (
    <aside className="space-y-4 p-4">
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-body font-medium">{t(($) => $.roles.title)}</h2>
        <span className="text-caption text-muted-foreground">
          {enabled.length} · {t(($) => $.overview.roles)}
        </span>
      </div>
      {roles.isPending && <div role="status" className="h-40 animate-pulse rounded-xl bg-muted" />}
      {roles.isError && (
        <div role="alert">
          <p>{t(($) => $.workbench.load_failed)}</p>
          <Button variant="outline" onClick={() => void roles.refetch()}>
            {t(($) => $.workbench.retry)}
          </Button>
        </div>
      )}
      {enabled.map((role) => {
        const binding = roles.data!.config.roles[role];
        return (
          <div key={role} className="space-y-2 rounded-xl border border-surface-border bg-surface p-3 shadow-[var(--surface-shadow)]">
            <div className="flex items-center gap-2">
              <span className="grid size-8 place-items-center rounded-lg bg-brand/12 text-caption text-brand-text">
                {t(($) => $.roles[role]).slice(0, 1)}
              </span>
              <h3 className="text-label font-medium">{t(($) => $.roles[role])}</h3>
            </div>
            {binding.agent_id ? (
              <AppLink href={paths.agentDetail(binding.agent_id)} className="text-caption text-brand-text hover:underline">
                {getActorName("agent", binding.agent_id)}
              </AppLink>
            ) : (
              <button type="button" onClick={onConfigure} className="text-caption text-muted-foreground hover:text-brand-text">
                {t(($) => $.workbench.not_set)}
              </button>
            )}
          </div>
        );
      })}
      <Button variant="outline" className="w-full" onClick={onConfigure}>
        {t(($) => $.overview.settings)}
      </Button>
    </aside>
  );
}
