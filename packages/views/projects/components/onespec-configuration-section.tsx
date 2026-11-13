"use client";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useCurrentMember } from "@multica/core/permissions";
import { isRuntimeUsableForUser, runtimeDisplayLabel, runtimeListOptions } from "@multica/core/runtimes";
import { projectResourcesOptions } from "@multica/core/projects";
import {
  oneSpecConfigurationOptions,
  useSaveOneSpecConfiguration,
  useProvisionOneSpecTeam,
  type OneSpecConfiguration,
} from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { Switch } from "@multica/ui/components/ui/switch";
import { ConfigurationSelect } from "./configuration-select";
import { useT } from "../../i18n";

export function OneSpecConfigurationSection({
  projectId,
}: {
  projectId?: string;
}) {
  const wsId = useWorkspaceId();
  const { t } = useT("projects");
  const member = useCurrentMember(wsId);
  const canEdit = (
    projectId ? ["owner", "admin", "member"] : ["owner", "admin"]
  ).includes(member.role ?? "");
  const saved = useQuery(oneSpecConfigurationOptions(wsId, projectId));
  const runtimes = useQuery(runtimeListOptions(wsId));
  const repositories = useQuery({
    ...projectResourcesOptions(wsId, projectId ?? ""),
    enabled: !!projectId,
  });
  const save = useSaveOneSpecConfiguration(wsId, projectId);
  const team = useProvisionOneSpecTeam(wsId, projectId);
  const [draft, setDraft] = useState<OneSpecConfiguration>();
  const current = draft ?? saved.data;
  const busy = save.isPending || team.isPending;
  const runtimeItems = [
    {
      value: "",
      label: projectId
        ? t(($) => $.onespec.inherit)
        : t(($) => $.onespec.no_runtime),
    },
    ...(runtimes.data ?? []).filter((runtime) =>
      isRuntimeUsableForUser(runtime, member.userId) || runtime.id === (projectId ? current?.runtime_id : current?.default_runtime_id),
    ).map((runtime) => ({
      value: runtime.id,
      label: `${runtimeDisplayLabel(runtime)} · ${runtime.status}`,
    })),
  ];
  const update = (patch: Partial<OneSpecConfiguration>) => {
    if (current) setDraft({ ...current, ...patch });
  };
  const setupStatus = (status: string) => {
    switch (status) {
      case "ready": return t(($) => $.onespec.setup_ready);
      case "pending_runtime": return t(($) => $.onespec.setup_pending_runtime);
      case "needs_setup": return t(($) => $.onespec.setup_needs_setup);
      case "disabled": return t(($) => $.onespec.setup_disabled);
      case "access_denied": return t(($) => $.onespec.setup_access_denied);
      case "archived": return t(($) => $.onespec.setup_archived);
      case "runtime_offline": return t(($) => $.onespec.setup_runtime_offline);
      default: return t(($) => $.onespec.setup_binding_invalid);
    }
  };
  return (
    <section
      className="space-y-3 rounded-lg border p-4"
      aria-label={t(($) => $.onespec.title)}
    >
      <h3 className="text-label font-medium">
        {projectId
          ? t(($) => $.onespec.title)
          : t(($) => $.onespec.default_runtime)}
      </h3>
      {saved.isError && (
        <p role="alert">
          {t(($) => $.onespec.error)}{" "}
          <Button variant="ghost" onClick={() => void saved.refetch()}>
            {t(($) => $.onespec.retry)}
          </Button>
        </p>
      )}
      {current && (
        <fieldset disabled={!canEdit || busy} className="space-y-3">
          {projectId && (
            <label className="flex items-center gap-2">
              <Switch
                checked={current.enabled}
                onCheckedChange={(enabled) => update({ enabled })}
              />
              {t(($) => $.onespec.enabled)}
            </label>
          )}
          <ConfigurationSelect
            label={t(($) => $.onespec.runtime)}
            value={
              (projectId ? current.runtime_id : current.default_runtime_id) ??
              ""
            }
            items={runtimeItems}
            onChange={(value) =>
              update(
                projectId
                  ? { runtime_id: value || null }
                  : { default_runtime_id: value || null },
              )
            }
          />
          {projectId && (
            <>
              <p className="text-caption text-muted-foreground">
                {t(($) => $.onespec.effective)}{" "}
                {runtimes.data?.find(
                  (runtime) => runtime.id === current.effective_runtime_id,
                )?.name ?? t(($) => $.onespec.no_runtime)}
              </p>
              <ConfigurationSelect
                label={t(($) => $.onespec.overview)}
                value={current.overview_repository_id ?? ""}
                items={[
                  { value: "", label: t(($) => $.onespec.no_runtime) },
                  ...(repositories.data ?? [])
                    .filter((repo) =>
                      ["git_repo", "github_repo", "local_directory"].includes(
                        repo.resource_type,
                      ),
                    )
                    .map((repo) => ({
                      value: repo.id,
                      label: repo.label || repo.id,
                    })),
                ]}
                onChange={(value) =>
                  update({ overview_repository_id: value || null })
                }
              />
              <p className="text-caption text-muted-foreground">
                {t(($) => $.onespec.snapshot)}
              </p>
            </>
          )}
          <div className="flex flex-wrap gap-2">
            <Button
              disabled={!draft || busy}
              aria-busy={save.isPending}
              onClick={() =>
                save.mutate(current, { onSuccess: () => setDraft(undefined) })
              }
            >
              {t(($) => $.onespec.save)}
            </Button>
            {(!projectId || current.enabled) && (
              <Button
                variant="outline"
                disabled={!!draft || busy}
                aria-busy={team.isPending}
                onClick={() => team.mutate()}
              >
                {t(($) => $.onespec.team)}
              </Button>
            )}
          </div>
        </fieldset>
      )}
      {saved.data?.setup && (!projectId || saved.data.enabled) && (
        <div className="space-y-2">
          <h4 className="text-label font-medium">{t(($) => $.onespec.builtin_team)}</h4>
          <ul className="space-y-1">
            {saved.data.setup.roles.map((role) => (
              <li key={role.key} className="flex flex-wrap justify-between gap-2 text-caption">
                <span>{role.name}</span>
                <span className="text-muted-foreground">{setupStatus(role.status)}</span>
              </li>
            ))}
          </ul>
        </div>
      )}
      {(save.error || team.error) && (
        <p role="alert" className="text-caption text-destructive">
          {save.error?.message ?? team.error?.message}
        </p>
      )}
    </section>
  );
}
