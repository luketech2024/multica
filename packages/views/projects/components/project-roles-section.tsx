"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useCurrentMember } from "@multica/core/permissions";
import { agentListOptions } from "@multica/core/workspace/queries";
import { runtimeListOptions } from "@multica/core/runtimes";
import {
  projectRoleOrder,
  projectRolesOptions,
  useSaveProjectRoles,
  type ProjectRoles,
  type ProjectRoleKey,
} from "@multica/core/workbench/configuration";
import { Button } from "@multica/ui/components/ui/button";
import { Switch } from "@multica/ui/components/ui/switch";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { ConfigurationSelect } from "./configuration-select";
import { useT } from "../../i18n";

export function ProjectRolesSection({ projectId }: { projectId: string }) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const member = useCurrentMember(wsId);
  const canEdit = ["owner", "admin", "member"].includes(member.role ?? "");
  const saved = useQuery(projectRolesOptions(wsId, projectId));
  const agents = useQuery(agentListOptions(wsId));
  const runtimes = useQuery(runtimeListOptions(wsId));
  const save = useSaveProjectRoles(wsId, projectId);
  const [draft, setDraft] = useState<ProjectRoles>();
  const [error, setError] = useState<string>();
  const current = draft ?? saved.data;
  const none = { value: "", label: t(($) => $.workbench.not_set) };
  const agentItems = (agents.data ?? [])
    .filter((agent) => !agent.archived_at)
    .map((agent) => ({ value: agent.id, label: agent.name }));
  const runtimeItems = (runtimes.data ?? []).map((runtime) => ({
    value: runtime.id,
    label: runtime.name,
  }));
  const bindingItems = (
    items: { value: string; label: string }[],
    value: string | null,
  ) => [
    none,
    ...items,
    ...(value && !items.some((item) => item.value === value)
      ? [{ value, label: `${t(($) => $.roles.unavailable)} (${value})` }]
      : []),
  ];
  const change = (
    role: ProjectRoleKey,
    patch: Partial<ProjectRoles["config"]["roles"][ProjectRoleKey]>,
  ) => {
    if (!current) return;
    setDraft({
      ...current,
      config: {
        ...current.config,
        roles: {
          ...current.config.roles,
          [role]: { ...current.config.roles[role], ...patch },
        },
      },
    });
  };
  const submit = async () => {
    if (!draft) return;
    setError(undefined);
    try {
      await save.mutateAsync(draft);
      setDraft(undefined);
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : t(($) => $.workbench.load_failed),
      );
    }
  };

  return (
    <section className="space-y-3" aria-label={t(($) => $.roles.title)}>
      <h3 className="text-label font-medium">{t(($) => $.roles.title)}</h3>
      {saved.isPending && <Skeleton className="h-24 w-full" />}
      {saved.isError && (
        <p role="alert">
          {t(($) => $.workbench.load_failed)}{" "}
          <Button
            variant="ghost"
            size="sm"
            onClick={() => void saved.refetch()}
          >
            {t(($) => $.workbench.retry)}
          </Button>
        </p>
      )}
      {current && (
        <fieldset disabled={!canEdit || save.isPending} className="space-y-2">
          {projectRoleOrder.map((role) => {
            const entry = current.config.roles[role];
            return (
              <div
                key={role}
                className="space-y-2 rounded-xl border bg-surface p-3"
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="text-label">{t(($) => $.roles[role])}</span>
                  <Switch
                    checked={entry.enabled}
                    disabled={
                      !canEdit ||
                      save.isPending ||
                      role === "sched" ||
                      role === "product"
                    }
                    aria-label={t(($) => $.roles[role])}
                    onCheckedChange={(enabled) => change(role, { enabled })}
                  />
                </div>
                {entry.enabled && (
                  <div className="grid grid-cols-1 gap-2">
                    <ConfigurationSelect
                      label={t(($) => $.roles.agent)}
                      value={entry.agent_id ?? ""}
                      items={bindingItems(agentItems, entry.agent_id)}
                      disabled={!canEdit || save.isPending || agents.isPending}
                      onChange={(value) =>
                        change(role, { agent_id: value || null })
                      }
                    />
                    <ConfigurationSelect
                      label={t(($) => $.workbench.runtime)}
                      value={entry.runtime_id ?? ""}
                      items={bindingItems(runtimeItems, entry.runtime_id)}
                      disabled={
                        !canEdit || save.isPending || runtimes.isPending
                      }
                      onChange={(value) =>
                        change(role, { runtime_id: value || null })
                      }
                    />
                  </div>
                )}
              </div>
            );
          })}
          {draft && (
            <div className="flex gap-2">
              <Button
                disabled={save.isPending}
                aria-busy={save.isPending}
                onClick={() => void submit()}
              >
                {t(($) => $.workbench.save)}
              </Button>
              <Button
                variant="outline"
                disabled={save.isPending}
                onClick={() => {
                  setDraft(undefined);
                  setError(undefined);
                }}
              >
                {t(($) => $.workbench.cancel)}
              </Button>
            </div>
          )}
        </fieldset>
      )}
      {error && (
        <p role="alert" className="text-caption text-destructive">
          {error}
        </p>
      )}
    </section>
  );
}
