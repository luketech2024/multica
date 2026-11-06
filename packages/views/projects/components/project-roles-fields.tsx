"use client";

import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { agentListOptions } from "@multica/core/workspace/queries";
import { runtimeListOptions } from "@multica/core/runtimes";
import {
  projectRoleOrder,
  type ProjectRoles,
  type ProjectRoleKey,
} from "@multica/core/workbench/configuration";
import { Switch } from "@multica/ui/components/ui/switch";
import { ConfigurationSelect } from "./configuration-select";
import { useT } from "../../i18n";

export function ProjectRolesFields({
  config,
  onChange,
  disabled = false,
}: {
  config: ProjectRoles["config"];
  onChange: (config: ProjectRoles["config"]) => void;
  disabled?: boolean;
}) {
  const { t } = useT("projects");
  const wsId = useWorkspaceId();
  const agents = useQuery(agentListOptions(wsId));
  const runtimes = useQuery(runtimeListOptions(wsId));
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
    { value: "", label: t(($) => $.workbench.not_set) },
    ...items,
    ...(value && !items.some((item) => item.value === value)
      ? [{ value, label: `${t(($) => $.roles.unavailable)} (${value})` }]
      : []),
  ];
  const change = (
    role: ProjectRoleKey,
    patch: Partial<ProjectRoles["config"]["roles"][ProjectRoleKey]>,
  ) => {
    onChange({
      ...config,
      roles: { ...config.roles, [role]: { ...config.roles[role], ...patch } },
    });
  };

  return (
    <div className="space-y-2">
      {(agents.isError || runtimes.isError) && (
        <p role="alert" className="text-caption text-destructive">
          {t(($) => $.workbench.load_failed)}
        </p>
      )}
      {projectRoleOrder.map((role) => {
        const entry = config.roles[role];
        return (
          <div
            key={role}
            className="space-y-2 rounded-xl border bg-surface p-3"
          >
            <div className="flex items-center justify-between gap-2">
              <span className="text-label">{t(($) => $.roles[role])}</span>
              <Switch
                checked={entry.enabled}
                disabled={disabled || role === "sched" || role === "product"}
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
                  disabled={disabled || agents.isPending || agents.isError}
                  onChange={(value) =>
                    change(role, { agent_id: value || null })
                  }
                />
                <ConfigurationSelect
                  label={t(($) => $.workbench.runtime)}
                  value={entry.runtime_id ?? ""}
                  items={bindingItems(runtimeItems, entry.runtime_id)}
                  disabled={disabled || runtimes.isPending || runtimes.isError}
                  onChange={(value) =>
                    change(role, { runtime_id: value || null })
                  }
                />
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
